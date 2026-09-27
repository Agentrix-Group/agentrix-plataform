package api

import (
	"agentrix/backend/internal/auth"
	"agentrix/backend/internal/runner"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestRoundEndpointIsAuthorizedAndIdempotent(t *testing.T) {
	s, ctx, user, arena, agents := fixtureAPI(t)
	admin, err := auth.GenerateToken(user, "judge", "admin", s.cfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	player, err := auth.GenerateToken(user, "player", "player", s.cfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	request := func(token string, seed int) *httptest.ResponseRecorder {
		data, err := json.Marshal(map[string]any{"idempotency_key": "round-one", "agent_version_ids": agents, "seed": seed})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/arenas/%d/rounds", arena), bytes.NewReader(data))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	if w := request("", 42); w.Code != 401 {
		t.Fatalf("anonymous: %d", w.Code)
	}
	if w := request(player, 42); w.Code != 403 {
		t.Fatalf("player: %d", w.Code)
	}
	var id int
	for i := 0; i < 2; i++ {
		w := request(admin, 42)
		if w.Code != 202 {
			t.Fatalf("round: %d %s", w.Code, w.Body.String())
		}
		var receipt runner.RoundReceipt
		if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Total != 5 || receipt.ID == 0 || id != 0 && receipt.ID != id {
			t.Fatal("round receipt mismatch")
		}
		id = receipt.ID
	}
	if w := request(admin, 43); w.Code != 409 {
		t.Fatalf("conflict: %d %s", w.Code, w.Body.String())
	}
	var count int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM tournament_rounds WHERE arena_id=$1", arena).Scan(&count); err != nil || count != 1 {
		t.Fatalf("round count %d: %v", count, err)
	}
}
