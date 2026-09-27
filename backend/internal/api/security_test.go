package api

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"agentrix/backend/internal/auth"
	"agentrix/backend/internal/config"
)

func TestAdministrativeRoutesRequireRole(t *testing.T) {
	s := NewServer(&config.Config{JWTSecret: "test-secret"}, nil, nil, nil)
	for _, path := range []string{"/api/v1/matches/trigger", "/api/v1/matches/1/rerun", "/api/v1/agents/1/enable", "/api/v1/agents/1/disqualify", "/api/v1/arenas/1/freeze"} {
		for _, role := range []string{"", "player"} {
			r := httptest.NewRequest("POST", path, nil)
			want := 401
			if role != "" {
				token, err := auth.GenerateToken(1, "player", role, "test-secret")
				if err != nil {
					t.Fatal(err)
				}
				r.Header.Set("Authorization", "Bearer "+token)
				want = 403
			}
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, r)
			if w.Code != want {
				t.Fatalf("%s role %q: got %d want %d", path, role, w.Code, want)
			}
		}
	}
}

func TestReplayStorageConfinement(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{outside, link, root} {
		if _, err := confinedReplayPath(root, path); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	valid := filepath.Join(root, "match.json")
	if err := os.WriteFile(valid, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := confinedReplayPath(root, valid); err != nil {
		t.Fatal(err)
	}
}
