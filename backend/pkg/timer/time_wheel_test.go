package timer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimeWheel_OneShotAccuracy(t *testing.T) {
	tw, err := NewTimeWheel(20*time.Millisecond, 60)
	if err != nil {
		t.Fatalf("new wheel failed: %v", err)
	}
	if err := tw.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer tw.Stop()

	done := make(chan time.Time, 1)
	start := time.Now()

	if err := tw.AddJob(120*time.Millisecond, "one-shot", func() {
		done <- time.Now()
	}); err != nil {
		t.Fatalf("add job failed: %v", err)
	}

	select {
	case firedAt := <-done:
		elapsed := firedAt.Sub(start)
		if elapsed < 90*time.Millisecond || elapsed > 260*time.Millisecond {
			t.Fatalf("unexpected trigger latency: %v", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("job did not trigger in time")
	}
}

func TestTimeWheel_OneShotPrecisionUnder100ms(t *testing.T) {
	tw, err := NewTimeWheel(20*time.Millisecond, 120)
	if err != nil {
		t.Fatalf("new wheel failed: %v", err)
	}
	if err := tw.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer tw.Stop()

	const targetDelay = 200 * time.Millisecond
	const maxError = 100 * time.Millisecond

	done := make(chan time.Time, 1)
	start := time.Now()

	if err := tw.AddJob(targetDelay, "precision-100ms", func() {
		done <- time.Now()
	}); err != nil {
		t.Fatalf("add job failed: %v", err)
	}

	select {
	case firedAt := <-done:
		elapsed := firedAt.Sub(start)
		diff := elapsed - targetDelay
		if diff < 0 {
			diff = -diff
		}
		if diff >= maxError {
			t.Fatalf("precision check failed: target=%v elapsed=%v abs_error=%v", targetDelay, elapsed, diff)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("job did not trigger in time")
	}
}

func TestTimeWheel_RemoveJob(t *testing.T) {
	tw, err := NewTimeWheel(20*time.Millisecond, 60)
	if err != nil {
		t.Fatalf("new wheel failed: %v", err)
	}
	if err := tw.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer tw.Stop()

	triggered := int32(0)
	if err := tw.AddJob(120*time.Millisecond, "to-remove", func() {
		atomic.StoreInt32(&triggered, 1)
	}); err != nil {
		t.Fatalf("add job failed: %v", err)
	}
	tw.RemoveJob("to-remove")

	time.Sleep(300 * time.Millisecond)
	if atomic.LoadInt32(&triggered) == 1 {
		t.Fatal("removed job should not be triggered")
	}
}

func TestTimeWheel_ConcurrentOperations(t *testing.T) {
	tw, err := NewTimeWheel(10*time.Millisecond, 100)
	if err != nil {
		t.Fatalf("new wheel failed: %v", err)
	}
	if err := tw.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer tw.Stop()

	var fired int32
	var wg sync.WaitGroup

	for i := 0; i < 80; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			id := "job-" + time.Now().Add(time.Duration(i)*time.Microsecond).Format("150405.000000")
			_ = tw.AddJob(50*time.Millisecond, id, func() {
				atomic.AddInt32(&fired, 1)
			})
			if i%5 == 0 {
				tw.RemoveJob(id)
			}
		}()
	}
	wg.Wait()

	time.Sleep(400 * time.Millisecond)
	if atomic.LoadInt32(&fired) == 0 {
		t.Fatal("expected some jobs to be triggered")
	}
}
