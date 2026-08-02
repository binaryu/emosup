package service

import (
	"math"
	"sync"
	"syscall"
	"time"

	"emosup/backend/internal/store"
	"emosup/backend/internal/utils"
)

// Auto-tune controller: adapts parallelism to measured bandwidth and free
// disk space, AIMD-style (additive increase / multiplicative decrease).
//
// Tuned quantities (all applied as floors above the user-configured values
// when enabled):
//
//	DownloadConcurrency — parallel download tasks the scheduler may run
//	DownloadThreads     — per-task download threads
//	UploadConcurrency   — parallel upload tasks the scheduler may run
//	UploadParts         — per-task multipart part concurrency
//	UploadChunkMB       — multipart chunk size
const (
	tuneInterval          = 2 * time.Second
	maxDLUnits            = 16
	maxULUnits            = 10
	maxDownloadConc       = 8
	maxDownloadThreads    = 8
	diskHeadroomBytes     = 5e9
	diskPerFileBytes      = 2e9 // assume ~2GB per pending file
	gainThreshold         = 0.15
	plateauLimit          = 3
	cooldownAfterPlateau  = 90 * time.Second
	cooldownAfterDrop     = 30 * time.Second
	slowStartUnits        = 4
	idleResetAfter        = 6 * time.Second
)

type TuneSnapshot struct {
	Enabled            bool `json:"enabled"`
	DownloadConcurrency int `json:"download_concurrency"`
	DownloadThreads    int  `json:"download_threads"`
	UploadConcurrency  int  `json:"upload_concurrency"`
	UploadParts        int  `json:"upload_parts"`
	UploadChunkMB      int  `json:"upload_chunk_mb"`
}

type Tuner struct {
	mu       sync.Mutex
	store    *store.FileStore
	dlMeter  *utils.ThroughputMeter
	ulMeter  *utils.ThroughputMeter
	dlUnits  int
	ulUnits  int
	lastDl   float64
	lastUl   float64
	lastDlAt time.Time
	lastUlAt time.Time
	plateau  int
	cooldown time.Time
	stop     chan struct{}
	diskFn   func() int64 // test hook
}

func NewTuner(store *store.FileStore) *Tuner {
	return &Tuner{
		store:   store,
		dlMeter: utils.NewThroughputMeter(),
		ulMeter: utils.NewThroughputMeter(),
		dlUnits: 1,
		ulUnits: 1,
	}
}

func (t *Tuner) Start() {
	t.mu.Lock()
	if t.stop != nil {
		t.mu.Unlock()
		return
	}
	t.stop = make(chan struct{})
	t.mu.Unlock()

	go func() {
		ticker := time.NewTicker(tuneInterval)
		defer ticker.Stop()
		for {
			select {
			case <-t.stop:
				return
			case now := <-ticker.C:
				t.tick(now)
			}
		}
	}()
}

func (t *Tuner) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stop != nil {
		close(t.stop)
		t.stop = nil
	}
}

func (t *Tuner) RecordDownloadBytes(n int64) {
	t.dlMeter.Add(n)
}

func (t *Tuner) RecordUploadBytes(n int64) {
	t.ulMeter.Add(n)
}

func (t *Tuner) enabled() bool {
	if t.store == nil {
		return false
	}
	cfg, err := t.store.LoadConfig()
	if err != nil {
		return false
	}
	// nil means "not configured yet" → enabled by default.
	return cfg.Worker.AutoTune == nil || *cfg.Worker.AutoTune
}

func (t *Tuner) downloadDir() string {
	if t.store == nil {
		return ""
	}
	cfg, err := t.store.LoadConfig()
	if err != nil {
		return ""
	}
	return cfg.Download.Dir
}

// Snapshot returns the current tuned floors. Callers combine them with the
// user-configured values (max) before use.
func (t *Tuner) Snapshot() TuneSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled() {
		return TuneSnapshot{}
	}

	dl := t.dlUnits
	ul := t.ulUnits
	disk := diskAllowance(t.freeDiskLocked())

	conc := clamp(minInt(disk, dl), 1, maxDownloadConc)
	threads := clamp(int(math.Ceil(float64(dl)/float64(conc))), 1, maxDownloadThreads)

	return TuneSnapshot{
		Enabled:             true,
		DownloadConcurrency: conc,
		DownloadThreads:     threads,
		UploadConcurrency:   clamp(ul, 1, maxDownloadConc),
		UploadParts:         clamp(ul, 1, maxULUnits),
		UploadChunkMB:       clamp(ul*4, 4, 64),
	}
}

func (t *Tuner) freeDiskLocked() int64 {
	if t.diskFn != nil {
		return t.diskFn()
	}
	dir := t.downloadDir()
	if dir == "" {
		return 0
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0
	}
	return int64(stat.Bavail) * int64(stat.Bsize)
}

// tick runs the AIMD controller once. now is injected for testability.
func (t *Tuner) tick(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	dl := t.dlMeter.Sample(now)
	ul := t.ulMeter.Sample(now)

	t.adjustUnits(&t.dlUnits, &t.lastDl, &t.lastDlAt, dl, now, maxDLUnits)
	t.adjustUnits(&t.ulUnits, &t.lastUl, &t.lastUlAt, ul, now, maxULUnits)
}

// adjustUnits applies one AIMD step for a direction. Both the previous and the
// current window must have seen traffic (prev>0, rate>0); a zero window means
// the transfers ended or stalled, which we must not punish.
func (t *Tuner) adjustUnits(units *int, last *float64, lastAt *time.Time, rate float64, now time.Time, cap int) {
	idle := lastAt.IsZero() || now.Sub(*lastAt) > idleResetAfter
	prev := *last
	if rate > 0 {
		*lastAt = now
	}
	*last = rate

	if idle || rate <= 0 || prev <= 0 {
		return
	}

	gain := (rate - prev) / prev
	if now.Before(t.cooldown) && gain > gainThreshold {
		// Cooldown only suppresses increases; drops always react.
		return
	}

	switch {
	case gain > gainThreshold:
		if *units < slowStartUnits {
			*units = minInt(*units*2, cap)
		} else {
			*units = minInt(*units+1, cap)
		}
		t.plateau = 0
	case gain < -gainThreshold:
		*units = maxInt(1, int(float64(*units)*0.7))
		t.cooldown = now.Add(cooldownAfterDrop)
		t.plateau = 0
	default:
		t.plateau++
		if t.plateau >= plateauLimit {
			t.cooldown = now.Add(cooldownAfterPlateau)
			t.plateau = 0
		}
	}
}

// diskAllowance computes how many concurrent downloads the free space can hold.
func diskAllowance(free int64) int {
	if free <= diskHeadroomBytes {
		return 1
	}
	n := int((free-diskHeadroomBytes)/diskPerFileBytes) + 1
	return clamp(n, 1, maxDownloadConc)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
