#!/usr/bin/env python3
"""Run the hermetic PostgreSQL/API/sandbox acceptance fixtures, never a live demo DB."""
import os
from pathlib import Path
import subprocess
import sys

def main():
    required = (
        "AGENTRIX_TEST_DATABASE_URL",
        "AGENTRIX_RUNTIME_ROOT",
        "AGENTRIX_RUNTIME_SHA256",
        "AGENTRIX_TEST_ARBITER_PATH",
        "AGENTRIX_TEST_PACKAGES_DIR",
    )
    missing = [name for name in required if not os.environ.get(name)]
    if missing:
        print("Missing explicit test configuration: " + ", ".join(missing), file=sys.stderr)
        return 2
    env = os.environ.copy()
    env.update(AGENTRIX_TEST_ACCEPTANCE="1", AGENTRIX_TEST_SANDBOX="1", AGENTRIX_TEST_ONNX="1")
    root = Path(__file__).resolve().parent.parent
    return subprocess.run(
        ["go", "test", "./...", "-race", "-count=1"],
        cwd=root / "backend", env=env, check=False,
    ).returncode

if __name__ == "__main__":
    sys.exit(main())
