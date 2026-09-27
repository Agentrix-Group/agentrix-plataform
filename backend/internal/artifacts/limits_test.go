package artifacts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadBoundedBoundaryAndTypes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte("1234"), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadBounded(path, 4); err != nil || string(data) != "1234" {
		t.Fatalf("%q %v", data, err)
	}
	if data, err := ReadBounded(path, 3); err == nil || data != nil {
		t.Fatal("oversize returned partial data")
	}
	if _, err := ReadBounded(filepath.Dir(path), 4); err == nil {
		t.Fatal("accepted directory")
	}
}

func TestSparseOversizeRejectedBeforeReading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Truncate(MaxReplayBytes + 1)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if data, err := ReadBounded(path, MaxReplayBytes); err == nil || data != nil {
		t.Fatal("accepted oversized replay")
	}
}
