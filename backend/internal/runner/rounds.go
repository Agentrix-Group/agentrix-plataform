package runner

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"agentrix/backend/internal/tournament"
	"github.com/jackc/pgx/v5"
)

var ErrRoundConflict = errors.New("idempotency key already belongs to a different round")

type RoundReceipt struct {
	ID      int    `json:"round_id"`
	Total   int    `json:"total_matches"`
	Version string `json:"format_version"`
}

func lockArena(ctx context.Context, tx pgx.Tx, arenaID int) error {
	var active bool
	if err := tx.QueryRow(ctx, "SELECT is_active FROM arenas WHERE id=$1 FOR UPDATE", arenaID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return errors.New("arena is inactive")
	}
	return nil
}

func (r *MatchRunner) ScheduleRound(ctx context.Context, arenaID int, key string, roster []int, seed *int64) (RoundReceipt, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return RoundReceipt{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockArena(ctx, tx, arenaID); err != nil {
		return RoundReceipt{}, err
	}
	runtimeSHA := ""
	if r.cfg != nil {
		runtimeSHA = r.cfg.RuntimeSHA256
	}
	receipt, err := insertRound(ctx, tx, arenaID, key, roster, seed, runtimeSHA)
	if err != nil {
		return receipt, err
	}
	return receipt, tx.Commit(ctx)
}

func insertRound(ctx context.Context, tx pgx.Tx, arenaID int, key string, roster []int, seed *int64, runtimeSHA256 string) (RoundReceipt, error) {
	if len(key) < 1 || len(key) > 128 {
		return RoundReceipt{}, errors.New("idempotency_key must contain 1..128 bytes")
	}
	if len(roster) < 5 {
		return RoundReceipt{}, errors.New("at least five distinct teams required; smaller rosters wait without points")
	}
	if seed != nil && (*seed < 0 || *seed > 4294967295) {
		return RoundReceipt{}, errors.New("round seed must fit uint32")
	}
	var existing RoundReceipt
	var oldRosterJSON []byte
	var oldSeed int64
	var oldRuntime *string
	err := tx.QueryRow(ctx, "SELECT id,total_matches,format_version,roster_json,seed,runtime_sha256 FROM tournament_rounds WHERE arena_id=$1 AND idempotency_key=$2", arenaID, key).Scan(&existing.ID, &existing.Total, &existing.Version, &oldRosterJSON, &oldSeed, &oldRuntime)
	if err == nil {
		var oldRoster []int
		if err := json.Unmarshal(oldRosterJSON, &oldRoster); err != nil {
			return RoundReceipt{}, err
		}
		if !reflect.DeepEqual(roster, oldRoster) || (seed != nil && *seed != oldSeed) || existing.Version != tournament.Version || oldRuntime == nil || *oldRuntime != runtimeSHA256 {
			return RoundReceipt{}, ErrRoundConflict
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return RoundReceipt{}, err
	}
	var randomSeed [4]byte
	if _, err := rand.Read(randomSeed[:]); err != nil {
		return RoundReceipt{}, fmt.Errorf("generate round seed: %w", err)
	}
	baseSeed := int64(uint32(randomSeed[0])<<24 | uint32(randomSeed[1])<<16 | uint32(randomSeed[2])<<8 | uint32(randomSeed[3]))
	if seed != nil {
		baseSeed = *seed
	}
	plan, err := tournament.New(roster, uint32(baseSeed))
	if err != nil {
		return RoundReceipt{}, err
	}
	teamIDs := map[int]bool{}
	for _, id := range roster {
		var teamID int
		if err := tx.QueryRow(ctx, "SELECT team_id FROM agent_versions WHERE id=$1 AND arena_id=$2 AND status='active'", id, arenaID).Scan(&teamID); err != nil {
			return RoundReceipt{}, errors.New("roster agent must be active in selected arena")
		}
		if teamIDs[teamID] {
			return RoundReceipt{}, errors.New("roster may contain only one version per team")
		}
		teamIDs[teamID] = true
	}
	data, err := json.Marshal(plan.Roster)
	if err != nil {
		return RoundReceipt{}, err
	}
	receipt := RoundReceipt{Total: plan.Total, Version: tournament.Version}
	rules, hash, err := snapshotArenaRules(ctx, tx, arenaID)
	if err != nil {
		return RoundReceipt{}, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO tournament_rounds(arena_id,idempotency_key,format_version,roster_json,seed,total_matches,rules_json,rules_sha256,runtime_sha256)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, arenaID, key, tournament.Version, data, baseSeed, plan.Total, string(rules), hash, runtimeSHA256).Scan(&receipt.ID)
	return receipt, err
}

// ScheduleAutomaticRound snapshots latest admitted versions once. Arena locks
// serialize schedulers across processes; a pending round blocks another cycle.
func (r *MatchRunner) ScheduleAutomaticRound(ctx context.Context, arenaID int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockArena(ctx, tx, arenaID); err != nil {
		return err
	}
	var busy bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tournament_rounds WHERE arena_id=$1 AND status='scheduled')
	 OR EXISTS(SELECT 1 FROM matches WHERE arena_id=$1 AND status IN ('scheduled','running'))`, arenaID).Scan(&busy); err != nil {
		return err
	}
	if busy {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT id FROM (SELECT DISTINCT ON(team_id) id,team_id
	 FROM agent_versions WHERE arena_id=$1 AND status='active' AND length(artifact_sha256)=64
	 ORDER BY team_id,version DESC,id DESC)latest ORDER BY team_id`, arenaID)
	if err != nil {
		return err
	}
	var roster []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		roster = append(roster, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(roster) < 5 {
		return nil
	}
	var last int
	if err := tx.QueryRow(ctx, "SELECT COALESCE(MAX(id),0) FROM tournament_rounds WHERE arena_id=$1", arenaID).Scan(&last); err != nil {
		return err
	}
	runtimeSHA := ""
	if r.cfg != nil {
		runtimeSHA = r.cfg.RuntimeSHA256
	}
	if _, err := insertRound(ctx, tx, arenaID, fmt.Sprintf("automatic-after-%d", last), roster, nil, runtimeSHA); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Keep at most 25 waiting/running matches per arena. Cursor and materialized
// matches commit together, so a scheduler crash cannot skip or duplicate seats.
func (r *MatchRunner) FillRoundQueue(ctx context.Context) error {
	rows, err := r.pool.Query(ctx, "SELECT id FROM tournament_rounds WHERE status='scheduled' ORDER BY id LIMIT 10")
	if err != nil {
		return err
	}
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := r.fillRound(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *MatchRunner) fillRound(ctx context.Context, id int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var arenaID, total, next int
	var seed int64
	var rosterJSON []byte
	var version, status string
	var rules, hash, runtimeSHA256 *string
	err = tx.QueryRow(ctx, `SELECT arena_id,seed,total_matches,next_match,roster_json,format_version,status,rules_json,rules_sha256,runtime_sha256
	 FROM tournament_rounds WHERE id=$1 FOR UPDATE SKIP LOCKED`, id).Scan(&arenaID, &seed, &total, &next, &rosterJSON, &version, &status, &rules, &hash, &runtimeSHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status != "scheduled" {
		return nil
	}
	if rules == nil || hash == nil {
		return errors.New("round lacks explicit rules snapshot")
	}
	if runtimeSHA256 == nil || *runtimeSHA256 != r.cfg.RuntimeSHA256 {
		return errors.New("round runtime differs from this worker; refusing non-reproducible materialization")
	}
	if _, err := validateSnapshot([]byte(*rules), *hash); err != nil {
		return err
	}
	if version != tournament.Version {
		return errors.New("unsupported round format")
	}
	if seed < 0 || seed > 4294967295 || next < 0 || next > total {
		return errors.New("invalid persisted round seed or cursor")
	}
	var roster []int
	if err := json.Unmarshal(rosterJSON, &roster); err != nil {
		return err
	}
	plan, err := tournament.New(roster, uint32(seed))
	if err != nil {
		return err
	}
	if plan.Total != total {
		return errors.New("round total disagrees with immutable plan")
	}
	// Shared advisory lock bounds the queue across independent rounds as well.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(arenaID)); err != nil {
		return err
	}
	var waiting int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM matches WHERE arena_id=$1 AND status IN ('scheduled','running')", arenaID).Scan(&waiting); err != nil {
		return err
	}
	for waiting < 25 && next < total {
		match, err := plan.At(next)
		if err != nil {
			return err
		}
		var matchID int
		if err := tx.QueryRow(ctx, "INSERT INTO matches(arena_id,seed,round_id,round_match_index,rules_json,rules_sha256,runtime_sha256) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id", arenaID, int64(match.Seed), id, next, *rules, *hash, *runtimeSHA256).Scan(&matchID); err != nil {
			return err
		}
		for seat, agent := range match.Agents {
			if _, err := tx.Exec(ctx, "INSERT INTO match_participants(match_id,agent_version_id,seat,rank_place) VALUES($1,$2,$3,0)", matchID, agent, seat); err != nil {
				return err
			}
		}
		next++
		waiting++
	}
	if _, err := tx.Exec(ctx, "UPDATE tournament_rounds SET next_match=$2 WHERE id=$1", id, next); err != nil {
		return err
	}
	if next == total {
		var outstanding, failed int
		if err := tx.QueryRow(ctx, "SELECT count(*) FILTER(WHERE status IN ('scheduled','running')),count(*) FILTER(WHERE status='failed') FROM matches WHERE round_id=$1", id).Scan(&outstanding, &failed); err != nil {
			return err
		}
		if outstanding == 0 {
			state := "finished"
			if failed > 0 {
				state = "failed"
			}
			if _, err := tx.Exec(ctx, "UPDATE tournament_rounds SET status=$2 WHERE id=$1", id, state); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}
