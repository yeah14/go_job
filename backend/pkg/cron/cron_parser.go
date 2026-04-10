package cron

import (
	"errors"
	"fmt"
	"strings"
	"time"

	robfigcron "github.com/robfig/cron/v3"
)

var (
	ErrEmptySpec     = errors.New("empty cron spec")
	ErrZeroDelay     = errors.New("computed zero delay")
	ErrInvalidWindow = errors.New("invalid time window")
)

// NextRun returns the next trigger time after from.
// It supports standard 5-field cron spec: "min hour dom mon dow".
func NextRun(spec string, from time.Time) (time.Time, error) {
	if spec == "" {
		return time.Time{}, ErrEmptySpec
	}

	schedule, err := parseSchedule(spec)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron spec failed: %w", err)
	}

	next := schedule.Next(from)
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("cron has no next run: %w", ErrZeroDelay)
	}
	return next, nil
}

// NextDelay converts cron spec into a duration delay for time wheel.
// If next equals from due to precision boundary, it returns at least minDelay.
func NextDelay(spec string, from time.Time, minDelay time.Duration) (time.Duration, time.Time, error) {
	next, err := NextRun(spec, from)
	if err != nil {
		return 0, time.Time{}, err
	}

	delay := next.Sub(from)
	if delay <= 0 {
		if minDelay <= 0 {
			minDelay = time.Second
		}
		delay = minDelay
	}

	return delay, next, nil
}

// DelayUntil computes delay from now until target.
func DelayUntil(now time.Time, target time.Time, minDelay time.Duration) (time.Duration, error) {
	if target.IsZero() {
		return 0, ErrInvalidWindow
	}
	delay := target.Sub(now)
	if delay <= 0 {
		if minDelay <= 0 {
			minDelay = time.Second
		}
		return minDelay, nil
	}
	return delay, nil
}

func parseSchedule(spec string) (robfigcron.Schedule, error) {
	fields := strings.Fields(spec)
	switch len(fields) {
	case 5:
		return robfigcron.ParseStandard(spec)
	case 6:
		// Support second-level cron for scheduler tests and high-frequency jobs.
		parser := robfigcron.NewParser(
			robfigcron.Second |
				robfigcron.Minute |
				robfigcron.Hour |
				robfigcron.Dom |
				robfigcron.Month |
				robfigcron.Dow,
		)
		return parser.Parse(spec)
	default:
		return nil, fmt.Errorf("invalid cron field count: %d", len(fields))
	}
}
