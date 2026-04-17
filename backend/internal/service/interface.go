package service

import (
	"context"
	"time"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
	"go-job/internal/repository"
)

type ScheduleService interface {
	Start(ctx context.Context) error
	Stop() error
	LoadRunningJobs(ctx context.Context) error
	ScheduleJob(ctx context.Context, job model.JobInfo) error
	RemoveJob(jobID int)
}

type TriggerCallback func(ctx context.Context, job model.JobInfo)

type JobService interface {
	Create(ctx context.Context, job *model.JobInfo) error
	GetByID(ctx context.Context, id int) (*model.JobInfo, error)
	List(ctx context.Context, page repository.PageQuery, query repository.JobQuery) ([]model.JobInfo, int64, error)
	Update(ctx context.Context, job *model.JobInfo) error
	UpdateStatus(ctx context.Context, id int, status enum.JobStatus) error
	Delete(ctx context.Context, id int) error
}

type ExecutorService interface {
	Create(ctx context.Context, executor *model.JobExecutor) error
	GetByID(ctx context.Context, id int) (*model.JobExecutor, error)
	List(ctx context.Context, page repository.PageQuery, query repository.ExecutorQuery) ([]model.JobExecutor, int64, error)
	Update(ctx context.Context, executor *model.JobExecutor) error
	UpdateStatus(ctx context.Context, id int, status enum.ExecutorStatus) error
	Delete(ctx context.Context, id int) error
}

type LogService interface {
	Create(ctx context.Context, log *model.JobLog) error
	Update(ctx context.Context, log *model.JobLog) error
	GetByID(ctx context.Context, id int64) (*model.JobLog, error)
	List(ctx context.Context, page repository.PageQuery, query repository.LogQuery) ([]model.JobLog, int64, error)
	MarkRunning(ctx context.Context, id int64, startTime time.Time) error
	MarkSuccess(ctx context.Context, id int64, costMS int, endTime time.Time) error
	MarkFailed(ctx context.Context, id int64, costMS int, errMsg string, endTime time.Time) error
	DeleteByJobID(ctx context.Context, jobID int) error
}

type TriggerService interface {
	TriggerJob(
		ctx context.Context,
		job model.JobInfo,
		executorID int,
		executorAddress string,
		shardIndex int8,
		run func(context.Context) error,
	) (int64, error)
}
