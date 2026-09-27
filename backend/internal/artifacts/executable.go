package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

// SnapshotExecutable binds the executed ELF bytes to the attempt, rather than
// hashing a deployment pathname which could be replaced before exec.
func SnapshotExecutable(source, destination string) (string, error) {
	const limit int64 = 64 << 20
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	before, err := input.Stat()
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm()&0111 == 0 || before.Size() > limit {
		return "", errors.New("invalid arbiter executable type, mode or size")
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0500)
	if err != nil {
		return "", err
	}
	defer output.Close()
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(output, hash), io.LimitReader(input, limit+1))
	if err != nil {
		return "", err
	}
	after, err := input.Stat()
	if err != nil {
		return "", err
	}
	if count != before.Size() || count > limit || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return "", errors.New("arbiter changed while snapshotting")
	}
	if err = output.Sync(); err != nil {
		return "", err
	}
	if err = output.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
