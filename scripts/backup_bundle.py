#!/usr/bin/env python3
"""Create a fresh backup bundle while API and worker are stopped by the operator."""
import argparse
from datetime import datetime, timezone
import gzip
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import tarfile
import tempfile

from verify_backup import MAX_ENTRIES, MAX_EXPANDED, verify, open_regular


def sync_directory(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def record(path, **metadata):
    with open_regular(path) as source:
        return dict(file=path.name, sha256=hashlib.file_digest(source, "sha256").hexdigest(),
                    size_bytes=os.fstat(source.fileno()).st_size, **metadata)


def pack_tree(root, output, label):
    # The API and worker must be stopped; symlinks and devices are never followed.
    with output.open("xb") as destination:
        with tarfile.open(fileobj=destination, mode="w:gz", dereference=False) as archive:
            entries, expanded = 0, 0
            def visit(path, name):
                nonlocal entries, expanded
                entries += 1
                if entries > MAX_ENTRIES or len(name) > 4096 or "\\" in name:
                    raise ValueError("backup source entry/path quota exceeded")
                info = path.lstat()
                member = tarfile.TarInfo(name)
                member.mode = 0o600 | (info.st_mode & 0o111)
                if stat.S_ISDIR(info.st_mode):
                    member.type = tarfile.DIRTYPE
                    archive.addfile(member)
                    with os.scandir(path) as children:
                        child_entries = sorted(children, key=lambda child: child.name)
                    for child in child_entries:
                        visit(Path(child.path), name + "/" + child.name)
                elif stat.S_ISREG(info.st_mode):
                    with open_regular(path) as source:
                        opened = os.fstat(source.fileno())
                        if (opened.st_dev, opened.st_ino) != (info.st_dev, info.st_ino):
                            raise ValueError("artifact changed while opening backup")
                        member.size = opened.st_size
                        expanded += member.size
                        if expanded > MAX_EXPANDED:
                            raise ValueError("expanded backup quota exceeded")
                        archive.addfile(member, source)
                        after = os.fstat(source.fileno())
                        if (after.st_size, after.st_mtime_ns, after.st_ctime_ns) != (
                                opened.st_size, opened.st_mtime_ns, opened.st_ctime_ns):
                            raise ValueError("artifact changed during backup; stop writers")
                else:
                    raise ValueError("backup source contains a link or special file")
            visit(root, label)
        destination.flush()
        os.fsync(destination.fileno())


def create_bundle(backup_parent, bots_root, replays_root, connection, offline_confirmed=False):
    if not offline_confirmed:
        raise ValueError("stop API/worker and explicitly confirm offline backup")
    roots = {"bots": Path(bots_root).resolve(strict=True), "replays": Path(replays_root).resolve(strict=True)}
    for root in roots.values():
        if not root.is_dir() or root == Path("/"):
            raise ValueError("artifact roots must be dedicated existing directories")
    if roots["bots"] == roots["replays"] or any(
            root in other.parents for root in roots.values() for other in roots.values() if root != other):
        raise ValueError("artifact roots must not overlap")
    parent = Path(backup_parent).resolve(strict=True)
    if any(parent == root or root in parent.parents for root in roots.values()):
        raise ValueError("backup output must be outside artifact trees")
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S")
    bundle = Path(tempfile.mkdtemp(prefix="agentrix-backup-" + timestamp + "-", dir=parent))
    # Failed bundles deliberately retain diagnostics but have NO manifest. Only
    # a successfully verified bundle receives the final completion marker.
    sql = bundle / ("agentrix_db_" + timestamp + ".sql.gz")
    with sql.open("xb") as destination:
        with gzip.GzipFile(fileobj=destination, mode="wb", mtime=0) as compressed:
            with subprocess.Popen(["pg_dump", *connection, "--no-owner", "--no-privileges"],
                                  stdout=subprocess.PIPE) as process:
                while chunk := process.stdout.read(1024 * 1024):
                    compressed.write(chunk)
                if process.wait() != 0:
                    raise RuntimeError("pg_dump failed; incomplete bundle: " + str(bundle))
        destination.flush()
        os.fsync(destination.fileno())
    manifest = {"format_version": 2, "timestamp": timestamp,
                "consistency": "operator-confirmed-offline",
                "database": record(sql, format="postgresql-plain-sql-gzip")}
    for label, root in roots.items():
        output = bundle / ("agentrix_" + label + "_" + timestamp + ".tar.gz")
        pack_tree(root, output, label)
        manifest[label] = record(output, source_root=str(root), archive_root=label)
    temporary_manifest = bundle / "manifest.pending.json"
    with temporary_manifest.open("x", encoding="utf-8") as destination:
        json.dump(manifest, destination, sort_keys=True)
        destination.write("\n")
        destination.flush()
        os.fsync(destination.fileno())
    verify(temporary_manifest, sql)
    final_manifest = bundle / ("manifest_" + timestamp + ".json")
    temporary_manifest.rename(final_manifest)
    sync_directory(bundle)
    sync_directory(parent)
    return final_manifest


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--offline-confirmed", action="store_true")
    parser.add_argument("--backup-dir", required=True, type=Path)
    parser.add_argument("--bots-dir", required=True, type=Path)
    parser.add_argument("--replays-dir", required=True, type=Path)
    args = parser.parse_args()
    connection = ["-h", os.environ.get("DB_HOST", "127.0.0.1"),
                  "-p", os.environ.get("DB_PORT", "5432"),
                  "-U", os.environ.get("DB_USER", "postgres"),
                  "-d", os.environ.get("DB_NAME", "agentrix_platform")]
    print(create_bundle(args.backup_dir, args.bots_dir, args.replays_dir, connection, args.offline_confirmed))
