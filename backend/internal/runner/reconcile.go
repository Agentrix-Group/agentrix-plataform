package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"agentrix/backend/internal/artifacts"
)

var attemptReplayName = regexp.MustCompile(`^match_[1-9][0-9]*_attempt_[1-9][0-9]*_[0-9a-f]{64}\.json\.gz$`)

const (
	replayReconcileGrace     = 2 * leaseSeconds * time.Second
	replayReconcileBatch     = 256
	replayReconcileScanLimit = 4096
)

// ReconcileOrphanReplays deletes only stale, regular files in the runner's
// immutable attempt namespace that PostgreSQL does not reference. The grace
// period exceeds the maximum worker lease, preventing cleanup of an in-flight
// attempt. Other files and symlinks are left untouched.
func (r *MatchRunner) ReconcileOrphanReplays(ctx context.Context, directory string) (int, error) {
	if directory == "" {
		return 0, errors.New("replay directory is required")
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return 0, err
	}
	if _, err := os.Stat(abs); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("inspect replay directory: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil || canonical == "/" {
		return 0, errors.New("replay directory must resolve to a dedicated filesystem path")
	}
	root, err := os.OpenRoot(canonical)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	entries, err := root.Open(".")
	if err != nil {
		return 0, err
	}
	defer entries.Close()
	cutoff := time.Now().Add(-replayReconcileGrace)
	removed, scanned := 0, 0
	for {
		batch, err := entries.ReadDir(replayReconcileBatch)
		for _, entry := range batch {
			scanned++
			if scanned > replayReconcileScanLimit {
				log.Printf("[RUNNER] Replay reconciliation reached its %d-entry scan cap", replayReconcileScanLimit)
				return removed, nil
			}
			name := entry.Name()
			if !attemptReplayName.MatchString(name) {
				continue
			}
			info, err := entry.Info() // Lstat: links and special files are never removed.
			if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > artifacts.MaxReplayBytes || !info.ModTime().Before(cutoff) {
				continue
			}
			absolutePath := filepath.Join(canonical, name)
			configuredPath := filepath.Join(directory, name)
			var referenced bool
			if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM replays WHERE file_path=$1 OR file_path=$2)`, absolutePath, configuredPath).Scan(&referenced); err != nil {
				return removed, fmt.Errorf("check replay reference for %s: %w", name, err)
			}
			if referenced {
				continue
			}
			if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
				return removed, fmt.Errorf("remove orphan replay %s: %w", name, err)
			}
			removed++
			if removed == replayReconcileBatch {
				log.Printf("[RUNNER] Reconciled %d stale unreferenced replay files; remaining candidates will be handled at next start", removed)
				return removed, nil
			}
		}
		if errors.Is(err, io.EOF) {
			return removed, nil
		}
		if err != nil {
			return removed, err
		}
		if len(batch) == 0 {
			return removed, nil
		}
	}
}
