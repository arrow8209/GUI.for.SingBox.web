package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandboxAllowsRelativeWithinBase(t *testing.T) {
	base := t.TempDir()
	sb := NewSandbox(base)
	got, err := sb.Resolve("subdir/file.txt")
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	want := filepath.Join(base, "subdir/file.txt")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestSandboxRejectsAbsolute(t *testing.T) {
	sb := NewSandbox(t.TempDir())
	if _, err := sb.Resolve("/etc/passwd"); err == nil {
		t.Error("absolute path must be rejected")
	}
}

func TestSandboxRejectsTraversal(t *testing.T) {
	sb := NewSandbox(t.TempDir())
	if _, err := sb.Resolve("../escape"); err == nil {
		t.Error(".. must be rejected")
	}
	if _, err := sb.Resolve("a/../../escape"); err == nil {
		t.Error("nested .. must be rejected")
	}
}

func TestSandboxRejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(base, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	sb := NewSandbox(base)
	_, err := sb.Resolve("link/target")
	if err == nil {
		t.Error("symlink escape must be rejected")
	}
}

func TestSandboxAcceptsEmptyPath(t *testing.T) {
	base := t.TempDir()
	sb := NewSandbox(base)
	got, err := sb.Resolve("")
	if err != nil {
		t.Fatalf("empty path should resolve to base, got err %v", err)
	}
	if got != base {
		t.Errorf("got %q want %q", got, base)
	}
}
