package scheduler

import (
	"testing"

	"emosup/backend/internal/model"
)

func TestTaskDiskBytes(t *testing.T) {
	cases := []struct {
		name string
		task model.Task
		want int64
	}{
		{
			name: "probed total wins",
			task: model.Task{
				Source:   model.TaskSource{FileSize: 1000},
				Download: model.TaskDownload{TotalBytes: 6600, CompletedBytes: 3000},
			},
			want: 6600,
		},
		{
			name: "completed bytes for parked uploads",
			task: model.Task{
				Source:   model.TaskSource{FileSize: 1000},
				Download: model.TaskDownload{CompletedBytes: 4400},
			},
			want: 4400,
		},
		{
			name: "scan file size fallback",
			task: model.Task{
				Source: model.TaskSource{FileSize: 2048},
			},
			want: 2048,
		},
		{
			name: "unknown size is zero",
			task: model.Task{},
			want: 0,
		},
	}
	for _, c := range cases {
		if got := taskDiskBytes(c.task); got != c.want {
			t.Errorf("%s: taskDiskBytes = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestDiskAllowsDownload(t *testing.T) {
	const gb = int64(1e9)
	cases := []struct {
		name      string
		free      int64
		committed int64
		next      int64
		want      bool
	}{
		// 40G free, 6.6G files: first file fits, sixth does not.
		{"first 6.6G file fits in 40G", 40 * gb, 0, 6600 * 1e6, true},
		{"sixth 6.6G file exceeds 40G", 40 * gb, 5 * 6600 * 1e6, 6600 * 1e6, false},
		{"exactly at headroom boundary is allowed", 11 * gb, 0, 6 * gb, true},
		{"below headroom is refused", 10 * gb, 0, 6 * gb, false},
		{"unknown next size still keeps headroom", 5 * gb, 0, 0, true},
		{"unknown next size with no headroom refused", 4 * gb, 0, 0, false},
	}
	for _, c := range cases {
		if got := diskAllowsDownload(c.free, c.committed, c.next); got != c.want {
			t.Errorf("%s: diskAllowsDownload(%d, %d, %d) = %v, want %v",
				c.name, c.free, c.committed, c.next, got, c.want)
		}
	}
}
