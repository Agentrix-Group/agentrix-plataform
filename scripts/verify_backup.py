#!/usr/bin/env python3
"""Read-only integrity preflight. Checksums do not authenticate SQL backups.

Never extract archives or execute SQL in this verifier. Restore must only use
trusted backups and separately verify target safety and path remapping.
"""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tarfile

MAX_ENTRIES = 1_000_000
MAX_EXPANDED = 512 * 1024**3

def check_gzip(stream, max_expanded=MAX_EXPANDED):
    """Read through EOF, checking every gzip trailer without retaining content."""
    expanded = 0
    with gzip.GzipFile(fileobj=stream, mode="rb") as content:
        while chunk := content.read(1024 * 1024):
            expanded += len(chunk)
            if expanded > max_expanded:
                raise ValueError("expanded backup quota exceeded")

def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate manifest key")
        result[key] = value
    return result

def open_regular(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    if not stat.S_ISREG(os.fstat(descriptor).st_mode):
        os.close(descriptor)
        raise ValueError("backup member must be a regular file")
    return os.fdopen(descriptor, "rb")

def check_tar(stream, max_entries=MAX_ENTRIES, max_expanded=MAX_EXPANDED, expected_root=None):
    seen, files, roots, parents, nodes = set(), set(), set(), set(), set()
    expanded = 0
    with tarfile.open(fileobj=stream, mode="r|gz") as archive:
        for member in archive:
            name = member.name.rstrip("/") if member.isdir() else member.name
            path = PurePosixPath(name)
            if (not name or len(name) > 4096 or "\\" in name or "\x00" in name
                    or path.is_absolute() or str(path) != name or ".." in path.parts):
                raise ValueError("unsafe or noncanonical TAR path")
            if name in seen or len(seen) >= max_entries:
                raise ValueError("duplicate TAR path or entry quota exceeded")
            if not member.isdir() and not member.isreg():
                raise ValueError("TAR links and special files forbidden")
            if member.isreg() and len(path.parts) == 1:
                raise ValueError("TAR root must be a directory")
            if any(str(parent) in files for parent in path.parents):
                raise ValueError("TAR file used as parent directory")
            if member.isreg():
                if name in parents:
                    raise ValueError("TAR parent collides with existing child")
                files.add(name)
                expanded += member.size
                if member.size < 0 or expanded > max_expanded:
                    raise ValueError("expanded backup quota exceeded")
                with archive.extractfile(member) as content:
                    remaining = member.size
                    while remaining:
                        chunk = content.read(min(1024 * 1024, remaining))
                        if not chunk:
                            raise ValueError("truncated TAR content")
                        remaining -= len(chunk)
            ancestor_names = {str(parent) for parent in path.parents if str(parent) != "."}
            parents.update(ancestor_names)
            nodes.update(ancestor_names)
            nodes.add(name)
            if len(nodes) > max_entries:
                raise ValueError("TAR implicit directory quota exceeded")
            roots.add(path.parts[0])
            seen.add(name)
    if len(roots) != 1:
        raise ValueError("backup TAR must contain one root directory")
    if expected_root is not None and roots != {expected_root}:
        raise ValueError("TAR root disagrees with manifest")

def verify(manifest_path, database_file=None):
    manifest_path = Path(manifest_path).absolute()
    with open_regular(manifest_path) as source:
        data = source.read(65537)
    if len(data) > 65536:
        raise ValueError("backup manifest exceeds 64 KiB")
    manifest = json.loads(data, object_pairs_hook=unique_object)
    if not isinstance(manifest, dict):
        raise ValueError("manifest must be an object")
    version = manifest.get("format_version", 1)
    if type(version) is not int or version not in (1, 2):
        raise ValueError("unsupported backup format version")
    if version == 2 and manifest.get("consistency") != "operator-confirmed-offline":
        raise ValueError("unsupported backup consistency contract")
    filenames = set()
    checked = {}
    for section in ("database", "bots", "replays"):
        record = manifest.get(section)
        if not isinstance(record, dict):
            raise ValueError("missing backup section")
        name, digest = record.get("file"), record.get("sha256")
        if version == 2:
            if type(record.get("size_bytes")) is not int or record["size_bytes"] < 0:
                raise ValueError("invalid backup size")
            if section == "database":
                if record.get("format") != "postgresql-plain-sql-gzip":
                    raise ValueError("unsupported database dump format")
            else:
                origin = record.get("source_root")
                if (not isinstance(origin, str) or not origin.startswith("/") or origin == "/"
                        or "\x00" in origin or "\\" in origin or str(PurePosixPath(origin)) != origin
                        or ".." in PurePosixPath(origin).parts or record.get("archive_root") != section):
                    raise ValueError("invalid original artifact root")
        if (not isinstance(name, str) or not name or Path(name).name != name
                or name in (".", "..") or "\\" in name or "\x00" in name or name in filenames):
            raise ValueError("invalid or duplicate manifest filename")
        filenames.add(name)
        path = manifest_path.parent / name
        if digest == "none" and section != "database":
            if path.exists() or path.is_symlink():
                raise ValueError("unexpected unverified archive")
            continue
        if not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest):
            raise ValueError("missing SHA-256")
        with open_regular(path) as source:
            if version == 2 and os.fstat(source.fileno()).st_size != record["size_bytes"]:
                raise ValueError("backup size disagrees with manifest")
            if hashlib.file_digest(source, "sha256").hexdigest() != digest:
                raise ValueError("backup checksum mismatch")
            source.seek(0)
            check_gzip(source)
            if section != "database":
                source.seek(0)
                check_tar(source, expected_root=section if version == 2 else None)
        checked[section] = str(path)
    if database_file is not None:
        if Path(database_file).absolute() != Path(checked["database"]):
            raise ValueError("selected SQL file disagrees with manifest")
    return checked

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--database-file", type=Path)
    args = parser.parse_args()
    print(json.dumps(verify(args.manifest, args.database_file), sort_keys=True))
