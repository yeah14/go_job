package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
	"go-job/internal/repository"
	"go-job/pkg/timer"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeJobRepo struct {
	jobs []model.JobInfo
}

func (f *fakeJobRepo) Create(ctx context.Context, job *model.JobInfo) error { return nil }
func (f *fakeJobRepo) GetByID(ctx context.Context, id int) (*model.JobInfo, error) {
	return nil, nil
}
func (f *fakeJobRepo) List(ctx context.Context, page repository.PageQuery, query repository.JobQuery) ([]model.JobInfo, int64, error) {
	return nil, 0, nil
}
func (f *fakeJobRepo) Update(ctx context.Context, job *model.JobInfo) error { return nil }
func (f *fakeJobRepo) UpdateStatus(ctx context.Context, id int, status int8) error {
	return nil
}
func (f *fakeJobRepo) Delete(ctx context.Context, id int) error { return nil }
func (f *fakeJobRepo) BatchCreate(ctx context.Context, jobs []model.JobInfo) error {
	return nil
}
func (f *fakeJobRepo) ListRunning(ctx context.Context) ([]model.JobInfo, error) {
	return f.jobs, nil
}

func TestScheduleService_LoadAndTrigger(t *testing.T) {
	tw, err := timer.NewTimeWheel(100*time.Millisecond, 120)
	if err != nil {
		t.Fatalf("new wheel failed: %v", err)
	}

	repo := &fakeJobRepo{
		jobs: []model.JobInfo{
			{
				ID:              1,
				JobName:         "cron-job",
				ExecutorID:      1,
				ExecutorHandler: "demo",
				Cron:            "*/1 * * * * *", // every second
				ShardTotal:      1,
				Timeout:         30,
				Priority:        1,
				Status:          1,
			},
		},
	}

	done := make(chan struct{}, 1)
	var triggerCount int32
	svc := NewScheduleService(repo, tw, func(ctx context.Context, job model.JobInfo) {
		if job.ID == 1 && atomic.AddInt32(&triggerCount, 1) == 1 {
			done <- struct{}{}
		}
	})

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start scheduler failed: %v", err)
	}
	defer svc.Stop()

	select {
	case <-done:
		// pass
	case <-time.After(3 * time.Second):
		t.Fatal("scheduled job did not trigger in expected window")
	}
}

func TestScheduleService_RemoveJob(t *testing.T) {
	tw, err := timer.NewTimeWheel(50*time.Millisecond, 120)
	if err != nil {
		t.Fatalf("new wheel failed: %v", err)
	}

	repo := &fakeJobRepo{
		jobs: []model.JobInfo{
			{
				ID:              2,
				JobName:         "remove-job",
				ExecutorID:      1,
				ExecutorHandler: "demo",
				Cron:            "*/1 * * * * *",
				ShardTotal:      1,
				Timeout:         30,
				Priority:        1,
				Status:          1,
			},
		},
	}

	var triggerCount int32
	svc := NewScheduleService(repo, tw, func(ctx context.Context, job model.JobInfo) {
		atomic.AddInt32(&triggerCount, 1)
	})

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start scheduler failed: %v", err)
	}
	defer svc.Stop()

	// Remove before first trigger window arrives.
	time.Sleep(100 * time.Millisecond)
	svc.RemoveJob(2)

	time.Sleep(1500 * time.Millisecond)
	if atomic.LoadInt32(&triggerCount) != 0 {
		t.Fatalf("removed job should not trigger, got=%d", triggerCount)
	}
}

func TestScheduleService_InvalidCron(t *testing.T) {
	tw, err := timer.NewTimeWheel(50*time.Millisecond, 120)
	if err != nil {
		t.Fatalf("new wheel failed: %v", err)
	}

	repo := &fakeJobRepo{
		jobs: []model.JobInfo{
			{
				ID:              3,
				JobName:         "bad-cron",
				ExecutorID:      1,
				ExecutorHandler: "demo",
				Cron:            "invalid cron spec",
				ShardTotal:      1,
				Timeout:         30,
				Priority:        1,
				Status:          1,
			},
		},
	}

	svc := NewScheduleService(repo, tw, nil)

	if err := tw.Start(); err != nil {
		t.Fatalf("start wheel failed: %v", err)
	}
	defer tw.Stop()

	err = svc.LoadRunningJobs(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid cron spec, got nil")
	}
}

