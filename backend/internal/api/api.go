package api

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

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
)

type Server struct {
	cfg        *config.Config
	pool       *db.Pool
	runner     *runner.MatchRunner
	matchmaker *matchmaker.Matchmaker
	router     chi.Router
}

func NewServer(cfg *config.Config, pool *db.Pool, r *runner.MatchRunner, m *matchmaker.Matchmaker) *Server {
	s := &Server{
		cfg:        cfg,
		pool:       pool,
		runner:     r,
		matchmaker: m,
		router:     chi.NewRouter(),
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
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
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

		// Ladder / Standings
		r.Get("/arenas/{id}/ladder", s.handleGetLadder)

		// Matches
		r.Get("/matches", s.handleListMatches)
		r.Get("/matches/{id}", s.handleGetMatch)
		r.Get("/matches/{id}/replay", s.handleGetMatchReplay)
		r.Get("/matches/{id}/download", s.handleDownloadMatchReplay)
		r.Post("/matches/trigger", s.handleTriggerMatch)
		r.Post("/matches/{id}/rerun", s.handleRerunMatch)

		// Agents / Bots
		r.Get("/agents", s.handleListAgents)
		r.Get("/agents/{id}", s.handleGetAgent)
		r.Post("/agents/upload", s.handleUploadAgent)
		r.Post("/agents/{id}/disqualify", s.handleDisqualifyAgent)
		r.Post("/agents/{id}/enable", s.handleEnableAgent)

		// Teams & Users
		r.Get("/teams", s.handleListTeams)
		r.Get("/users", s.handleListUsers)

		// Audit Logs
		r.Get("/audit-logs", s.handleListAuditLogs)
	})
}

// -----------------------------------------------------------------------------
// Handlers: System & Auth
// -----------------------------------------------------------------------------

func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	dbErr := s.pool.Ping(r.Context())
	_, arbiterErr := os.Stat(s.cfg.ArbiterPath)

	var runningMatches int
	_ = s.pool.QueryRow(r.Context(), "SELECT count(*) FROM matches WHERE status = 'running'").Scan(&runningMatches)

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
		SELECT id, slug, name, description, game_type, max_players, max_ticks, config_json, is_active, created_at
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
		if err := rows.Scan(&a.ID, &a.Slug, &a.Name, &a.Description, &a.GameType, &a.MaxPlayers, &a.MaxTicks, &cfgBytes, &a.IsActive, &a.CreatedAt); err == nil {
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
		SELECT id, slug, name, description, game_type, max_players, max_ticks, config_json, is_active, created_at
		FROM arenas WHERE id = $1
	`, id).Scan(&a.ID, &a.Slug, &a.Name, &a.Description, &a.GameType, &a.MaxPlayers, &a.MaxTicks, &cfgBytes, &a.IsActive, &a.CreatedAt)
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

	rows, err := s.pool.Query(r.Context(), `
		SELECT le.id, le.arena_id, le.agent_version_id, av.name, t.id, t.name,
		       le.rating_mu, le.rating_sigma, le.display_rating, le.matches_played,
		       le.wins, le.kills, le.survival_ticks_total, le.last_match_at, le.updated_at,
		       av.status
		FROM ladder_entries le
		JOIN agent_versions av ON le.agent_version_id = av.id
		JOIN teams t ON av.team_id = t.id
		WHERE le.arena_id = $1
		ORDER BY le.display_rating DESC, le.wins DESC, le.kills DESC
	`, arenaID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var entries []models.LadderEntry
	for rows.Next() {
		var e models.LadderEntry
		if err := rows.Scan(
			&e.ID, &e.ArenaID, &e.AgentVersionID, &e.AgentName, &e.TeamID, &e.TeamName,
			&e.RatingMu, &e.RatingSigma, &e.DisplayRating, &e.MatchesPlayed,
			&e.Wins, &e.Kills, &e.SurvivalTicksTotal, &e.LastMatchAt, &e.UpdatedAt,
			&e.Status,
		); err == nil {
			if e.MatchesPlayed > 0 {
				e.WinRate = float64(e.Wins) / float64(e.MatchesPlayed) * 100.0
				e.AvgSurvivalTicks = float64(e.SurvivalTicksTotal) / float64(e.MatchesPlayed)
			}
			entries = append(entries, e)
		}
	}

	writeJSON(w, http.StatusOK, entries)
}

// -----------------------------------------------------------------------------
// Handlers: Matches & Replays
// -----------------------------------------------------------------------------

func (s *Server) handleListMatches(w http.ResponseWriter, r *http.Request) {
	arenaID := r.URL.Query().Get("arena_id")
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	query := `
		SELECT m.id, m.arena_id, a.name, m.seed, m.status, m.ticks_played,
		       m.winner_agent_id, av.name, m.error_message, m.started_at, m.finished_at, m.created_at
		FROM matches m
		JOIN arenas a ON m.arena_id = a.id
		LEFT JOIN agent_versions av ON m.winner_agent_id = av.id
		WHERE ($1 = '' OR m.arena_id::text = $1)
		  AND ($2 = '' OR m.status = $2)
		ORDER BY m.id DESC
		LIMIT $3
	`
	rows, err := s.pool.Query(r.Context(), query, arenaID, status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var winnerName *string
		if err := rows.Scan(
			&m.ID, &m.ArenaID, &m.ArenaName, &m.Seed, &m.Status, &m.TicksPlayed,
			&m.WinnerAgentID, &winnerName, &m.ErrorMessage, &m.StartedAt, &m.FinishedAt, &m.CreatedAt,
		); err == nil {
			if winnerName != nil {
				m.WinnerName = *winnerName
			}
			m.ReplayURL = fmt.Sprintf("/api/v1/matches/%d/replay", m.ID)
			matches = append(matches, m)
		}
	}
	rows.Close()

	// Populate participants for each match
	for i := range matches {
		pRows, err := s.pool.Query(r.Context(), `
			SELECT mp.id, mp.match_id, mp.agent_version_id, av.name, t.name,
			       mp.seat, mp.rank_place, mp.kills, mp.survival_ticks, mp.score,
			       mp.disqualified, mp.disqualification_reason, mp.old_rating, mp.new_rating, mp.rating_delta
			FROM match_participants mp
			JOIN agent_versions av ON mp.agent_version_id = av.id
			JOIN teams t ON av.team_id = t.id
			WHERE mp.match_id = $1
			ORDER BY mp.rank_place ASC, mp.seat ASC
		`, matches[i].ID)
		if err == nil {
			for pRows.Next() {
				var p models.MatchParticipant
				if err := pRows.Scan(
					&p.ID, &p.MatchID, &p.AgentVersionID, &p.AgentName, &p.TeamName,
					&p.Seat, &p.RankPlace, &p.Kills, &p.SurvivalTicks, &p.Score,
					&p.Disqualified, &p.DisqualificationReason, &p.OldRating, &p.NewRating, &p.RatingDelta,
				); err == nil {
					matches[i].Participants = append(matches[i].Participants, p)
				}
			}
			pRows.Close()
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
		WHERE m.id = $1
	`, id).Scan(
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
	err := s.pool.QueryRow(r.Context(), "SELECT file_path FROM replays WHERE match_id = $1", id).Scan(&filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "replay not found for this match")
		return
	}

	f, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "replay file not found on storage")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/json")

	// Decompress gzip and stream raw JSON to browser
	gzReader, err := gzip.NewReader(f)
	if err != nil {
		// Not gzipped, stream raw
		f.Seek(0, io.SeekStart)
		_, _ = io.Copy(w, f)
		return
	}
	defer gzReader.Close()

	_, _ = io.Copy(w, gzReader)
}

