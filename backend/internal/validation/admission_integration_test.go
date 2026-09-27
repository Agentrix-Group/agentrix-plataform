package validation

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRealSandboxArbiterAdmission(t *testing.T) {
	if os.Getenv("AGENTRIX_TEST_SANDBOX") != "1" {
		t.Skip("real sandbox opt-in required")
	}
	bin := os.Getenv("AGENTRIX_TEST_ARBITER_PATH")
	if bin == "" {
		t.Fatal("AGENTRIX_TEST_ARBITER_PATH required")
	}
	dir := t.TempDir()
	for _, name := range []string{"agent.py", "agentrix.json"} {
		data, err := os.ReadFile(filepath.Join("../../../bots/heuristic_bot", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := InspectBotDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := TestBotProtocol(dir, manifest, bin); err != nil {
		t.Fatal(err)
	}
	// READY-only substitutions must never pass the real action loop.
	bad := []byte("import sys,json\nfor line in sys.stdin:\n print(json.dumps({'status':'READY'}),flush=True)\n")
	if err := os.WriteFile(filepath.Join(dir, "agent.py"), bad, 0644); err != nil {
		t.Fatal(err)
	}
	if err := TestBotProtocol(dir, manifest, bin); err == nil {
		t.Fatal("READY-only fake bot admitted")
	}
	for name, script := range map[string]string{
		"oversized-line":   "import sys\nsys.stdin.readline()\nprint('x'*65537,flush=True)\n",
		"sustained-output": "import sys\nsys.stdin.readline()\nwhile True: print('x'*60000,flush=True)\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, "agent.py"), []byte(script), 0644); err != nil {
				t.Fatal(err)
			}
			started := time.Now()
			if err := TestBotProtocol(dir, manifest, bin); err == nil {
				t.Fatal("output-flooding bot admitted")
			}
			if time.Since(started) >= 8*time.Second {
				t.Fatal("output flood was not rejected before the normal 10-second warmup deadline")
			}
		})
	}
}

func TestRealSandboxONNXAdmission(t *testing.T) {
	if os.Getenv("AGENTRIX_TEST_ONNX") != "1" {
		t.Skip("AGENTRIX_TEST_ONNX=1 and ML runtime required")
	}
	bin := os.Getenv("AGENTRIX_TEST_ARBITER_PATH")
	if bin == "" {
		t.Fatal("arbiter path required")
	}
	dir := t.TempDir()
	for _, name := range []string{"agent.py", "agentrix.json", "model.onnx"} {
		data, err := os.ReadFile(filepath.Join("../../../bots/onnx_bot", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := InspectBotDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := TestBotProtocol(dir, manifest, bin); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "model.onnx"), []byte("corrupt model"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := TestBotProtocol(dir, manifest, bin); err == nil {
		t.Fatal("corrupt ONNX model admitted")
	}
}
