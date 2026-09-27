package validation

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectUnsafeArchives(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths []string
		mode  os.FileMode
	}{
		{"traversal", []string{"../secret"}, 0644},
		{"absolute", []string{"/secret"}, 0644},
		{"duplicate", []string{"agent.py", "agent.py"}, 0644},
		{"noncanonical", []string{"foo/../agent.py"}, 0644},
		{"symlink", []string{"agent.py"}, os.ModeSymlink | 0777},
		{"backslash", []string{"foo\\agent.py"}, 0644},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "bot.zip")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(file)
			for _, name := range tc.paths {
				h := &zip.FileHeader{Name: name, Method: zip.Deflate}
				h.SetMode(tc.mode)
				entry, err := writer.CreateHeader(h)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := entry.Write([]byte("test")); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			file.Close()
			if err := SafelyExtractZip(path, filepath.Join(dir, "out")); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}

func TestEntrypointCannotEscapePackage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent.py"), []byte("pass"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"python3 ../agent.py", "/bin/sh", "sh -c evil", "python3 /etc/passwd", "python3 missing.py"} {
		if err := validateEntrypoint(dir, entry); err == nil {
			t.Fatalf("accepted %q", entry)
		}
	}
	if err := validateEntrypoint(dir, "python3 agent.py"); err != nil {
		t.Fatal(err)
	}
}

func TestProtocolFailsClosedWithoutRuntime(t *testing.T) {
	t.Setenv("AGENTRIX_RUNTIME_ROOT", "")
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/sh\ntouch executed\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := TestBotProtocol(dir, BotManifest{Entrypoint: "./run.sh"}, "missing-arbiter"); err == nil {
		t.Fatal("expected missing sandbox error")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("participant ran on host")
	}
}
