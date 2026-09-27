package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDigestRegularRejectsQuotaLinksAndSpecials(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "original.zip")
	data := []byte("archive")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	digest, err := DigestRegular(path, int64(len(data)))
	if err != nil || digest != hex.EncodeToString(sum[:]) {
		t.Fatalf("%q %v", digest, err)
	}
	if _, err := DigestRegular(path, int64(len(data)-1)); err == nil {
		t.Fatal("oversize accepted")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := DigestRegular(link, 100); err == nil {
		t.Fatal("symlink accepted")
	}
	fifo := filepath.Join(root, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := DigestRegular(fifo, 100); err == nil {
		t.Fatal("FIFO accepted")
	}
}
