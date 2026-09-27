package runner

import (
	"agentrix/backend/internal/sandbox"
	"agentrix/backend/internal/validation"
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

func TestMixedExamplesCompleteIsolatedMatch(t *testing.T) {
	packages := os.Getenv("AGENTRIX_TEST_PACKAGES_DIR")
	if packages == "" {
		t.Skip("real packaged examples required")
	}
	bin := os.Getenv("AGENTRIX_TEST_ARBITER_PATH")
	if bin == "" || os.Getenv("AGENTRIX_TEST_SANDBOX") != "1" {
		t.Fatal("real arbiter and sandbox required")
	}
	output := t.TempDir()
	resultPath := filepath.Join(output, "results.json")
	replayPath := filepath.Join(output, "replay.json")
	args := []string{"--seed", "42", "--duration", "20", "--out-results", resultPath, "--out-replay", replayPath}
	for seat, name := range []string{"heuristic_bot", "cpp_bot", "onnx_bot", "cpp_bot", "heuristic_bot"} {
		dir := t.TempDir()
		if err := validation.SafelyExtractZip(filepath.Join(packages, name+".zip"), dir); err != nil {
			t.Fatal(err)
		}
		manifest, err := validation.InspectBotDirectory(dir)
		if err != nil {
			t.Fatal(err)
		}
		launch, err := sandbox.Command(dir, manifest.Entrypoint)
		if err != nil {
			t.Fatal(err)
		}
		defer sandbox.Stop(launch)
		quoted := make([]string, len(launch))
		for i, arg := range launch {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
		args = append(args, fmt.Sprintf("--b%d", seat), strings.Join(quoted, " "))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "XDG_RUNTIME_DIR=" + os.Getenv("XDG_RUNTIME_DIR")}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("isolated match: %v: %s", err, output)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	var result ArbiterResults
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	replay, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResultReplay(result, replay, 42, 5); err != nil {
		t.Fatal(err)
	}
	for _, player := range result.Players {
		if player.Disqualified {
			t.Fatalf("example seat %d disqualified", player.ID)
		}
	}
	if result.Ticks == 0 || result.WinnerID == nil {
		t.Fatal("isolated match produced no competitive result")
	}
}
