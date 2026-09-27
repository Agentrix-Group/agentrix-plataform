import io
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
import unittest
from unittest.mock import patch

from local_supervisor import Supervisor, healthy


def command(code):
    return [sys.executable, "-c", code]


class SupervisorTests(unittest.TestCase):
    def supervisor(self, backend, frontend=None, health=lambda: False):
        return Supervisor(command(backend), command(frontend or "import time; time.sleep(60)"),
                          health, startup_timeout=0.2, grace_timeout=0.1, poll_interval=0.01)

    def test_backend_exit_fails_promptly_without_frontend(self):
        supervisor = self.supervisor("import sys; sys.exit(7)")
        start = time.monotonic()
        self.assertEqual(supervisor.run(), 7)
        self.assertLess(time.monotonic()-start, 2)
        self.assertEqual(supervisor.children, [])

    def test_unhealthy_backend_has_a_deadline(self):
        supervisor = self.supervisor("import time; time.sleep(60)")
        self.assertEqual(supervisor.run(), 1)
        self.assertEqual(supervisor.children, [])

    def test_failed_frontend_stops_backend_but_not_unrelated_process(self):
        unrelated = subprocess.Popen(command("import time; time.sleep(60)"))
        try:
            supervisor = self.supervisor("import time; time.sleep(60)", "import sys; sys.exit(9)", lambda: True)
            self.assertEqual(supervisor.run(), 9)
            self.assertIsNone(unrelated.poll())
        finally:
            unrelated.terminate()
            unrelated.wait()

    def test_requested_stop_preserves_signal_exit_code(self):
        supervisor = self.supervisor("import time; time.sleep(60)")
        supervisor.request_stop(signal.SIGTERM)
        self.assertEqual(supervisor.run(), 143)
        self.assertEqual(supervisor.children, [])

    def test_sigterm_ignoring_child_is_killed_and_reaped(self):
        supervisor = self.supervisor("import time; time.sleep(60)")
        child = supervisor.launch(command("import signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); print('ready',flush=True); time.sleep(60)"))
        # A separate child verifies cleanup doesn't rely on leaders cooperating.
        time.sleep(0.1)
        start = time.monotonic()
        supervisor.close()
        self.assertLess(time.monotonic()-start, 2)
        self.assertIsNotNone(child.returncode)
        with self.assertRaises(ProcessLookupError):
            os.kill(child.pid, 0)
        supervisor.close()

    def test_health_rejects_http_errors_false_status_and_oversize(self):
        class Response(io.BytesIO):
            status = 200
        for body, status, expected in [
            (b'{"database_healthy":true,"arbiter_healthy":true}',200,True),
            (b'{"database_healthy":false,"arbiter_healthy":true}',200,False),
            (b'{"database_healthy":true,"arbiter_healthy":true}',500,False),
            (b'[]',200,False), (b'bad-json',200,False), (b'x'*65537,200,False),
        ]:
            response = Response(body)
            response.status = status
            with patch("local_supervisor.urllib.request.urlopen",return_value=response):
                self.assertEqual(healthy("http://fixture"),expected)

    def test_owned_descendant_process_is_stopped(self):
        supervisor = self.supervisor("import time; time.sleep(60)")
        child = supervisor.launch(command("import os,signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); os.fork(); time.sleep(60)"))
        try:
            children = Path(f"/proc/{child.pid}/task/{child.pid}/children")
            deadline = time.monotonic()+2
            descendant = None
            while time.monotonic() < deadline:
                ids = children.read_text().split()
                if ids:
                    descendant = int(ids[0])
                    break
                time.sleep(0.01)
            self.assertIsNotNone(descendant)
            supervisor.close()
            # A zombie awaiting init's reap is terminated, not a surviving bot.
            stat = Path(f"/proc/{descendant}/stat")
            deadline = time.monotonic() + 2
            while time.monotonic() < deadline:
                try:
                    state = stat.read_text().split(") ", 1)[1][0]
                    if state == "Z":
                        break
                except (FileNotFoundError, ProcessLookupError, IndexError):
                    break
                time.sleep(0.01)
            try:
                state = stat.read_text().split(") ", 1)[1][0]
                self.assertEqual(state, "Z")
            except (FileNotFoundError, ProcessLookupError):
                # The descendant was reaped completely by init, satisfying termination.
                pass
        finally:
            supervisor.close()

    def test_real_start_wrapper_rejects_missing_secret_without_waiting(self):
        env = os.environ.copy()
        env.pop("JWT_SECRET",None)
        env["PORT"]="8080"
        script = Path(__file__).resolve().parent / "start_all.sh"
        result = subprocess.run(["bash",str(script)],env=env,capture_output=True,text=True,timeout=3)
        self.assertEqual(result.returncode,1)
        self.assertIn("JWT_SECRET",result.stderr)

    def test_local_stack_rejects_port_that_disagrees_with_proxy(self):
        env = os.environ.copy()
        env["PORT"]="8085"
        script = Path(__file__).resolve().parent / "start_all.sh"
        result = subprocess.run(["bash",str(script)],env=env,capture_output=True,text=True,timeout=3)
        self.assertEqual(result.returncode,2)
        self.assertIn("proxy",result.stderr)


if __name__ == "__main__":
    unittest.main()
