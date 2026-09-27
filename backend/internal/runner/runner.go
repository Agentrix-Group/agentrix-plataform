package runner

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"agentrix/backend/internal/artifacts"
	"agentrix/backend/internal/config"
	"agentrix/backend/internal/db"
	"agentrix/backend/internal/ladder"
	"agentrix/backend/internal/sandbox"
	"agentrix/backend/internal/validation"
	"strings"
)

type ArbiterPlayerReport struct {
	Name         string  `json:"name"`
	ID           int     `json:"id"`
	Seat         int     `json:"seat"`
	Disqualified bool    `json:"disqualified"`
	Reason       *string `json:"disqualification_reason,omitempty"`
	Kills        int     `json:"kills"`
	SurvivalTime float64 `json:"survival_time"`
	DeathTick    *int    `json:"death_tick"`
	Score        float64 `json:"score"`
}

type ArbiterRankItem struct {
	SurvivalPlace int     `json:"survival_place"`
	LastKillTick  *uint32 `json:"last_kill_tick"`
	SurvivalPart  float64 `json:"survival_part"`
	KillPart      float64 `json:"kill_part"`
	ID            int     `json:"id"`
	Place         int     `json:"place"`
	Score         float64 `json:"score"`
	Kills         int     `json:"kills"`
	Disqualified  bool    `json:"disqualified"`
}

type ArbiterResults struct {
	RulesVersion    string                `json:"rules_version"`
	EffectiveConfig json.RawMessage       `json:"effective_config"`
	Seed            int64                 `json:"seed"`
	ScoreVersion    string                `json:"score_version"`
	EngineVersion   string                `json:"engine_version"`
	WinnerID        *int                  `json:"winner_id"`
	Winner          string                `json:"winner"`
	Ticks           int                   `json:"ticks"`
	Players         []ArbiterPlayerReport `json:"players"`
	Ranking         []ArbiterRankItem     `json:"ranking"`
}

type MatchRunner struct {
	cfg  *config.Config
	pool *db.Pool
}

func NewMatchRunner(cfg *config.Config, pool *db.Pool) *MatchRunner {
	return &MatchRunner{
		cfg:  cfg,
		pool: pool,
	}
}

