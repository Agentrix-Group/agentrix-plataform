package api

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalZipStorageVerifiesAndPreservesBytes(t *testing.T) {
	data := []byte("original archive bytes")
	sum := sha256.Sum256(data)
	expected := hex.EncodeToString(sum[:])
	source := filepath.Join(t.TempDir(), "input.zip")
	dest := filepath.Join(t.TempDir(), "stored.zip")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := storeOriginalZip(source, dest, expected); err != nil {
		t.Fatal(err)
	}
	if err := storeOriginalZip(source, dest, expected); err == nil {
		t.Fatal("existing archive overwritten")
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(data) {
		t.Fatal("archive bytes changed during storage")
	}
	if err := storeOriginalZip(source, dest+".invalid", "wrong"); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
}
