package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	JWTSecret              string
	ArbiterPath            string
	BotsDir                string
	ReplaysDir             string
	RuntimeSHA256          string
	AutoMatchmaker         bool
	MatchInterval          int // in seconds
	BootstrapAdminUsername string
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
}

func Load() (*Config, error) {
	port := getEnv("PORT", "8080")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must be explicit; there is no unauthenticated local database default")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 || strings.TrimSpace(jwtSecret) != jwtSecret ||
		jwtSecret == "agentrix-secret-jwt-key-super-secure-change-in-prod" ||
		jwtSecret == "agentrix-enterprise-jwt-secret-replace-in-production" {
		return nil, fmt.Errorf("JWT_SECRET must be an explicit private key of at least 32 bytes, not a legacy example")
	}
	runtimeSHA256 := os.Getenv("AGENTRIX_RUNTIME_SHA256")
	if os.Getenv("AGENTRIX_DEV_MODE") == "true" && runtimeSHA256 == "" {
		runtimeSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	if len(runtimeSHA256) != 64 || strings.ToLower(runtimeSHA256) != runtimeSHA256 {
		return nil, fmt.Errorf("AGENTRIX_RUNTIME_SHA256 must be the lowercase SHA-256 digest of the dedicated runtime filesystem")
	}
	if _, err := hex.DecodeString(runtimeSHA256); err != nil {
		return nil, fmt.Errorf("AGENTRIX_RUNTIME_SHA256 must be a lowercase SHA-256 digest")
	}
	username, email, password := os.Getenv("BOOTSTRAP_ADMIN_USERNAME"), os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if username != "" || email != "" || password != "" {
		if len(username) < 1 || len(username) > 64 || strings.TrimSpace(username) != username ||
			len(email) > 255 || !strings.Contains(email, "@") || strings.TrimSpace(email) != email ||
			len(password) < 16 || len(password) > 72 {
			return nil, fmt.Errorf("bootstrap requires BOOTSTRAP_ADMIN_USERNAME, BOOTSTRAP_ADMIN_EMAIL and a 16..72 byte BOOTSTRAP_ADMIN_PASSWORD")
		}
	}
	arbiterPath := getEnv("ARBITER_PATH", "simulation/arbiter/target/release/agentrix-arbiter")
	botsDir := getEnv("BOTS_DIR", "var/agentrix/bots")
	replaysDir := getEnv("REPLAYS_DIR", "var/agentrix/replays")
	autoMatchmaker := getEnv("AUTO_MATCHMAKER", "true") == "true"
	matchInterval, _ := strconv.Atoi(getEnv("MATCH_INTERVAL_SECONDS", "30"))
	if matchInterval <= 0 {
		matchInterval = 30
	}

	return &Config{
		Port:                   port,
		DatabaseURL:            dbURL,
		JWTSecret:              jwtSecret,
		ArbiterPath:            arbiterPath,
		BotsDir:                botsDir,
		ReplaysDir:             replaysDir,
		RuntimeSHA256:          runtimeSHA256,
		AutoMatchmaker:         autoMatchmaker,
		MatchInterval:          matchInterval,
		BootstrapAdminUsername: username,
		BootstrapAdminEmail:    email,
		BootstrapAdminPassword: password,
	}, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