// ExecuteMatch runs an entire 5-player match from DB record to finish.
func (r *MatchRunner) ExecuteMatch(ctx context.Context, matchID int) (runErr error) {
	// 1. Fetch match and participants
	lease, err := r.claim(ctx, matchID)
	if err != nil {
		return err
	}
	matchID, arenaID, seed, attempt, owner := lease.ID, lease.ArenaID, lease.Seed, lease.Attempt, lease.Owner
	log.Printf("[RUNNER] Starting execution for match #%d", matchID)
	ctx, cancel := context.WithCancel(context.WithValue(ctx, leaseKey{}, owner))
	defer cancel()
	go r.heartbeat(ctx, matchID, owner, cancel)
	defer func() {
		if runErr != nil {
			cleanupCtx, cancel := context.WithTimeout(context.WithValue(context.Background(), leaseKey{}, owner), 10*time.Second)
			defer cancel()
			r.failMatch(cleanupCtx, matchID, runErr.Error())
		}
	}()
	var storedRules, storedHash, storedRuntime *string
	if err := r.pool.QueryRow(ctx, "SELECT rules_json,rules_sha256,runtime_sha256 FROM matches WHERE id=$1", matchID).Scan(&storedRules, &storedHash, &storedRuntime); err != nil {
		return err
	}
	if storedRules == nil || storedHash == nil || storedRuntime == nil {
		return errors.New("match lacks immutable rules/runtime snapshot; reschedule legacy job")
	}
	if *storedRuntime != r.cfg.RuntimeSHA256 {
		return errors.New("scheduled runtime digest differs from this worker; refusing non-reproducible execution")
	}
	rules, err := validateSnapshot([]byte(*storedRules), *storedHash)
	if err != nil {
		return err
	}
	var effective map[string]map[string]interface{}
	_ = json.Unmarshal(rules, &effective)
	maxTicks := int(math.Round(effective["match_rules"]["duration"].(float64) * 60))
	wallSeconds := maxTicks/20 + 120

	// Fetch 5 participants
	rows, err := r.pool.Query(ctx, `
		SELECT mp.seat, av.id, av.name, av.entrypoint, av.artifact_path, av.artifact_sha256,av.sha256,av.runtime,le.display_rating, le.matches_played
		FROM match_participants mp
		JOIN agent_versions av ON mp.agent_version_id = av.id
		LEFT JOIN ladder_entries le ON le.arena_id = $1 AND le.agent_version_id = av.id
		WHERE mp.match_id = $2
		ORDER BY mp.seat ASC
	`, arenaID, matchID)
	if err != nil {
		r.failMatch(ctx, matchID, fmt.Sprintf("failed to fetch participants: %v", err))
		return err
	}
	defer rows.Close()

	type seatInfo struct {
		Seat           int
		AgentVersionID int
		AgentName      string
		Entrypoint     string
		ArtifactPath   string
		ArtifactSHA256 string
		ZipSHA256      string
		Runtime        string
		Rating         int
		MatchesPlayed  int
	}

	var seats []seatInfo
	for rows.Next() {
		var s seatInfo
		var rating, matches *int
		if err := rows.Scan(&s.Seat, &s.AgentVersionID, &s.AgentName, &s.Entrypoint, &s.ArtifactPath, &s.ArtifactSHA256, &s.ZipSHA256, &s.Runtime, &rating, &matches); err != nil {
			r.failMatch(ctx, matchID, fmt.Sprintf("scan participant error: %v", err))
			return err
		}
		if rating != nil {
			s.Rating = *rating
		} else {
			s.Rating = 1500
		}
		if matches != nil {
			s.MatchesPlayed = *matches
		}
		seats = append(seats, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(seats) != 5 {
		err := fmt.Errorf("expected 5 participants, found %d", len(seats))
		r.failMatch(ctx, matchID, err.Error())
		return err
	}

	// 2. Prepare temp working directory
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("agentrix_match_%d_*", matchID))
	if err != nil {
		r.failMatch(ctx, matchID, err.Error())
		return err
	}
	defer os.RemoveAll(tempDir)

	resultsPath := filepath.Join(tempDir, "results.json")
	replayPath := filepath.Join(tempDir, "replay.json")
	rulesPath := filepath.Join(tempDir, "rules.json")
	if err := os.WriteFile(rulesPath, rules, 0600); err != nil {
		return err
	}

	// 3. Resolve exactly the configured arbiter. A cwd-dependent legacy
	// fallback would make the reviewed build differ from the executed build.
	arbiterBin := r.cfg.ArbiterPath
	if !filepath.IsAbs(arbiterBin) {
		cwd, _ := os.Getwd()
		arbiterBin = filepath.Join(cwd, arbiterBin)
	}

	if _, err := os.Stat(arbiterBin); err != nil {
		err := fmt.Errorf("configured arbiter is unavailable: %w", err)
		r.failMatch(ctx, matchID, err.Error())
		return err
	}

	privateArbiter := filepath.Join(tempDir, "arbiter")
	arbiterSHA256, err := artifacts.SnapshotExecutable(arbiterBin, privateArbiter)
	if err != nil {
		return err
	}
	arbiterBin = privateArbiter

	// 4. Construct execution command
	botProvenance := make([]map[string]interface{}, 0, 5)
	for i := range seats {
		if seats[i].Seat != i {
			return errors.New("participant seats must be exactly 0..4")
		}
		zipDigest, err := artifacts.DigestRegular(seats[i].ArtifactPath+".zip", validation.MaxZipSize)
		if err != nil {
			return err
		}
		if zipDigest != seats[i].ZipSHA256 {
			return errors.New("original bot ZIP failed checksum verification")
		}
		manifest, err := validation.InspectBotDirectory(seats[i].ArtifactPath)
		if err != nil {
			return err
		}
		if manifest.Entrypoint != seats[i].Entrypoint || manifest.Runtime != seats[i].Runtime {
			return errors.New("stored entrypoint disagrees with package manifest")
		}
		digest, err := validation.DigestPackage(seats[i].ArtifactPath)
		if err != nil {
			return err
		}
		if len(seats[i].ArtifactSHA256) != 64 || digest != seats[i].ArtifactSHA256 {
			return errors.New("stored bot artifact failed checksum verification")
		}
		botProvenance = append(botProvenance, map[string]interface{}{
			"seat": seats[i].Seat, "agent_version_id": seats[i].AgentVersionID,
			"zip_sha256": zipDigest, "tree_sha256": digest,
			"runtime": seats[i].Runtime, "entrypoint": seats[i].Entrypoint,
		})
		// Legacy host paths are rejected by the sandbox runtime instead of
		// becoming an escape hatch around package validation.
		launchArgs, err := sandbox.CommandWithLifetime(seats[i].ArtifactPath, seats[i].Entrypoint, wallSeconds+20)
		if err != nil {
			r.failMatch(ctx, matchID, err.Error())
			return err
		}
		defer sandbox.Stop(launchArgs)
		quoted := make([]string, len(launchArgs))
		for j, arg := range launchArgs {
			quoted[j] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
		seats[i].Entrypoint = strings.Join(quoted, " ")
	}
	args := []string{
		"--b0", seats[0].Entrypoint,
		"--b1", seats[1].Entrypoint,
		"--b2", seats[2].Entrypoint,
		"--b3", seats[3].Entrypoint,
		"--b4", seats[4].Entrypoint,
		"--seed", fmt.Sprintf("%d", seed),
		"--config", rulesPath,
		"--tick-ms", "50",
		"--warmup-ms", "10000",
		"--out-results", resultsPath,
		"--out-replay", replayPath,
	}

	log.Printf("[RUNNER] Executing arbiter: %s with seed %d", arbiterBin, seed)

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(wallSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, arbiterBin, args...)
	var stdoutBuf, stderrBuf boundedLog
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	execErr := cmd.Run()
	fullLog := fmt.Sprintf("--- STDOUT ---\n%s\n--- STDERR ---\n%s", stdoutBuf.String(), stderrBuf.String())

	if execErr != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("arbiter execution failed: %v", execErr), fullLog)
		return execErr
	}

	// 5. Parse results.json
	resultsData, err := artifacts.ReadBounded(resultsPath, artifacts.MaxResultBytes)
	if err != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("failed to read results.json: %v", err), fullLog)
		return err
	}

	var arbiterResults ArbiterResults
	if err := json.Unmarshal(resultsData, &arbiterResults); err != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("failed to parse results.json: %v", err), fullLog)
		return err
	}

	// 6. Gzip compress replay.json
	if err := os.MkdirAll(r.cfg.ReplaysDir, 0755); err != nil {
		return err
	}

	compressedReplayPath := filepath.Join(r.cfg.ReplaysDir, fmt.Sprintf("match_%d_replay.json.gz", matchID))
	replayBytes, err := artifacts.ReadBounded(replayPath, artifacts.MaxReplayBytes)
	if err != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("failed to read replay.json: %v", err), fullLog)
		return err
	}
	if err := validateResultReplay(arbiterResults, replayBytes, seed, len(seats)); err != nil {
		return err
	}
	actualRules, actualHash, err := canonicalRules(arbiterResults.EffectiveConfig)
	if err != nil || actualHash != *storedHash || !bytes.Equal(actualRules, rules) || arbiterResults.Ticks > maxTicks {
		return errors.New("arbiter result disagrees with scheduled rules snapshot")
	}

	var gzippedBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzippedBuf)
	if _, err := gzWriter.Write(replayBytes); err != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("failed to compress replay: %v", err), fullLog)
		return err
	}
	gzWriter.Close()

	replaySHA := sha256.Sum256(gzippedBuf.Bytes())
	replaySHAHex := hex.EncodeToString(replaySHA[:])
	compressedReplayPath = filepath.Join(r.cfg.ReplaysDir, fmt.Sprintf("match_%d_attempt_%d_%s.json.gz", matchID, attempt, replaySHAHex))
	if err := publishReplay(compressedReplayPath, gzippedBuf.Bytes()); err != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("failed to write compressed replay: %v", err), fullLog)
		return err
	}

	replaySize := int64(gzippedBuf.Len())

	// 7. Calculate ratings
	ratingInputs := make([]ladder.ParticipantRatingInput, 5)
	for i, s := range seats {
		var place int = 5
		var disq bool = false

		for _, rk := range arbiterResults.Ranking {
			if rk.ID == s.Seat {
				place = rk.Place
				disq = rk.Disqualified
				break
			}
		}

		ratingInputs[i] = ladder.ParticipantRatingInput{
			AgentVersionID: s.AgentVersionID,
			OldRating:      float64(s.Rating),
			RankPlace:      place,
			Disqualified:   disq,
			MatchesPlayed:  s.MatchesPlayed,
		}
	}

	ratingResults := ladder.CalculateMultiplayerElo(ratingInputs)

	// 8. DB Transaction: save participants, replay, ladder updates, match finish
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize rating reads and writes per arena. Ratings fetched before the
	// simulation are only hints; another match may have finished meanwhile.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(arenaID)); err != nil {
		return err
	}
	if err := verifyLease(ctx, tx, matchID, owner); err != nil {
		return err
	}
	for i := range seats {
		if err := tx.QueryRow(ctx, "SELECT display_rating, matches_played FROM ladder_entries WHERE arena_id=$1 AND agent_version_id=$2 FOR UPDATE", arenaID, seats[i].AgentVersionID).Scan(&seats[i].Rating, &seats[i].MatchesPlayed); err != nil {
			return err
		}
		ratingInputs[i].OldRating = float64(seats[i].Rating)
		ratingInputs[i].MatchesPlayed = seats[i].MatchesPlayed
	}
	ratingResults = ladder.CalculateMultiplayerElo(ratingInputs)

	var winnerAgentID *int
	for _, rk := range arbiterResults.Ranking {
		if rk.Place == 1 && !rk.Disqualified {
			for _, s := range seats {
				if s.Seat == rk.ID {
					aid := s.AgentVersionID
					winnerAgentID = &aid
					break
				}
			}
			break
		}
	}

	finishTime := time.Now()
	_, err = tx.Exec(ctx, `
		UPDATE matches
		SET status = 'finished',
		    ticks_played = $1,
		    winner_agent_id = $2,
		    execution_log = $3,
		    finished_at = $4, lease_owner=NULL, lease_expires_at=NULL
		WHERE id = $5 AND lease_owner=$6 AND status='running'
	`, arbiterResults.Ticks, winnerAgentID, fullLog, finishTime, matchID, owner)
	if err != nil {
		return err
	}

	// Update participants
	for _, s := range seats {
		var rankPlace int = 5
		var score float64 = 0.0
		var kills int = 0
		var survivalTicks int = 0
		var disqualified bool = false
		var disqReason *string

		for _, rk := range arbiterResults.Ranking {
			if rk.ID == s.Seat {
				rankPlace = rk.Place
				score = rk.Score
				kills = rk.Kills
				disqualified = rk.Disqualified
				break
			}
		}

		for _, p := range arbiterResults.Players {
			if p.ID == s.Seat {
				survivalTicks = arbiterResults.Ticks
				if p.DeathTick != nil {
					survivalTicks = *p.DeathTick
				}
			}
			if p.ID == s.Seat && p.Disqualified {
				disqualified = true
				disqReason = p.Reason
				break
			}
		}

		var oldR, newR, deltaR float64
		for _, rr := range ratingResults {
			if rr.AgentVersionID == s.AgentVersionID {
				oldR = rr.OldRating
				newR = rr.NewRating
				deltaR = rr.RatingDelta
				break
			}
		}

		_, err = tx.Exec(ctx, `
			UPDATE match_participants
			SET rank_place = $1,
			    kills = $2,
			    survival_ticks = $3,
			    score = $4,
			    disqualified = $5,
			    disqualification_reason = $6,
			    old_rating = $7,
			    new_rating = $8,
			    rating_delta = $9
			WHERE match_id = $10 AND seat = $11
		`, rankPlace, kills, survivalTicks, score, disqualified, disqReason, oldR, newR, deltaR, matchID, s.Seat)
		if err != nil {
			return err
		}

		// Update ladder entry
		isWin := 0
		if rankPlace == 1 && !disqualified {
			isWin = 1
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO ladder_entries (arena_id, agent_version_id, rating_mu, display_rating, matches_played, wins, kills, survival_ticks_total, last_match_at, updated_at)
			VALUES ($1, $2, $3, $4, 1, $5, $6, $7, $8, $8)
			ON CONFLICT (arena_id, agent_version_id) DO UPDATE
			SET rating_mu = $3,
			    display_rating = $4,
			    matches_played = ladder_entries.matches_played + 1,
			    wins = ladder_entries.wins + $5,
			    kills = ladder_entries.kills + $6,
			    survival_ticks_total = ladder_entries.survival_ticks_total + $7,
			    last_match_at = $8,
			    updated_at = $8
		`, arenaID, s.AgentVersionID, newR, int(newR), isWin, kills, survivalTicks, finishTime)
		if err != nil {
			return err
		}
	}

	// Insert replay
	summaryMap := map[string]interface{}{
		"bots":             botProvenance,
		"arbiter_sha256":   arbiterSHA256,
		"runtime_sha256":   r.cfg.RuntimeSHA256,
		"rules_sha256":     *storedHash,
		"rules_version":    arbiterResults.RulesVersion,
		"effective_config": arbiterResults.EffectiveConfig,
		"score_version":    arbiterResults.ScoreVersion,
		"engine_version":   arbiterResults.EngineVersion,
		"seed":             arbiterResults.Seed,
		"attempt":          attempt,
		"ticks":            arbiterResults.Ticks,
		"winner":           arbiterResults.Winner,
		"ranking":          arbiterResults.Ranking,
	}
	summaryJSON, _ := json.Marshal(summaryMap)

	_, err = tx.Exec(ctx, `
		INSERT INTO replays (match_id, file_path, sha256, size_bytes, tick_count, summary_json)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (match_id) DO UPDATE
		SET file_path = $2, sha256 = $3, size_bytes = $4, tick_count = $5, summary_json = $6
	`, matchID, compressedReplayPath, replaySHAHex, replaySize, arbiterResults.Ticks, summaryJSON)
	if err != nil {
		return err
	}

	// Audit log
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (action, target_type, target_id, ip_address, details_json)
		VALUES ('MATCH_FINISHED', 'MATCH', $1, '127.0.0.1', $2)
	`, fmt.Sprintf("%d", matchID), summaryJSON)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	log.Printf("[RUNNER] Match #%d successfully executed and recorded! Ticks: %d", matchID, arbiterResults.Ticks)
	return nil
}

