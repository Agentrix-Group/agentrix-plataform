package sandbox

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

const maxRuntimeEntries = 200_000
const maxRuntimeBytes int64 = 16 << 30

// RuntimeDigest binds the executable sandbox filesystem to a deterministic
// digest. Symlinks are recorded as links (never traversed); special files are
// rejected. Paths, executable bits, object type, link target and file bytes all
// contribute to the digest.
func RuntimeDigest(rootPath string) (string, error) {
	rootPath, err := filepath.EvalSymlinks(rootPath)
	if err != nil || rootPath == "/" {
		return "", errors.New("runtime root is unavailable or unsafe")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return "", err
	}
	defer root.Close()

	hash := sha256.New()
	entries := 0
	var totalBytes int64
	err = filepath.WalkDir(rootPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == rootPath {
			return nil
		}
		entries++
		if entries > maxRuntimeEntries {
			return errors.New("runtime filesystem exceeds entry limit")
		}
		rel, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if uint64(len(name)) > uint64(^uint32(0)) {
			return errors.New("runtime path is too long")
		}
		_ = binary.Write(hash, binary.BigEndian, uint32(len(name)))
		_, _ = io.WriteString(hash, name)
		_ = binary.Write(hash, binary.BigEndian, uint32(info.Mode().Perm()))
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("runtime filesystem metadata is unsupported")
		}
		_ = binary.Write(hash, binary.BigEndian, uint64(stat.Uid))
		_ = binary.Write(hash, binary.BigEndian, uint64(stat.Gid))
		switch {
		case info.IsDir():
			_, _ = hash.Write([]byte{'d'})
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_, _ = hash.Write([]byte{'l'})
			_ = binary.Write(hash, binary.BigEndian, uint32(len(target)))
			_, _ = io.WriteString(hash, target)
		case info.Mode().IsRegular():
			_, _ = hash.Write([]byte{'f'})
			if info.Size() < 0 || info.Size() > maxRuntimeBytes-totalBytes {
				return errors.New("runtime filesystem exceeds byte limit")
			}
			_ = binary.Write(hash, binary.BigEndian, uint64(info.Size()))
			file, err := root.Open(rel)
			if err != nil {
				return err
			}
			written, copyErr := io.Copy(hash, io.LimitReader(file, info.Size()+1))
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if written != info.Size() {
				return errors.New("runtime changed while hashing")
			}
			totalBytes += written
		default:
			return errors.New("runtime contains a special file")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
