package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"agentrix/backend/internal/validation"
)

func storeOriginalZip(source, dest, expectedSHA string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(in, validation.MaxZipSize+1))
	if copyErr != nil {
		out.Close()
		return copyErr
	}
	if n > validation.MaxZipSize || hex.EncodeToString(hash.Sum(nil)) != expectedSHA {
		out.Close()
		return errors.New("original ZIP failed size/checksum verification")
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(dest))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
