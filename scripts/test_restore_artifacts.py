import hashlib
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

from restore_artifacts import extract_staging


class ZeroStream:
    def read(self, size):
        return b"\0" * size


class RestoreArtifactTests(unittest.TestCase):
    def test_100_mib_original_and_executable_restored_without_overwrite(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive_path = root / "bots.tar.gz"
            with tarfile.open(archive_path, "w:gz") as archive:
                info = tarfile.TarInfo("bots/version.zip")
                info.size = 100 * 1024**2
                archive.addfile(info, ZeroStream())
                info = tarfile.TarInfo("bots/version/bot_bin")
                info.mode = 0o6755
                info.size = 3
                archive.addfile(info, io.BytesIO(b"exe"))
            digest = hashlib.sha256(archive_path.read_bytes()).hexdigest()
            live = root / "bots"
            live.mkdir()
            sentinel = live / "do-not-overwrite"
            sentinel.write_bytes(b"live")
            restored = extract_staging(archive_path, digest, root)
            self.assertEqual(sentinel.read_bytes(), b"live")
            self.assertEqual(restored.stat().st_mode & 0o7777, 0o700)
            binary = restored / "bots/version/bot_bin"
            self.assertEqual(binary.read_bytes(), b"exe")
            self.assertEqual(binary.stat().st_mode & 0o7777, 0o711)
            restored_zip = restored / "bots/version.zip"
            self.assertEqual(restored_zip.stat().st_size, 100 * 1024**2)
            expected = hashlib.sha256()
            for _ in range(100):
                expected.update(b"\0" * 1024**2)
            with restored_zip.open("rb") as source:
                self.assertEqual(hashlib.file_digest(source, "sha256").hexdigest(), expected.hexdigest())

    def test_rejects_links_and_bad_hash_before_creating_staging(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive_path = root / "bots.tar.gz"
            with tarfile.open(archive_path, "w:gz") as archive:
                info = tarfile.TarInfo("bots/escape")
                info.type = tarfile.SYMTYPE
                info.linkname = "/outside"
                archive.addfile(info)
            digest = hashlib.sha256(archive_path.read_bytes()).hexdigest()
            for checksum in (digest, "0" * 64):
                with self.assertRaises(ValueError):
                    extract_staging(archive_path, checksum, root)
                self.assertEqual(list(root.glob("agentrix-restore-*")), [])

    def test_extraction_failure_removes_only_its_private_staging(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            sentinel = root / "live-file"
            sentinel.write_bytes(b"preserve")
            archive_path = root / "bots.tar.gz"
            with tarfile.open(archive_path, "w:gz") as archive:
                info = tarfile.TarInfo("bots/file")
                info.size = 1
                archive.addfile(info, io.BytesIO(b"x"))
            digest = hashlib.sha256(archive_path.read_bytes()).hexdigest()
            with patch("restore_artifacts.os.fsync", side_effect=OSError("injected disk failure")):
                with self.assertRaises(OSError):
                    extract_staging(archive_path, digest, root)
            self.assertEqual(list(root.glob("agentrix-restore-*")), [])
            self.assertEqual(sentinel.read_bytes(), b"preserve")


if __name__ == "__main__":
    unittest.main()
