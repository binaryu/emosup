package utils

import (
	"sync"
	"time"
)

// SpeedTracker computes a smoothed instantaneous transfer rate from cumulative
// byte samples. It uses exponential moving average so short bursts or stalls
// don't dominate the displayed speed.
type SpeedTracker struct {
	mu        sync.Mutex
	lastTime  time.Time
	lastBytes int64
	ema       int64
	lastSpeed int64
}

func NewSpeedTracker() *SpeedTracker {
	return &SpeedTracker{}
}

// Sample records a cumulative byte count at time now and returns the smoothed
// speed in bytes/second. Gaps longer than gapReset treat the sample as a fresh
// baseline (e.g. after a stall or resume), keeping the EMA from going stale.
const gapReset = 5 * time.Second

func (t *SpeedTracker) Sample(bytes int64, now time.Time) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.lastTime.IsZero() {
		t.lastTime = now
		t.lastBytes = bytes
		t.lastSpeed = 0
		return 0
	}

	dt := now.Sub(t.lastTime)
	if dt <= 0 {
		return t.lastSpeed
	}

	delta := bytes - t.lastBytes
	if delta < 0 {
		delta = 0
	}

	if dt > gapReset {
		// Long gap: treat as fresh baseline to avoid a tiny delta being
		// divided by a huge interval. Keep the last speed to avoid a hard
		// drop to zero on the very next sample.
		t.lastTime = now
		t.lastBytes = bytes
		return t.lastSpeed
	}

	instant := int64(float64(delta) / dt.Seconds())
	if t.ema == 0 {
		t.ema = instant
	} else {
		// alpha 0.3 → ~4s effective window at 1s samples.
		t.ema = int64(0.7*float64(t.ema) + 0.3*float64(instant))
	}
	if t.ema < 0 {
		t.ema = 0
	}

	t.lastSpeed = t.ema
	t.lastTime = now
	t.lastBytes = bytes
	return t.ema
}

// Speed returns the last computed speed without recording a sample.
func (t *SpeedTracker) Speed() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastSpeed
}

// Reset clears the tracker state (e.g. when a transfer restarts).
func (t *SpeedTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastTime = time.Time{}
	t.lastBytes = 0
	t.ema = 0
	t.lastSpeed = 0
}
