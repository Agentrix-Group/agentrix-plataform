package sandbox

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRuntimeDigestIsStableAndBindsContentAndSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "usr"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "usr", "python"), []byte("runtime-a"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("python", filepath.Join(root, "usr", "python3")); err != nil {
		t.Fatal(err)
	}
	first, err := RuntimeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RuntimeDigest(root)
	if err != nil || first != second {
		t.Fatalf("runtime digest is not stable: %q %q (%v)", first, second, err)
	}
	pythonPath := filepath.Join(root, "usr", "python")
	if err := os.Chmod(pythonPath, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pythonPath, []byte("runtime-b"), 0644); err != nil {
		t.Fatal(err)
	}
	changed, err := RuntimeDigest(root)
	if err != nil || changed == first {
		t.Fatalf("content change did not change digest: %q (%v)", changed, err)
	}
	if err := os.Remove(filepath.Join(root, "usr", "python3")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("different", filepath.Join(root, "usr", "python3")); err != nil {
		t.Fatal(err)
	}
	linkChanged, err := RuntimeDigest(root)
	if err != nil || linkChanged == changed {
		t.Fatalf("symlink change did not change digest: %q (%v)", linkChanged, err)
	}
}

func TestRuntimeDigestRejectsSpecialFiles(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "control"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RuntimeDigest(root); err == nil {
		t.Fatal("accepted special runtime filesystem entry")
	}
}
