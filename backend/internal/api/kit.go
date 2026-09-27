package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"agentrix/backend/internal/validation"
	"github.com/go-chi/chi/v5"
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
	PythonSDK  string   `json:"python_sdk"`
	CLITools   []string `json:"cli_tools"`
	Quickstart []string `json:"quickstart"`
	GuideURL   string   `json:"guide_url"`
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
			PythonSDK: "agentrix-training",
			CLITools: []string{
				"agentrix-arbiter train-env",
				"agentrix-eval",
				"agentrix-pack",
			},
			Quickstart: []string{
				"cargo build --release -p agentrix-arbiter",
				"pip install -e sdk/",
				"python -m agentrix_training.evaluate --help",
				"python -m agentrix_training.pack --help",
			},
			GuideURL: "/training",
		},
	}

	writeJSON(w, http.StatusOK, manifest)
}
