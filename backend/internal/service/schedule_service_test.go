package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go-job/internal/domain/model"
	"go-job/internal/repository"
	"go-job/pkg/timer"
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
