package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-job/internal/domain/model"
	"go-job/internal/repository"
	cronpkg "go-job/pkg/cron"
	"go-job/pkg/logger"
	"go-job/pkg/timer"

	"go.uber.org/zap"
)

type scheduleService struct {
	jobRepo   repository.JobRepository
	timeWheel *timer.TimeWheel
	onTrigger TriggerCallback
	minDelay  time.Duration

	mu      sync.Mutex
	started bool
}

func NewScheduleService(jobRepo repository.JobRepository, timeWheel *timer.TimeWheel, onTrigger TriggerCallback) ScheduleService {
	return &scheduleService{
		jobRepo:   jobRepo,
		timeWheel: timeWheel,
		onTrigger: onTrigger,
		minDelay:  time.Second,
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

	var firstErr error
	for _, job := range jobs {
		if err := s.ScheduleJob(ctx, job); err != nil {
			logger.L().Error("schedule running job failed",
				zap.Int("job_id", job.ID),
				zap.String("cron", job.Cron),
				zap.Error(err),
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("schedule job %d failed: %w", job.ID, err)
			}
		}
	}
	return firstErr
}

func (s *scheduleService) ScheduleJob(ctx context.Context, job model.JobInfo) error {
	delay, _, err := cronpkg.NextDelay(job.Cron, time.Now(), s.minDelay)
	if err != nil {
		return err
	}

	jobID := s.wheelJobID(job.ID)
	return s.timeWheel.AddJob(delay, jobID, func() {
		if s.onTrigger != nil {
			s.onTrigger(ctx, job)
		} else {
			logger.L().Warn("trigger callback is nil", zap.Int("job_id", job.ID))
		}
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
