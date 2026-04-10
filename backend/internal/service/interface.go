package service

import (
	"context"

	"go-job/internal/domain/model"
)

type ScheduleService interface {
	Start(ctx context.Context) error
	Stop() error
	LoadRunningJobs(ctx context.Context) error
	ScheduleJob(ctx context.Context, job model.JobInfo) error
	RemoveJob(jobID int)
}

type TriggerCallback func(ctx context.Context, job model.JobInfo)
