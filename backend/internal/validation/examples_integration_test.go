package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackagedExamplesAdmittedInRealSandbox(t *testing.T) {
	packages := os.Getenv("AGENTRIX_TEST_PACKAGES_DIR")
	if packages == "" {
		t.Skip("AGENTRIX_TEST_PACKAGES_DIR required")
	}
	if os.Getenv("AGENTRIX_TEST_SANDBOX") != "1" {
		t.Fatal("real sandbox must be enabled for example acceptance")
	}
	bin := os.Getenv("AGENTRIX_TEST_ARBITER_PATH")
	if bin == "" {
		t.Fatal("arbiter path required")
	}
	for _, name := range []string{"heuristic_bot", "cpp_bot", "onnx_bot"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := SafelyExtractZip(filepath.Join(packages, name+".zip"), dir); err != nil {
				t.Fatal(err)
			}
			manifest, err := InspectBotDirectory(dir)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DigestPackage(dir); err != nil {
				t.Fatal(err)
			}
			if err := TestBotProtocol(dir, manifest, bin); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dir, "run.sh")); !os.IsNotExist(err) {
				t.Fatal("legacy run script entered package")
			}
		})
	}
}
