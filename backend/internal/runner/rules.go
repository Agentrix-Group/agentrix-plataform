package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	"strconv"
)

const rulesVersion = "agentrix-rules-v1"
const defaultRules = `{"match_rules":{"duration":180,"walls":9,"zone":true,"show_vision":true,"mobs_as_kills":false},"mobs":{"count":18,"hp":24,"dmg":6,"speed":55,"aggro":140,"xp":25,"respawn":2},"xp":{"base":40,"growth":1.3,"per_player_kill":60,"heal_pct":20},"up":{"vida":25,"velocidad":10,"vision":30,"dano":2}}`

type ruleRange struct {
	Min, Max float64
	Integer  bool
}

var ruleRanges = map[string]ruleRange{
	"match_rules.duration": {20, 900, false}, "match_rules.walls": {0, 30, true},
	"mobs.count": {0, 60, true}, "mobs.hp": {1, 1000, false}, "mobs.dmg": {0, 200, false}, "mobs.speed": {0, 300, false}, "mobs.aggro": {0, 600, false}, "mobs.xp": {0, 1000, false}, "mobs.respawn": {.5, 60, false},
	"xp.base": {5, 2000, false}, "xp.growth": {1, 3, false}, "xp.per_player_kill": {0, 2000, false}, "xp.heal_pct": {0, 100, false},
	"up.vida": {0, 500, false}, "up.velocidad": {0, 200, false}, "up.vision": {0, 400, false}, "up.dano": {0, 100, false},
}

// Canonicalize the complete schema in the engine's float32 representation.
// Stored bytes, not JSONB formatting, define the SHA-256 input.
func canonicalRules(raw []byte) ([]byte, string, error) {
	if len(raw) > 16384 {
		return nil, "", errors.New("rules exceed 16 KiB")
	}
	var defaults, input map[string]map[string]interface{}
	if err := json.Unmarshal([]byte(defaultRules), &defaults); err != nil {
		return nil, "", err
	}
	if err := json.Unmarshal(raw, &input); err != nil || input == nil {
		return nil, "", errors.New("rules must be an object")
	}
	for section, values := range input {
		target, ok := defaults[section]
		if !ok || values == nil {
			return nil, "", fmt.Errorf("unsupported rules section %s", section)
		}
		for key, value := range values {
			if _, ok := target[key]; !ok {
				return nil, "", fmt.Errorf("unsupported rule %s.%s", section, key)
			}
			target[key] = value
		}
	}
	for section, values := range defaults {
		for key, value := range values {
			path := section + "." + key
			bounds, numeric := ruleRanges[path]
			if !numeric {
				if _, ok := value.(bool); !ok {
					return nil, "", fmt.Errorf("%s must be boolean", path)
				}
				continue
			}
			number, ok := value.(float64)
			if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < bounds.Min || number > bounds.Max || bounds.Integer && math.Trunc(number) != number {
				return nil, "", fmt.Errorf("invalid rule %s", path)
			}
			if !bounds.Integer {
				normalized, _ := strconv.ParseFloat(strconv.FormatFloat(float64(float32(number)), 'g', -1, 32), 64)
				values[key] = normalized
			}
		}
	}
	data, err := json.Marshal(defaults)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

func snapshotArenaRules(ctx context.Context, tx pgx.Tx, arena int) ([]byte, string, error) {
	var game string
	var players, ticks int
	var raw []byte
	var active bool
	if err := tx.QueryRow(ctx, "SELECT game_type,max_players,max_ticks,config_json,is_active FROM arenas WHERE id=$1 FOR SHARE", arena).Scan(&game, &players, &ticks, &raw, &active); err != nil {
		return nil, "", err
	}
	if !active || game != "battle_royale_5p" || players != 5 {
		return nil, "", errors.New("unsupported or inactive arena")
	}
	if ticks < 1200 || ticks > 54000 || ticks%60 != 0 {
		return nil, "", errors.New("max_ticks must be 1200..54000 and a multiple of 60")
	}
	data, _, err := canonicalRules(raw)
	if err != nil {
		return nil, "", err
	}
	var config map[string]map[string]interface{}
	_ = json.Unmarshal(data, &config)
	var input map[string]map[string]interface{}
	_ = json.Unmarshal(raw, &input)
	duration := float64(ticks) / 60
	if explicit, ok := input["match_rules"]["duration"]; ok && explicit != duration {
		return nil, "", errors.New("duration disagrees with arena max_ticks")
	}
	config["match_rules"]["duration"] = duration
	data, _ = json.Marshal(config)
	return canonicalRules(data)
}

func validateSnapshot(data []byte, hash string) ([]byte, error) {
	canonical, actual, err := canonicalRules(data)
	if err != nil {
		return nil, err
	}
	if actual != hash || !bytes.Equal(data, canonical) {
		return nil, errors.New("rules snapshot checksum or canonical encoding mismatch")
	}
	return canonical, nil
}
