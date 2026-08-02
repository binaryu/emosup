package utils

import (
	"sync"
	"time"
)

// ThroughputMeter aggregates bytes from multiple concurrent sources and
// reports a smoothed aggregate rate in bytes/second. It is the bandwidth
// sensor for the auto-tune controller.
type ThroughputMeter struct {
	mu     sync.Mutex
	window int64 // bytes accumulated since last sample
	last   time.Time
	ema    float64
}

func NewThroughputMeter() *ThroughputMeter {
	return &ThroughputMeter{}
}

// Add records n bytes transferred. Safe for concurrent use.
func (m *ThroughputMeter) Add(n int64) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	m.window += n
	m.mu.Unlock()
}

// Sample consumes the accumulated window and returns the EMA-smoothed rate.
// The window is measured over the time since the previous sample, so callers
// should sample at a regular interval (e.g. every 1-2s).
func (m *ThroughputMeter) Sample(now time.Time) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.last.IsZero() {
		m.last = now
		m.ema = 0
		return 0
	}

	dt := now.Sub(m.last).Seconds()
	bytes := m.window
	m.window = 0
	m.last = now

	if dt <= 0 {
		return m.ema
	}
	instant := float64(bytes) / dt
	if m.ema == 0 {
		m.ema = instant
	} else {
		// Alpha 0.5 at 1-2s samples keeps the estimate responsive yet stable.
		m.ema = 0.5*m.ema + 0.5*instant
	}
	return m.ema
}
