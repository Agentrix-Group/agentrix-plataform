package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"agentrix/backend/internal/artifacts"
	"agentrix/backend/internal/auth"
	"agentrix/backend/internal/config"
	"agentrix/backend/internal/db"
	"agentrix/backend/internal/matchmaker"
	"agentrix/backend/internal/models"
	"agentrix/backend/internal/runner"
	"agentrix/backend/internal/validation"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5"
)

type Server struct {
	cfg         *config.Config
	pool        *db.Pool
	runner      *runner.MatchRunner
	matchmaker  *matchmaker.Matchmaker
	router      chi.Router
	uploadSlots chan struct{}
	replaySlots chan struct{}
}

func NewServer(cfg *config.Config, pool *db.Pool, r *runner.MatchRunner, m *matchmaker.Matchmaker) *Server {
	s := &Server{
		cfg:         cfg,
		pool:        pool,
		runner:      r,
		matchmaker:  m,
		router:      chi.NewRouter(),
		uploadSlots: make(chan struct{}, 2),
		replaySlots: make(chan struct{}, 2),
	}
	s.setupRoutes()
	return s
}

func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) setupRoutes() {
	r := s.router

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			next.ServeHTTP(w, r)
		})
	})
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(auth.Middleware(s.cfg.JWTSecret))

	r.Route("/api/v1", func(r chi.Router) {
		// Health & System
		r.Get("/system/status", s.handleSystemStatus)

		// Auth
		r.Post("/auth/login", s.handleLogin)
		r.Get("/auth/me", s.handleMe)

		// Arenas
		r.Get("/arenas", s.handleListArenas)
		r.Get("/arenas/{id}", s.handleGetArena)
		r.Get("/arenas/{id}/kit", s.handleGetArenaKit)
		r.Get("/arenas/{id}/kit/download", s.handleDownloadArenaKit)
		r.Head("/arenas/{id}/kit/download", s.handleDownloadArenaKit)
		r.Post("/arenas/{id}/freeze", auth.RequireAdmin(s.handleFreezeArena))
		r.Post("/arenas/{id}/phase", auth.RequireAdmin(s.handleSetArenaPhase))
		r.Post("/arenas/{id}/rounds", auth.RequireAdmin(s.handleScheduleRound))

		// Ladder / Standings
		r.Get("/arenas/{id}/ladder", s.handleGetLadder)

		// Matches
		r.Get("/matches", s.handleListMatches)
		r.Get("/matches/{id}", s.handleGetMatch)
		r.Get("/matches/{id}/replay", s.handleGetMatchReplay)
		r.Get("/matches/{id}/download", s.handleDownloadMatchReplay)
		r.Post("/matches/trigger", auth.RequireAdmin(s.handleTriggerMatch))
		r.Post("/matches/{id}/rerun", auth.RequireAdmin(s.handleRerunMatch))

		// Agents / Bots
		r.Get("/agents", s.handleListAgents)
		r.Get("/agents/{id}", s.handleGetAgent)
		r.Post("/agents/upload", auth.RequireAuth(s.handleUploadAgent))
		r.Post("/agents/{id}/disqualify", auth.RequireAdmin(s.handleDisqualifyAgent))
		r.Post("/agents/{id}/enable", auth.RequireAdmin(s.handleEnableAgent))

		// Teams & Users
		r.Get("/teams", s.handleListTeams)
		r.Get("/users", auth.RequireAdmin(s.handleListUsers))

		// Audit Logs
		r.Get("/audit-logs", auth.RequireAdmin(s.handleListAuditLogs))
	})
}

// -----------------------------------------------------------------------------
// Handlers: System & Auth
// -----------------------------------------------------------------------------

