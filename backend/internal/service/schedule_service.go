package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-job/internal/domain/model"
	"go-job/internal/repository"
	cronpkg "go-job/pkg/cron"
	lockpkg "go-job/pkg/lock"
	"go-job/pkg/logger"
	"go-job/pkg/timer"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type scheduleService struct {
	jobRepo   repository.JobRepository
	timeWheel *timer.TimeWheel
	onTrigger TriggerCallback
	minDelay  time.Duration
	lockTTL   time.Duration

	lockClient *redis.Client
	lockPrefix string

	mu      sync.Mutex
	started bool
}

type loadSummary struct {
	total      int
	successful int
	failed     int
}

func NewScheduleService(jobRepo repository.JobRepository, timeWheel *timer.TimeWheel, onTrigger TriggerCallback) ScheduleService {
	return &scheduleService{
		jobRepo:    jobRepo,
		timeWheel:  timeWheel,
		onTrigger:  onTrigger,
		minDelay:   time.Second,
		lockTTL:    15 * time.Second,
		lockPrefix: "go_job:trigger_lock",
	}
}

// NewScheduleServiceWithLock enables distributed trigger de-duplication by Redis lock.
func NewScheduleServiceWithLock(
	jobRepo repository.JobRepository,
	timeWheel *timer.TimeWheel,
	onTrigger TriggerCallback,
	lockClient *redis.Client,
) ScheduleService {
	svc := NewScheduleService(jobRepo, timeWheel, onTrigger).(*scheduleService)
	svc.lockClient = lockClient
	return svc
}

// NewTriggerCallbackFromService builds a schedule callback that routes trigger events
// into TriggerService, so scheduling and log lifecycle are connected by default.
//
// executorAddress is a fallback address used in log records before executor routing is implemented.
// run can be nil; in that case TriggerService marks the trigger as success immediately.
func NewTriggerCallbackFromService(
	triggerSvc TriggerService,
	executorAddress string,
	shardIndex int8,
	run func(context.Context, model.JobInfo) error,
) TriggerCallback {
	return func(ctx context.Context, job model.JobInfo) {
		if triggerSvc == nil {
			logger.L().Warn("trigger service is nil", zap.Int("job_id", job.ID))
			return
		}
		addr := executorAddress
		if addr == "" {
			addr = "scheduler-local"
		}
		runFn := func(runCtx context.Context) error {
			if run == nil {
				return nil
			}
			return run(runCtx, job)
		}
		if _, err := triggerSvc.TriggerJob(ctx, job, job.ExecutorID, addr, shardIndex, runFn); err != nil {
			logger.L().Error("trigger service execution failed",
				zap.Int("job_id", job.ID),
				zap.String("executor_address", addr),
				zap.Error(err),
			)
		}
	}
}

func (s *scheduleService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	if err := s.timeWheel.Start(); err != nil {
		return err
	}
	if err := s.LoadRunningJobs(ctx); err != nil {
		logger.L().Error("load running jobs failed", zap.Error(err))
		return err
	}
	logger.L().Info("schedule service started")
	return nil
}

func (s *scheduleService) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	s.mu.Unlock()
	if err := s.timeWheel.Stop(); err != nil {
		logger.L().Error("stop time wheel failed", zap.Error(err))
		return err
	}
	logger.L().Info("schedule service stopped")
	return nil
}

func (s *scheduleService) LoadRunningJobs(ctx context.Context) error {
	jobs, err := s.jobRepo.ListRunning(ctx)
	if err != nil {
		return err
	}

	summary, firstErr := s.loadJobsToWheel(ctx, jobs)
	logger.L().Info("running jobs loaded to time wheel",
		zap.Int("total", summary.total),
		zap.Int("successful", summary.successful),
		zap.Int("failed", summary.failed),
	)
	return firstErr
}

func (s *scheduleService) loadJobsToWheel(ctx context.Context, jobs []model.JobInfo) (loadSummary, error) {
	summary := loadSummary{total: len(jobs)}
	var firstErr error
	for _, job := range jobs {
		if ctx.Err() != nil {
			if firstErr == nil {
				firstErr = ctx.Err()
			}
			summary.failed++
			continue
		}
		if err := s.ScheduleJob(ctx, job); err != nil {
			logger.L().Error("schedule running job failed",
				zap.Int("job_id", job.ID),
				zap.String("cron", job.Cron),
				zap.Error(err),
			)
			summary.failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("schedule job %d failed: %w", job.ID, err)
			}
			continue
		}
		summary.successful++
	}
	return summary, firstErr
}

func (s *scheduleService) ScheduleJob(ctx context.Context, job model.JobInfo) error {
	delay, _, err := cronpkg.NextDelay(job.Cron, time.Now(), s.minDelay)
	if err != nil {
		return err
	}

	jobID := s.wheelJobID(job.ID)
	return s.timeWheel.AddJob(delay, jobID, func() {
		s.triggerWithDistributedLock(ctx, job)
		// Re-schedule by cron rule after execution.
		if err := s.ScheduleJob(ctx, job); err != nil {
			logger.L().Error("re-schedule job failed",
				zap.Int("job_id", job.ID),
				zap.String("cron", job.Cron),
				zap.Error(err),
			)
		}
	})
}

func (s *scheduleService) RemoveJob(jobID int) {
	s.timeWheel.RemoveJob(s.wheelJobID(jobID))
}

func (s *scheduleService) wheelJobID(id int) string {
	return fmt.Sprintf("job:%d", id)
}

func (s *scheduleService) triggerWithDistributedLock(ctx context.Context, job model.JobInfo) {
	if s.lockClient == nil {
		s.invokeTrigger(ctx, job)
		return
	}

	lockKey := fmt.Sprintf("%s:%d", s.lockPrefix, job.ID)
	l, err := lockpkg.NewRedisLock(s.lockClient, lockKey, s.lockTTL)
	if err != nil {
		logger.L().Error("create distributed lock failed",
			zap.Int("job_id", job.ID),
			zap.Error(err),
		)
		return
	}

	ok, err := l.Lock(ctx)
	if err != nil {
		logger.L().Error("acquire distributed lock failed",
			zap.Int("job_id", job.ID),
			zap.String("lock_key", lockKey),
			zap.Error(err),
		)
		return
	}
	if !ok {
		logger.L().Info("skip duplicate trigger due to lock not acquired",
			zap.Int("job_id", job.ID),
			zap.String("lock_key", lockKey),
		)
		return
	}
	defer l.UnlockBestEffort(ctx)

	s.invokeTrigger(ctx, job)
}

func (s *scheduleService) invokeTrigger(ctx context.Context, job model.JobInfo) {
	if s.onTrigger != nil {
		s.onTrigger(ctx, job)
		return
	}
	logger.L().Warn("trigger callback is nil", zap.Int("job_id", job.ID))
}
