package api

import (
	"agentrix/backend/internal/runner"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"strconv"
)

func (s *Server) handleScheduleRound(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid arena")
		return
	}
	var req struct {
		Key    string `json:"idempotency_key"`
		Roster []int  `json:"agent_version_ids"`
		Seed   *int64 `json:"seed"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, 400, "invalid round request")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, 400, "unexpected trailing data")
		return
	}
	receipt, err := s.runner.ScheduleRound(r.Context(), id, req.Key, req.Roster, req.Seed)
	if errors.Is(err, runner.ErrRoundConflict) {
		writeError(w, 409, err.Error())
		return
	}
	if err != nil {
		writeError(w, 400, "round could not be scheduled: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(receipt)
}