func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	dbErr := s.pool.Ping(r.Context())
	_, arbiterErr := os.Stat(s.cfg.ArbiterPath)

	var runningMatches int
	_ = s.pool.QueryRow(r.Context(), "SELECT count(*) FROM matches m WHERE m.status = 'running' AND "+matchVisibility("$1"), isAdmin(r)).Scan(&runningMatches)

	resp := models.SystemStatusResponse{
		Status:           "operational",
		DatabaseHealthy:  dbErr == nil,
		ArbiterHealthy:   arbiterErr == nil,
		ArbiterPath:      s.cfg.ArbiterPath,
		MatchmakerActive: s.matchmaker.IsActive(),
		RunningMatches:   runningMatches,
		ServerTime:       time.Now().UTC(),
	}
	if dbErr != nil || arbiterErr != nil {
		resp.Status = "degraded"
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var user models.User
	err := s.pool.QueryRow(r.Context(), `
		SELECT id, username, email, password_hash, role, created_at, updated_at
		FROM users
		WHERE username = $1
	`, req.Username).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.Role, s.cfg.JWTSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, models.LoginResponse{
		Token: token,
		User:  user,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var user models.User
	err := s.pool.QueryRow(r.Context(), `
		SELECT id, username, email, role, created_at, updated_at
		FROM users WHERE id = $1
	`, u.ID).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// -----------------------------------------------------------------------------
// Handlers: Arenas & Ladder
// -----------------------------------------------------------------------------

func (s *Server) handleListArenas(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, slug, name, description, game_type, max_players, max_ticks, config_json, is_active, created_at,
		 EXISTS(SELECT 1 FROM arena_freezes f WHERE f.arena_id=arenas.id),
		 COALESCE(phase, 'warmup')
		FROM arenas
		ORDER BY id ASC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var arenas []models.Arena
	for rows.Next() {
		var a models.Arena
		var cfgBytes []byte
		if err := rows.Scan(&a.ID, &a.Slug, &a.Name, &a.Description, &a.GameType, &a.MaxPlayers, &a.MaxTicks, &cfgBytes, &a.IsActive, &a.CreatedAt, &a.Frozen, &a.Phase); err == nil {
			_ = json.Unmarshal(cfgBytes, &a.ConfigJSON)
			arenas = append(arenas, a)
		}
	}

	writeJSON(w, http.StatusOK, arenas)
}

func (s *Server) handleGetArena(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var a models.Arena
	var cfgBytes []byte
	err := s.pool.QueryRow(r.Context(), `
		SELECT id, slug, name, description, game_type, max_players, max_ticks, config_json, is_active, created_at,
		 EXISTS(SELECT 1 FROM arena_freezes f WHERE f.arena_id=arenas.id),
		 COALESCE(phase, 'warmup')
		FROM arenas WHERE id = $1
	`, id).Scan(&a.ID, &a.Slug, &a.Name, &a.Description, &a.GameType, &a.MaxPlayers, &a.MaxTicks, &cfgBytes, &a.IsActive, &a.CreatedAt, &a.Frozen, &a.Phase)
	if err != nil {
		writeError(w, http.StatusNotFound, "arena not found")
		return
	}
	_ = json.Unmarshal(cfgBytes, &a.ConfigJSON)

	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleGetLadder(w http.ResponseWriter, r *http.Request) {
	arenaIDStr := chi.URLParam(r, "id")
	arenaID, _ := strconv.Atoi(arenaIDStr)
	var data []byte
	var frozen bool
	err := s.pool.QueryRow(r.Context(), `SELECT CASE WHEN $2 OR f.arena_id IS NULL
	 THEN (`+ladderSnapshot+`) ELSE f.ladder_json END,
	 (f.arena_id IS NOT NULL AND NOT $2)
	 FROM arenas a LEFT JOIN arena_freezes f ON f.arena_id=a.id WHERE a.id=$1`, arenaID, isAdmin(r)).Scan(&data, &frozen)
	if err == pgx.ErrNoRows {
		writeError(w, 404, "arena not found")
		return
	}
	if err != nil {
		writeError(w, 500, "ladder query failed")
		return
	}
	w.Header().Set("X-Agentrix-Frozen", strconv.FormatBool(frozen))
	writeJSON(w, 200, json.RawMessage(data))
}

// -----------------------------------------------------------------------------
// Handlers: Matches & Replays
// -----------------------------------------------------------------------------

func (s *Server) handleListMatches(w http.ResponseWriter, r *http.Request) {
	arenaID, beforeID := 0, 0
	for name, target := range map[string]*int{"arena_id": &arenaID, "before_id": &beforeID} {
		if raw := r.URL.Query().Get(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				writeError(w, http.StatusBadRequest, "invalid "+name)
				return
			}
			*target = n
		}
	}
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 100 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = l
	}

	query := `
		SELECT m.id, m.arena_id, a.name, m.seed, m.status, m.ticks_played,
		       m.winner_agent_id, av.name, m.error_message, m.started_at, m.finished_at, m.created_at
		FROM matches m
		JOIN arenas a ON m.arena_id = a.id
		LEFT JOIN agent_versions av ON m.winner_agent_id = av.id
		WHERE ($1::integer = 0 OR m.arena_id = $1)
		  AND ($5::integer = 0 OR m.id < $5)
		  AND ($2 = '' OR m.status = $2) AND ` + matchVisibility("$4") + `
		ORDER BY m.id DESC
		LIMIT $3
	`
	rows, err := s.pool.Query(r.Context(), query, arenaID, status, limit, isAdmin(r), beforeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	matches := make([]models.Match, 0, limit)
	for rows.Next() {
		var m models.Match
		var winnerName *string
		if err := rows.Scan(
			&m.ID, &m.ArenaID, &m.ArenaName, &m.Seed, &m.Status, &m.TicksPlayed,
			&m.WinnerAgentID, &winnerName, &m.ErrorMessage, &m.StartedAt, &m.FinishedAt, &m.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if winnerName != nil {
			m.WinnerName = *winnerName
		}
		m.ReplayURL = fmt.Sprintf("/api/v1/matches/%d/replay", m.ID)
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows.Close()

	// One bounded participant query for the entire page, rather than N+1 queries.
	ids := make([]int, len(matches))
	indexes := make(map[int]int, len(matches))
	for i := range matches {
		ids[i], indexes[matches[i].ID] = matches[i].ID, i
	}
	if len(ids) > 0 {
		pRows, err := s.pool.Query(r.Context(), `
			SELECT mp.id, mp.match_id, mp.agent_version_id, av.name, t.name,
			       mp.seat, mp.rank_place, mp.kills, mp.survival_ticks, mp.score,
			       mp.disqualified, mp.disqualification_reason, mp.old_rating, mp.new_rating, mp.rating_delta
			FROM match_participants mp
			JOIN agent_versions av ON mp.agent_version_id = av.id
			JOIN teams t ON av.team_id = t.id
			WHERE mp.match_id = ANY($1::integer[])
			ORDER BY mp.match_id, mp.rank_place ASC, mp.seat ASC
		`, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer pRows.Close()
		for pRows.Next() {
			var p models.MatchParticipant
			if err := pRows.Scan(
				&p.ID, &p.MatchID, &p.AgentVersionID, &p.AgentName, &p.TeamName,
				&p.Seat, &p.RankPlace, &p.Kills, &p.SurvivalTicks, &p.Score,
				&p.Disqualified, &p.DisqualificationReason, &p.OldRating, &p.NewRating, &p.RatingDelta,
			); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			i := indexes[p.MatchID]
			matches[i].Participants = append(matches[i].Participants, p)
		}
		if err := pRows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, matches)
}

func (s *Server) handleGetMatch(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var m models.Match
	var winnerName *string
	err := s.pool.QueryRow(r.Context(), `
		SELECT m.id, m.arena_id, a.name, m.seed, m.status, m.ticks_played,
		       m.winner_agent_id, av.name, m.error_message, m.execution_log,
		       m.started_at, m.finished_at, m.created_at
		FROM matches m
		JOIN arenas a ON m.arena_id = a.id
		LEFT JOIN agent_versions av ON m.winner_agent_id = av.id
		WHERE m.id = $1 AND `+matchVisibility("$2")+`
	`, id, isAdmin(r)).Scan(
		&m.ID, &m.ArenaID, &m.ArenaName, &m.Seed, &m.Status, &m.TicksPlayed,
		&m.WinnerAgentID, &winnerName, &m.ErrorMessage, &m.ExecutionLog,
		&m.StartedAt, &m.FinishedAt, &m.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "match not found")
		return
	}
	if winnerName != nil {
		m.WinnerName = *winnerName
	}
	if !isAdmin(r) {
		m.ExecutionLog = nil
	}
	m.ReplayURL = fmt.Sprintf("/api/v1/matches/%d/replay", m.ID)

	// Fetch participants
	rows, err := s.pool.Query(r.Context(), `
		SELECT mp.id, mp.match_id, mp.agent_version_id, av.name, t.name,
		       mp.seat, mp.rank_place, mp.kills, mp.survival_ticks, mp.score,
		       mp.disqualified, mp.disqualification_reason, mp.old_rating, mp.new_rating, mp.rating_delta
		FROM match_participants mp
		JOIN agent_versions av ON mp.agent_version_id = av.id
		JOIN teams t ON av.team_id = t.id
		WHERE mp.match_id = $1
		ORDER BY mp.rank_place ASC, mp.seat ASC
	`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p models.MatchParticipant
			if err := rows.Scan(
				&p.ID, &p.MatchID, &p.AgentVersionID, &p.AgentName, &p.TeamName,
				&p.Seat, &p.RankPlace, &p.Kills, &p.SurvivalTicks, &p.Score,
				&p.Disqualified, &p.DisqualificationReason, &p.OldRating, &p.NewRating, &p.RatingDelta,
			); err == nil {
				m.Participants = append(m.Participants, p)
			}
		}
	}

	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleGetMatchReplay(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var filePath string
	err := s.pool.QueryRow(r.Context(), "SELECT rp.file_path FROM replays rp JOIN matches m ON rp.match_id=m.id WHERE m.id=$1 AND "+matchVisibility("$2"), id, isAdmin(r)).Scan(&filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "replay not found for this match")
		return
	}

	filePath, err = confinedReplayPath(s.cfg.ReplaysDir, filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "replay file unavailable")
		return
	}
	select {
	case s.replaySlots <- struct{}{}:
		defer func() { <-s.replaySlots }()
	default:
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusTooManyRequests, "replay capacity busy; retry shortly")
		return
	}
	f, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "replay file not found on storage")
		return
	}
	defer f.Close()

	stream, closeStream, err := replayStream(r.Context(), f, artifacts.MaxReplayBytes)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		writeError(w, http.StatusInternalServerError, "replay corrupt or exceeds quota")
		return
	}
	defer closeStream()
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, stream)
}

