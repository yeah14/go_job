package timer

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"go-job/pkg/logger"

	"go.uber.org/zap"
)

var (
	ErrInvalidInterval = errors.New("invalid interval: must be > 0")
	ErrInvalidSlotNum  = errors.New("invalid slot number: must be > 0")
	ErrNilJob          = errors.New("nil job function")
	ErrEmptyJobID      = errors.New("empty job id")
	ErrWheelNotRunning = errors.New("time wheel is not running")
	ErrWheelRunning    = errors.New("time wheel already running")
	ErrDuplicateJobID  = errors.New("duplicate job id")
	ErrInvalidDelay    = errors.New("invalid delay: must be >= 0")
)

type JobFunc func()

type task struct {
	id                string
	fn                JobFunc
	rounds            int
	interval          time.Duration
	recurring         bool
	cancelled         bool
	createdAt         time.Time
	lastScheduledTime time.Time
}

// TimeWheel is a concurrent-safe timer wheel for high-volume scheduling.
// It is suitable for second-level scheduling and short/medium delay tasks.
type TimeWheel struct {
	mu         sync.Mutex
	interval   time.Duration
	slotNum    int
	currentPos int

	slots     []map[string]*task
	taskIndex map[string]int

	ticker  *time.Ticker
	stopCh  chan struct{}
	doneCh  chan struct{}
	running bool
}

// NewTimeWheel creates a new time wheel instance.
func NewTimeWheel(interval time.Duration, slotNum int) (*TimeWheel, error) {
	if interval <= 0 {
		return nil, ErrInvalidInterval
	}
	if slotNum <= 0 {
		return nil, ErrInvalidSlotNum
	}

	slots := make([]map[string]*task, slotNum)
	for i := range slots {
		slots[i] = make(map[string]*task)
	}

	return &TimeWheel{
		interval:  interval,
		slotNum:   slotNum,
		slots:     slots,
		taskIndex: make(map[string]int),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}, nil
}

// Start starts the wheel loop. It returns error if called repeatedly.
func (tw *TimeWheel) Start() error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.running {
		return ErrWheelRunning
	}

	tw.ticker = time.NewTicker(tw.interval)
	tw.running = true

	go tw.loop()
	return nil
}

// Stop gracefully stops the wheel and waits for loop exit.
func (tw *TimeWheel) Stop() error {
	tw.mu.Lock()
	if !tw.running {
		tw.mu.Unlock()
		return ErrWheelNotRunning
	}
	close(tw.stopCh)
	tw.running = false
	tw.mu.Unlock()

	<-tw.doneCh
	return nil
}

// AddJob adds a one-time delayed job to the wheel.
func (tw *TimeWheel) AddJob(delay time.Duration, jobID string, job JobFunc) error {
	return tw.addTask(delay, jobID, job, false, 0)
}

// AddRecurringJob adds a recurring job. First run happens after interval.
func (tw *TimeWheel) AddRecurringJob(interval time.Duration, jobID string, job JobFunc) error {
	if interval <= 0 {
		return fmt.Errorf("%w: recurring interval must be > 0", ErrInvalidInterval)
	}
	return tw.addTask(interval, jobID, job, true, interval)
}

func (tw *TimeWheel) addTask(delay time.Duration, jobID string, job JobFunc, recurring bool, recurringInterval time.Duration) error {
	if delay < 0 {
		return ErrInvalidDelay
	}
	if jobID == "" {
		return ErrEmptyJobID
	}
	if job == nil {
		return ErrNilJob
	}

	tw.mu.Lock()
	defer tw.mu.Unlock()

	if !tw.running {
		return ErrWheelNotRunning
	}
	if _, exists := tw.taskIndex[jobID]; exists {
		return ErrDuplicateJobID
	}

	ticks := tw.durationToTicks(delay)
	rounds := ticks / tw.slotNum
	pos := (tw.currentPos + ticks) % tw.slotNum

	t := &task{
		id:                jobID,
		fn:                job,
		rounds:            rounds,
		recurring:         recurring,
		interval:          recurringInterval,
		createdAt:         time.Now(),
		lastScheduledTime: time.Now(),
	}

	tw.slots[pos][jobID] = t
	tw.taskIndex[jobID] = pos
	return nil
}

// RemoveJob removes a job by its id. Returns true if removed.
func (tw *TimeWheel) RemoveJob(jobID string) bool {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	pos, ok := tw.taskIndex[jobID]
	if !ok {
		return false
	}
	if t, exists := tw.slots[pos][jobID]; exists {
		t.cancelled = true
		delete(tw.slots[pos], jobID)
	}
	delete(tw.taskIndex, jobID)
	return true
}

func (tw *TimeWheel) loop() {
	defer close(tw.doneCh)
	defer tw.ticker.Stop()

	for {
		select {
		case <-tw.ticker.C:
			tw.tick()
		case <-tw.stopCh:
			return
		}
	}
}

func (tw *TimeWheel) tick() {
	var toRun []JobFunc

	tw.mu.Lock()
	slot := tw.slots[tw.currentPos]

	for jobID, t := range slot {
		if t.cancelled {
			delete(slot, jobID)
			delete(tw.taskIndex, jobID)
			continue
		}

		if t.rounds > 0 {
			t.rounds--
			continue
		}

		// Remove from current slot first to avoid duplicate execution.
		delete(slot, jobID)
		delete(tw.taskIndex, jobID)
		toRun = append(toRun, t.fn)

		if t.recurring && !t.cancelled {
			ticks := tw.durationToTicks(t.interval)
			t.rounds = ticks / tw.slotNum
			nextPos := (tw.currentPos + ticks) % tw.slotNum
			t.lastScheduledTime = time.Now()
			tw.slots[nextPos][jobID] = t
			tw.taskIndex[jobID] = nextPos
		}
	}

	tw.currentPos = (tw.currentPos + 1) % tw.slotNum
	tw.mu.Unlock()

	// Execute outside lock to avoid blocking scheduler.
	for _, fn := range toRun {
		go safeRun(fn)
	}
}

func (tw *TimeWheel) durationToTicks(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	ticks := int(d / tw.interval)
	if d%tw.interval != 0 {
		ticks++
	}
	if ticks <= 0 {
		return 1
	}
	return ticks
}

func safeRun(fn JobFunc) {
	defer func() {
		// Avoid scheduler crash by isolating panic in user job.
		if r := recover(); r != nil {
			logger.L().Error("time wheel job panicked",
				zap.Any("panic", r),
				zap.ByteString("stack", debug.Stack()),
			)
		}
	}()
	fn()
}
