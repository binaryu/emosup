package service

import (
	"testing"
	"time"

	"emosup/backend/internal/store"
)

func newTestTuner(t *testing.T) *Tuner {
	t.Helper()
	fileStore := store.NewFileStore(t.TempDir())
	if err := fileStore.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fileStore.Close() })
	return NewTuner(fileStore)
}

// feedAndTick records n bytes over one tune interval and advances one tick.
func feedAndTick(tuner *Tuner, now time.Time, dl, ul int64) time.Time {
	if dl > 0 {
		tuner.RecordDownloadBytes(dl)
	}
	if ul > 0 {
		tuner.RecordUploadBytes(ul)
	}
	tuner.tick(now)
	return now.Add(tuneInterval)
}

func TestTunerRampsUpOnGrowth(t *testing.T) {
	tuner := newTestTuner(t)
	now := time.Now()

	// Throughput grows steadily: 5, 10, 20, 40, 80, 120 MB/s.
	for i, mb := range []int64{10, 20, 40, 80, 160, 240} {
		now = feedAndTick(tuner, now, mb*1024*1024, 0)
		if i == 0 {
			continue // first sample establishes baseline
		}
	}
	tuner.mu.Lock()
	units := tuner.dlUnits
	tuner.mu.Unlock()
	if units <= 1 {
		t.Fatalf("expected units to ramp up, got %d", units)
	}
	if units > maxDLUnits {
		t.Fatalf("units exceeded cap: %d", units)
	}
}

func TestTunerRetreatsOnThroughputDrop(t *testing.T) {
	tuner := newTestTuner(t)
	now := time.Now()

	// Ramp up first (increasing throughput), then establish a stable high rate.
	for i, mb := range []int64{10, 20, 40, 80, 160, 200, 200, 200} {
		now = feedAndTick(tuner, now, mb*1024*1024, 0)
		if i == 0 {
			continue
		}
	}
	tuner.mu.Lock()
	before := tuner.dlUnits
	tuner.mu.Unlock()
	if before <= 1 {
		t.Fatalf("ramp-up failed, units=%d", before)
	}

	// Throughput halves: the drop must shrink the allowance.
	for i := 0; i < 3; i++ {
		now = feedAndTick(tuner, now, 30*1024*1024, 0)
	}
	tuner.mu.Lock()
	after := tuner.dlUnits
	tuner.mu.Unlock()
	if after >= before {
		t.Fatalf("expected units to shrink on drop: before=%d after=%d", before, after)
	}
}

func TestTunerIdleKeepsUnits(t *testing.T) {
	tuner := newTestTuner(t)
	now := time.Now()
	now = feedAndTick(tuner, now, 100*1024*1024, 0)
	now = feedAndTick(tuner, now, 100*1024*1024, 0)

	// No traffic for several intervals: units must not collapse.
	now = feedAndTick(tuner, now, 0, 0)
	now = feedAndTick(tuner, now, 0, 0)
	now = feedAndTick(tuner, now, 0, 0)

	tuner.mu.Lock()
	defer tuner.mu.Unlock()
	if tuner.dlUnits != 1 {
		t.Fatalf("expected units to stay at 1 while idle, got %d", tuner.dlUnits)
	}
}

func TestTunerPlateauTriggersCooldown(t *testing.T) {
	tuner := newTestTuner(t)
	now := time.Now()

	// Ramp to 4 units.
	for i := 0; i < 5; i++ {
		now = feedAndTick(tuner, now, 100*1024*1024, 0)
	}
	tuner.mu.Lock()
	tuner.dlUnits = 8
	tuner.mu.Unlock()

	// Constant rate → plateau → cooldown after plateauLimit ticks.
	for i := 0; i < plateauLimit+2; i++ {
		now = feedAndTick(tuner, now, 100*1024*1024, 0)
	}
	tuner.mu.Lock()
	inCooldown := now.Before(tuner.cooldown)
	tuner.mu.Unlock()
	if !inCooldown {
		t.Fatal("expected cooldown after repeated plateaus")
	}

	// During cooldown, growth must not increase units.
	before := tuner.dlUnits
	now = feedAndTick(tuner, now, 200*1024*1024, 0)
	tuner.mu.Lock()
	after := tuner.dlUnits
	tuner.mu.Unlock()
	if after != before {
		t.Fatalf("expected cooldown to suppress increase: before=%d after=%d", before, after)
	}
}

func TestTunerSnapshotDerivation(t *testing.T) {
	tuner := newTestTuner(t)

	tuner.mu.Lock()
	tuner.dlUnits = 16
	tuner.ulUnits = 6
	tuner.mu.Unlock()

	snap := tuner.Snapshot()
	if !snap.Enabled {
		t.Fatal("expected snapshot enabled")
	}
	if snap.DownloadConcurrency != maxDownloadConc {
		t.Fatalf("concurrency = %d, want %d (dlUnits 16 clamped)", snap.DownloadConcurrency, maxDownloadConc)
	}
	if snap.DownloadConcurrency*snap.DownloadThreads < 16 {
		t.Fatalf("concurrency×threads must cover dlUnits: %d×%d", snap.DownloadConcurrency, snap.DownloadThreads)
	}
	if snap.UploadParts != 6 || snap.UploadChunkMB != 24 {
		t.Fatalf("upload snapshot wrong: parts=%d chunk=%d", snap.UploadParts, snap.UploadChunkMB)
	}
}

func TestTunerDisabledWithoutStore(t *testing.T) {
	tuner := NewTuner(nil)
	snap := tuner.Snapshot()
	if snap.Enabled {
		t.Fatal("tuner without store must be disabled")
	}
}