func (s *Server) handleDownloadMatchReplay(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var filePath string
	err := s.pool.QueryRow(r.Context(), "SELECT rp.file_path FROM replays rp JOIN matches m ON rp.match_id=m.id WHERE m.id=$1 AND "+matchVisibility("$2"), id, isAdmin(r)).Scan(&filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "replay not found")
		return
	}

	filePath, err = confinedReplayPath(s.cfg.ReplaysDir, filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "replay file unavailable")
		return
	}
	f, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "replay unavailable")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > artifacts.MaxReplayBytes+(1<<20) {
		writeError(w, http.StatusInternalServerError, "stored replay exceeds quota")
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"agentrix_match_%d_replay.json.gz\"", id))
	http.ServeContent(w, r, filepath.Base(filePath), info.ModTime(), f)
}

func (s *Server) handleTriggerMatch(w http.ResponseWriter, r *http.Request) {
	var req models.TriggerMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ArenaID == 0 {
		req.ArenaID = 1 // default arena
	}

	var selectedIDs = req.AgentVersionIDs
	if len(selectedIDs) < 5 {
		// Auto-fill from active agents in arena
		rows, err := s.pool.Query(r.Context(), `
			SELECT id FROM agent_versions
			WHERE arena_id = $1 AND status = 'active'
			ORDER BY RANDOM()
			LIMIT 5
		`, req.ArenaID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to query agents")
			return
		}
		defer rows.Close()

		selectedIDs = nil
		for rows.Next() {
			var aid int
			if err := rows.Scan(&aid); err == nil {
				selectedIDs = append(selectedIDs, aid)
			}
		}
		if len(selectedIDs) < 5 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("not enough active agents in arena (need 5, found %d)", len(selectedIDs)))
			return
		}
	}

	matchID, err := s.runner.ScheduleMatch(r.Context(), req.ArenaID, selectedIDs, req.Seed)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// A bounded queue worker claims this durable scheduled record.

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"message":  "match scheduled successfully",
		"match_id": matchID,
	})
}

