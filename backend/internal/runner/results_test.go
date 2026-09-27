package runner

import (
	"agentrix/backend/internal/artifacts"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func resultFixture() ArbiterResults {
	winner := 0
	result := ArbiterResults{Seed: 42, Ticks: 100, ScoreVersion: scoreVersion, EngineVersion: "0.2.0", WinnerID: &winner, Winner: "winner"}
	result.RulesVersion = "agentrix-rules-v1"
	result.EffectiveConfig = json.RawMessage(`{"match_rules":{"duration":180,"walls":9,"zone":true,"show_vision":true,"mobs_as_kills":false},"mobs":{"count":18,"hp":24,"dmg":6,"speed":55,"aggro":140,"xp":25,"respawn":2},"xp":{"base":40,"growth":1.3,"per_player_kill":60,"heal_pct":20},"up":{"vida":25,"velocidad":10,"vision":30,"dano":2}}`)
	for i := 0; i < 5; i++ {
		name := "other"
		if i == 0 {
			name = "winner"
		}
		result.Players = append(result.Players, ArbiterPlayerReport{ID: i, Name: name})
		result.Ranking = append(result.Ranking, ArbiterRankItem{ID: i, Place: i + 1, SurvivalPlace: i + 1, Score: float64(100 - i*20)})
	}
	return result
}

func TestRealArbiterArtifactsAgreeWithRunner(t *testing.T) {
	for _, seed := range []uint32{0, 42} {
		t.Run(fmt.Sprint(seed), func(t *testing.T) { testRealArbiterArtifacts(t, seed) })
	}
}

func TestRealArbiterRejectsInvalidSeed(t *testing.T) {
	bin := os.Getenv("AGENTRIX_TEST_ARBITER_PATH")
	if bin == "" {
		t.Skip("AGENTRIX_TEST_ARBITER_PATH required for Rust/Go integration")
	}
	for _, seed := range []string{"-1", "4294967296", "invalid", ""} {
		t.Run(seed, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, bin, "--seed", seed).CombinedOutput()
			if err == nil || !strings.Contains(string(output), "Invalid seed") {
				t.Fatalf("invalid seed accepted or unrelated error: %v %s", err, output)
			}
		})
	}
}

func testRealArbiterArtifacts(t *testing.T, seed uint32) {
	bin := os.Getenv("AGENTRIX_TEST_ARBITER_PATH")
	if bin == "" {
		t.Skip("AGENTRIX_TEST_ARBITER_PATH required for Rust/Go integration")
	}
	bot, err := filepath.Abs("testdata/protocol_bot.py")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privateArbiter := filepath.Join(dir, "arbiter")
	if _, err := artifacts.SnapshotExecutable(bin, privateArbiter); err != nil {
		t.Fatal(err)
	}
	bin = privateArbiter
	resultsPath := filepath.Join(dir, "results.json")
	replayPath := filepath.Join(dir, "replay.json")
	args := []string{"--seed", fmt.Sprint(seed), "--duration", "20", "--warmup-ms", "1000", "--tick-ms", "50", "--out-results", resultsPath, "--out-replay", replayPath}
	for i := 0; i < 5; i++ {
		args = append(args, fmt.Sprintf("--b%d", i), "python3 '"+bot+"'")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, bin, args...).CombinedOutput(); err != nil {
		t.Fatalf("arbiter: %v %s", err, output)
	}
	data, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatal(err)
	}
	var results ArbiterResults
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatal(err)
	}
	replay, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResultReplay(results, replay, int64(seed), 5); err != nil {
		t.Fatal(err)
	}
	if results.Ticks == 0 || results.WinnerID == nil {
		t.Fatal("fixture did not produce a competitive match")
	}
}

func replayFor(t *testing.T, r ArbiterResults) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{"seed": r.Seed, "ticks": r.Ticks, "score_version": r.ScoreVersion, "engine_version": r.EngineVersion, "event_format": "delta-v1", "entity_format": "keyframe-delta-v1", "ranking": r.Ranking, "rules_version": r.RulesVersion, "effective_config": r.EffectiveConfig})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRejectDivergentEffectiveRules(t *testing.T) {
	result := resultFixture()
	var replay map[string]interface{}
	if err := json.Unmarshal(replayFor(t, result), &replay); err != nil {
		t.Fatal(err)
	}
	replay["effective_config"].(map[string]interface{})["match_rules"].(map[string]interface{})["duration"] = 20
	data, err := json.Marshal(replay)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResultReplay(result, data, 42, 5); err == nil {
		t.Fatal("different replay rules accepted")
	}
}

