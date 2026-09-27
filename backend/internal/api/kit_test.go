package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetArenaKit(t *testing.T) {
	s, _, _, arenaID, _ := fixtureAPI(t)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/arenas/1/kit", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var manifest ArenaKitManifest
	if err := json.Unmarshal(w.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("failed to decode kit manifest: %v", err)
	}

	if manifest.ArenaID != arenaID {
		t.Errorf("expected arena_id %d, got %d", arenaID, manifest.ArenaID)
	}
	if manifest.Engine.Version != "agentrix-engine-v1" {
		t.Errorf("unexpected engine version: %s", manifest.Engine.Version)
	}
	if manifest.PackageLimits.MaxZipBytes != 100*1024*1024 {
		t.Errorf("expected 100 MiB zip limit, got %d", manifest.PackageLimits.MaxZipBytes)
	}
	if manifest.PackageLimits.TickTimeoutMs != 50 {
		t.Errorf("expected 50ms tick timeout, got %d", manifest.PackageLimits.TickTimeoutMs)
	}
	if len(manifest.PackageLimits.SupportedRuntimes) < 3 {
		t.Errorf("expected at least 3 supported runtimes, got %v", manifest.PackageLimits.SupportedRuntimes)
	}
}

func TestGetArenaKitNotFound(t *testing.T) {
	s, _, _, _, _ := fixtureAPI(t)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/arenas/9999/kit", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", w.Code)
	}
}