func (s *Server) handleRerunMatch(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var arenaID int
	var seed int64
	var rules, hash, runtimeSHA256 *string
	var status string
	err := s.pool.QueryRow(r.Context(), "SELECT arena_id, seed,rules_json,rules_sha256,runtime_sha256,status FROM matches WHERE id = $1", id).Scan(&arenaID, &seed, &rules, &hash, &runtimeSHA256, &status)
	if err != nil {
		writeError(w, http.StatusNotFound, "match not found")
		return
	}
	if status != "finished" && status != "failed" {
		writeError(w, 409, "only terminal matches can be rerun")
		return
	}
	if rules == nil || hash == nil || runtimeSHA256 == nil {
		writeError(w, 409, "legacy match lacks immutable rules/runtime snapshot; schedule a new match explicitly")
		return
	}
	if *runtimeSHA256 != s.cfg.RuntimeSHA256 {
		writeError(w, 409, "current worker runtime differs from original match snapshot")
		return
	}

	rows, err := s.pool.Query(r.Context(), `
		SELECT agent_version_id FROM match_participants
		WHERE match_id = $1
		ORDER BY seat ASC
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var agentIDs []int
	for rows.Next() {
		var aid int
		if err := rows.Scan(&aid); err == nil {
			agentIDs = append(agentIDs, aid)
		}
	}

	if len(agentIDs) != 5 {
		writeError(w, http.StatusBadRequest, "original match does not have 5 participants")
		return
	}

	newMatchID, err := s.runner.ScheduleMatchWithSnapshot(r.Context(), arenaID, agentIDs, seed, []byte(*rules), *hash, *runtimeSHA256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"message":      "re-match scheduled with identical seed and agents",
		"new_match_id": newMatchID,
		"seed":         seed,
	})
}

// -----------------------------------------------------------------------------
// Handlers: Agents & Ingestion
// -----------------------------------------------------------------------------

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	arenaID := r.URL.Query().Get("arena_id")
	teamID := r.URL.Query().Get("team_id")

	query := `
		SELECT av.id, av.team_id, t.name, av.arena_id, av.name, av.version,
		       av.runtime, av.entrypoint, av.sha256, av.status, av.failure_reason, av.created_at
		FROM agent_versions av
		JOIN teams t ON av.team_id = t.id
		WHERE ($1 = '' OR av.arena_id::text = $1)
		  AND ($2 = '' OR av.team_id::text = $2)
		ORDER BY av.id DESC
	`
	rows, err := s.pool.Query(r.Context(), query, arenaID, teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var agents []models.AgentVersion
	for rows.Next() {
		var a models.AgentVersion
		if err := rows.Scan(
			&a.ID, &a.TeamID, &a.TeamName, &a.ArenaID, &a.Name, &a.Version,
			&a.Runtime, &a.Entrypoint, &a.SHA256, &a.Status, &a.FailureReason, &a.CreatedAt,
		); err == nil {
			agents = append(agents, a)
		}
	}

	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var a models.AgentVersion
	err := s.pool.QueryRow(r.Context(), `
		SELECT av.id, av.team_id, t.name, av.arena_id, av.name, av.version,
		       av.runtime, av.entrypoint, av.sha256, av.status, av.failure_reason, av.created_at
		FROM agent_versions av
		JOIN teams t ON av.team_id = t.id
		WHERE av.id = $1
	`, id).Scan(
		&a.ID, &a.TeamID, &a.TeamName, &a.ArenaID, &a.Name, &a.Version,
		&a.Runtime, &a.Entrypoint, &a.SHA256, &a.Status, &a.FailureReason, &a.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleUploadAgent(w http.ResponseWriter, r *http.Request) {
	select {
	case s.uploadSlots <- struct{}{}:
		defer func() { <-s.uploadSlots }()
	default:
		writeError(w, 429, "upload workers busy; retry later")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, validation.MaxZipSize+1024*1024)
	err := r.ParseMultipartForm(8 * 1024 * 1024)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, 413, "request exceeds 101 MiB HTTP limit")
			return
		}
		writeError(w, http.StatusBadRequest, "failed to parse multipart form or file too large")
		return
	}
	defer r.MultipartForm.RemoveAll()
	fileCount := 0
	for _, files := range r.MultipartForm.File {
		fileCount += len(files)
	}
	if fileCount != 1 || len(r.MultipartForm.File["bot_archive"]) != 1 {
		writeError(w, 400, "exactly one bot_archive ZIP is required")
		return
	}

	file, header, err := r.FormFile("bot_archive")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bot_archive file is required")
		return
	}
	defer file.Close()
	if header.Size > validation.MaxZipSize {
		writeError(w, 413, "ZIP exceeds 100 MiB")
		return
	}

	teamIDStr := r.FormValue("team_id")
	teamID, _ := strconv.Atoi(teamIDStr)
	user, _ := auth.GetUserFromContext(r.Context())
	var ownerID int
	if err := s.pool.QueryRow(r.Context(), "SELECT owner_id FROM teams WHERE id=$1", teamID).Scan(&ownerID); err != nil {
		writeError(w, http.StatusBadRequest, "valid team_id required")
		return
	}
	if user.Role != "admin" && ownerID != user.ID {
		writeError(w, http.StatusForbidden, "team ownership required")
		return
	}

	arenaIDStr := r.FormValue("arena_id")
	arenaID, _ := strconv.Atoi(arenaIDStr)
	if arenaID == 0 {
		arenaID = 1
	}

	botName := r.FormValue("bot_name")
	if botName == "" {
		botName = filepath.Base(header.Filename)
	}

	// Save temporary upload
	tempZip, err := os.CreateTemp("", "agentrix_upload_*.zip")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp file")
		return
	}
	defer os.Remove(tempZip.Name())

	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempZip, hasher)
	written, err := io.Copy(multiWriter, io.LimitReader(file, validation.MaxZipSize+1))
	if err != nil {
		tempZip.Close()
		writeError(w, http.StatusInternalServerError, "failed to save upload")
		return
	}
	tempZip.Close()
	if written > validation.MaxZipSize {
		writeError(w, 413, "ZIP exceeds 100 MiB")
		return
	}

	sha256Hex := hex.EncodeToString(hasher.Sum(nil))

	// Safe extraction
	if err := os.MkdirAll(s.cfg.BotsDir, 0755); err != nil {
		writeError(w, 500, "artifact storage unavailable")
		return
	}
	extractDir, err := os.MkdirTemp(s.cfg.BotsDir, "bot_"+sha256Hex[:12]+"_")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create extraction dir")
		return
	}
	retained := false
	defer func() {
		if !retained {
			_ = os.RemoveAll(extractDir)
			_ = os.Remove(extractDir + ".zip")
		}
	}()

	if err := validation.SafelyExtractZip(tempZip.Name(), extractDir); err != nil {
		_ = os.RemoveAll(extractDir)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("zip extraction failed: %v", err))
		return
	}

	// Inspect manifest
	manifest, err := validation.InspectBotDirectory(extractDir)
	if err != nil {
		_ = os.RemoveAll(extractDir)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("bot inspection failed: %v", err))
		return
	}
	if err := validation.TestBotProtocol(extractDir, manifest, s.cfg.ArbiterPath); err != nil {
		_ = os.RemoveAll(extractDir)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("bot protocol validation failed: %v", err))
		return
	}
	artifactSHA, err := validation.DigestPackage(extractDir)
	if err != nil {
		writeError(w, 400, "artifact checksum failed")
		return
	}

	// Destination directory in var/agentrix/bots
	targetDir := extractDir
	if err := storeOriginalZip(tempZip.Name(), targetDir+".zip", sha256Hex); err != nil {
		writeError(w, 500, "failed to preserve original ZIP")
		return
	}

	entrypoint := manifest.Entrypoint

	// Insert agent version
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "artifact transaction failed")
		return
	}
	defer tx.Rollback(r.Context())
	var lockedTeam int
	if err := tx.QueryRow(r.Context(), "SELECT id FROM teams WHERE id=$1 FOR UPDATE", teamID).Scan(&lockedTeam); err != nil {
		writeError(w, 400, "team unavailable")
		return
	}
	var version int
	if err := tx.QueryRow(r.Context(), "SELECT COALESCE(MAX(version),0)+1 FROM agent_versions WHERE team_id=$1 AND arena_id=$2", teamID, arenaID).Scan(&version); err != nil {
		writeError(w, 500, "version allocation failed")
		return
	}
	var agentID int
	err = tx.QueryRow(r.Context(), `
		INSERT INTO agent_versions (team_id, arena_id, name, runtime, entrypoint, artifact_path, sha256, artifact_sha256,version,status)
		VALUES ($1, $2, $3, $4, $5, $6, $7,$8,$9, 'active')
		RETURNING id
	`, teamID, arenaID, botName, manifest.Runtime, entrypoint, targetDir, sha256Hex, artifactSHA, version).Scan(&agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save agent: %v", err))
		return
	}

	// Register in ladder
	_, err = tx.Exec(r.Context(), `
		INSERT INTO ladder_entries (arena_id, agent_version_id, rating_mu, display_rating, matches_played, wins, kills)
		VALUES ($1, $2, 1500.0, 1500, 0, 0, 0)
		ON CONFLICT (arena_id, agent_version_id) DO NOTHING
	`, arenaID, agentID)
	if err != nil {
		writeError(w, 500, "ladder registration failed")
		return
	}
	// A lost COMMIT acknowledgement must not delete artifacts a committed row may reference.
	retained = true
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "artifact commit failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":         "bot uploaded, validated, and registered in ladder successfully",
		"agent_id":        agentID,
		"name":            botName,
		"sha256":          sha256Hex,
		"artifact_sha256": artifactSHA,
		"version":         version,
		"runtime":         manifest.Runtime,
	})
}

func (s *Server) handleDisqualifyAgent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var req models.DisqualifyAgentRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	reason := req.Reason
	if reason == "" {
		reason = "Disqualified by platform administrator"
	}

	_, err := s.pool.Exec(r.Context(), `
		UPDATE agent_versions
		SET status = 'disqualified', failure_reason = $1
		WHERE id = $2
	`, reason, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "agent disqualified"})
}

func (s *Server) handleEnableAgent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	_, err := s.pool.Exec(r.Context(), `
		UPDATE agent_versions
		SET status = 'active', failure_reason = NULL
		WHERE id = $1
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "agent enabled"})
}

// -----------------------------------------------------------------------------
// Handlers: Teams, Users, Audit Logs
// -----------------------------------------------------------------------------

func (s *Server) handleListTeams(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT t.id, t.name, t.affiliation, t.owner_id, t.created_at
		FROM teams t
		ORDER BY t.name ASC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Affiliation, &t.OwnerID, &t.CreatedAt); err == nil {
			teams = append(teams, t)
		}
	}

	writeJSON(w, http.StatusOK, teams)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, username, email, role, created_at, updated_at
		FROM users
		ORDER BY id ASC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt); err == nil {
			users = append(users, u)
		}
	}

	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT al.id, al.user_id, COALESCE(u.username, 'system'), al.action,
		       al.target_type, al.target_id, al.ip_address, al.details_json, al.created_at
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		ORDER BY al.id DESC
		LIMIT 100
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		var detailsBytes []byte
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.Action, &l.TargetType, &l.TargetID, &l.IPAddress, &detailsBytes, &l.CreatedAt); err == nil {
			_ = json.Unmarshal(detailsBytes, &l.DetailsJSON)
			logs = append(logs, l)
		}
	}

	writeJSON(w, http.StatusOK, logs)
}

// -----------------------------------------------------------------------------
// Helper utilities
// -----------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
