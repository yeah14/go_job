package service

import (
	"context"
	"errors"
	"time"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
)

var ErrInvalidTriggerInput = errors.New("invalid trigger input")

type triggerService struct {
	logService LogService
}

func NewTriggerService(logService LogService) TriggerService {
	return &triggerService{logService: logService}
}

func (s *triggerService) TriggerJob(
	ctx context.Context,
	job model.JobInfo,
	executorID int,
	executorAddress string,
	shardIndex int8,
	run func(context.Context) error,
) (int64, error) {
	if job.ID <= 0 || executorID <= 0 || executorAddress == "" || run == nil {
		return 0, ErrInvalidTriggerInput
	}

	now := time.Now()
	logRow := &model.JobLog{
		JobID:           job.ID,
		ExecutorID:      executorID,
		ExecutorAddress: executorAddress,
		ShardIndex:      shardIndex,
		ExecutorParam:   job.ExecutorParam,
		TriggerTime:     now,
		Status:          int8(enum.LogStatusPending),
	}
	if err := s.logService.Create(ctx, logRow); err != nil {
		return 0, err
	}

	if err := s.logService.MarkRunning(ctx, logRow.ID, now); err != nil {
		return logRow.ID, err
	}

	start := time.Now()
	err := run(ctx)
	cost := int(time.Since(start).Milliseconds())
	end := time.Now()
	if err != nil {
		_ = s.logService.MarkFailed(ctx, logRow.ID, cost, err.Error(), end)
		return logRow.ID, err
	}
	if err := s.logService.MarkSuccess(ctx, logRow.ID, cost, end); err != nil {
		return logRow.ID, err
	}
	return logRow.ID, nil
}
