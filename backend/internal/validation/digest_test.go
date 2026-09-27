package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageDigestDetectsModifiedBytesAndExecutableBits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.py")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	first, err := DigestPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	again, err := DigestPackage(dir)
	if err != nil || again != first {
		t.Fatal("root permissions changed archive identity")
	}
	if err := os.WriteFile(path, []byte("replaced"), 0644); err != nil {
		t.Fatal(err)
	}
	changed, err := DigestPackage(dir)
	if err != nil || changed == first {
		t.Fatal("tampered bytes not detected")
	}
	if err := os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
	executable, err := DigestPackage(dir)
	if err != nil || executable == changed {
		t.Fatal("changed executable bit not detected")
	}
	if err := os.Symlink(path, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := DigestPackage(dir); err == nil {
		t.Fatal("symlink accepted in stored package")
	}
}
