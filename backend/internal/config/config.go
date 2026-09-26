package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	ArbiterPath    string
	BotsDir        string
	ReplaysDir     string
	AutoMatchmaker bool
	MatchInterval  int // in seconds
}

func Load() *Config {
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres@127.0.0.1:5432/agentrix_platform?sslmode=disable")
	jwtSecret := getEnv("JWT_SECRET", "agentrix-secret-jwt-key-super-secure-change-in-prod")
	arbiterPath := getEnv("ARBITER_PATH", "agentrix/arbiter/target/release/agentrix-arbiter")
	botsDir := getEnv("BOTS_DIR", "var/agentrix/bots")
	replaysDir := getEnv("REPLAYS_DIR", "var/agentrix/replays")
	autoMatchmaker := getEnv("AUTO_MATCHMAKER", "true") == "true"
	matchInterval, _ := strconv.Atoi(getEnv("MATCH_INTERVAL_SECONDS", "30"))
	if matchInterval <= 0 {
		matchInterval = 30
	}

	return &Config{
		Port:           port,
		DatabaseURL:    dbURL,
		JWTSecret:      jwtSecret,
		ArbiterPath:    arbiterPath,
		BotsDir:        botsDir,
		ReplaysDir:     replaysDir,
		AutoMatchmaker: autoMatchmaker,
		MatchInterval:  matchInterval,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
