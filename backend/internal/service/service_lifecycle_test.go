package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
	"go-job/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockScheduleService struct {
	scheduled []int
	removed   []int
}

func (m *mockScheduleService) Start(ctx context.Context) error { return nil }
func (m *mockScheduleService) Stop() error                     { return nil }
func (m *mockScheduleService) LoadRunningJobs(ctx context.Context) error {
	return nil
}
func (m *mockScheduleService) ScheduleJob(ctx context.Context, job model.JobInfo) error {
	m.scheduled = append(m.scheduled, job.ID)
	return nil
}
func (m *mockScheduleService) RemoveJob(jobID int) {
	m.removed = append(m.removed, jobID)
}

func newServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:service_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	schema := []string{
		`CREATE TABLE job_executor (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_name TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			address_type INTEGER NOT NULL DEFAULT 0,
			address_list TEXT NULL,
			status INTEGER NOT NULL DEFAULT 1,
			creator TEXT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE job_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_name TEXT NOT NULL,
			executor_id INTEGER NOT NULL,
			executor_handler TEXT NOT NULL,
			executor_param TEXT NULL,
			cron TEXT NOT NULL,
			shard_total INTEGER NOT NULL DEFAULT 1,
			shard_param TEXT NULL,
			timeout INTEGER NOT NULL DEFAULT 30,
			retry_count INTEGER NOT NULL DEFAULT 0,
			priority INTEGER NOT NULL DEFAULT 1,
			status INTEGER NOT NULL DEFAULT 0,
			creator TEXT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE job_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER NOT NULL,
			executor_id INTEGER NOT NULL,
			executor_address TEXT NOT NULL,
			shard_index INTEGER NOT NULL DEFAULT 0,
			executor_param TEXT NULL,
			trigger_time DATETIME NOT NULL,
			start_time DATETIME NULL,
			end_time DATETIME NULL,
			cost_time INTEGER NULL,
			status INTEGER NOT NULL,
			error_msg TEXT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}
	for _, sql := range schema {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("create schema failed: %v", err)
		}
	}
	return db
}

func TestJobService_CRUDAndStartPauseSchedule(t *testing.T) {
	db := newServiceTestDB(t)
	ctx := context.Background()

	exeRepo := repository.NewExecutorRepository(db)
	jobRepo := repository.NewJobRepository(db)
	mockSchedule := &mockScheduleService{}
	jobSvc := NewJobService(jobRepo, mockSchedule)

	exe := &model.JobExecutor{
		AppName:     "svc-test",
		Name:        "service test executor",
		AddressType: 0,
		Status:      1,
	}
	if err := exeRepo.Create(ctx, exe); err != nil {
		t.Fatalf("create executor failed: %v", err)
	}

	job := &model.JobInfo{
		JobName:         "job-crud",
		ExecutorID:      exe.ID,
		ExecutorHandler: "demoHandler",
		Cron:            "*/1 * * * * *",
		ShardTotal:      1,
		Timeout:         30,
		RetryCount:      0,
		Priority:        1,
		Status:          int8(enum.JobStatusRunning),
	}
	if err := jobSvc.Create(ctx, job); err != nil {
		t.Fatalf("create job failed: %v", err)
	}
	if len(mockSchedule.scheduled) == 0 {
		t.Fatal("running job should be scheduled on create")
	}

	loaded, err := jobSvc.GetByID(ctx, job.ID)
	if err != nil || loaded.ID != job.ID {
		t.Fatalf("get by id failed: %v", err)
	}

	if err := jobSvc.UpdateStatus(ctx, job.ID, enum.JobStatusPaused); err != nil {
		t.Fatalf("pause job failed: %v", err)
	}
	if len(mockSchedule.removed) == 0 {
		t.Fatal("paused job should be removed from scheduler")
	}

	if err := jobSvc.UpdateStatus(ctx, job.ID, enum.JobStatusRunning); err != nil {
		t.Fatalf("resume job failed: %v", err)
	}
	if len(mockSchedule.scheduled) < 2 {
		t.Fatal("running job should be scheduled on resume")
	}

	if err := jobSvc.Delete(ctx, job.ID); err != nil {
		t.Fatalf("delete job failed: %v", err)
	}
	if len(mockSchedule.removed) < 2 {
		t.Fatal("deleted job should be removed from scheduler")
	}
}

func TestTriggerService_LogLifecycle(t *testing.T) {
	db := newServiceTestDB(t)
	ctx := context.Background()

	exeRepo := repository.NewExecutorRepository(db)
	jobRepo := repository.NewJobRepository(db)
	logRepo := repository.NewLogRepository(db)
	logSvc := NewLogService(logRepo)
	triggerSvc := NewTriggerService(logSvc)

	exe := &model.JobExecutor{
		AppName:     "trigger-test",
		Name:        "trigger executor",
		AddressType: 0,
		Status:      1,
	}
	if err := exeRepo.Create(ctx, exe); err != nil {
		t.Fatalf("create executor failed: %v", err)
	}
	job := &model.JobInfo{
		JobName:         "trigger-job",
		ExecutorID:      exe.ID,
		ExecutorHandler: "demo",
		Cron:            "*/1 * * * * *",
		ShardTotal:      1,
		Timeout:         30,
		Priority:        1,
		Status:          1,
	}
	if err := jobRepo.Create(ctx, job); err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	id1, err := triggerSvc.TriggerJob(ctx, *job, exe.ID, "127.0.0.1:9090", 0, func(context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("trigger success path failed: %v", err)
	}
	row1, err := logSvc.GetByID(ctx, id1)
	if err != nil {
		t.Fatalf("query success log failed: %v", err)
	}
	if row1.Status != int8(enum.LogStatusSuccess) {
		t.Fatalf("expected success status, got %d", row1.Status)
	}

	_, err = triggerSvc.TriggerJob(ctx, *job, exe.ID, "127.0.0.1:9090", 1, func(context.Context) error {
		return ErrInvalidTriggerInput
	})
	if err == nil {
		t.Fatal("expected trigger error in failed path")
	}

	status := int8(enum.LogStatusFailed)
	rows, total, err := logSvc.List(ctx, repository.PageQuery{Page: 1, PageSize: 20}, repository.LogQuery{
		JobID:  &job.ID,
		Status: &status,
	})
	if err != nil {
		t.Fatalf("query failed logs failed: %v", err)
	}
	if total == 0 || len(rows) == 0 {
		t.Fatal("expected at least one failed log record")
	}
}
