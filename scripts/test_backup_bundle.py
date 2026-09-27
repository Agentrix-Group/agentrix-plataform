import io
import gzip
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
import subprocess
import uuid
import zipfile
from unittest.mock import patch

from backup_bundle import create_bundle, pack_tree
from verify_backup import verify
from restore_artifacts import extract_staging


class DumpProcess:
    def __init__(self, *args, **kwargs):
        self.stdout = io.BytesIO(b"-- trusted fixture dump\nSELECT 1;\n")
    def __enter__(self): return self
    def __exit__(self, *args): self.stdout.close()
    def wait(self): return 0


class BackupBundleTests(unittest.TestCase):
    def test_fresh_bundle_records_roots_and_verifies_before_publication(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bots, replays, output = (root / name for name in ("custom-bots", "custom-replays", "backups"))
            for path in (bots, replays, output): path.mkdir()
            (bots / "original.zip").write_bytes(b"fixture")
            with patch("backup_bundle.subprocess.Popen", DumpProcess):
                manifests = [create_bundle(output, bots, replays, [], True) for _ in range(2)]
            self.assertNotEqual(manifests[0].parent, manifests[1].parent)
            for path in manifests:
                data = json.loads(path.read_text())
                self.assertEqual(data["format_version"], 2)
                self.assertEqual(data["bots"]["source_root"], str(bots))
                self.assertEqual(data["bots"]["archive_root"], "bots")
                self.assertEqual(set(verify(path)), {"database", "bots", "replays"})
                self.assertFalse((path.parent / "manifest.pending.json").exists())

    def test_offline_acknowledgment_required_before_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaises(ValueError):
                create_bundle(root, root, root, [])
            self.assertEqual(list(root.iterdir()), [])

    def test_symlink_source_is_not_followed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bots = root / "bots"
            bots.mkdir()
            (bots / "link").symlink_to("/etc/passwd")
            with self.assertRaises(ValueError):
                pack_tree(bots, root / "bots.tar.gz", "bots")

    def test_all_execute_bits_survive_backup_and_restore(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); bots = root / "bots"; bots.mkdir()
            nested = bots / "nested"; nested.mkdir(); nested.chmod(0o755)
            for mode in (0o700,0o750,0o755,0o644):
                path = nested / str(mode); path.write_bytes(b"fixture"); path.chmod(mode)
            archive = root / "bots.tar.gz"; pack_tree(bots,archive,"bots")
            with archive.open("rb") as source:
                checksum = hashlib.file_digest(source,"sha256").hexdigest()
            staged = extract_staging(archive,checksum,root) / "bots"
            for path in [nested,*nested.iterdir()]:
                self.assertEqual((staged/path.relative_to(bots)).stat().st_mode & 0o111,path.stat().st_mode & 0o111)

    def test_failed_dump_has_no_final_manifest(self):
        class FailedDump(DumpProcess):
            def wait(self): return 1
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bots, replays, output = (root / name for name in ("bots", "replays", "backups"))
            for path in (bots, replays, output): path.mkdir()
            with patch("backup_bundle.subprocess.Popen", FailedDump):
                with self.assertRaises(RuntimeError):
                    create_bundle(output, bots, replays, [], True)
            self.assertEqual(list(output.glob("*/manifest_*.json")), [])

    @unittest.skipUnless(os.environ.get("AGENTRIX_TEST_BACKUP_DATABASE"), "requires isolated PostgreSQL fixture connection")
    def test_real_postgres_dump_and_100_mib_artifact_bundle(self):
        # The connection must point to the dedicated test server, never production.
        connection = ["-h", os.environ.get("AGENTRIX_TEST_PG_HOST", "127.0.0.1"),
                      "-p", os.environ.get("AGENTRIX_TEST_PG_PORT", "55439"),
                      "-U", os.environ.get("AGENTRIX_TEST_PG_USER", "f4nk1")]
        database = "agentrix_backup_test_" + uuid.uuid4().hex
        subprocess.run(["createdb", *connection, database], check=True)
        try:
            subprocess.run(["psql", "-X", *connection, "-d", database, "-v", "ON_ERROR_STOP=1", "-c",
                            "CREATE TABLE backup_fixture (id int, marker text); "
                            "INSERT INTO backup_fixture VALUES (1, 'agentrix-backup-real-fixture');"],
                           check=True, stdout=subprocess.DEVNULL)
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                bots, replays, output = (root / name for name in ("bots", "replays", "backups"))
                for path in (bots, replays, output): path.mkdir()
                original = bots / "100-mib-original.zip"
                example = Path(__file__).resolve().parent.parent / "bots/heuristic_bot"
                def write_zip(padding_bytes):
                    with zipfile.ZipFile(original, "w", compression=zipfile.ZIP_STORED) as archive:
                        for name in ("agent.py", "agentrix.json"):
                            archive.write(example / name, name)
                        with archive.open("padding.bin", "w") as destination:
                            while padding_bytes:
                                count = min(padding_bytes, 1024**2)
                                destination.write(b"\0" * count)
                                padding_bytes -= count
                write_zip(0)
                write_zip(100 * 1024**2 - original.stat().st_size)
                self.assertEqual(original.stat().st_size, 100 * 1024**2)
                manifest = create_bundle(output, bots, replays, [*connection, "-d", database], True)
                checked = verify(manifest)
                with gzip.open(checked["database"], "rt") as source:
                    self.assertIn("agentrix-backup-real-fixture", source.read())
                metadata = json.loads(manifest.read_text())
                staging = extract_staging(checked["bots"], metadata["bots"]["sha256"], root)
                restored = staging / "bots" / original.name
                self.assertEqual(restored.stat().st_size, original.stat().st_size)
                with zipfile.ZipFile(restored) as archive:
                    self.assertIsNone(archive.testzip())
                    self.assertEqual(archive.read("agent.py"), (example / "agent.py").read_bytes())
                import hashlib
                with original.open("rb") as before, restored.open("rb") as after:
                    self.assertEqual(hashlib.file_digest(before, "sha256").digest(),
                                     hashlib.file_digest(after, "sha256").digest())
        finally:
            subprocess.run(["dropdb", *connection, database], check=True)


if __name__ == "__main__": unittest.main()