// Persist immutable replay bytes before publishing their path in PostgreSQL.
// Unreferenced files after a crash are safe; existing attempts are never replaced.
func publishReplay(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (r *MatchRunner) failMatch(ctx context.Context, matchID int, errMsg string) {
	r.failMatchWithLog(ctx, matchID, errMsg, "")
}

func (r *MatchRunner) failMatchWithLog(ctx context.Context, matchID int, errMsg, fullLog string) {
	log.Printf("[RUNNER] Match #%d failed: %s", matchID, errMsg)
	now := time.Now()
	_, _ = r.pool.Exec(ctx, `
		UPDATE matches
		SET status = 'failed',
		    error_message = $1,
		    execution_log = $2,
		    finished_at = $3
		WHERE id = $4 AND status = 'running' AND lease_owner=$5
	`, errMsg, fullLog, now, matchID, ctx.Value(leaseKey{}))
}

// Drain arbitrarily noisy supervisor output without retaining more than 1 MiB.
type boundedLog struct{ bytes.Buffer }

func (b *boundedLog) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 1024*1024 - b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}

// ScheduleMatch creates a match record with 5 agents in the database and returns match ID.
func (r *MatchRunner) ScheduleMatch(ctx context.Context, arenaID int, agentIDs []int, customSeed *int64) (int, error) {
	return r.scheduleMatch(ctx, arenaID, agentIDs, customSeed, nil, "")
}

