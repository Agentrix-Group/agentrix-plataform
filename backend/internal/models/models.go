package models

import (
	"time"
)

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // "admin", "player"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ApiKey struct {
	ID         int        `json:"id"`
	UserID     int        `json:"user_id"`
	KeyHash    string     `json:"-"`
	KeyPrefix  string     `json:"key_prefix"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Team struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Affiliation string    `json:"affiliation"`
	OwnerID     int       `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type Arena struct {
	ID          int                    `json:"id"`
	Slug        string                 `json:"slug"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	GameType    string                 `json:"game_type"`
	MaxPlayers  int                    `json:"max_players"`
	MaxTicks    int                    `json:"max_ticks"`
	ConfigJSON  map[string]interface{} `json:"config_json"`
	IsActive    bool                   `json:"is_active"`
	Frozen      bool                   `json:"frozen"`
	CreatedAt   time.Time              `json:"created_at"`
}

type AgentVersion struct {
	ID            int       `json:"id"`
	TeamID        int       `json:"team_id"`
	TeamName      string    `json:"team_name,omitempty"`
	ArenaID       int       `json:"arena_id"`
	Name          string    `json:"name"`
	Version       int       `json:"version"`
	Runtime       string    `json:"runtime"`
	Entrypoint    string    `json:"entrypoint"`
	ArtifactPath  string    `json:"-"`
	SHA256        string    `json:"sha256"`
	Status        string    `json:"status"` // "pending_validation", "active", "disqualified", "retired"
	FailureReason *string   `json:"failure_reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type LadderEntry struct {
	ID                 int        `json:"id"`
	ArenaID            int        `json:"arena_id"`
	AgentVersionID     int        `json:"agent_version_id"`
	AgentName          string     `json:"agent_name"`
	TeamID             int        `json:"team_id"`
	TeamName           string     `json:"team_name"`
	RatingMu           float64    `json:"rating_mu"`
	RatingSigma        float64    `json:"rating_sigma"`
	DisplayRating      int        `json:"display_rating"`
	MatchesPlayed      int        `json:"matches_played"`
	Wins               int        `json:"wins"`
	Kills              int        `json:"kills"`
	SurvivalTicksTotal int64      `json:"survival_ticks_total"`
	LastMatchAt        *time.Time `json:"last_match_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Status             string     `json:"status"` // agent status
	WinRate            float64    `json:"win_rate"`
	AvgSurvivalTicks   float64    `json:"avg_survival_ticks"`
}

type Match struct {
	ID            int                `json:"id"`
	ArenaID       int                `json:"arena_id"`
	ArenaName     string             `json:"arena_name,omitempty"`
	Seed          int64              `json:"seed"`
	Status        string             `json:"status"` // "scheduled", "running", "finished", "failed"
	TicksPlayed   int                `json:"ticks_played"`
	WinnerAgentID *int               `json:"winner_agent_id,omitempty"`
	WinnerName    string             `json:"winner_name,omitempty"`
	ErrorMessage  *string            `json:"error_message,omitempty"`
	ExecutionLog  *string            `json:"execution_log,omitempty"`
	StartedAt     *time.Time         `json:"started_at,omitempty"`
	FinishedAt    *time.Time         `json:"finished_at,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	Participants  []MatchParticipant `json:"participants,omitempty"`
	ReplayURL     string             `json:"replay_url,omitempty"`
}

type MatchParticipant struct {
	ID                     int     `json:"id"`
	MatchID                int     `json:"match_id"`
	AgentVersionID         int     `json:"agent_version_id"`
	AgentName              string  `json:"agent_name"`
	TeamName               string  `json:"team_name"`
	Seat                   int     `json:"seat"`
	RankPlace              int     `json:"rank_place"`
	Kills                  int     `json:"kills"`
	SurvivalTicks          int     `json:"survival_ticks"`
	Score                  float64 `json:"score"`
	Disqualified           bool    `json:"disqualified"`
	DisqualificationReason *string `json:"disqualification_reason,omitempty"`
	OldRating              float64 `json:"old_rating"`
	NewRating              float64 `json:"new_rating"`
	RatingDelta            float64 `json:"rating_delta"`
}

type Replay struct {
	ID          int                    `json:"id"`
	MatchID     int                    `json:"match_id"`
	FilePath    string                 `json:"-"`
	SHA256      string                 `json:"sha256"`
	SizeBytes   int64                  `json:"size_bytes"`
	TickCount   int                    `json:"tick_count"`
	SummaryJSON map[string]interface{} `json:"summary_json"`
	CreatedAt   time.Time              `json:"created_at"`
}

type AuditLog struct {
	ID          int                    `json:"id"`
	UserID      *int                   `json:"user_id,omitempty"`
	Username    string                 `json:"username,omitempty"`
	Action      string                 `json:"action"`
	TargetType  string                 `json:"target_type"`
	TargetID    string                 `json:"target_id"`
	IPAddress   string                 `json:"ip_address"`
	DetailsJSON map[string]interface{} `json:"details_json"`
	CreatedAt   time.Time              `json:"created_at"`
}

type SystemSetting struct {
	Key         string                 `json:"key"`
	ValueJSON   map[string]interface{} `json:"value_json"`
	Description string                 `json:"description"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Request and Response DTOs
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type TriggerMatchRequest struct {
	ArenaID         int    `json:"arena_id"`
	AgentVersionIDs []int  `json:"agent_version_ids,omitempty"` // empty to auto-fill
	Seed            *int64 `json:"seed,omitempty"`
}

type DisqualifyAgentRequest struct {
	Reason string `json:"reason"`
}

type SystemStatusResponse struct {
	Status           string    `json:"status"`
	DatabaseHealthy  bool      `json:"database_healthy"`
	ArbiterHealthy   bool      `json:"arbiter_healthy"`
	ArbiterPath      string    `json:"arbiter_path"`
	MatchmakerActive bool      `json:"matchmaker_active"`
	RunningMatches   int       `json:"running_matches"`
	ServerTime       time.Time `json:"server_time"`
}
