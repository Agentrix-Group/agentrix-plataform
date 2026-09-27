package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestGetArenaKit(t *testing.T) {
	s, _, _, arenaID, _ := fixtureAPI(t)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/arenas/"+strings.TrimSpace(strconv.Itoa(arenaID))+"/kit", nil)
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
	expectedDownloadURL := "/api/v1/arenas/" + strconv.Itoa(arenaID) + "/kit/download"
	if manifest.StarterKit.DownloadURL != expectedDownloadURL {
		t.Errorf("expected download URL %s, got %s", expectedDownloadURL, manifest.StarterKit.DownloadURL)
	}
	if manifest.StarterKit.Filename == "" {
		t.Errorf("expected non-empty kit filename")
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

func TestDownloadArenaKit(t *testing.T) {
	s, _, _, arenaID, _ := fixtureAPI(t)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/arenas/"+strconv.Itoa(arenaID)+"/kit/download", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %s", contentType)
	}

	disp := w.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "agentrix-starter-kit") {
		t.Errorf("expected Content-Disposition to reference agentrix-starter-kit, got %s", disp)
	}

	bodyBytes := w.Body.Bytes()
	if len(bodyBytes) < 1000 {
		t.Fatalf("expected kit payload to be at least 1 KB, got %d bytes", len(bodyBytes))
	}

	// Validate valid ZIP structure
	zr, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
	if err != nil {
		t.Fatalf("downloaded payload is not a valid zip archive: %v", err)
	}

	var foundArbiter, foundSDK, foundMyBot bool
	for _, f := range zr.File {
		if strings.Contains(f.Name, "bin/agentrix-arbiter") {
			foundArbiter = true
		}
		if strings.Contains(f.Name, "sdk/agentrix_training") {
			foundSDK = true
		}
		if strings.Contains(f.Name, "my_bot/agent.py") {
			foundMyBot = true
		}
	}

	if !foundArbiter {
		t.Errorf("zip archive missing bin/agentrix-arbiter")
	}
	if !foundSDK {
		t.Errorf("zip archive missing sdk/agentrix_training")
	}
	if !foundMyBot {
		t.Errorf("zip archive missing my_bot/agent.py")
	}
}

func TestDownloadArenaKitNotFound(t *testing.T) {
	s, _, _, _, _ := fixtureAPI(t)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/arenas/9999/kit/download", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", w.Code)
	}
}
