package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplayPublicationCannotOverwriteAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempt.json.gz")
	if err := publishReplay(path, []byte("original")); err != nil {
		t.Fatal(err)
	}
	if err := publishReplay(path, []byte("replacement")); err == nil {
		t.Fatal("immutable replay overwritten")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatal("original replay changed")
	}
}
