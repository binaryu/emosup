package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLocalRootSlash(t *testing.T) {
	os.Setenv("EMOSUP_LOCAL_ROOT", "/")
	defer os.Unsetenv("EMOSUP_LOCAL_ROOT")
	s := &LocalService{}

	for _, p := range []string{"/", "/tmp", "/etc", "/vol1"} {
		_, full, _, err := s.resolve(p)
		if err != nil {
			t.Fatalf("resolve(%q) with root /: unexpected error %v", p, err)
		}
		if full != filepath.Clean(p) {
			t.Fatalf("resolve(%q) = %q, want %q", p, full, p)
		}
	}
}

func TestResolveTraversalBlocked(t *testing.T) {
	root := t.TempDir()
	os.Setenv("EMOSUP_LOCAL_ROOT", root)
	defer os.Unsetenv("EMOSUP_LOCAL_ROOT")
	s := &LocalService{}

	traversal := []string{"../etc/passwd", "../../etc", "..", "sub/../../secret"}
	for _, p := range traversal {
		if _, _, _, err := s.resolve(p); err == nil {
			t.Fatalf("resolve(%q): expected access denied, got nil", p)
		}
	}

	if _, _, _, err := s.resolve("sub"); err != nil {
		t.Fatalf("resolve(sub) under root: unexpected error %v", err)
	}
}
