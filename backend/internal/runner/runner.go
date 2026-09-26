package runner

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"agentrix/backend/internal/config"
	"agentrix/backend/internal/db"
	"agentrix/backend/internal/ladder"
)

type ArbiterPlayerReport struct {
	ID           int     `json:"id"`
	Seat         int     `json:"seat"`
	Disqualified bool    `json:"disqualified"`
	Reason       *string `json:"disqualification_reason,omitempty"`
	Kills        int     `json:"kills"`
	SurvivalTime float64 `json:"survival_time"`
	Score        float64 `json:"score"`
}

type ArbiterRankItem struct {
	ID            int     `json:"id"`
	Place         int     `json:"place"`
	Score         float64 `json:"score"`
	Kills         int     `json:"kills"`
	Disqualified  bool    `json:"disqualified"`
}

type ArbiterResults struct {
	Winner  string            `json:"winner"`
	Ticks   int               `json:"ticks"`
	Players []ArbiterPlayerReport `json:"players"`
	Ranking []ArbiterRankItem     `json:"ranking"`
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
func (r *MatchRunner) ExecuteMatch(ctx context.Context, matchID int) error {
	log.Printf("[RUNNER] Starting execution for match #%d", matchID)

	// 1. Fetch match and participants
	var arenaID int
	var seed int64
	var status string
	err := r.pool.QueryRow(ctx, "SELECT arena_id, seed, status FROM matches WHERE id = $1", matchID).Scan(&arenaID, &seed, &status)
	if err != nil {
		return fmt.Errorf("failed to fetch match: %w", err)
	}

	if status == "finished" {
		return errors.New("match already finished")
	}

	// Update status to running
	now := time.Now()
	_, _ = r.pool.Exec(ctx, "UPDATE matches SET status = 'running', started_at = $1 WHERE id = $2", now, matchID)

	// Fetch 5 participants
	rows, err := r.pool.Query(ctx, `
		SELECT mp.seat, av.id, av.name, av.entrypoint, av.artifact_path, le.display_rating, le.matches_played
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
		Rating         int
		MatchesPlayed  int
	}

	var seats []seatInfo
	for rows.Next() {
		var s seatInfo
		var rating, matches *int
		if err := rows.Scan(&s.Seat, &s.AgentVersionID, &s.AgentName, &s.Entrypoint, &s.ArtifactPath, &rating, &matches); err != nil {
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

	// 3. Resolve absolute arbiter binary path
	arbiterBin := r.cfg.ArbiterPath
	if !filepath.IsAbs(arbiterBin) {
		cwd, _ := os.Getwd()
		arbiterBin = filepath.Join(cwd, arbiterBin)
	}

	if _, err := os.Stat(arbiterBin); err != nil {
		// Fallback checks
		fb1 := filepath.Join("simulation", "arbiter", "target", "release", "agentrix-arbiter")
		fb2 := filepath.Join("agentrix", "arbiter", "target", "release", "agentrix-arbiter")
		if _, err1 := os.Stat(fb1); err1 == nil {
			arbiterBin, _ = filepath.Abs(fb1)
		} else if _, err2 := os.Stat(fb2); err2 == nil {
			arbiterBin, _ = filepath.Abs(fb2)
		} else {
			r.failMatch(ctx, matchID, fmt.Sprintf("arbiter binary not found at %s (tried %s, %s)", arbiterBin, fb1, fb2))
			return fmt.Errorf("arbiter binary not found: %s", arbiterBin)
		}
	}

	// 4. Construct execution command
	args := []string{
		"--b0", seats[0].Entrypoint,
		"--b1", seats[1].Entrypoint,
		"--b2", seats[2].Entrypoint,
		"--b3", seats[3].Entrypoint,
		"--b4", seats[4].Entrypoint,
		"--seed", fmt.Sprintf("%d", seed),
		"--duration", "180.0",
		"--tick-ms", "50",
		"--warmup-ms", "10000",
		"--out-results", resultsPath,
		"--out-replay", replayPath,
	}

	log.Printf("[RUNNER] Executing arbiter: %s with seed %d", arbiterBin, seed)

	cmdCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, arbiterBin, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	execErr := cmd.Run()
	fullLog := fmt.Sprintf("--- STDOUT ---\n%s\n--- STDERR ---\n%s", stdoutBuf.String(), stderrBuf.String())

	if execErr != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("arbiter execution failed: %v", execErr), fullLog)
		return execErr
	}

	// 5. Parse results.json
	resultsData, err := os.ReadFile(resultsPath)
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
		_ = os.MkdirAll("var/agentrix/replays", 0755)
		r.cfg.ReplaysDir = "var/agentrix/replays"
	}

	compressedReplayPath := filepath.Join(r.cfg.ReplaysDir, fmt.Sprintf("match_%d_replay.json.gz", matchID))
	replayBytes, err := os.ReadFile(replayPath)
	if err != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("failed to read replay.json: %v", err), fullLog)
		return err
	}

	var gzippedBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzippedBuf)
	if _, err := gzWriter.Write(replayBytes); err != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("failed to compress replay: %v", err), fullLog)
		return err
	}
	gzWriter.Close()

	if err := os.WriteFile(compressedReplayPath, gzippedBuf.Bytes(), 0644); err != nil {
		r.failMatchWithLog(ctx, matchID, fmt.Sprintf("failed to write compressed replay: %v", err), fullLog)
		return err
	}

	replaySHA := sha256.Sum256(gzippedBuf.Bytes())
	replaySHAHex := hex.EncodeToString(replaySHA[:])
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
		    finished_at = $4
		WHERE id = $5
	`, arbiterResults.Ticks, winnerAgentID, fullLog, finishTime, matchID)
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
		"ticks":   arbiterResults.Ticks,
		"winner":  arbiterResults.Winner,
		"ranking": arbiterResults.Ranking,
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
		WHERE id = $4
	`, errMsg, fullLog, now, matchID)
}

// ScheduleMatch creates a match record with 5 agents in the database and returns match ID.
func (r *MatchRunner) ScheduleMatch(ctx context.Context, arenaID int, agentIDs []int, customSeed *int64) (int, error) {
	if len(agentIDs) != 5 {
		return 0, fmt.Errorf("must provide exactly 5 agent IDs, provided %d", len(agentIDs))
	}

	var seed int64
	if customSeed != nil {
		seed = *customSeed
	} else {
		seed = rand.Int63n(900000) + 100000
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var matchID int
	err = tx.QueryRow(ctx, `
		INSERT INTO matches (arena_id, seed, status)
		VALUES ($1, $2, 'scheduled')
		RETURNING id
	`, arenaID, seed).Scan(&matchID)
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
