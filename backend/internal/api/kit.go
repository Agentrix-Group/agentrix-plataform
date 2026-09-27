package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"agentrix/backend/internal/validation"
	"github.com/go-chi/chi/v5"
)

var (
	kitCacheMu sync.RWMutex
	kitSHA256  string
	kitSize    int64
	kitPath    string
)

type EngineInfo struct {
	Version               string `json:"version"`
	RulesVersion          string `json:"rules_version"`
	ObservationVersion    string `json:"observation_version"`
	FeatureEncoderVersion string `json:"feature_encoder_version"`
	ScoreVersion          string `json:"score_version"`
	TicksPerSecond        int    `json:"ticks_per_second"`
}

type PackageLimits struct {
	MaxZipBytes       int64    `json:"max_zip_bytes"`
	MaxExtractedBytes int64    `json:"max_extracted_bytes"`
	MaxManifestBytes  int64    `json:"max_manifest_bytes"`
	MaxFileCount      int      `json:"max_file_count"`
	WarmupTimeoutMs   int      `json:"warmup_timeout_ms"`
	TickTimeoutMs     int      `json:"tick_timeout_ms"`
	MemoryLimitBytes  int64    `json:"memory_limit_bytes"`
	CPULimit          string   `json:"cpu_limit"`
	SupportedRuntimes []string `json:"supported_runtimes"`
}

type StarterKitInfo struct {
	PythonSDK   string   `json:"python_sdk"`
	CLITools    []string `json:"cli_tools"`
	Quickstart  []string `json:"quickstart"`
	GuideURL    string   `json:"guide_url"`
	DownloadURL string   `json:"download_url"`
	Filename    string   `json:"filename"`
	SHA256      string   `json:"sha256"`
	SizeBytes   int64    `json:"size_bytes"`
	ColabURL    string   `json:"colab_url"`
}

type ArenaKitManifest struct {
	ArenaID       int                    `json:"arena_id"`
	ArenaSlug     string                 `json:"arena_slug"`
	ArenaName     string                 `json:"arena_name"`
	Phase         string                 `json:"phase"`
	MaxPlayers    int                    `json:"max_players"`
	MaxTicks      int                    `json:"max_ticks"`
	Engine        EngineInfo             `json:"engine"`
	PackageLimits PackageLimits          `json:"package_limits"`
	Rules         map[string]interface{} `json:"rules"`
	StarterKit    StarterKitInfo         `json:"starter_kit"`
}

func (s *Server) findStarterKitZip() (string, int64, string, error) {
	kitCacheMu.RLock()
	if kitPath != "" && kitSHA256 != "" {
		if fi, err := os.Stat(kitPath); err == nil && fi.Mode().IsRegular() {
			defer kitCacheMu.RUnlock()
			return kitPath, kitSize, kitSHA256, nil
		}
	}
	kitCacheMu.RUnlock()

	kitCacheMu.Lock()
	defer kitCacheMu.Unlock()

	candidates := []string{
		filepath.Join(s.cfg.KitsDir, "agentrix-starter-kit-v0.2.0.zip"),
		filepath.Join(s.cfg.KitsDir, "agentrix-starter-kit.zip"),
		"../../../var/agentrix/artifacts/kits/agentrix-starter-kit-v0.2.0.zip",
		"../../../var/agentrix/artifacts/kits/agentrix-starter-kit.zip",
		"../../var/agentrix/artifacts/kits/agentrix-starter-kit-v0.2.0.zip",
		"../../var/agentrix/artifacts/kits/agentrix-starter-kit.zip",
		"var/agentrix/artifacts/kits/agentrix-starter-kit-v0.2.0.zip",
		"var/agentrix/artifacts/kits/agentrix-starter-kit.zip",
		"/var/agentrix/artifacts/kits/agentrix-starter-kit.zip",
		"/opt/agentrix/kits/agentrix-starter-kit.zip",
	}

	var foundPath string
	var fi os.FileInfo
	for _, c := range candidates {
		info, err := os.Stat(c)
		if err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			foundPath = c
			fi = info
			break
		}
	}

	if foundPath == "" {
		return "", 0, "", fmt.Errorf("starter kit archive not found in configured kits directories")
	}

	// Try reading companion .sha256 file
	shaCandidate := foundPath + ".sha256"
	shaBytes, err := os.ReadFile(shaCandidate)
	var digest string
	if err == nil {
		parts := strings.Fields(string(shaBytes))
		if len(parts) > 0 && len(parts[0]) == 64 {
			digest = strings.ToLower(parts[0])
		}
	}

	// If .sha256 file not present or unparseable, compute dynamically
	if digest == "" {
		f, err := os.Open(foundPath)
		if err == nil {
			h := sha256.New()
			if _, err := io.Copy(h, f); err == nil {
				digest = hex.EncodeToString(h.Sum(nil))
			}
			f.Close()
		}
	}

	kitPath = foundPath
	kitSize = fi.Size()
	kitSHA256 = digest

	return kitPath, kitSize, kitSHA256, nil
}