func (s *Server) handleDownloadMatchReplay(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var filePath string
	err := s.pool.QueryRow(r.Context(), "SELECT file_path FROM replays WHERE match_id = $1", id).Scan(&filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "replay not found")
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"agentrix_match_%d_replay.json.gz\"", id))
	http.ServeFile(w, r, filePath)
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

	// Trigger async execution
	go func(mid int) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = s.runner.ExecuteMatch(ctx, mid)
	}(matchID)

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
	err := s.pool.QueryRow(r.Context(), "SELECT arena_id, seed FROM matches WHERE id = $1", id).Scan(&arenaID, &seed)
	if err != nil {
		writeError(w, http.StatusNotFound, "match not found")
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

	newMatchID, err := s.runner.ScheduleMatch(r.Context(), arenaID, agentIDs, &seed)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go func(mid int) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = s.runner.ExecuteMatch(ctx, mid)
	}(newMatchID)

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
	err := r.ParseMultipartForm(validation.MaxZipSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form or file too large")
		return
	}

	file, header, err := r.FormFile("bot_archive")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bot_archive file is required")
		return
	}
	defer file.Close()

	teamIDStr := r.FormValue("team_id")
	teamID, _ := strconv.Atoi(teamIDStr)
	if teamID == 0 {
		teamID = 1 // default team fallback
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
	if _, err := io.Copy(multiWriter, file); err != nil {
		tempZip.Close()
		writeError(w, http.StatusInternalServerError, "failed to save upload")
		return
	}
	tempZip.Close()

	sha256Hex := hex.EncodeToString(hasher.Sum(nil))

	// Safe extraction
	extractDir, err := os.MkdirTemp("", "agentrix_bot_*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create extraction dir")
		return
	}

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

	// Destination directory in var/agentrix/bots
	targetDir := filepath.Join(s.cfg.BotsDir, fmt.Sprintf("bot_%s_%d", sha256Hex[:12], time.Now().Unix()))
	_ = os.MkdirAll(filepath.Dir(targetDir), 0755)
	if err := os.Rename(extractDir, targetDir); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store bot artifact")
		return
	}

	entrypoint := filepath.Join(targetDir, manifest.Entrypoint)
	if manifest.Runtime == "python-standard" && !filepath.IsAbs(manifest.Entrypoint) {
		parts := filepath.SplitList(manifest.Entrypoint)
		entrypoint = fmt.Sprintf("python3 %s", filepath.Join(targetDir, parts[0]))
	}

	// Insert agent version
	var agentID int
	err = s.pool.QueryRow(r.Context(), `
		INSERT INTO agent_versions (team_id, arena_id, name, runtime, entrypoint, artifact_path, sha256, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
		RETURNING id
	`, teamID, arenaID, botName, manifest.Runtime, entrypoint, targetDir, sha256Hex).Scan(&agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save agent: %v", err))
		return
	}

	// Register in ladder
	_, _ = s.pool.Exec(r.Context(), `
		INSERT INTO ladder_entries (arena_id, agent_version_id, rating_mu, display_rating, matches_played, wins, kills)
		VALUES ($1, $2, 1500.0, 1500, 0, 0, 0)
		ON CONFLICT (arena_id, agent_version_id) DO NOTHING
	`, arenaID, agentID)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":   "bot uploaded, validated, and registered in ladder successfully",
		"agent_id":  agentID,
		"name":      botName,
		"sha256":    sha256Hex,
		"runtime":   manifest.Runtime,
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
