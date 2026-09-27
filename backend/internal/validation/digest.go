package validation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// DigestPackage binds normalized paths, executable bits, sizes and bytes. It
// excludes timestamps/root directory permissions so restore preserves identity.
func DigestPackage(dir string) (string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	hash := sha256.New()
	var total int64
	count := 0
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir {
			return nil
		}
		count++
		if count > MaxFileCount {
			return errors.New("stored package exceeds entry quota")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("stored package contains special file")
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		_ = binary.Write(hash, binary.BigEndian, uint64(len(name)))
		_, _ = io.WriteString(hash, name)
		_ = binary.Write(hash, binary.BigEndian, uint32(info.Mode().Perm()&0111))
		if info.IsDir() {
			_, _ = hash.Write([]byte{'d'})
			return nil
		}
		_, _ = hash.Write([]byte{'f'})
		_ = binary.Write(hash, binary.BigEndian, uint64(info.Size()))
		if info.Size() < 0 || info.Size() > MaxExtractedSize-total {
			return errors.New("stored package exceeds expanded quota")
		}
		f, err := root.Open(rel)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(hash, io.LimitReader(f, info.Size()+1))
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written != info.Size() {
			return errors.New("package changed while hashing")
		}
		total += written
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
