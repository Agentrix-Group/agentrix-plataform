#!/usr/bin/env python3
"""Build explicit example ZIPs, without compiling anything during admission.

Only repository-controlled sources are built by this SDK utility. Never invoke
this host-side builder on untrusted submissions. The production server only
executes submitted binaries inside the sandbox.
"""
import argparse
import hashlib
from pathlib import Path
import stat
import subprocess
import tempfile
import zipfile

JSON_SHA256 = "aaf127c04cb31c406e5b04a63f1ae89369fccde6d8fa7cdda1ed4f32dfc5de63"
ROOT = Path(__file__).resolve().parent.parent

def package(output, header, compiler):
    if hashlib.sha256(header.read_bytes()).hexdigest() != JSON_SHA256:
        raise ValueError("nlohmann/json v3.12.0 header checksum mismatch")
    output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="agentrix-sdk-") as build:
        binary = Path(build) / "bot_bin"
        subprocess.run([compiler, "-std=c++17", "-O2", "-static", "-s",
                        "-include", str(header.resolve()), str(ROOT / "bots/cpp_bot/agent.cpp"),
                        "-o", str(binary)], check=True)
        specs = {
            "heuristic_bot": [("agent.py", ROOT / "bots/heuristic_bot/agent.py", False)],
            "onnx_bot": [("agent.py", ROOT / "bots/onnx_bot/agent.py", False),
                         ("model.onnx", ROOT / "bots/onnx_bot/model.onnx", False)],
            "cpp_bot": [("bot_bin", binary, True),
                        ("agent.cpp", ROOT / "bots/cpp_bot/agent.cpp", False),
                        ("json.hpp", header, False)],
        }
        for name, files in specs.items():
            files.append(("agentrix.json", ROOT / "bots" / name / "agentrix.json", False))
            # Exclusive output: never silently overwrite an operator's package.
            with zipfile.ZipFile(output / (name + ".zip"), "x", compression=zipfile.ZIP_DEFLATED) as archive:
                for path, source, executable in sorted(files):
                    info = zipfile.ZipInfo(path, date_time=(2026, 1, 1, 0, 0, 0))
                    info.create_system = 3
                    info.external_attr = (stat.S_IFREG | (0o755 if executable else 0o644)) << 16
                    info.compress_type = zipfile.ZIP_DEFLATED
                    archive.writestr(info, source.read_bytes())

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--json-header", type=Path, required=True)
    parser.add_argument("--compiler", default="g++")
    args = parser.parse_args()
    package(args.output, args.json_header, args.compiler)
