#!/usr/bin/env python3
"""Restore one verified artifact archive into a NEW private staging directory.

This is a building block, not a database restore or publication command.
The returned directory belongs to the caller; nothing replaces a live target.
"""
import hashlib
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import tarfile
import tempfile

from verify_backup import MAX_ENTRIES, MAX_EXPANDED, check_gzip, check_tar, open_regular


def extract_staging(archive_path, expected_sha256, staging_parent):
    if not isinstance(expected_sha256, str) or not re.fullmatch(r"[0-9a-f]{64}", expected_sha256):
        raise ValueError("required SHA-256 is invalid")
    parent = Path(staging_parent).resolve(strict=True)
    if not parent.is_dir():
        raise ValueError("staging parent must be an existing directory")
    # Hold the same descriptor from checksum through extraction. Replacing the
    # pathname cannot substitute another archive between validation and use.
    with open_regular(archive_path) as source:
        if hashlib.file_digest(source, "sha256").hexdigest() != expected_sha256:
            raise ValueError("backup checksum mismatch")
        source.seek(0)
        check_gzip(source)
        source.seek(0)
        check_tar(source)
        source.seek(0)
        staging = Path(tempfile.mkdtemp(prefix="agentrix-restore-", dir=parent))
        try:
            entries, expanded = 0, 0
            directory_modes = {}
            with tarfile.open(fileobj=source, mode="r|gz") as archive:
                for member in archive:
                    # check_tar rejects links, specials, ambiguous paths and
                    # collisions. Never invoke extract()/extractall(), and never
                    # apply archive owners, permissions, timestamps or xattrs.
                    name = member.name.rstrip("/") if member.isdir() else member.name
                    relative = PurePosixPath(name)
                    entries += 1
                    expanded += member.size
                    # Recheck each header during use: an operator-owned file can
                    # still be accidentally modified in place between passes.
                    if (not name or len(name) > 4096 or "\\" in name or "\x00" in name
                            or relative.is_absolute() or str(relative) != name or ".." in relative.parts
                            or not (member.isdir() or member.isreg())
                            or entries > MAX_ENTRIES or member.size < 0 or expanded > MAX_EXPANDED
                            or (member.isreg() and len(relative.parts) == 1)):
                        raise ValueError("unsafe TAR header during extraction")
                    path = staging.joinpath(*relative.parts)
                    if member.isdir():
                        path.mkdir(mode=0o700, parents=True, exist_ok=True)
                        directory_modes[path] = 0o600 | (member.mode & 0o111)
                        continue
                    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
                    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
                    with os.fdopen(descriptor, "wb") as destination, archive.extractfile(member) as content:
                        remaining = member.size
                        while remaining:
                            chunk = content.read(min(1024 * 1024, remaining))
                            if not chunk:
                                raise ValueError("truncated TAR content")
                            destination.write(chunk)
                            remaining -= len(chunk)
                        destination.flush()
                        # Preserve only executable-vs-data semantics, which the
                        # Agentrix tree digest covers; discard setuid/setgid.
                        os.fchmod(destination.fileno(), 0o600 | (member.mode & 0o111))
                        os.fsync(destination.fileno())
            # Detect in-place modification of an already-open source too. Backup
            # repositories must still be operated by a trusted owner.
            source.seek(0)
            if hashlib.file_digest(source, "sha256").hexdigest() != expected_sha256:
                raise ValueError("backup changed during extraction")
            for directory, _, _ in os.walk(staging, topdown=False):
                descriptor = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
                try:
                    if Path(directory) in directory_modes:
                        os.fchmod(descriptor, directory_modes[Path(directory)])
                    os.fsync(descriptor)
                finally:
                    os.close(descriptor)
            return staging
        except BaseException:
            # Only this operation's exact private mkdtemp directory is removed.
            shutil.rmtree(staging)
            raise
