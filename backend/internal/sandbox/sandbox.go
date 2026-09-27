// Package sandbox is the sole launcher for participant code. Missing isolation
// infrastructure is an error; there is deliberately no host execution fallback.
package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ValidateHost exercises the same isolation stack before the API, migrations or
// queue become available. A container without a delegated user manager/cgroup
// must not advertise an operational service that cannot admit or run bots.
func ValidateHost(parent context.Context) error {
	if os.Getenv("AGENTRIX_DEV_MODE") == "true" {
		return nil
	}
	root := os.Getenv("AGENTRIX_RUNTIME_ROOT")
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) == "/" {
		return fmt.Errorf("AGENTRIX_RUNTIME_ROOT must point to a dedicated runtime filesystem")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved == "/" {
		return fmt.Errorf("sandbox runtime filesystem is unavailable or unsafe")
	}
	expectedDigest := os.Getenv("AGENTRIX_RUNTIME_SHA256")
	if len(expectedDigest) != 64 || strings.ToLower(expectedDigest) != expectedDigest {
		return fmt.Errorf("AGENTRIX_RUNTIME_SHA256 must pin the dedicated runtime filesystem")
	}
	if _, err := hex.DecodeString(expectedDigest); err != nil {
		return fmt.Errorf("AGENTRIX_RUNTIME_SHA256 is not a SHA-256 digest")
	}
	actualDigest, err := RuntimeDigest(resolved)
	if err != nil {
		return fmt.Errorf("hash sandbox runtime: %w", err)
	}
	if actualDigest != expectedDigest {
		return fmt.Errorf("sandbox runtime digest mismatch")
	}
	python := filepath.Join(resolved, "usr/bin/python3")
	info, err := os.Stat(python)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("sandbox runtime must include executable usr/bin/python3")
	}
	for _, name := range []string{"systemd-run", "bwrap"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("sandbox prerequisite %s is unavailable: %w", name, err)
		}
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("create sandbox preflight identity: %w", err)
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	unit := "agentrix-preflight-" + hex.EncodeToString(nonce[:])
	args := []string{
		"--user", "--quiet", "--wait", "--collect", "--unit=" + unit,
		"--property=MemoryMax=64M", "--property=MemorySwapMax=0", "--property=TasksMax=16",
		"--property=CPUQuota=100%", "--property=RuntimeMaxSec=10s", "--property=KillMode=control-group",
		"--property=LimitFSIZE=1048576", "--property=LimitNOFILE=64", "--",
		"bwrap", "--unshare-all", "--unshare-user", "--unshare-cgroup", "--disable-userns",
		"--uid", "65534", "--gid", "65534", "--cap-drop", "ALL", "--die-with-parent", "--new-session", "--clearenv",
		"--ro-bind", resolved, "/", "--proc", "/proc", "--dev", "/dev", "--remount-ro", "/dev",
		"--size", "1048576", "--tmpfs", "/tmp", "--chdir", "/tmp", "--setenv", "PATH", "/usr/bin:/bin",
		"--", "/usr/bin/python3", "-I", "-c", "assert 1 + 1 == 2",
	}
	cmd := exec.CommandContext(ctx, "systemd-run", args...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "XDG_RUNTIME_DIR=" + os.Getenv("XDG_RUNTIME_DIR")}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("sandbox host preflight timed out")
		}
		return fmt.Errorf("systemd/cgroup/Bubblewrap sandbox preflight failed: %s", boundedDiagnostic(output, 2048))
	}
	return nil
}

func boundedDiagnostic(value []byte, limit int) string {
	if len(value) > limit {
		value = value[:limit]
	}
	return strings.TrimSpace(string(value))
}

func Command(botDir, entrypoint string) ([]string, error) {
	return CommandWithLifetime(botDir, entrypoint, 700)
}

func CommandWithLifetime(botDir, entrypoint string, seconds int) ([]string, error) {
	if seconds < 1 || seconds > 2840 {
		return nil, fmt.Errorf("sandbox lifetime outside supported match budget")
	}
	if os.Getenv("AGENTRIX_DEV_MODE") == "true" {
		parts := strings.Fields(entrypoint)
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty entrypoint")
		}
		abs, err := filepath.Abs(botDir)
		if err != nil {
			return nil, err
		}
		return []string{"/bin/sh", "-c", fmt.Sprintf("cd '%s' && exec %s", abs, entrypoint)}, nil
	}
	root := os.Getenv("AGENTRIX_RUNTIME_ROOT")
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) == "/" {
		return nil, fmt.Errorf("AGENTRIX_RUNTIME_ROOT must identify a dedicated runtime filesystem")
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("runtime filesystem unavailable")
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil || root == "/" {
		return nil, fmt.Errorf("runtime filesystem must not resolve to host root")
	}
	for _, executable := range []string{"systemd-run", "bwrap"} {
		if _, err := exec.LookPath(executable); err != nil {
			return nil, err
		}
	}
	abs, err := filepath.Abs(botDir)
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(entrypoint)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty entrypoint")
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	args := []string{"systemd-run", "--user", "--quiet", "--pipe", "--wait", "--collect", "--unit=agentrix-bot-" + hex.EncodeToString(token[:]),
		"--property=MemoryMax=2G", "--property=MemorySwapMax=0", "--property=TasksMax=64",
		"--property=CPUQuota=100%", fmt.Sprintf("--property=RuntimeMaxSec=%d", seconds), "--property=KillMode=control-group",
		"--property=LimitFSIZE=16777216", "--property=LimitNOFILE=64",
		"bwrap", "--unshare-all", "--unshare-user", "--unshare-cgroup", "--disable-userns", "--uid", "65534", "--gid", "65534", "--cap-drop", "ALL", "--die-with-parent", "--new-session", "--clearenv",
		"--ro-bind", root, "/", "--ro-bind", abs, "/bot", "--proc", "/proc", "--dev", "/dev", "--remount-ro", "/dev",
		"--size", "16777216", "--tmpfs", "/tmp", "--chdir", "/bot", "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin",
		"--setenv", "HOME", "/tmp", "--setenv", "PYTHONUNBUFFERED", "1",
		"--setenv", "OMP_NUM_THREADS", "1", "--setenv", "OPENBLAS_NUM_THREADS", "1", "--"}
	return append(args, parts...), nil
}

// Stop terminates the entire cgroup even if its launcher was killed earlier.
func Stop(args []string) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--unit=agentrix-bot-") {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = exec.CommandContext(ctx, "systemctl", "--user", "stop", strings.TrimPrefix(arg, "--unit=")).Run()
			cancel()
		}
	}
}

func ShellCommand(botDir, entrypoint string) (string, error) {
	args, err := Command(botDir, entrypoint)
	if err != nil {
		return "", err
	}
	for i, arg := range args {
		args[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(args, " "), nil
}
