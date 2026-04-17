package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
	"go-job/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrInvalidID            = errors.New("invalid id")
	ErrInvalidPageQuery     = errors.New("invalid page query")
	ErrInvalidJob           = errors.New("invalid job payload")
	ErrInvalidJobStatus     = errors.New("invalid job status")
	ErrInvalidExecutorID    = errors.New("invalid executor id")
	ErrInvalidJobName       = errors.New("invalid job name")
	ErrInvalidCron          = errors.New("invalid cron")
	ErrInvalidJobHandler    = errors.New("invalid executor handler")
	ErrJobNotFound          = errors.New("job not found")
	ErrExecutorNotFound     = errors.New("executor not found")
	ErrLogNotFound          = errors.New("log not found")
	ErrInvalidExecutor      = errors.New("invalid executor payload")
	ErrInvalidExecutorState = errors.New("invalid executor status")
	ErrInvalidLog           = errors.New("invalid log payload")
)

type jobService struct {
	jobRepo    repository.JobRepository
	scheduler  ScheduleService
}

func NewJobService(jobRepo repository.JobRepository, scheduler ...ScheduleService) JobService {
	s := &jobService{jobRepo: jobRepo}
	if len(scheduler) > 0 {
		s.scheduler = scheduler[0]
	}
	return s
}

func (s *jobService) Create(ctx context.Context, job *model.JobInfo) error {
	if err := validateJobPayload(job); err != nil {
		return err
	}
	if err := wrapRepoErr(s.jobRepo.Create(ctx, job)); err != nil {
		return err
	}
	if s.scheduler != nil && enum.JobStatus(job.Status) == enum.JobStatusRunning {
		if err := s.scheduler.ScheduleJob(ctx, *job); err != nil {
			return fmt.Errorf("schedule created running job failed: %w", err)
		}
	}
	return nil
}

func (s *jobService) GetByID(ctx context.Context, id int) (*model.JobInfo, error) {
	if id <= 0 {
		return nil, ErrInvalidID
	}
	job, err := s.jobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, wrapRepoErr(err)
	}
	return job, nil
}

func (s *jobService) List(ctx context.Context, page repository.PageQuery, query repository.JobQuery) ([]model.JobInfo, int64, error) {
	if page.Page < 0 || page.PageSize < 0 {
		return nil, 0, ErrInvalidPageQuery
	}
	if query.Status != nil && !enum.JobStatus(*query.Status).IsValid() {
		return nil, 0, ErrInvalidJobStatus
	}
	rows, total, err := s.jobRepo.List(ctx, page, query)
	if err != nil {
		return nil, 0, wrapRepoErr(err)
	}
	return rows, total, nil
}

func (s *jobService) Update(ctx context.Context, job *model.JobInfo) error {
	if job == nil || job.ID <= 0 {
		return ErrInvalidJob
	}
	if err := validateJobPayload(job); err != nil {
		return err
	}
	return wrapRepoErr(s.jobRepo.Update(ctx, job))
}

func (s *jobService) UpdateStatus(ctx context.Context, id int, status enum.JobStatus) error {
	if id <= 0 {
		return ErrInvalidID
	}
	if !status.IsValid() {
		return ErrInvalidJobStatus
	}
	if err := wrapRepoErr(s.jobRepo.UpdateStatus(ctx, id, int8(status))); err != nil {
		return err
	}
	if s.scheduler == nil {
		return nil
	}
	if status == enum.JobStatusPaused {
		s.scheduler.RemoveJob(id)
		return nil
	}
	job, err := s.jobRepo.GetByID(ctx, id)
	if err != nil {
		return wrapRepoErr(err)
	}
	if err := s.scheduler.ScheduleJob(ctx, *job); err != nil {
		return fmt.Errorf("schedule running job failed: %w", err)
	}
	return nil
}

func (s *jobService) Delete(ctx context.Context, id int) error {
	if id <= 0 {
		return ErrInvalidID
	}
	if err := wrapRepoErr(s.jobRepo.Delete(ctx, id)); err != nil {
		return err
	}
	if s.scheduler != nil {
		s.scheduler.RemoveJob(id)
	}
	return nil
}

func validateJobPayload(job *model.JobInfo) error {
	if job == nil {
		return ErrInvalidJob
	}
	if strings.TrimSpace(job.JobName) == "" {
		return ErrInvalidJobName
	}
	if job.ExecutorID <= 0 {
		return ErrInvalidExecutorID
	}
	if strings.TrimSpace(job.Cron) == "" {
		return ErrInvalidCron
	}
	if strings.TrimSpace(job.ExecutorHandler) == "" {
		return ErrInvalidJobHandler
	}
	if !enum.JobStatus(job.Status).IsValid() {
		return ErrInvalidJobStatus
	}
	return nil
}

func wrapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrJobNotFound
	}
	return fmt.Errorf("repository error: %w", err)
}
