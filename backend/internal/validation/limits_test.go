package validation

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

// Store makes the boundary fixture incompressible by policy without allocating
// 100 MiB of test memory. Expanded payload plus ZIP framing equals exactly size.
func writeBoundaryZip(t *testing.T, path string, size int64) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(file)
	entry, err := w.CreateHeader(&zip.FileHeader{Name: "model.bin", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(entry, zeros{}, size-132); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	file.Close()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// ZIP overhead depends only on this fixed header; ensure fixture measures it.
	if st.Size() != size {
		t.Fatalf("boundary fixture size %d, expected %d", st.Size(), size)
	}
}

func TestZipSizeBoundariesAndCRC(t *testing.T) {
	for _, size := range []int64{1024 * 1024, MaxZipSize} {
		t.Run(fmt.Sprintf("%dMiB", size/(1024*1024)), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "model.zip")
			writeBoundaryZip(t, path, size)
			if err := SafelyExtractZip(path, filepath.Join(dir, "out")); err != nil {
				t.Fatal(err)
			}
			st, err := os.Stat(filepath.Join(dir, "out", "model.bin"))
			if err != nil {
				t.Fatal(err)
			}
			if st.Size() != size-132 {
				t.Fatal("expanded payload truncated")
			}
		})
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "over.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxZipSize + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := SafelyExtractZip(path, filepath.Join(dir, "out")); err == nil {
		t.Fatal("oversized ZIP accepted")
	}
	corrupt := filepath.Join(dir, "corrupt.zip")
	writeBoundaryZip(t, corrupt, 1024*1024)
	file, err := os.OpenFile(corrupt, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{1}, 39); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if err := SafelyExtractZip(corrupt, filepath.Join(dir, "corrupt-out")); err == nil {
		t.Fatal("corrupt CRC accepted")
	}
}

func TestExpansionAndEntryQuotasPreflight(t *testing.T) {
	for _, files := range [][]*zip.File{
		{{FileHeader: zip.FileHeader{Name: "model", UncompressedSize64: MaxExtractedSize + 1, CompressedSize64: MaxZipSize}}},
		{{FileHeader: zip.FileHeader{Name: "a", UncompressedSize64: MaxExtractedSize, CompressedSize64: MaxZipSize}}, {FileHeader: zip.FileHeader{Name: "b", UncompressedSize64: 1, CompressedSize64: 1}}},
		{{FileHeader: zip.FileHeader{Name: "bomb", UncompressedSize64: 10 * 1024 * 1024, CompressedSize64: 1}}},
		make([]*zip.File, MaxFileCount+1),
		{{FileHeader: zip.FileHeader{Name: strings.Repeat("x/", 500) + "file"}}, {FileHeader: zip.FileHeader{Name: strings.Repeat("y/", 500) + "file"}}},
	} {
		if err := validateArchive(files); err == nil {
			t.Fatal("quota bypass accepted")
		}
	}
}

func TestManifestContract(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent.py"), []byte("pass"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectBotDirectory(dir); err == nil {
		t.Fatal("manifestless package accepted")
	}
	for _, content := range []string{
		`{"name":"x","runtime":"python-standard","entrypoint":"python3 agent.py"}`,
		`{"version":1,"protocol_version":1,"name":"x","runtime":"unknown","entrypoint":"python3 agent.py"}`,
		`{"version":1,"protocol_version":1,"name":"x","runtime":"python-standard","entrypoint":"python3 ../secret"}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, "agentrix.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectBotDirectory(dir); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
	valid := `{"version":1,"protocol_version":1,"name":"x","runtime":"python-standard","entrypoint":"python3 agent.py"}`
	if err := os.WriteFile(filepath.Join(dir, "agentrix.json"), []byte(valid), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectBotDirectory(dir); err != nil {
		t.Fatal(err)
	}
}
