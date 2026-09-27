package sandbox

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCgroupLimitsAndDescendantCleanup(t *testing.T) {
	if os.Getenv("AGENTRIX_TEST_SANDBOX") != "1" {
		t.Skip("real sandbox probe opt-in required")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	script := `import os,time
children=0
for _ in range(80):
 try: pid=os.fork()
 except OSError: break
 if pid==0:
  os.setsid()
  time.sleep(600)
  os._exit(0)
 children+=1
assert 0 < children < 80
print('LIMITED',flush=True)
time.sleep(600)
`
	if err := os.WriteFile(filepath.Join(dir, "probe.py"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	args, err := Command(dir, "python3 probe.py")
	if err != nil {
		t.Fatal(err)
	}
	defer Stop(args)
	var unit string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--unit=") {
			unit = strings.TrimPrefix(arg, "--unit=")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { Stop(args); _ = cmd.Wait() }()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "LIMITED" {
		t.Fatal("PID quota probe failed")
	}
	value, err := exec.CommandContext(ctx, "systemctl", "--user", "show", unit, "--property=ControlGroup", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	group := strings.TrimSpace(string(value))
	if !strings.HasPrefix(group, "/user.slice/") {
		t.Fatalf("unexpected cgroup: %q", group)
	}
	root := filepath.Join("/sys/fs/cgroup", group)
	for file, expected := range map[string]string{"memory.max": "2147483648", "memory.swap.max": "0", "pids.max": "64", "cpu.max": "100000 100000"} {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(data)) != expected {
			t.Fatalf("%s: %s", file, data)
		}
	}
	procs, err := os.ReadFile(filepath.Join(root, "cgroup.procs"))
	if err != nil || len(strings.Fields(string(procs))) < 2 {
		t.Fatalf("descendants absent: %s %v", procs, err)
	}
	Stop(args)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(root, "cgroup.procs"))
		if os.IsNotExist(err) || err == nil && len(strings.Fields(string(data))) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("processes remain in cgroup after Stop")
}

func TestIsolationProbe(t *testing.T) {
	if os.Getenv("AGENTRIX_TEST_SANDBOX") != "1" {
		t.Skip("AGENTRIX_TEST_SANDBOX=1 and dedicated runtime required")
	}
	dir := t.TempDir()
	script := `import os,socket
assert os.getuid()==65534 and os.getgid()==65534
assert 'DB_PASS' not in os.environ and 'AGENTRIX_PROBE_SECRET' not in os.environ
assert not os.path.exists('/home/f4nk1')
assert not os.path.exists('/run/user/1000/bus')
assert os.listdir('/proc').count('1') == 1
assert len([n for n in os.listdir('/proc') if n.isdigit()]) <= 3
for path in ['/bot/forbidden','/usr/forbidden','/dev/forbidden']:
 try:
  open(path,'w').write('bad')
  raise AssertionError('write outside scratch allowed')
 except OSError: pass
s=socket.socket()
s.settimeout(.2)
try:
 s.connect(('1.1.1.1',80))
 raise AssertionError('external network allowed')
except OSError: pass
try:
 with open('/tmp/quota','wb') as f:
  for _ in range(32): f.write(b'x'*(1024*1024))
 raise AssertionError('scratch quota missing')
except OSError: pass
print('ISOLATED',flush=True)
`
	if err := os.WriteFile(filepath.Join(dir, "probe.py"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	// Inner UID must be able to traverse its read-only package, like real uploads.
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	args, err := Command(dir, "python3 probe.py")
	if err != nil {
		t.Fatal(err)
	}
	defer Stop(args)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "DB_PASS=must-not-leak", "AGENTRIX_PROBE_SECRET=must-not-leak")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v: %s", err, output)
	}
	if !strings.Contains(string(output), "ISOLATED") {
		t.Fatalf("probe not executed: %s", output)
	}
}