func (s *Server) handleGetArenaKit(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid arena id")
		return
	}

	var slug, name, phase string
	var maxPlayers, maxTicks int
	var cfgBytes []byte
	err = s.pool.QueryRow(r.Context(), `
		SELECT slug, name, max_players, max_ticks, config_json, COALESCE(phase, 'warmup')
		FROM arenas WHERE id = $1
	`, id).Scan(&slug, &name, &maxPlayers, &maxTicks, &cfgBytes, &phase)
	if err != nil {
		writeError(w, http.StatusNotFound, "arena not found")
		return
	}

	var rules map[string]interface{}
	if len(cfgBytes) > 0 {
		_ = json.Unmarshal(cfgBytes, &rules)
	}
	if rules == nil {
		rules = make(map[string]interface{})
	}

	_, kitSize, kitDigest, _ := s.findStarterKitZip()
	downloadURL := fmt.Sprintf("/api/v1/arenas/%d/kit/download", id)
	filename := "agentrix-starter-kit-v0.2.0.zip"
	colabURL := "https://colab.research.google.com/github/Agentrix-Group/agentrix-plataform/blob/main/notebooks/agentrix_colab_starter.ipynb"

	manifest := ArenaKitManifest{
		ArenaID:    id,
		ArenaSlug:  slug,
		ArenaName:  name,
		Phase:      phase,
		MaxPlayers: maxPlayers,
		MaxTicks:   maxTicks,
		Engine: EngineInfo{
			Version:               "agentrix-engine-v1",
			RulesVersion:          "agentrix-rules-v1",
			ObservationVersion:    "agentrix-obs-v1",
			FeatureEncoderVersion: "agentrix-features-v1",
			ScoreVersion:          "agentrix-score-v1",
			TicksPerSecond:        60,
		},
		PackageLimits: PackageLimits{
			MaxZipBytes:       validation.MaxZipSize,
			MaxExtractedBytes: validation.MaxExtractedSize,
			MaxManifestBytes:  validation.MaxManifestSize,
			MaxFileCount:      validation.MaxFileCount,
			WarmupTimeoutMs:   10000,
			TickTimeoutMs:     50,
			MemoryLimitBytes:  2 * 1024 * 1024 * 1024, // 2 GiB
			CPULimit:          "1.0 core",
			SupportedRuntimes: []string{"python-standard", "python-onnx", "binary"},
		},
		Rules: rules,
		StarterKit: StarterKitInfo{
			PythonSDK:   "agentrix-training",
			DownloadURL: downloadURL,
			Filename:    filename,
			SHA256:      kitDigest,
			SizeBytes:   kitSize,
			ColabURL:    colabURL,
			CLITools: []string{
				"agentrix-arbiter train-env",
				"agentrix-eval",
				"agentrix-train",
				"agentrix-pack",
			},
			Quickstart: []string{
				fmt.Sprintf("curl -fsSL -O http://%s%s", r.Host, downloadURL),
				"unzip agentrix-starter-kit-v0.2.0.zip && cd agentrix-starter-kit-v0.2.0",
				"make setup",
				"agentrix-eval --policies hunter,mob_farmer,survivor,zone_controller,random --seeds 10",
				"agentrix-train --matches 15 --epochs 40 --output-dir my_bot",
				"agentrix-pack --bot-dir my_bot --output my_bot.zip",
			},
			GuideURL: "/training",
		},
	}

	writeJSON(w, http.StatusOK, manifest)
}

func (s *Server) handleDownloadArenaKit(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid arena id")
		return
	}

	var dummy int
	err = s.pool.QueryRow(r.Context(), "SELECT 1 FROM arenas WHERE id = $1", id).Scan(&dummy)
	if err != nil {
		writeError(w, http.StatusNotFound, "arena not found")
		return
	}

	path, size, _, err := s.findStarterKitZip()
	if err != nil {
		writeError(w, http.StatusNotFound, "starter kit distribution package not found")
		return
	}

	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "starter kit package unavailable")
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusInternalServerError, "starter kit package corrupted")
		return
	}

	filename := "agentrix-starter-kit-v0.2.0.zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	http.ServeContent(w, r, filename, info.ModTime(), f)
}
