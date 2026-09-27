#!/usr/bin/env python3
"""
Starter Kit Packager for Agentrix Platform.
Generates a self-contained, pre-compiled distribution ZIP for participants:
- Official Linux x86_64 agentrix-arbiter binary (stripped, executable)
- Python training SDK (agentrix-training)
- my_bot starter template (agent.py, agentrix.json)
- Google Colab notebook
- requirements.txt, Makefile, README.md

Participants can unzip and train immediately with zero Rust compilation.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import zipfile

KIT_VERSION = "0.2.0"
KIT_NAME = "agentrix-starter-kit"


def find_arbiter_source(custom_bin: str | None = None) -> Path:
    repo_root = Path(__file__).resolve().parents[1]
    candidates = []
    if custom_bin:
        candidates.append(Path(custom_bin))

    candidates.extend([
        repo_root / "simulation/arbiter/target/release/agentrix-arbiter",
        Path("/usr/local/bin/agentrix-arbiter"),
        Path("/opt/agentrix/bin/agentrix-arbiter"),
    ])

    for c in candidates:
        if c.is_file() and os.access(c, os.X_OK):
            return c.resolve()

    # Try building if cargo is available
    if shutil.which("cargo"):
        print("[KIT BUILDER] Arbiter binary not found. Compiling release binary with cargo...")
        arbiter_dir = repo_root / "simulation/arbiter"
        res = subprocess.run(
            ["cargo", "build", "--release", "--locked"],
            cwd=arbiter_dir,
            capture_output=True,
            text=True,
        )
        if res.returncode == 0:
            target_bin = repo_root / "simulation/arbiter/target/release/agentrix-arbiter"
            if target_bin.is_file():
                return target_bin

    raise FileNotFoundError(
        "Could not find or compile agentrix-arbiter release binary. "
        "Run 'cargo build --release -p agentrix-arbiter' first or supply --arbiter-bin."
    )


def compute_sha256(file_path: Path) -> str:
    h = hashlib.sha256()
    with file_path.open("rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest().lower()


def build_kit(
    output_dir: Path,
    arbiter_bin: Path,
    repo_root: Path,
    version: str = KIT_VERSION,
) -> Tuple[Path, str, int]:
    output_dir.mkdir(parents=True, exist_ok=True)
    zip_filename = f"{KIT_NAME}-v{version}.zip"
    default_zip_filename = f"{KIT_NAME}.zip"
    target_zip = output_dir / zip_filename
    alias_zip = output_dir / default_zip_filename

    templates_dir = repo_root / "templates/starter-kit"
    sdk_dir = repo_root / "sdk"
    notebooks_dir = repo_root / "notebooks"

    with tempfile.TemporaryDirectory() as staging_str:
        staging = Path(staging_str)
        kit_root = staging / f"{KIT_NAME}-v{version}"
        kit_root.mkdir(parents=True, exist_ok=True)

        # 1. Copy bin/agentrix-arbiter
        bin_dir = kit_root / "bin"
        bin_dir.mkdir(parents=True, exist_ok=True)
        dest_bin = bin_dir / "agentrix-arbiter"
        shutil.copy2(arbiter_bin, dest_bin)
        dest_bin.chmod(0o755)

        # Try stripping to reduce footprint if strip utility is available
        if shutil.which("strip"):
            subprocess.run(["strip", str(dest_bin)], capture_output=True)

        # 2. Copy SDK
        dest_sdk = kit_root / "sdk"
        dest_sdk.mkdir(parents=True, exist_ok=True)
        shutil.copytree(
            sdk_dir / "agentrix_training",
            dest_sdk / "agentrix_training",
            ignore=shutil.ignore_patterns("__pycache__", "*.pyc", "*.egg-info"),
        )
        if (sdk_dir / "pyproject.toml").is_file():
            shutil.copy2(sdk_dir / "pyproject.toml", dest_sdk / "pyproject.toml")
        if (sdk_dir / "README.md").is_file():
            shutil.copy2(sdk_dir / "README.md", dest_sdk / "README.md")

        # 3. Copy my_bot starter template
        dest_my_bot = kit_root / "my_bot"
        if (templates_dir / "my_bot").is_dir():
            shutil.copytree(templates_dir / "my_bot", dest_my_bot)
        else:
            dest_my_bot.mkdir(parents=True, exist_ok=True)
            # Fallback creation
            (dest_my_bot / "agentrix.json").write_text(
                json.dumps(
                    {
                        "version": 1,
                        "protocol_version": 1,
                        "name": "MyStarterBot",
                        "runtime": "python-standard",
                        "entrypoint": "python3 agent.py",
                    },
                    indent=2,
                )
            )

        if (dest_my_bot / "agent.py").is_file():
            (dest_my_bot / "agent.py").chmod(0o755)

        # 4. Copy notebooks
        dest_notebooks = kit_root / "notebooks"
        dest_notebooks.mkdir(parents=True, exist_ok=True)
        colab_nb = notebooks_dir / "agentrix_colab_starter.ipynb"
        if colab_nb.is_file():
            shutil.copy2(colab_nb, dest_notebooks / "agentrix_colab_starter.ipynb")
            # Also keep a copy at kit root for immediate visibility
            shutil.copy2(colab_nb, kit_root / "agentrix_colab_starter.ipynb")

        # 5. Copy requirements.txt, Makefile, README.md
        if (templates_dir / "requirements.txt").is_file():
            shutil.copy2(templates_dir / "requirements.txt", kit_root / "requirements.txt")
        else:
            (kit_root / "requirements.txt").write_text(
                "numpy>=1.21.0\ngymnasium>=1.0.0\npettingzoo>=1.24.0\n"
            )

        if (templates_dir / "Makefile").is_file():
            shutil.copy2(templates_dir / "Makefile", kit_root / "Makefile")

        if (templates_dir / "README.md").is_file():
            shutil.copy2(templates_dir / "README.md", kit_root / "README.md")

        # Build ZIP archive with executable file permissions preserved
        with zipfile.ZipFile(target_zip, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for item in kit_root.rglob("*"):
                arcname = item.relative_to(staging)
                zinfo = zipfile.ZipInfo.from_file(item, arcname=str(arcname))
                # Preserve execute permission
                if item.is_file() and os.access(item, os.X_OK):
                    zinfo.external_attr = 0o755 << 16
                else:
                    zinfo.external_attr = 0o644 << 16
                if item.is_file():
                    with item.open("rb") as f:
                        zf.writestr(zinfo, f.read())

    # Create unversioned alias/copy
    shutil.copy2(target_zip, alias_zip)

    # Compute checksums
    sha256 = compute_sha256(target_zip)
    size_bytes = target_zip.stat().st_size

    # Write checksum file
    sha_file = output_dir / f"{zip_filename}.sha256"
    sha_file.write_text(f"{sha256}  {zip_filename}\n")
    alias_sha_file = output_dir / f"{default_zip_filename}.sha256"
    alias_sha_file.write_text(f"{sha256}  {default_zip_filename}\n")

    # Write kit metadata manifest JSON
    meta = {
        "kit_name": KIT_NAME,
        "version": version,
        "filename": zip_filename,
        "alias_filename": default_zip_filename,
        "sha256": sha256,
        "size_bytes": size_bytes,
        "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "engine": "agentrix-engine-v1",
        "supported_os": "Linux x86_64",
    }
    (output_dir / "agentrix-starter-kit.json").write_text(json.dumps(meta, indent=2))

    return target_zip, sha256, size_bytes


def verify_extracted_kit(zip_path: Path):
    print(f"[VERIFY] Testing extracted kit from {zip_path}...")
    with tempfile.TemporaryDirectory() as temp_dir:
        with zipfile.ZipFile(zip_path, "r") as zf:
            zf.extractall(temp_dir)

        # Locate extracted kit dir
        extracted_root = None
        for item in Path(temp_dir).iterdir():
            if item.is_dir() and item.name.startswith(KIT_NAME):
                extracted_root = item
                break

        assert extracted_root is not None, "Extracted kit root not found"
        arbiter_bin = extracted_root / "bin/agentrix-arbiter"
        assert arbiter_bin.is_file(), "bin/agentrix-arbiter is missing from kit"

        # Ensure executable
        arbiter_bin.chmod(0o755)

        # Test executing arbiter binary
        res = subprocess.run([str(arbiter_bin), "--version"], capture_output=True, text=True)
        assert res.returncode == 0, f"Arbiter failed: {res.stderr}"
        assert "agentrix-arbiter" in res.stdout, f"Unexpected version output: {res.stdout}"

        # Verify SDK layout
        assert (extracted_root / "sdk/agentrix_training").is_dir(), "sdk/agentrix_training missing"
        assert (extracted_root / "my_bot/agent.py").is_file(), "my_bot/agent.py missing"
        assert (extracted_root / "my_bot/agentrix.json").is_file(), "my_bot/agentrix.json missing"
        assert (extracted_root / "requirements.txt").is_file(), "requirements.txt missing"

        print(f"[VERIFY] SUCCESS: All kit components verified. Arbiter version: {res.stdout.strip()}")


def main():
    parser = argparse.ArgumentParser(description="Build Agentrix Starter Kit Distribution ZIP")
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path("var/agentrix/artifacts/kits"),
        help="Destination directory for kit ZIP and metadata",
    )
    parser.add_argument(
        "--arbiter-bin",
        type=str,
        default=None,
        help="Path to pre-compiled agentrix-arbiter binary",
    )
    parser.add_argument(
        "--version",
        type=str,
        default=KIT_VERSION,
        help="Starter kit version string",
    )
    parser.add_argument(
        "--skip-verify",
        action="store_true",
        help="Skip verification of extracted kit",
    )
    parser.add_argument(
        "--repo-root",
        type=Path,
        default=None,
        help="Repository root containing sdk/, templates/, notebooks/",
    )

    args = parser.parse_args()
    repo_root = (args.repo_root or Path(__file__).resolve().parents[1]).resolve()

    arbiter_bin = find_arbiter_source(args.arbiter_bin)
    print(f"[KIT BUILDER] Using arbiter binary: {arbiter_bin}")

    zip_path, sha256, size_bytes = build_kit(
        output_dir=args.output_dir,
        arbiter_bin=arbiter_bin,
        repo_root=repo_root,
        version=args.version,
    )

    print("\n" + "=" * 60)
    print("Agentrix Starter Kit Generated Successfully!")
    print(f"Archive:    {zip_path}")
    print(f"Size:       {size_bytes / 1024:.1f} KiB ({size_bytes} bytes)")
    print(f"SHA-256:    {sha256}")
    print("=" * 60 + "\n")

    if not args.skip_verify:
        verify_extracted_kit(zip_path)


if __name__ == "__main__":
    main()