func (r *MatchRunner) ScheduleMatchWithSnapshot(ctx context.Context, arenaID int, agentIDs []int, seed int64, rules []byte, hash, runtimeSHA256 string) (int, error) {
	if _, err := validateSnapshot(rules, hash); err != nil {
		return 0, err
	}
	if runtimeSHA256 != r.cfg.RuntimeSHA256 {
		return 0, errors.New("rerun runtime differs from original snapshot")
	}
	return r.scheduleMatch(ctx, arenaID, agentIDs, &seed, rules, hash)
}

func (r *MatchRunner) scheduleMatch(ctx context.Context, arenaID int, agentIDs []int, customSeed *int64, originalRules []byte, originalHash string) (int, error) {
	if len(agentIDs) != 5 {
		return 0, fmt.Errorf("must provide exactly 5 agent IDs, provided %d", len(agentIDs))
	}
	seen := make(map[int]bool)
	for _, id := range agentIDs {
		if id <= 0 || seen[id] {
			return 0, errors.New("participants must be five distinct valid agents")
		}
		seen[id] = true
	}

	var seed int64
	if customSeed != nil {
		if *customSeed < 0 || *customSeed > 4294967295 {
			return 0, errors.New("seed must fit an unsigned 32-bit integer")
		}
		seed = *customSeed
	} else {
		var seedBytes [4]byte
		if _, err := rand.Read(seedBytes[:]); err != nil {
			return 0, fmt.Errorf("generate match seed: %w", err)
		}
		seed = int64(uint32(seedBytes[0])<<24 | uint32(seedBytes[1])<<16 | uint32(seedBytes[2])<<8 | uint32(seedBytes[3]))
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	teams := make(map[int]bool)
	var active bool
	if err := tx.QueryRow(ctx, "SELECT is_active FROM arenas WHERE id=$1 FOR SHARE", arenaID).Scan(&active); err != nil {
		return 0, errors.New("arena not found")
	}
	if !active {
		return 0, errors.New("arena is inactive")
	}
	rules, hash := originalRules, originalHash
	if originalRules == nil {
		rules, hash, err = snapshotArenaRules(ctx, tx, arenaID)
		if err != nil {
			return 0, err
		}
	}
	for _, aid := range agentIDs {
		var teamID int
		if err := tx.QueryRow(ctx, "SELECT team_id FROM agent_versions WHERE id=$1 AND arena_id=$2 AND status='active'", aid, arenaID).Scan(&teamID); err != nil {
			return 0, errors.New("agent must be active in the selected arena")
		}
		if teams[teamID] {
			return 0, errors.New("a team cannot face itself")
		}
		teams[teamID] = true
	}

	var matchID int
	err = tx.QueryRow(ctx, `
		INSERT INTO matches (arena_id, seed, status,rules_json,rules_sha256,runtime_sha256)
		VALUES ($1, $2, 'scheduled',$3,$4,$5)
		RETURNING id
	`, arenaID, seed, string(rules), hash, r.cfg.RuntimeSHA256).Scan(&matchID)
	if err != nil {
		return 0, err
	}

	for seat, aid := range agentIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO match_participants (match_id, agent_version_id, seat, rank_place)
			VALUES ($1, $2, $3, 0)
		`, matchID, aid, seat)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return matchID, nil
}
