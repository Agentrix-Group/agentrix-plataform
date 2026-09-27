#!/usr/bin/env python3
"""Bounded local development supervisor; only signals its own service sessions."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.request


def healthy(url):
    try:
        with urllib.request.urlopen(url, timeout=1) as response:
            if response.status != 200:
                return False
            # This is a tiny trusted status endpoint, not an unbounded download.
            data = response.read(65537)
        if len(data) > 65536:
            return False
        status = json.loads(data)
        return (isinstance(status, dict) and status.get("database_healthy") is True
                and status.get("arbiter_healthy") is True)
    except (OSError, urllib.error.URLError, ValueError):
        return False


class Supervisor:
    def __init__(self, backend, frontend, health_check, startup_timeout=180,
                 grace_timeout=60, poll_interval=0.2):
        self.backend = backend
        self.frontend = frontend
        self.health_check = health_check
        self.startup_timeout = startup_timeout
        self.grace_timeout = grace_timeout
        self.poll_interval = poll_interval
        self.children = []
        self.stop_signal = None

    def request_stop(self, signum, _frame=None):
        self.stop_signal = signum

    def launch(self, command):
        # A new session means neither TERM nor KILL can reach the invoking shell
        # or unrelated jobs in its process group. Never use kill(0).
        child = subprocess.Popen(command, start_new_session=True)
        self.children.append(child)
        return child

    def close(self):
        # Keep leader PIDs unreaped until all group signals have been sent: their
        # IDs cannot be reused for unrelated processes during cleanup.
        for child in self.children:
            try:
                os.killpg(child.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
        deadline = time.monotonic() + self.grace_timeout
        while any(self.exit_code(child) is None for child in self.children):
            if time.monotonic() >= deadline:
                break
            time.sleep(min(self.poll_interval, max(0, deadline-time.monotonic())))
        for child in self.children:
            # Leaders remain unreaped until after the last group signal, so
            # their group IDs cannot have been recycled into unrelated sessions.
            try:
                os.killpg(child.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            child.wait()
        self.children.clear()

    @staticmethod
    def exit_code(child):
        status = os.waitid(os.P_PID, child.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT)
        if status is None:
            return None
        return status.si_status if status.si_code == os.CLD_EXITED else 128+status.si_status

    def run(self):
        try:
            backend = self.launch(self.backend)
            deadline = time.monotonic() + self.startup_timeout
            while not self.stop_signal:
                code = self.exit_code(backend)
                if code is not None:
                    print("Backend exited before becoming ready.", file=sys.stderr)
                    return code if code > 0 else 1
                if self.health_check():
                    break
                if time.monotonic() >= deadline:
                    print("Backend readiness deadline exceeded.", file=sys.stderr)
                    return 1
                time.sleep(self.poll_interval)
            if self.stop_signal:
                return 128+self.stop_signal
            self.launch(self.frontend)
            print("Agentrix local services started. Press Ctrl+C to stop.", flush=True)
            while not self.stop_signal:
                for child in self.children:
                    code = self.exit_code(child)
                    if code is not None:
                        print("A local service exited; stopping the other service.", file=sys.stderr)
                        return code if code > 0 else 1
                time.sleep(self.poll_interval)
            return 128+self.stop_signal
        finally:
            self.close()


def main():
    root = Path(__file__).resolve().parent
    port = os.environ.get("PORT", "8080")
    if not port.isascii() or not port.isdigit() or not 1 <= int(port) <= 65535:
        print("PORT must be an integer between 1 and 65535.", file=sys.stderr)
        return 2
    # Vite's local proxy is intentionally fixed at 8080. Do not claim a working
    # two-service stack when frontend requests would reach a different backend.
    if int(port) != 8080:
        print("The local frontend proxy requires PORT=8080. Start the backend separately for another port.", file=sys.stderr)
        return 2
    supervisor = Supervisor(
        ["bash", str(root / "start_backend.sh")],
        ["bash", str(root / "start_frontend.sh")],
        lambda: healthy(f"http://127.0.0.1:{port}/api/v1/system/status"),
    )
    for signum in (signal.SIGTERM, signal.SIGINT):
        signal.signal(signum, supervisor.request_stop)
    try:
        return supervisor.run()
    except OSError as error:
        print(f"Could not start local services: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
