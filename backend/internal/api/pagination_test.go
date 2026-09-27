package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"agentrix/backend/internal/models"
)

func TestMatchPaginationTraversesHistoryWithoutDuplicates(t *testing.T) {
	s, ctx, _, arena, agents := fixtureAPI(t)
	for i := 0; i < 123; i++ {
		var id int
		if err := s.pool.QueryRow(ctx, "INSERT INTO matches(arena_id,seed,status) VALUES($1,$2,'finished') RETURNING id", arena, i).Scan(&id); err != nil {
			t.Fatal(err)
		}
		for seat, agent := range agents {
			if _, err := s.pool.Exec(ctx, "INSERT INTO match_participants(match_id,agent_version_id,seat,rank_place) VALUES($1,$2,$3,$4)", id, agent, seat, 5-seat); err != nil {
				t.Fatal(err)
			}
		}
	}
	page := func(cursor int) []models.Match {
		t.Helper()
		path := fmt.Sprintf("/api/v1/matches?arena_id=%d&status=finished&limit=50", arena)
		if cursor != 0 {
			path += fmt.Sprintf("&before_id=%d", cursor)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		var matches []models.Match
		if err := json.Unmarshal(w.Body.Bytes(), &matches); err != nil {
			t.Fatal(err)
		}
		if matches == nil {
			t.Fatal("empty page must be an array, not null")
		}
		return matches
	}
	seen, cursor := make(map[int]bool), 0
	for n, expected := range []int{50, 50, 23, 0} {
		matches := page(cursor)
		if len(matches) != expected {
			t.Fatalf("page %d: got %d want %d", n, len(matches), expected)
		}
		for _, match := range matches {
			if seen[match.ID] || (cursor != 0 && match.ID >= cursor) {
				t.Fatal("cursor repeated or reordered a match")
			}
			if len(match.Participants) != 5 || match.Participants[0].RankPlace != 1 || match.Participants[4].RankPlace != 5 {
				t.Fatal("batch participant query lost or reordered participants")
			}
			seen[match.ID], cursor = true, match.ID
		}
		if n == 0 {
			// Concurrent newer matches must not shift older pages as OFFSET would.
			if _, err := s.pool.Exec(ctx, "INSERT INTO matches(arena_id,seed,status) VALUES($1,999,'finished')", arena); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(seen) != 123 {
		t.Fatalf("only traversed %d matches", len(seen))
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=x", "before_id=-1", "before_id=x", "arena_id=0"} {
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/matches?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("%s: got %d", query, w.Code)
		}
	}
}
