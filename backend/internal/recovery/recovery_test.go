package recovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRelativePathRejectsEscapesAndNoncanonicalPaths(t *testing.T) {
	for _, path := range []string{"/bots", "/elsewhere/version", "/bots2/version", "/bots/../elsewhere", "relative", "/bots//version"} {
		if _, err := relativePath("/bots", path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	if rel, err := relativePath("/bots", "/bots/version"); err != nil || rel != "version" {
		t.Fatalf("%q %v", rel, err)
	}
}

func TestStagedPathRejectsSymlinks(t *testing.T) {
	stage := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(stage, "version")); err != nil {
		t.Fatal(err)
	}
	roots := Roots{Original: "/bots", Staged: stage, Final: "/new-bots"}
	if _, _, err := stagedPath(roots, "/bots/version"); err == nil {
		t.Fatal("accepted symlink")
	}
	if err := os.Mkdir(filepath.Join(stage, "real"), 0700); err != nil {
		t.Fatal(err)
	}
	staged, final, err := stagedPath(roots, "/bots/real")
	if err != nil || staged != filepath.Join(stage, "real") || final != "/new-bots/real" {
		t.Fatalf("%q %q %v", staged, final, err)
	}
}
