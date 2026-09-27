package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

type leaseKey struct{}

// Longer than the sandbox's maximum 2840-second lifetime: recovery cannot
// overlap with orphaned bots from the preceding attempt.
const leaseSeconds = 3000

type matchLease struct {
	ID, ArenaID, Attempt int
	Seed                 int64
	Owner                string
}

func (r *MatchRunner) claim(ctx context.Context, matchID int) (matchLease, error) {
	lease := matchLease{}
	owner, err := leaseToken()
	if err != nil {
		return lease, err
	}
	lease.Owner = owner
	err = r.pool.QueryRow(ctx, `WITH candidate AS (
		SELECT id FROM matches WHERE status='scheduled' AND ($1=0 OR id=$1)
		ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE matches m SET status='running', started_at=now(), finished_at=NULL,
		lease_owner=$2, heartbeat_at=now(), lease_expires_at=now()+make_interval(secs=>$3),
		attempt=attempt+1 FROM candidate c WHERE m.id=c.id
		RETURNING m.id,m.arena_id,m.seed,m.attempt`, matchID, owner, leaseSeconds).Scan(&lease.ID, &lease.ArenaID, &lease.Seed, &lease.Attempt)
	return lease, err
}

func (r *MatchRunner) recoverExpired(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `UPDATE matches SET status='scheduled', lease_owner=NULL,
		lease_expires_at=NULL, error_message='Previous worker lease expired'
		WHERE status='running' AND lease_expires_at<=now()`)
	return err
}

func verifyLease(ctx context.Context, tx pgx.Tx, id int, owner string) error {
	var owned bool
	if err := tx.QueryRow(ctx, `SELECT COALESCE(lease_owner=$2 AND status='running' AND lease_expires_at>now(),false)
		FROM matches WHERE id=$1 FOR UPDATE`, id, owner).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return errors.New("worker lost ownership before publishing result")
	}
	return nil
}

func leaseToken() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func (r *MatchRunner) heartbeat(ctx context.Context, matchID int, owner string, cancel context.CancelFunc) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := r.pool.Exec(ctx, `UPDATE matches SET heartbeat_at=now(),
				lease_expires_at=now()+make_interval(secs=>$3)
				WHERE id=$1 AND lease_owner=$2 AND status='running' AND lease_expires_at>now()`, matchID, owner, leaseSeconds)
			if err != nil || result.RowsAffected() != 1 {
				cancel()
				return
			}
		}
	}
}

// Bounded queue processing remains enabled when automatic scheduling is off.
func (r *MatchRunner) RunQueue(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := r.recoverExpired(ctx)
			if err != nil {
				log.Printf("[QUEUE] recovery failed: %v", err)
				continue
			}
			if err := r.FillRoundQueue(ctx); err != nil {
				log.Printf("[QUEUE] round materialization failed: %v", err)
				continue
			}
			if err := r.ExecuteMatch(ctx, 0); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				log.Printf("[QUEUE] execution failed: %v", err)
			}
		}
	}
}
