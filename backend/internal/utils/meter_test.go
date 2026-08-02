package utils

import (
	"testing"
	"time"
)

func TestThroughputMeterSteadyRate(t *testing.T) {
	m := NewThroughputMeter()
	now := time.Now()
	if got := m.Sample(now); got != 0 {
		t.Fatalf("first sample = %v, want 0", got)
	}
	// 10MB every second for 3 windows.
	last := 0.0
	for i := 1; i <= 3; i++ {
		m.Add(10 * 1024 * 1024)
		got := m.Sample(now.Add(time.Duration(i) * time.Second))
		last = got
	}
	if last < 9*1024*1024 || last > 11*1024*1024 {
		t.Fatalf("steady rate = %.0f, want ~10MB/s", last)
	}
}

func TestThroughputMeterConcurrentAdds(t *testing.T) {
	m := NewThroughputMeter()
	now := time.Now()
	m.Sample(now)
	for i := 0; i < 100; i++ {
		go m.Add(1024 * 1024)
	}
	time.Sleep(50 * time.Millisecond)
	m.Add(0)
	got := m.Sample(now.Add(1 * time.Second))
	if got <= 0 {
		t.Fatalf("expected aggregated rate, got %v", got)
	}
}
