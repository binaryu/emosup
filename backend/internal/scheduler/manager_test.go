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

func TestTaskRemainingBytes(t *testing.T) {
	cases := []struct {
		name string
		task model.Task
		want int64
	}{
		{
			name: "in-flight download counts only remainder",
			task: model.Task{
				Source:   model.TaskSource{FileSize: 10_000},
				Download: model.TaskDownload{TotalBytes: 10_000, CompletedBytes: 4_000},
			},
			want: 6_000,
		},
		{
			name: "file size fallback when total unknown",
			task: model.Task{
				Source:   model.TaskSource{FileSize: 10_000},
				Download: model.TaskDownload{CompletedBytes: 3_000},
			},
			want: 7_000,
		},
		{
			name: "fully written file leaves nothing",
			task: model.Task{
				Source:   model.TaskSource{FileSize: 10_000},
				Download: model.TaskDownload{TotalBytes: 10_000, CompletedBytes: 10_000},
			},
			want: 0,
		},
		{
			name: "parked file (no remaining) is zero",
			task: model.Task{
				Source:   model.TaskSource{FileSize: 10_000},
				Download: model.TaskDownload{CompletedBytes: 10_000},
			},
			want: 0,
		},
		{
			name: "unknown size is zero",
			task: model.Task{},
			want: 0,
		},
	}
	for _, c := range cases {
		if got := taskRemainingBytes(c.task); got != c.want {
			t.Errorf("%s: taskRemainingBytes = %d, want %d", c.name, got, c.want)
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
		// 40G disk, 10G files: one fully-written file uploading, 1+2 uploaded and
		// deleted (30G free). The idle pipeline must admit the next file.
		{"idle pipeline with room admits next file", 30 * gb, 0, 10 * gb, true},
		// Second file: the first admitted download still has all 10G to write.
		{"admission accounts for in-flight remainder", 30 * gb, 10 * gb, 10 * gb, true},
		// 20G already on disk, 10G still to write, next 10G → peak fills the disk.
		{"two in-flight files refuse a third when tight", 20 * gb, 10 * gb, 10 * gb, false},
		// Fully-written files are NOT counted in committed — statfs free (7G)
		// already reflects them, and the next 6G file would leave no headroom.
		{"fully-written files count via statfs free, not committed", 7 * gb, 0, 6 * gb, false},
		// Unknown sizes keep only the headroom.
		{"unknown sizes keep the headroom", 6 * gb, 0, 0, true},
		{"below headroom refused even for unknown sizes", 4 * gb, 0, 0, false},
	}
	for _, c := range cases {
		if got := diskAllowsDownload(c.free, c.committed, c.next); got != c.want {
			t.Errorf("%s: diskAllowsDownload(%d, %d, %d) = %v, want %v",
				c.name, c.free, c.committed, c.next, got, c.want)
		}
	}
}
