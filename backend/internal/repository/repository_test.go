package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-job/internal/domain/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:repo_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
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
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(executor_id) REFERENCES job_executor(id) ON DELETE CASCADE
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
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(job_id) REFERENCES job_info(id) ON DELETE CASCADE,
			FOREIGN KEY(executor_id) REFERENCES job_executor(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE job_executor_heartbeat (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			executor_app_name TEXT NOT NULL,
			executor_address TEXT NOT NULL,
			heartbeat_time DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(executor_app_name, executor_address)
		);`,
	}
	for _, sql := range schema {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("create schema failed: %v", err)
		}
	}

	return db
}

func createExecutor(t *testing.T, repo ExecutorRepository, appName string) model.JobExecutor {
	t.Helper()
	e := model.JobExecutor{
		AppName:     appName,
		Name:        "executor-" + appName,
		AddressType: 0,
		Status:      1,
	}
	if err := repo.Create(context.Background(), &e); err != nil {
		t.Fatalf("create executor failed: %v", err)
	}
	return e
}

func createJob(t *testing.T, repo JobRepository, executorID int, name string, status int8) model.JobInfo {
	t.Helper()
	j := model.JobInfo{
		JobName:         name,
		ExecutorID:      executorID,
		ExecutorHandler: "handler",
		Cron:            "*/1 * * * *",
		ShardTotal:      1,
		Timeout:         30,
		RetryCount:      0,
		Priority:        1,
		Status:          status,
	}
	if err := repo.Create(context.Background(), &j); err != nil {
		t.Fatalf("create job failed: %v", err)
	}
	return j
}

func TestJobRepository_CRUDAndQuery(t *testing.T) {
	db := newTestDB(t)
	executorRepo := NewExecutorRepository(db)
	jobRepo := NewJobRepository(db)

	exe := createExecutor(t, executorRepo, "app-a")
	_ = createJob(t, jobRepo, exe.ID, "sync-user", 1)
	_ = createJob(t, jobRepo, exe.ID, "sync-order", 0)

	ctx := context.Background()
	got, err := jobRepo.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.JobName == "" {
		t.Fatalf("GetByID returned empty job")
	}

	status := int8(1)
	rows, total, err := jobRepo.List(ctx, PageQuery{Page: 1, PageSize: 10}, JobQuery{
		ExecutorID: &exe.ID,
		Status:     &status,
		Keyword:    "sync",
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("unexpected list result, total=%d len=%d", total, len(rows))
	}

	if err := jobRepo.UpdateStatus(ctx, got.ID, 0); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	updated, err := jobRepo.GetByID(ctx, got.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if updated.Status != 0 {
		t.Fatalf("expected status=0, got=%d", updated.Status)
	}

	batch := []model.JobInfo{
		{
			JobName:         "batch-1",
			ExecutorID:      exe.ID,
			ExecutorHandler: "handler",
			Cron:            "*/2 * * * *",
			ShardTotal:      1,
			Timeout:         30,
			RetryCount:      1,
			Priority:        2,
			Status:          1,
		},
		{
			JobName:         "batch-2",
			ExecutorID:      exe.ID,
			ExecutorHandler: "handler",
			Cron:            "*/3 * * * *",
			ShardTotal:      1,
			Timeout:         30,
			RetryCount:      1,
			Priority:        2,
			Status:          1,
		},
	}
	if err := jobRepo.BatchCreate(ctx, batch); err != nil {
		t.Fatalf("BatchCreate failed: %v", err)
	}

	running, err := jobRepo.ListRunning(ctx)
	if err != nil {
		t.Fatalf("ListRunning failed: %v", err)
	}
	if len(running) < 2 {
		t.Fatalf("expected at least 2 running jobs, got=%d", len(running))
	}
}

func TestExecutorRepository_CRUDAndList(t *testing.T) {
	db := newTestDB(t)
	repo := NewExecutorRepository(db)
	ctx := context.Background()

	if err := repo.BatchCreate(ctx, []model.JobExecutor{
		{AppName: "data-sync", Name: "Data Sync", Status: 1, AddressType: 0},
		{AppName: "report", Name: "Report", Status: 0, AddressType: 1},
	}); err != nil {
		t.Fatalf("BatchCreate failed: %v", err)
	}

	got, err := repo.GetByAppName(ctx, "data-sync")
	if err != nil {
		t.Fatalf("GetByAppName failed: %v", err)
	}
	if got.AppName != "data-sync" {
		t.Fatalf("unexpected app name: %s", got.AppName)
	}

	status := int8(1)
	rows, total, err := repo.List(ctx, PageQuery{Page: 1, PageSize: 10}, ExecutorQuery{
		Status:  &status,
		Keyword: "Data",
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("unexpected executor list result, total=%d len=%d", total, len(rows))
	}

	if err := repo.UpdateStatus(ctx, got.ID, 0); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	updated, err := repo.GetByID(ctx, got.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if updated.Status != 0 {
		t.Fatalf("expected status=0, got=%d", updated.Status)
	}
}

func TestLogRepository_QueryAndUpdateStatus(t *testing.T) {
	db := newTestDB(t)
	executorRepo := NewExecutorRepository(db)
	jobRepo := NewJobRepository(db)
	logRepo := NewLogRepository(db)
	ctx := context.Background()

	exe := createExecutor(t, executorRepo, "app-log")
	job := createJob(t, jobRepo, exe.ID, "log-job", 1)

	now := time.Now()
	err := logRepo.BatchCreate(ctx, []model.JobLog{
		{
			JobID:           job.ID,
			ExecutorID:      exe.ID,
			ExecutorAddress: "127.0.0.1:9090",
			ShardIndex:      0,
			TriggerTime:     now.Add(-2 * time.Minute),
			Status:          1,
		},
		{
			JobID:           job.ID,
			ExecutorID:      exe.ID,
			ExecutorAddress: "127.0.0.1:9090",
			ShardIndex:      1,
			TriggerTime:     now,
			Status:          2,
		},
	})
	if err != nil {
		t.Fatalf("BatchCreate failed: %v", err)
	}

	status := int8(2)
	start := now.Add(-10 * time.Second)
	end := now.Add(10 * time.Second)
	rows, total, err := logRepo.List(ctx, PageQuery{Page: 1, PageSize: 10}, LogQuery{
		JobID:     &job.ID,
		Status:    &status,
		StartTime: &start,
		EndTime:   &end,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("unexpected log list result, total=%d len=%d", total, len(rows))
	}

	cost := 1234
	msg := "timeout"
	finish := time.Now()
	if err := logRepo.UpdateStatus(ctx, rows[0].ID, 3, &cost, &msg, &finish); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	updated, err := logRepo.GetByID(ctx, rows[0].ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if updated.Status != 3 || updated.CostTime == nil || *updated.CostTime != 1234 {
		t.Fatalf("log status update not applied")
	}
}

func TestHeartbeatRepository_UpsertListExpiredAndDelete(t *testing.T) {
	db := newTestDB(t)
	repo := NewHeartbeatRepository(db)
	ctx := context.Background()

	oldTime := time.Now().Add(-1 * time.Hour)
	newTime := time.Now()

	if err := repo.Upsert(ctx, &model.JobExecutorHeartbeat{
		ExecutorAppName: "app",
		ExecutorAddress: "127.0.0.1:9090",
		HeartbeatTime:   oldTime,
	}); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	if err := repo.Upsert(ctx, &model.JobExecutorHeartbeat{
		ExecutorAppName: "app",
		ExecutorAddress: "127.0.0.1:9090",
		HeartbeatTime:   newTime,
	}); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	hb, err := repo.GetByAppAndAddress(ctx, "app", "127.0.0.1:9090")
	if err != nil {
		t.Fatalf("GetByAppAndAddress failed: %v", err)
	}
	if hb.HeartbeatTime.Before(oldTime) {
		t.Fatalf("heartbeat was not updated")
	}

	if err := repo.BatchUpsert(ctx, []model.JobExecutorHeartbeat{
		{ExecutorAppName: "app", ExecutorAddress: "127.0.0.1:9091", HeartbeatTime: oldTime},
		{ExecutorAppName: "app", ExecutorAddress: "127.0.0.1:9092", HeartbeatTime: newTime},
	}); err != nil {
		t.Fatalf("BatchUpsert failed: %v", err)
	}

	expired, err := repo.ListExpired(ctx, time.Now().Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("ListExpired failed: %v", err)
	}
	if len(expired) == 0 {
		t.Fatalf("expected expired heartbeats")
	}

	rows, total, err := repo.List(ctx, PageQuery{Page: 1, PageSize: 2}, HeartbeatQuery{AppName: "app"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total < 3 || len(rows) == 0 {
		t.Fatalf("unexpected heartbeat list result, total=%d len=%d", total, len(rows))
	}

	if err := repo.DeleteByAppAndAddress(ctx, "app", "127.0.0.1:9092"); err != nil {
		t.Fatalf("DeleteByAppAndAddress failed: %v", err)
	}
	if _, err := repo.GetByAppAndAddress(ctx, "app", "127.0.0.1:9092"); err == nil {
		t.Fatalf("expected deleted record not found")
	}
}
