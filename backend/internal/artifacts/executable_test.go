package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExecutableSnapshotHashesCopiedBytesAndDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "source"), filepath.Join(root, "snapshot")
	original := []byte("executable fixture")
	if err := os.WriteFile(source, original, 0700); err != nil {
		t.Fatal(err)
	}
	digest, err := SnapshotExecutable(source, destination)
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256(original)
	if digest != hex.EncodeToString(expected[:]) {
		t.Fatal("snapshot digest disagrees")
	}
	if err := os.WriteFile(source, []byte("replacement"), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != string(original) {
		t.Fatal("source mutation changed snapshot")
	}
	if _, err := SnapshotExecutable(source, destination); err == nil {
		t.Fatal("overwrote snapshot")
	}
	if err := os.Chmod(source, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SnapshotExecutable(source, filepath.Join(root, "non-executable")); err == nil {
		t.Fatal("accepted non-executable")
	}
}

func TestELFSnapshotExecutes(t *testing.T) {
	source, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true executable unavailable")
	}
	destination := filepath.Join(t.TempDir(), "trusted-executable")
	if _, err := SnapshotExecutable(source, destination); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(destination).Run(); err != nil {
		t.Fatal(err)
	}
}
