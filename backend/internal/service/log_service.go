package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
	"go-job/internal/repository"

	"gorm.io/gorm"
)

type logService struct {
	logRepo repository.LogRepository
}

func NewLogService(logRepo repository.LogRepository) LogService {
	return &logService{logRepo: logRepo}
}

func (s *logService) Create(ctx context.Context, log *model.JobLog) error {
	if err := validateLogPayload(log); err != nil {
		return err
	}
	return wrapLogRepoErr(s.logRepo.Create(ctx, log))
}

func (s *logService) Update(ctx context.Context, log *model.JobLog) error {
	if log == nil || log.ID <= 0 {
		return ErrInvalidLog
	}
	return wrapLogRepoErr(s.logRepo.Update(ctx, log))
}

func (s *logService) GetByID(ctx context.Context, id int64) (*model.JobLog, error) {
	if id <= 0 {
		return nil, ErrInvalidID
	}
	row, err := s.logRepo.GetByID(ctx, id)
	if err != nil {
		return nil, wrapLogRepoErr(err)
	}
	return row, nil
}

func (s *logService) List(ctx context.Context, page repository.PageQuery, query repository.LogQuery) ([]model.JobLog, int64, error) {
	if page.Page < 0 || page.PageSize < 0 {
		return nil, 0, ErrInvalidPageQuery
	}
	if query.Status != nil && !enum.LogStatus(*query.Status).IsValid() {
		return nil, 0, ErrInvalidLog
	}
	rows, total, err := s.logRepo.List(ctx, page, query)
	if err != nil {
		return nil, 0, wrapLogRepoErr(err)
	}
	return rows, total, nil
}

func (s *logService) MarkRunning(ctx context.Context, id int64, startTime time.Time) error {
	if id <= 0 {
		return ErrInvalidID
	}
	log, err := s.logRepo.GetByID(ctx, id)
	if err != nil {
		return wrapLogRepoErr(err)
	}
	if startTime.IsZero() {
		startTime = time.Now()
	}
	log.Status = int8(enum.LogStatusRunning)
	log.StartTime = &startTime
	return wrapLogRepoErr(s.logRepo.Update(ctx, log))
}

func (s *logService) MarkSuccess(ctx context.Context, id int64, costMS int, endTime time.Time) error {
	if id <= 0 || costMS < 0 {
		return ErrInvalidLog
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}
	return wrapLogRepoErr(s.logRepo.UpdateStatus(
		ctx,
		id,
		int8(enum.LogStatusSuccess),
		&costMS,
		nil,
		&endTime,
	))
}

func (s *logService) MarkFailed(ctx context.Context, id int64, costMS int, errMsg string, endTime time.Time) error {
	if id <= 0 || costMS < 0 || errMsg == "" {
		return ErrInvalidLog
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}
	msg := errMsg
	return wrapLogRepoErr(s.logRepo.UpdateStatus(
		ctx,
		id,
		int8(enum.LogStatusFailed),
		&costMS,
		&msg,
		&endTime,
	))
}

func (s *logService) DeleteByJobID(ctx context.Context, jobID int) error {
	if jobID <= 0 {
		return ErrInvalidID
	}
	return wrapLogRepoErr(s.logRepo.DeleteByJobID(ctx, jobID))
}

func validateLogPayload(log *model.JobLog) error {
	if log == nil {
		return ErrInvalidLog
	}
	if log.JobID <= 0 || log.ExecutorID <= 0 || log.ExecutorAddress == "" {
		return ErrInvalidLog
	}
	if !enum.LogStatus(log.Status).IsValid() {
		return ErrInvalidLog
	}
	if log.TriggerTime.IsZero() {
		log.TriggerTime = time.Now()
	}
	return nil
}

func wrapLogRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrLogNotFound
	}
	return fmt.Errorf("repository error: %w", err)
}
