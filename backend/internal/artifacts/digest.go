package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"syscall"
)

// DigestRegular hashes within quota without loading the artifact into memory.
// Linux no-follow/nonblocking flags reject links and avoid blocking on FIFOs.
func DigestRegular(path string, limit int64) (string, error) {
	if limit < 0 || limit == int64(^uint64(0)>>1) {
		return "", errors.New("invalid artifact quota")
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() || before.Size() > limit {
		return "", errors.New("artifact type or size exceeds quota")
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(f, limit+1))
	if err != nil {
		return "", err
	}
	after, err := f.Stat()
	if err != nil {
		return "", err
	}
	if count != before.Size() || count > limit || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return "", errors.New("artifact changed during digest")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
