// Package recovery verifies restored artifacts using the production validators
// before changing paths in a newly restored, offline database.
package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agentrix/backend/internal/validation"
	"github.com/jackc/pgx/v5"
)

type Roots struct{ Original, Staged, Final string }

func relativePath(root, path string) (string, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) || filepath.Clean(root) != root || filepath.Clean(path) != path || root == "/" {
		return "", errors.New("artifact path must be absolute and canonical")
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", errors.New("artifact outside original root")
	}
	return rel, nil
}

func stagedPath(roots Roots, original string) (string, string, error) {
	rel, err := relativePath(roots.Original, original)
	if err != nil {
		return "", "", err
	}
	staged := filepath.Join(roots.Staged, rel)
	real, err := filepath.EvalSymlinks(staged)
	if err != nil {
		return "", "", err
	}
	if real != staged {
		return "", "", errors.New("restored artifact contains a symlink")
	}
	return staged, filepath.Join(roots.Final, rel), nil
}

func fileHash(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	if !st.Mode().IsRegular() {
		return "", 0, errors.New("artifact must be regular")
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	if n != st.Size() {
		return "", 0, errors.New("artifact changed during verification")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// VerifyAndRemap never executes submitted code. The caller must keep the new DB
// and staging directories inaccessible to API/workers until publication succeeds.
func VerifyAndRemap(ctx context.Context, conn *pgx.Conn, bots, replays Roots) error {
	for _, roots := range []Roots{bots, replays} {
		for _, root := range []string{roots.Original, roots.Staged, roots.Final} {
			if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" {
				return errors.New("invalid recovery root")
			}
		}
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var running int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM matches WHERE status='running'").Scan(&running); err != nil {
		return err
	}
	if running != 0 {
		return errors.New("offline backup contains running matches; reconcile before recovery")
	}
	rows, err := tx.Query(ctx, "SELECT id,artifact_path,sha256,artifact_sha256,runtime,entrypoint,name FROM agent_versions ORDER BY id")
	if err != nil {
		return err
	}
	type agent struct {
		id                                                 int
		path, zipHash, treeHash, runtime, entrypoint, name string
	}
	var agents []agent
	for rows.Next() {
		var a agent
		if err = rows.Scan(&a.id, &a.path, &a.zipHash, &a.treeHash, &a.runtime, &a.entrypoint, &a.name); err != nil {
			rows.Close()
			return err
		}
		agents = append(agents, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range agents {
		staged, final, err := stagedPath(bots, a.path)
		if err != nil {
			return fmt.Errorf("agent %d: %w", a.id, err)
		}
		digest, size, err := fileHash(staged + ".zip")
		if err != nil {
			return err
		}
		if digest != a.zipHash || size > validation.MaxZipSize {
			return fmt.Errorf("agent %d original ZIP mismatch", a.id)
		}
		digest, err = validation.DigestPackage(staged)
		if err != nil {
			return err
		}
		if digest != a.treeHash {
			return fmt.Errorf("agent %d tree mismatch", a.id)
		}
		manifest, err := validation.InspectBotDirectory(staged)
		if err != nil {
			return err
		}
		if manifest.Runtime != a.runtime || manifest.Entrypoint != a.entrypoint {
			return fmt.Errorf("agent %d manifest mismatch", a.id)
		}
		temp, err := os.MkdirTemp("", "agentrix-recovery-zip-")
		if err != nil {
			return err
		}
		err = validation.SafelyExtractZip(staged+".zip", temp)
		if err == nil {
			var originalDigest string
			originalDigest, err = validation.DigestPackage(temp)
			if err == nil && originalDigest != a.treeHash {
				err = errors.New("original ZIP disagrees with stored tree")
			}
		}
		os.RemoveAll(temp) // Exact directory created above, never a configured root.
		if err != nil {
			return fmt.Errorf("agent %d: %w", a.id, err)
		}
		if _, err = tx.Exec(ctx, "UPDATE agent_versions SET artifact_path=$1 WHERE id=$2", final, a.id); err != nil {
			return err
		}
	}
	rows, err = tx.Query(ctx, "SELECT id,file_path,sha256,size_bytes FROM replays ORDER BY id")
	if err != nil {
		return err
	}
	type replay struct {
		id           int
		path, digest string
		size         int64
	}
	var records []replay
	for rows.Next() {
		var r replay
		if err = rows.Scan(&r.id, &r.path, &r.digest, &r.size); err != nil {
			rows.Close()
			return err
		}
		records = append(records, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range records {
		staged, final, err := stagedPath(replays, r.path)
		if err != nil {
			return err
		}
		digest, size, err := fileHash(staged)
		if err != nil {
			return err
		}
		if digest != r.digest || size != r.size {
			return fmt.Errorf("replay %d mismatch", r.id)
		}
		if _, err = tx.Exec(ctx, "UPDATE replays SET file_path=$1 WHERE id=$2", final, r.id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
