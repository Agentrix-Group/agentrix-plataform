package api

import (
	"agentrix/backend/internal/artifacts"
	"agentrix/backend/internal/auth"
	"agentrix/backend/internal/models"
	"agentrix/backend/internal/runner"
	"agentrix/backend/internal/validation"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealUploadMatchCommitAndFreeze(t *testing.T) {
	if os.Getenv("AGENTRIX_TEST_ACCEPTANCE") != "1" {
		t.Skip("AGENTRIX_TEST_ACCEPTANCE=1 required")
	}
	if os.Getenv("AGENTRIX_TEST_SANDBOX") != "1" || os.Getenv("AGENTRIX_TEST_DATABASE_URL") == "" {
		t.Fatal("acceptance requires real sandbox and PostgreSQL")
	}
	packages := os.Getenv("AGENTRIX_TEST_PACKAGES_DIR")
	bin := os.Getenv("AGENTRIX_TEST_ARBITER_PATH")
	if packages == "" || bin == "" {
		t.Fatal("real packages and arbiter required")
	}
	s, base, user, arena, fixtures := fixtureAPI(t)
	ctx, cancel := context.WithTimeout(base, 90*time.Second)
	defer cancel()
	s.cfg.ArbiterPath = bin
	s.cfg.RuntimeSHA256 = os.Getenv("AGENTRIX_RUNTIME_SHA256")
	if len(s.cfg.RuntimeSHA256) != 64 {
		t.Fatal("pinned runtime digest is required for execution provenance")
	}
	s.cfg.BotsDir = t.TempDir()
	s.cfg.ReplaysDir = t.TempDir()
	if _, err := s.pool.Exec(ctx, `UPDATE arenas SET max_ticks=1200,config_json='{"match_rules":{"zone":false,"walls":0},"mobs":{"count":0}}' WHERE id=$1`, arena); err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateToken(user, "judge", "admin", s.cfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body []byte, admin bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		if admin {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		return w
	}
	var agents []int
	for seat, name := range []string{"heuristic_bot", "cpp_bot", "onnx_bot", "cpp_bot", "heuristic_bot"} {
		var team int
		if err := s.pool.QueryRow(ctx, "SELECT team_id FROM agent_versions WHERE id=$1", fixtures[seat]).Scan(&team); err != nil {
			t.Fatal(err)
		}
		archive, err := os.ReadFile(filepath.Join(packages, name+".zip"))
		if err != nil {
			t.Fatal(err)
		}
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		if err := form.WriteField("team_id", fmt.Sprint(team)); err != nil {
			t.Fatal(err)
		}
		if err := form.WriteField("arena_id", fmt.Sprint(arena)); err != nil {
			t.Fatal(err)
		}
		part, err := form.CreateFormFile("bot_archive", name+".zip")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(archive); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/api/v1/agents/upload", &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 201 {
			t.Fatalf("upload %s: %d %s", name, w.Code, w.Body.String())
		}
		var receipt struct {
			ID int `json:"agent_id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		agents = append(agents, receipt.ID)
		var path, hash, tree, status string
		var version int
		if err := s.pool.QueryRow(ctx, "SELECT artifact_path,sha256,artifact_sha256,status,version FROM agent_versions WHERE id=$1", receipt.ID).Scan(&path, &hash, &tree, &status, &version); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(archive)
		actual, err := validation.DigestPackage(path)
		if err != nil || hash != hex.EncodeToString(sum[:]) || actual != tree || status != "active" || version != 2 {
			t.Fatalf("upload provenance invalid: %v", err)
		}
		stored, err := os.ReadFile(path + ".zip")
		if err != nil || !bytes.Equal(stored, archive) {
			t.Fatal("original ZIP not retained")
		}
	}
	body, _ := json.Marshal(map[string]any{"arena_id": arena, "agent_version_ids": agents, "seed": 42})
	w := request("POST", "/api/v1/matches/trigger", body, true)
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var scheduled struct {
		ID int `json:"match_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &scheduled); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, "UPDATE arenas SET max_ticks=10800,config_json='{}' WHERE id=$1", arena); err != nil {
		t.Fatal(err)
	}
	freezePath := fmt.Sprintf("/api/v1/arenas/%d/freeze", arena)
	if w := request("POST", freezePath, []byte(`{"frozen":true}`), true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if err := s.runner.ExecuteMatch(ctx, scheduled.ID); err != nil {
		t.Fatal(err)
	}
	var provenance struct {
		Arbiter string `json:"arbiter_sha256"`
		Runtime string `json:"runtime_sha256"`
		Bots    []struct {
			Seat       int    `json:"seat"`
			Agent      int    `json:"agent_version_id"`
			Zip        string `json:"zip_sha256"`
			Tree       string `json:"tree_sha256"`
			Runtime    string `json:"runtime"`
			Entrypoint string `json:"entrypoint"`
		} `json:"bots"`
	}
	var summary []byte
	if err := s.pool.QueryRow(ctx, "SELECT summary_json FROM replays WHERE match_id=$1", scheduled.ID).Scan(&summary); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(summary, &provenance); err != nil {
		t.Fatal(err)
	}
	binaryDigest, err := artifacts.DigestRegular(bin, 64<<20)
	if err != nil {
		t.Fatal(err)
	}
	if provenance.Arbiter != binaryDigest || provenance.Runtime != s.cfg.RuntimeSHA256 || len(provenance.Bots) != 5 {
		t.Fatal("incomplete execution provenance")
	}
	for i, bot := range provenance.Bots {
		var zipHash, treeHash, runtime, entrypoint string
		if err := s.pool.QueryRow(ctx, "SELECT sha256,artifact_sha256,runtime,entrypoint FROM agent_versions WHERE id=$1", agents[i]).Scan(&zipHash, &treeHash, &runtime, &entrypoint); err != nil {
			t.Fatal(err)
		}
		if bot.Seat != i || bot.Agent != agents[i] || bot.Zip != zipHash || bot.Tree != treeHash || bot.Runtime != runtime || bot.Entrypoint != entrypoint {
			t.Fatal("bot provenance mismatch")
		}
	}
	matchPath := fmt.Sprintf("/api/v1/matches/%d", scheduled.ID)
	if w := request("GET", matchPath, nil, false); w.Code != 404 {
		t.Fatal("frozen match leaked")
	}
	if w := request("GET", matchPath+"/replay", nil, false); w.Code != 404 {
		t.Fatal("frozen replay leaked")
	}
	replayResponse := request("GET", matchPath+"/replay", nil, true)
	if replayResponse.Code != 200 {
		t.Fatal(replayResponse.Body.String())
	}
	var replay struct {
		Ranking []runner.ArbiterRankItem `json:"ranking"`
		Ticks   int                      `json:"ticks"`
	}
	if err := json.Unmarshal(replayResponse.Body.Bytes(), &replay); err != nil {
		t.Fatal(err)
	}
	if len(replay.Ranking) != 5 {
		t.Fatal("replay must contain all five seats")
	}
	if replay.Ticks != 1200 {
		t.Fatalf("scheduled tick limit ignored: %d", replay.Ticks)
	}
	detailResponse := request("GET", matchPath, nil, true)
	var detail models.Match
	if detailResponse.Code != 200 {
		t.Fatal(detailResponse.Body.String())
	}
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Participants) != 5 || detail.WinnerAgentID == nil || *detail.WinnerAgentID != agents[replay.Ranking[0].ID] || detail.Seed != 42 {
		t.Fatal("API match/winner mismatch")
	}
	byAgent := map[int]models.MatchParticipant{}
	for _, participant := range detail.Participants {
		byAgent[participant.AgentVersionID] = participant
		var expected *runner.ArbiterRankItem
		for i := range replay.Ranking {
			if replay.Ranking[i].ID == participant.Seat {
				expected = &replay.Ranking[i]
			}
		}
		if expected == nil || participant.RankPlace != expected.Place || participant.Score != expected.Score || participant.Kills != expected.Kills || participant.Disqualified {
			t.Fatal("API participant/replay mismatch")
		}
	}
	var status string
	var ticks int
	if err := s.pool.QueryRow(ctx, "SELECT status,ticks_played FROM matches WHERE id=$1", scheduled.ID).Scan(&status, &ticks); err != nil || status != "finished" || ticks != replay.Ticks {
		t.Fatalf("commit mismatch: %s %v", status, err)
	}
	for _, rank := range replay.Ranking {
		var place, kills, played int
		var score float64
		var disqualified bool
		if err := s.pool.QueryRow(ctx, `SELECT mp.rank_place,mp.kills,mp.score,mp.disqualified,le.matches_played FROM match_participants mp JOIN ladder_entries le ON le.agent_version_id=mp.agent_version_id WHERE mp.match_id=$1 AND mp.seat=$2`, scheduled.ID, rank.ID).Scan(&place, &kills, &score, &disqualified, &played); err != nil {
			t.Fatal(err)
		}
		if place != rank.Place || kills != rank.Kills || score != rank.Score || disqualified || played != 1 {
			t.Fatalf("rank/ladder mismatch seat %d", rank.ID)
		}
	}
	ladderPath := fmt.Sprintf("/api/v1/arenas/%d/ladder", arena)
	for _, admin := range []bool{false, true} {
		w := request("GET", ladderPath, nil, admin)
		var entries []models.LadderEntry
		if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil || w.Code != 200 || len(entries) != 5 {
			t.Fatalf("ladder: %s %v", w.Body.String(), err)
		}
		for _, entry := range entries {
			expected := 0
			if admin {
				expected = 1
			}
			if entry.MatchesPlayed != expected {
				t.Fatal("freeze ladder mismatch")
			}
			if admin {
				participant := byAgent[entry.AgentVersionID]
				wins := 0
				if participant.RankPlace == 1 {
					wins = 1
				}
				if entry.Kills != participant.Kills || entry.SurvivalTicksTotal != int64(participant.SurvivalTicks) || entry.Wins != wins || entry.RatingMu != participant.NewRating {
					t.Fatal("ladder aggregates disagree with canonical persisted result")
				}
			}
		}
	}
	if w := request("POST", freezePath, []byte(`{"frozen":false}`), true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	downloaded := request("GET", matchPath+"/download", nil, false)
	if downloaded.Code != 200 {
		t.Fatal(downloaded.Body.String())
	}
	compressed, err := gzip.NewReader(downloaded.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	raw, err := io.ReadAll(compressed)
	if err != nil || !bytes.Equal(raw, replayResponse.Body.Bytes()) {
		t.Fatal("download and API replay differ")
	}
	var archivePath string
	if err := s.pool.QueryRow(ctx, "SELECT artifact_path FROM agent_versions WHERE id=$1", agents[0]).Scan(&archivePath); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(archivePath+".zip", os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write([]byte("corruption"))
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatal("cannot prepare corrupt original")
	}
	w = request("POST", "/api/v1/matches/trigger", body, true)
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &scheduled); err != nil {
		t.Fatal(err)
	}
	if err := s.runner.ExecuteMatch(ctx, scheduled.ID); err == nil || !strings.Contains(err.Error(), "ZIP failed checksum") {
		t.Fatalf("corrupt original accepted: %v", err)
	}
	var replayCount int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM replays WHERE match_id=$1", scheduled.ID).Scan(&replayCount); err != nil || replayCount != 0 {
		t.Fatal("corrupt original published a replay")
	}
	if err := s.pool.QueryRow(ctx, "SELECT status FROM matches WHERE id=$1", scheduled.ID).Scan(&status); err != nil || status != "failed" {
		t.Fatal("corrupt original did not fail the attempt")
	}
	var changed int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM ladder_entries WHERE agent_version_id=ANY($1) AND matches_played<>1", agents).Scan(&changed); err != nil || changed != 0 {
		t.Fatal("corrupt original modified competitive aggregates")
	}
}