func TestRejectUnsupportedVersionsEvenWhenArtifactsAgree(t *testing.T) {
	for _, value := range []interface{}{nil, "", 42} {
		result := resultFixture()
		var replay map[string]interface{}
		if err := json.Unmarshal(replayFor(t, result), &replay); err != nil {
			t.Fatal(err)
		}
		replay["event_format"] = value
		data, err := json.Marshal(replay)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateResultReplay(result, data, 42, 5); err == nil {
			t.Fatalf("invalid explicit event format accepted: %#v", value)
		}
	}
	for _, version := range []string{"", "0.3.0", "unrecognized"} {
		t.Run(version, func(t *testing.T) {
			result := resultFixture()
			result.EngineVersion = version
			if err := validateResultReplay(result, replayFor(t, result), 42, 5); err == nil {
				t.Fatal("mutually agreeing unsupported engine artifacts accepted")
			}
		})
	}
	for _, format := range []string{"", "delta-v1", "delta-v2"} {
		t.Run("events:"+format, func(t *testing.T) {
			result := resultFixture()
			var replay map[string]interface{}
			if err := json.Unmarshal(replayFor(t, result), &replay); err != nil {
				t.Fatal(err)
			}
			if format != "" {
				replay["event_format"] = format
			}
			data, err := json.Marshal(replay)
			if err != nil {
				t.Fatal(err)
			}
			err = validateResultReplay(result, data, 42, 5)
			if format == "delta-v2" && err == nil {
				t.Fatal("unsupported replay committed")
			}
			if format != "delta-v2" && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRejectMissingOrUnsupportedEntityReplayFormat(t *testing.T) {
	for _, format := range []interface{}{nil, "", 42, "full-v1", "keyframe-delta-v2"} {
		t.Run(fmt.Sprint(format), func(t *testing.T) {
			result := resultFixture()
			var replay map[string]interface{}
			if err := json.Unmarshal(replayFor(t, result), &replay); err != nil {
				t.Fatal(err)
			}
			if format == nil {
				delete(replay, "entity_format")
			} else {
				replay["entity_format"] = format
			}
			data, err := json.Marshal(replay)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateResultReplay(result, data, 42, 5); err == nil {
				t.Fatal("missing or unsupported entity format accepted")
			}
		})
	}
}

func TestRejectDivergentOrIncompleteOutcomes(t *testing.T) {
	good := resultFixture()
	if err := validateResultReplay(good, replayFor(t, good), 42, 5); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*ArbiterResults)
	}{
		{"missing-seat", func(r *ArbiterResults) { r.Players = r.Players[:4] }},
		{"duplicate-seat", func(r *ArbiterResults) { r.Ranking[1].ID = 0 }},
		{"incorrect-winner", func(r *ArbiterResults) { id := 1; r.WinnerID = &id }},
		{"wrong-seed", func(r *ArbiterResults) { r.Seed = 99 }},
		{"wrong-version", func(r *ArbiterResults) { r.ScoreVersion = "unknown" }},
		{"wrong-rules", func(r *ArbiterResults) { r.RulesVersion = "unknown" }},
		{"missing-rules", func(r *ArbiterResults) { r.EffectiveConfig = nil }},
		{"invalid-death", func(r *ArbiterResults) { tick := 101; r.Players[0].DeathTick = &tick }},
		{"disqualified-points", func(r *ArbiterResults) { r.Players[0].Disqualified = true; r.Ranking[0].Disqualified = true }},
		{"incorrect-order", func(r *ArbiterResults) { r.Ranking[1].Score = 101 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := resultFixture()
			tc.mutate(&r)
			if err := validateResultReplay(r, replayFor(t, r), 42, 5); err == nil {
				t.Fatal("invalid outcome accepted")
			}
		})
	}
	replay := resultFixture()
	replay.Ranking[0].Score = 99
	if err := validateResultReplay(good, replayFor(t, replay), 42, 5); err == nil {
		t.Fatal("divergent replay accepted")
	}
}
