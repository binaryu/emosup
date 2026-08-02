package utils

import (
	"testing"
	"time"
)

func TestSpeedTrackerSteadyRate(t *testing.T) {
	tr := NewSpeedTracker()
	now := time.Now()
	tr.Sample(0, now)
	for i := 1; i <= 10; i++ {
		got := tr.Sample(int64(i)*1024*1024, now.Add(time.Duration(i)*time.Second))
		if got != 1024*1024 {
			t.Fatalf("sample %d: speed = %d, want 1048576", i, got)
		}
	}
}

func TestSpeedTrackerSmoothsBurst(t *testing.T) {
	tr := NewSpeedTracker()
	now := time.Now()
	tr.Sample(0, now)
	tr.Sample(10*1024*1024, now.Add(time.Second)) // burst 10MB/s
	got := tr.Sample(12*1024*1024, now.Add(2*time.Second))
	// EMA after burst: 0.7*10MB/s + 0.3*2MB/s = 7.6MB/s
	burst := float64(10 * 1024 * 1024)
	steady := float64(2 * 1024 * 1024)
	want := int64(0.7*burst + 0.3*steady)
	if got != want {
		t.Fatalf("speed = %d, want %d", got, want)
	}
}

func TestSpeedTrackerStallDecays(t *testing.T) {
	tr := NewSpeedTracker()
	now := time.Now()
	tr.Sample(0, now)
	tr.Sample(10*1024*1024, now.Add(time.Second))
	got := tr.Sample(10*1024*1024, now.Add(2*time.Second)) // no progress → decays toward 0
	if got >= 10*1024*1024 {
		t.Fatalf("expected decay, got %d", got)
	}
}

func TestSpeedTrackerGapResetsBaseline(t *testing.T) {
	tr := NewSpeedTracker()
	now := time.Now()
	tr.Sample(0, now)
	tr.Sample(1024*1024, now.Add(time.Second))
	// 30s gap, only 1KB more: must not divide tiny delta by huge interval.
	got := tr.Sample(1024*1024+1024, now.Add(31*time.Second))
	if got > 1024*1024 {
		t.Fatalf("gap sample inflated speed: %d", got)
	}
}
