#!/usr/bin/env python3
"""Recover a trusted format-2 bundle into a NEW database and NEW artifact roots.

Never start services against the new database until a completion receipt exists.
Failures retain the new database/staging for diagnosis, never drop existing data.
"""
import argparse
import ctypes
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

from backup_bundle import sync_directory
from restore_artifacts import extract_staging
from verify_backup import open_regular, unique_object, verify


def new_target(path):
    path = Path(path).absolute()
    parent = path.parent.resolve(strict=True)
    result = parent / path.name
    if result.exists() or result.is_symlink() or path.name in ("", ".", ".."):
        raise ValueError("recovery destination must not exist")
    return result


def publish_new(source, destination):
    # Linux-only deployment: atomically refuse ALL existing destination types,
    # including empty directories and dangling symlinks. os.rename is insufficient.
    library = ctypes.CDLL(None, use_errno=True)
    rename = library.renameat2
    rename.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint]
    rename.restype = ctypes.c_int
    if rename(-100, os.fsencode(source), -100, os.fsencode(destination), 1) != 0:
        number = ctypes.get_errno()
        raise OSError(number, os.strerror(number), str(destination))
    sync_directory(destination.parent)


def restore_bundle(manifest_path, database, bots_target, replays_target, verifier, connection, trust_sql=False):
    if not trust_sql:
        raise ValueError("explicit acknowledgment of trusted SQL origin required")
    if not re.fullmatch(r"[a-z][a-z0-9_]{0,62}", database):
        raise ValueError("new database name must be a lowercase identifier, at most 63 bytes")
    with open_regular(manifest_path) as source:
        data = source.read(65537)
    if len(data) > 65536:
        raise ValueError("manifest too large")
    metadata = json.loads(data, object_pairs_hook=unique_object)
    if metadata.get("format_version") != 2:
        raise ValueError("full recovery requires format 2 original roots; legacy backups are verify-only")
    checked = verify(manifest_path)
    with open_regular(manifest_path) as source:
        if source.read(65537) != data:
            raise ValueError("manifest changed during preflight")
    targets = {"bots": new_target(bots_target), "replays": new_target(replays_target)}
    if targets["bots"] == targets["replays"] or any(
            root in other.parents for root in targets.values() for other in targets.values() if root != other):
        raise ValueError("new artifact roots must not overlap")
    executable = Path(verifier).resolve(strict=True)
    if not executable.is_file() or not os.access(executable, os.X_OK):
        raise ValueError("trusted recovery verifier must be an executable")
    stages = {}
    for section in ("bots", "replays"):
        if section not in checked:
            raise ValueError("format 2 recovery requires both artifact archives")
        stages[section] = extract_staging(checked[section], metadata[section]["sha256"], targets[section].parent)
    # createdb fails rather than overwrite an existing DB. No DROP, --force,
    # termination of connections, or restoration into a caller's live DB.
    subprocess.run(["createdb", *connection, "--template=template0", database], check=True)
    environment = os.environ.copy()
    environment["PGDATABASE"] = database
    with open_regular(checked["database"]) as source:
        if hashlib.file_digest(source, "sha256").hexdigest() != metadata["database"]["sha256"]:
            raise ValueError("SQL backup changed before recovery")
        source.seek(0)
        with subprocess.Popen(["psql", "-X", *connection, "-d", database, "--single-transaction",
                               "-v", "ON_ERROR_STOP=1", "-f", "-"], stdin=subprocess.PIPE,
                              stdout=subprocess.DEVNULL) as process:
            try:
                with gzip.GzipFile(fileobj=source, mode="rb") as sql:
                    while chunk := sql.read(1024 * 1024):
                        process.stdin.write(chunk)
                process.stdin.close()
            except BaseException:
                process.stdin.close()
                process.wait()
                raise
            if process.wait() != 0:
                raise RuntimeError("SQL recovery failed; new DB retained, no completion receipt")
    arguments = [str(executable)]
    for section in ("bots", "replays"):
        arguments.extend(["--original-" + section, metadata[section]["source_root"],
                          "--staged-" + section, str(stages[section] / section),
                          "--final-" + section, str(targets[section])])
    subprocess.run(arguments, env=environment, check=True)
    for section in ("bots", "replays"):
        publish_new(stages[section] / section, targets[section])
    receipt = targets["bots"].parent / ("agentrix-recovery-" + database + ".json")
    descriptor, pending_name = tempfile.mkstemp(prefix="agentrix-receipt-pending-",dir=receipt.parent)
    with os.fdopen(descriptor,"w",encoding="utf-8") as destination:
        json.dump({"status": "complete", "database": database, "manifest_sha256": hashlib.sha256(data).hexdigest(),
                   "bots_root": str(targets["bots"]), "replays_root": str(targets["replays"])}, destination)
        destination.flush()
        os.fsync(destination.fileno())
    publish_new(Path(pending_name),receipt)
    return receipt


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--trust-sql", action="store_true")
    parser.add_argument("--new-database", required=True)
    parser.add_argument("--new-bots-dir", required=True, type=Path)
    parser.add_argument("--new-replays-dir", required=True, type=Path)
    parser.add_argument("--verifier", required=True, type=Path)
    args = parser.parse_args()
    # Matching libpq environment for createdb/psql and the Go verifier.
    for source, target, default in (("DB_HOST", "PGHOST", "127.0.0.1"), ("DB_PORT", "PGPORT", "5432"),
                                    ("DB_USER", "PGUSER", "postgres")):
        os.environ[target] = os.environ.get(source, os.environ.get(target, default))
    print(restore_bundle(args.manifest, args.new_database, args.new_bots_dir, args.new_replays_dir,
                         args.verifier, [], args.trust_sql))
