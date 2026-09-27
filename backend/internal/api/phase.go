package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"agentrix/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (s *Server) handleSetArenaPhase(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid arena")
		return
	}
	var req struct {
		Phase string `json:"phase"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil || req.Phase == "" {
		writeError(w, 400, "phase string required")
		return
	}

	validPhases := map[string]bool{
		"warmup":   true,
		"running":  true,
		"frozen":   true,
		"finished": true,
	}
	if !validPhases[req.Phase] {
		writeError(w, 400, "invalid phase: must be one of 'warmup', 'running', 'frozen', 'finished'")
		return
	}

	tx, err := s.pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		writeError(w, 500, "phase transaction failed")
		return
	}
	defer tx.Rollback(r.Context())

	if _, err := tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock($1)", int64(id)); err != nil {
		writeError(w, 500, "phase lock failed")
		return
	}

	var exists bool
	var oldPhase string
	if err := tx.QueryRow(r.Context(), "SELECT true, COALESCE(phase, 'warmup') FROM arenas WHERE id=$1", id).Scan(&exists, &oldPhase); err != nil || !exists {
		writeError(w, 404, "arena not found")
		return
	}

	// If switching to 'frozen', ensure scoreboard is frozen
	if req.Phase == "frozen" {
		var frozen bool
		if err := tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM arena_freezes WHERE arena_id=$1)", id).Scan(&frozen); err != nil {
			writeError(w, 500, "freeze query failed")
			return
		}
		if !frozen {
			_, err = tx.Exec(r.Context(), `INSERT INTO arena_freezes(arena_id,ladder_json)
			 SELECT $1,(`+ladderSnapshot+`)`, id)
			if err != nil {
				writeError(w, 500, "snapshot failed")
				return
			}
			_, err = tx.Exec(r.Context(), `INSERT INTO arena_frozen_matches(arena_id,match_id)
			 SELECT $1,id FROM matches WHERE arena_id=$1 AND status IN ('finished','failed')`, id)
			if err != nil {
				writeError(w, 500, "match snapshot failed")
				return
			}
		}
	} else if req.Phase == "finished" || req.Phase == "warmup" || req.Phase == "running" {
		// Unfreeze if transitioning to finished, warmup, or running
		if _, err := tx.Exec(r.Context(), "DELETE FROM arena_freezes WHERE arena_id=$1", id); err != nil {
			writeError(w, 500, "unfreeze failed")
			return
		}
	}

	if _, err := tx.Exec(r.Context(), "UPDATE arenas SET phase=$2 WHERE id=$1", id, req.Phase); err != nil {
		writeError(w, 500, "phase update failed")
		return
	}

	user, _ := auth.GetUserFromContext(r.Context())
	details, _ := json.Marshal(map[string]string{
		"old_phase": oldPhase,
		"new_phase": req.Phase,
	})
	if _, err := tx.Exec(r.Context(), `INSERT INTO audit_logs(user_id,action,target_type,target_id,details_json)
	 VALUES($1,'ARENA_PHASE_CHANGE','ARENA',$2,$3)`, user.ID, strconv.Itoa(id), details); err != nil {
		writeError(w, 500, "phase audit failed")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "phase commit failed")
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"arena_id": id,
		"phase":    req.Phase,
		"frozen":   req.Phase == "frozen",
	})
}
