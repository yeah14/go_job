package repository

import (
	"context"
	"time"

	"go-job/internal/domain/model"
)

type PageQuery struct {
	Page     int
	PageSize int
}

func (q PageQuery) Normalize() (offset, limit int) {
	page := q.Page
	size := q.PageSize
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	if size > 200 {
		size = 200
	}
	return (page - 1) * size, size
}

type JobQuery struct {
	ExecutorID *int
	Status     *int8
	Keyword    string
}

type ExecutorQuery struct {
	Status  *int8
	Keyword string
}

type LogQuery struct {
	JobID      *int
	ExecutorID *int
	Status     *int8
	StartTime  *time.Time
	EndTime    *time.Time
}

type HeartbeatQuery struct {
	AppName      string
	Address      string
	BeforeTime   *time.Time
	AfterTime    *time.Time
	OnlyExpired  bool
	ExpireBefore *time.Time
}

type JobRepository interface {
	Create(ctx context.Context, job *model.JobInfo) error
	GetByID(ctx context.Context, id int) (*model.JobInfo, error)
	List(ctx context.Context, page PageQuery, query JobQuery) ([]model.JobInfo, int64, error)
	Update(ctx context.Context, job *model.JobInfo) error
	UpdateStatus(ctx context.Context, id int, status int8) error
	Delete(ctx context.Context, id int) error
	BatchCreate(ctx context.Context, jobs []model.JobInfo) error
	ListRunning(ctx context.Context) ([]model.JobInfo, error)
}

type ExecutorRepository interface {
	Create(ctx context.Context, executor *model.JobExecutor) error
	GetByID(ctx context.Context, id int) (*model.JobExecutor, error)
	GetByAppName(ctx context.Context, appName string) (*model.JobExecutor, error)
	List(ctx context.Context, page PageQuery, query ExecutorQuery) ([]model.JobExecutor, int64, error)
	Update(ctx context.Context, executor *model.JobExecutor) error
	UpdateStatus(ctx context.Context, id int, status int8) error
	Delete(ctx context.Context, id int) error
	BatchCreate(ctx context.Context, executors []model.JobExecutor) error
}

type LogRepository interface {
	Create(ctx context.Context, log *model.JobLog) error
	GetByID(ctx context.Context, id int64) (*model.JobLog, error)
	List(ctx context.Context, page PageQuery, query LogQuery) ([]model.JobLog, int64, error)
	Update(ctx context.Context, log *model.JobLog) error
	UpdateStatus(ctx context.Context, id int64, status int8, costTime *int, errorMsg *string, endTime *time.Time) error
	DeleteByJobID(ctx context.Context, jobID int) error
	BatchCreate(ctx context.Context, logs []model.JobLog) error
}

type HeartbeatRepository interface {
	Create(ctx context.Context, heartbeat *model.JobExecutorHeartbeat) error
	Upsert(ctx context.Context, heartbeat *model.JobExecutorHeartbeat) error
	GetByAppAndAddress(ctx context.Context, appName, address string) (*model.JobExecutorHeartbeat, error)
	List(ctx context.Context, page PageQuery, query HeartbeatQuery) ([]model.JobExecutorHeartbeat, int64, error)
	ListExpired(ctx context.Context, deadline time.Time) ([]model.JobExecutorHeartbeat, error)
	DeleteByAppAndAddress(ctx context.Context, appName, address string) error
	BatchUpsert(ctx context.Context, heartbeats []model.JobExecutorHeartbeat) error
}