func TestScheduleService_RestartReloadRunningJobs(t *testing.T) {
	tw, err := timer.NewTimeWheel(50*time.Millisecond, 120)
	if err != nil {
		t.Fatalf("new wheel failed: %v", err)
	}

	repo := &fakeJobRepo{
		jobs: []model.JobInfo{
			{
				ID:              10,
				JobName:         "restart-job",
				ExecutorID:      1,
				ExecutorHandler: "demo",
				Cron:            "*/1 * * * * *",
				ShardTotal:      1,
				Timeout:         30,
				Priority:        1,
				Status:          1,
			},
		},
	}

	var triggerCount int32
	svc := NewScheduleService(repo, tw, func(ctx context.Context, job model.JobInfo) {
		atomic.AddInt32(&triggerCount, 1)
	})

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start scheduler failed: %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	if err := svc.Stop(); err != nil {
		t.Fatalf("stop scheduler failed: %v", err)
	}

	beforeRestart := atomic.LoadInt32(&triggerCount)
	if beforeRestart == 0 {
		t.Fatal("expected trigger before restart")
	}

	// New service instance simulates process restart and reload.
	tw2, _ := timer.NewTimeWheel(50*time.Millisecond, 120)
	svc2 := NewScheduleService(repo, tw2, func(ctx context.Context, job model.JobInfo) {
		atomic.AddInt32(&triggerCount, 1)
	})
	if err := svc2.Start(context.Background()); err != nil {
		t.Fatalf("restart start failed: %v", err)
	}
	defer svc2.Stop()

	time.Sleep(1200 * time.Millisecond)
	if atomic.LoadInt32(&triggerCount) <= beforeRestart {
		t.Fatal("expected trigger after restart reload")
	}
}

func TestScheduleService_DistributedLockAvoidsDuplicateTrigger(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis failed: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	repo := &fakeJobRepo{
		jobs: []model.JobInfo{
			{
				ID:              99,
				JobName:         "lock-job",
				ExecutorID:      1,
				ExecutorHandler: "demo",
				Cron:            "*/1 * * * * *",
				ShardTotal:      1,
				Timeout:         30,
				Priority:        1,
				Status:          1,
			},
		},
	}

	var triggerCount int32
	tw1, _ := timer.NewTimeWheel(50*time.Millisecond, 120)
	tw2, _ := timer.NewTimeWheel(50*time.Millisecond, 120)

	s1 := NewScheduleServiceWithLock(repo, tw1, func(ctx context.Context, job model.JobInfo) {
		atomic.AddInt32(&triggerCount, 1)
	}, client)
	s2 := NewScheduleServiceWithLock(repo, tw2, func(ctx context.Context, job model.JobInfo) {
		atomic.AddInt32(&triggerCount, 1)
	}, client)

	if err := s1.Start(context.Background()); err != nil {
		t.Fatalf("start s1 failed: %v", err)
	}
	defer s1.Stop()
	if err := s2.Start(context.Background()); err != nil {
		t.Fatalf("start s2 failed: %v", err)
	}
	defer s2.Stop()

	time.Sleep(1200 * time.Millisecond)
	if c := atomic.LoadInt32(&triggerCount); c != 1 {
		t.Fatalf("expected exactly one trigger with distributed lock, got %d", c)
	}
}

func TestScheduleService_AutoTriggerCreatesLogLifecycle(t *testing.T) {
	db := newServiceTestDB(t)
	ctx := context.Background()

	exeRepo := repository.NewExecutorRepository(db)
	jobRepo := repository.NewJobRepository(db)
	logRepo := repository.NewLogRepository(db)
	logSvc := NewLogService(logRepo)
	triggerSvc := NewTriggerService(logSvc)

	exe := &model.JobExecutor{
		AppName:     "auto-trigger-app",
		Name:        "auto-trigger-executor",
		AddressType: 0,
		Status:      int8(enum.ExecutorStatusEnabled),
	}
	if err := exeRepo.Create(ctx, exe); err != nil {
		t.Fatalf("create executor failed: %v", err)
	}

	job := &model.JobInfo{
		JobName:         "auto-trigger-job",
		ExecutorID:      exe.ID,
		ExecutorHandler: "demo.handler",
		Cron:            "*/1 * * * * *",
		ShardTotal:      1,
		Timeout:         30,
		RetryCount:      0,
		Priority:        1,
		Status:          int8(enum.JobStatusRunning),
	}
	if err := jobRepo.Create(ctx, job); err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	tw, err := timer.NewTimeWheel(100*time.Millisecond, 120)
	if err != nil {
		t.Fatalf("new wheel failed: %v", err)
	}

	triggerCallback := NewTriggerCallbackFromService(triggerSvc, "scheduler-local", 0, nil)
	svc := NewScheduleService(jobRepo, tw, triggerCallback)
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start schedule service failed: %v", err)
	}
	defer svc.Stop()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rows, total, err := logSvc.List(ctx, repository.PageQuery{Page: 1, PageSize: 10}, repository.LogQuery{
			JobID: &job.ID,
		})
		if err != nil {
			t.Fatalf("query logs failed: %v", err)
		}
		if total > 0 && len(rows) > 0 {
			if rows[0].Status != int8(enum.LogStatusSuccess) {
				t.Fatalf("expected success log status, got %d", rows[0].Status)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatal("expected auto-scheduled trigger to create log within 3s")
}
