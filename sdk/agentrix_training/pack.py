"""
Agentrix Bot Submission Packager and Validator.
Validates bot directory layout, metadata, size limits, and admission checks before creating the ZIP.
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import zipfile
from typing import Dict, Any, List

from .env import find_arbiter_binary

MAX_ZIP_SIZE = 100 * 1024 * 1024       # 100 MiB
MAX_EXTRACTED_SIZE = 512 * 1024 * 1024 # 512 MiB
MAX_MANIFEST_SIZE = 16 * 1024          # 16 KiB
MAX_FILE_COUNT = 1000
ALLOWED_RUNTIMES = {"python-standard", "python-onnx", "binary"}


def validate_bot_directory(bot_dir: Path) -> Dict[str, Any]:
    """Validates the bot directory structure and returns parsed agentrix.json."""
    if not bot_dir.is_dir():
        raise ValueError(f"Bot directory does not exist or is not a directory: {bot_dir}")

    manifest_file = bot_dir / "agentrix.json"
    if not manifest_file.is_file():
        raise ValueError("Missing 'agentrix.json' in bot directory")

    if manifest_file.stat().st_size > MAX_MANIFEST_SIZE:
        raise ValueError(f"agentrix.json exceeds maximum size of {MAX_MANIFEST_SIZE} bytes")

    try:
        manifest = json.loads(manifest_file.read_text(encoding="utf-8"))
    except Exception as e:
        raise ValueError(f"Failed to parse agentrix.json: {e}") from e

    required_keys = ["name", "version", "runtime"]
    for k in required_keys:
        if k not in manifest:
            raise ValueError(f"agentrix.json missing required key: '{k}'")

    # Entrypoint can be specified as 'entrypoint' (preferred) or 'command'
    entrypoint = manifest.get("entrypoint") or manifest.get("command")
    if not entrypoint:
        raise ValueError("agentrix.json missing 'entrypoint'")
    manifest["entrypoint"] = entrypoint

    runtime = manifest["runtime"]
    if runtime not in ALLOWED_RUNTIMES:
        raise ValueError(
            f"Invalid runtime '{runtime}'. Must be one of: {sorted(ALLOWED_RUNTIMES)}"
        )

    # Check file count and uncompressed size
    total_size = 0
    file_count = 0

    for root, dirs, files in os.walk(bot_dir):
        # Exclude pycache, git, macos metadata
        dirs[:] = [d for d in dirs if d not in {"__pycache__", ".git", ".pytest_cache"}]
        for f in files:
            if f.endswith(".pyc") or f == ".DS_Store":
                continue
            f_path = Path(root) / f
            file_count += 1
            if file_count > MAX_FILE_COUNT:
                raise ValueError(f"Too many files: exceeds limit of {MAX_FILE_COUNT} files")
            total_size += f_path.stat().st_size
            if total_size > MAX_EXTRACTED_SIZE:
                raise ValueError(f"Uncompressed size exceeds limit of {MAX_EXTRACTED_SIZE} bytes")

    return manifest


def check_admission(bot_dir: Path, command: str, arbiter_bin: str) -> None:
    """Runs local arbiter admission check against the bot command."""
    print(f"Running admission check: '{command}' in {bot_dir}...")
    proc = subprocess.run(
        [arbiter_bin, "--admit", "--b0", command],
        cwd=str(bot_dir),
        capture_output=True,
        text=True,
        timeout=15,
    )
    if proc.returncode != 0:
        error_msg = proc.stderr.strip() or proc.stdout.strip()
        raise RuntimeError(f"Admission validation failed:\n{error_msg}")

    print("Admission check passed! Bot responds with READY and adheres to tick action schema.")


def package_bot(
    bot_dir: Path,
    output_zip: Path,
    skip_admission: bool = False,
    arbiter_bin: str | None = None,
) -> None:
    """Validates and packages the bot directory into a submission ZIP."""
    bot_dir = bot_dir.resolve()
    output_zip = output_zip.resolve()

    manifest = validate_bot_directory(bot_dir)

    if not skip_admission:
        bin_path = find_arbiter_binary(arbiter_bin)
        check_admission(bot_dir, manifest["entrypoint"], bin_path)

    output_zip.parent.mkdir(parents=True, exist_ok=True)
    temp_zip = output_zip.with_suffix(".tmp.zip")
    if temp_zip.exists():
        temp_zip.unlink()

    print(f"Creating package: {output_zip}...")
    with zipfile.ZipFile(temp_zip, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for root, dirs, files in os.walk(bot_dir):
            dirs[:] = sorted([d for d in dirs if d not in {"__pycache__", ".git", ".pytest_cache"}])
            for f in sorted(files):
                if f.endswith(".pyc") or f == ".DS_Store" or f.endswith(".tmp.zip"):
                    continue
                file_path = Path(root) / f
                rel_path = file_path.relative_to(bot_dir)
                is_executable = bool(file_path.stat().st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH))

                info = zipfile.ZipInfo(str(rel_path), date_time=(2026, 1, 1, 0, 0, 0))
                info.create_system = 3
                info.external_attr = (stat.S_IFREG | (0o755 if is_executable else 0o644)) << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                archive.writestr(info, file_path.read_bytes())

    # Check compressed size
    compressed_size = temp_zip.stat().st_size
    if compressed_size > MAX_ZIP_SIZE:
        temp_zip.unlink()
        raise ValueError(
            f"Compressed ZIP ({compressed_size} bytes) exceeds maximum limit of {MAX_ZIP_SIZE} bytes"
        )

    # Rename temp to target
    if output_zip.exists():
        output_zip.unlink()
    temp_zip.rename(output_zip)

    print(f"Successfully packaged '{manifest['name']}' v{manifest['version']} -> {output_zip} ({compressed_size / 1024:.1f} KiB)")


def main():
    parser = argparse.ArgumentParser(description="Package and validate an Agentrix bot submission ZIP")
    parser.add_argument("--bot-dir", type=Path, required=True, help="Path to bot directory containing agentrix.json")
    parser.add_argument("--output", type=Path, default=None, help="Output ZIP path (defaults to <bot_dir>.zip)")
    parser.add_argument("--skip-admission", action="store_true", help="Skip running local arbiter admission check")
    parser.add_argument("--arbiter-bin", type=str, default=None, help="Custom path to agentrix-arbiter binary")

    args = parser.parse_args()

    bot_dir = args.bot_dir
    output = args.output or bot_dir.with_suffix(".zip")

    try:
        package_bot(
            bot_dir=bot_dir,
            output_zip=output,
            skip_admission=args.skip_admission,
            arbiter_bin=args.arbiter_bin,
        )
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
