package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"agentrix/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// All public result-bearing queries use this predicate with aliases m and a.
// Membership captures committed terminal results, not a guessed time boundary.
const visibleMatch = `($ADMIN OR NOT EXISTS (
 SELECT 1 FROM arena_freezes f WHERE f.arena_id=m.arena_id
 AND NOT EXISTS (SELECT 1 FROM arena_frozen_matches fm WHERE fm.arena_id=f.arena_id AND fm.match_id=m.id)))`

func matchVisibility(adminParameter string) string {
	return strings.ReplaceAll(visibleMatch, "$ADMIN", adminParameter)
}

const ladderQuery = `SELECT le.id, le.arena_id, le.agent_version_id,
 av.name AS agent_name, t.id AS team_id, t.name AS team_name,
 le.rating_mu, le.rating_sigma, le.display_rating, le.matches_played,
 le.wins, le.kills, le.survival_ticks_total, le.last_match_at, le.updated_at, av.status
 FROM ladder_entries le JOIN agent_versions av ON le.agent_version_id=av.id
 JOIN teams t ON av.team_id=t.id WHERE le.arena_id=$1
 ORDER BY le.display_rating DESC,le.wins DESC,le.kills DESC,le.agent_version_id`

const ladderSnapshot = `SELECT COALESCE(jsonb_agg(to_jsonb(q)||jsonb_build_object(
 'win_rate',CASE WHEN q.matches_played>0 THEN 100.0*q.wins/q.matches_played ELSE 0 END,
 'avg_survival_ticks',CASE WHEN q.matches_played>0 THEN 1.0*q.survival_ticks_total/q.matches_played ELSE 0 END)
 ORDER BY q.display_rating DESC,q.wins DESC,q.kills DESC,q.agent_version_id), '[]'::jsonb)
 FROM (` + ladderQuery + `)q`

func isAdmin(r *http.Request) bool {
	user, ok := auth.GetUserFromContext(r.Context())
	return ok && user.Role == "admin"
}

func (s *Server) handleFreezeArena(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid arena")
		return
	}
	var req struct {
		Frozen *bool `json:"frozen"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil || req.Frozen == nil {
		writeError(w, 400, "frozen boolean required")
		return
	}
	tx, err := s.pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		writeError(w, 500, "freeze transaction failed")
		return
	}
	defer tx.Rollback(r.Context())
	// Match commits take the same advisory lock before writing standings.
	if _, err := tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock($1)", int64(id)); err != nil {
		writeError(w, 500, "freeze lock failed")
		return
	}
	var exists bool
	if err := tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM arenas WHERE id=$1)", id).Scan(&exists); err != nil || !exists {
		writeError(w, 404, "arena not found")
		return
	}
	if *req.Frozen {
		// Repeating freeze preserves the original snapshot.
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
	} else {
		if _, err := tx.Exec(r.Context(), "DELETE FROM arena_freezes WHERE arena_id=$1", id); err != nil {
			writeError(w, 500, "unfreeze failed")
			return
		}
	}
	user, _ := auth.GetUserFromContext(r.Context())
	details, _ := json.Marshal(map[string]bool{"frozen": *req.Frozen})
	if _, err := tx.Exec(r.Context(), `INSERT INTO audit_logs(user_id,action,target_type,target_id,details_json)
	 VALUES($1,'ARENA_FREEZE','ARENA',$2,$3)`, user.ID, strconv.Itoa(id), details); err != nil {
		writeError(w, 500, "freeze audit failed")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "freeze commit failed")
		return
	}
	writeJSON(w, 200, map[string]bool{"frozen": *req.Frozen})
}
