package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
)

const scoreVersion = "agentrix-score-v1"
const engineVersion = "0.2.0"

// The engine is the only scoring implementation. Reject incomplete or divergent
// artifacts rather than infer default ranks and silently change the ladder.
func validateResultReplay(result ArbiterResults, replayBytes []byte, seed int64, seats int) error {
	if result.ScoreVersion != scoreVersion || result.EngineVersion != engineVersion {
		return errors.New("unsupported result scoring/engine version")
	}
	if result.RulesVersion != "agentrix-rules-v1" {
		return errors.New("unsupported rules version")
	}
	var effective map[string]interface{}
	if err := json.Unmarshal(result.EffectiveConfig, &effective); err != nil || len(effective) == 0 {
		return errors.New("missing or invalid effective rules")
	}
	if result.Seed != seed || result.Ticks <= 0 || result.Ticks > 54000 {
		return errors.New("invalid result seed or tick count")
	}
	if len(result.Players) != seats || len(result.Ranking) != seats {
		return errors.New("incomplete result seats")
	}
	players := make(map[int]ArbiterPlayerReport, seats)
	for _, p := range result.Players {
		if p.ID < 0 || p.ID >= seats {
			return errors.New("invalid player seat")
		}
		if _, ok := players[p.ID]; ok {
			return errors.New("duplicate player seat")
		}
		if p.DeathTick != nil && (*p.DeathTick < 0 || *p.DeathTick > result.Ticks) {
			return errors.New("invalid death tick")
		}
		players[p.ID] = p
	}
	seen := make(map[int]bool, seats)
	var winner *int
	for i, rk := range result.Ranking {
		p, ok := players[rk.ID]
		if !ok || seen[rk.ID] || rk.Place < 1 || rk.Place > seats || rk.SurvivalPlace < 1 || rk.SurvivalPlace > seats {
			return errors.New("invalid ranking seats or places")
		}
		seen[rk.ID] = true
		if rk.Disqualified != p.Disqualified || rk.Kills < 0 || math.IsNaN(rk.Score) || math.IsInf(rk.Score, 0) || rk.Score < 0 || rk.Score > 100 {
			return errors.New("invalid ranking outcome")
		}
		for _, part := range []float64{rk.KillPart, rk.SurvivalPart} {
			if math.IsNaN(part) || math.IsInf(part, 0) || part < 0 || part > 1 {
				return errors.New("invalid score component")
			}
		}
		if rk.Disqualified && (rk.Score != 0 || rk.Place != seats) {
			return errors.New("disqualified player has points")
		}
		if i > 0 {
			prev := result.Ranking[i-1]
			if (!rk.Disqualified && prev.Disqualified) || (!rk.Disqualified && (rk.Score > prev.Score || rk.Place < prev.Place)) {
				return errors.New("ranking order is inconsistent")
			}
		}
		if winner == nil && !rk.Disqualified {
			id := rk.ID
			winner = &id
			if rk.Place != 1 {
				return errors.New("winner does not have first place")
			}
		}
	}
	if !reflect.DeepEqual(winner, result.WinnerID) {
		return errors.New("winner disagrees with ranking")
	}
	if winner != nil && result.Winner != players[*winner].Name {
		return errors.New("winner name disagrees with player")
	}
	var replay struct {
		EventFormat     json.RawMessage   `json:"event_format"`
		EntityFormat    json.RawMessage   `json:"entity_format"`
		RulesVersion    string            `json:"rules_version"`
		EffectiveConfig json.RawMessage   `json:"effective_config"`
		Seed            int64             `json:"seed"`
		Ticks           int               `json:"ticks"`
		ScoreVersion    string            `json:"score_version"`
		EngineVersion   string            `json:"engine_version"`
		Ranking         []ArbiterRankItem `json:"ranking"`
	}
	if err := json.Unmarshal(replayBytes, &replay); err != nil {
		return fmt.Errorf("invalid replay JSON: %w", err)
	}
	var eventFormat string
	if err := json.Unmarshal(replay.EventFormat, &eventFormat); err != nil || eventFormat != "delta-v1" {
		return errors.New("unsupported replay event format")
	}
	var entityFormat string
	if err := json.Unmarshal(replay.EntityFormat, &entityFormat); err != nil || entityFormat != "keyframe-delta-v1" {
		return errors.New("unsupported replay entity format")
	}
	var replayConfig map[string]interface{}
	if err := json.Unmarshal(replay.EffectiveConfig, &replayConfig); err != nil || !reflect.DeepEqual(effective, replayConfig) || replay.RulesVersion != result.RulesVersion {
		return errors.New("replay and result rules disagree")
	}
	if replay.Seed != result.Seed || replay.Ticks != result.Ticks || replay.ScoreVersion != result.ScoreVersion || replay.EngineVersion != result.EngineVersion || !reflect.DeepEqual(replay.Ranking, result.Ranking) {
		return errors.New("replay and result disagree")
	}
	return nil
}
