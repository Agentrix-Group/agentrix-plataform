import hashlib
import gzip
import io
import json
from pathlib import Path
import tarfile
import tempfile
import subprocess
import unittest
from verify_backup import check_gzip, check_tar, verify

class BackupVerificationTests(unittest.TestCase):
    def archive(self, entries):
        data = io.BytesIO()
        with tarfile.open(fileobj=data, mode="w:gz") as archive:
            for name, kind in entries:
                info = tarfile.TarInfo(name)
                info.type = kind
                if kind == tarfile.REGTYPE:
                    info.size = 1
                    archive.addfile(info, io.BytesIO(b"x"))
                else:
                    info.linkname = "/outside"
                    archive.addfile(info)
        data.seek(0)
        return data

    def test_tar_paths_types_duplicates_and_quotas(self):
        for entries in [[("../escape", tarfile.REGTYPE)],[("/absolute", tarfile.REGTYPE)],
                        [("bots/link",tarfile.SYMTYPE)],[("bots/link",tarfile.LNKTYPE)],
                        [("bots/a",tarfile.REGTYPE)]*2,
                        [("bots",tarfile.REGTYPE)],
                        [("bots/a",tarfile.REGTYPE),("bots/a/b",tarfile.REGTYPE)],
                        [("bots/a/b",tarfile.REGTYPE),("bots/a",tarfile.REGTYPE)]]:
            with self.subTest(entries=entries), self.assertRaises(ValueError):
                check_tar(self.archive(entries))
        with self.assertRaises(ValueError):
            check_tar(self.archive([("bots/a",tarfile.REGTYPE)]),max_expanded=0)
        with self.assertRaises(ValueError):
            check_tar(self.archive([("bots/a",tarfile.REGTYPE)]),max_entries=0)
        check_tar(self.archive([("bots/a",tarfile.REGTYPE)]))

    def test_manifest_checksums_and_selected_database(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);sql=root/"db.sql.gz";sql.write_bytes(gzip.compress(b"trusted SQL fixture"))
            tar=root/"bots.tar.gz";tar.write_bytes(self.archive([("bots/a",tarfile.REGTYPE)]).getvalue())
            manifest={"database":{"file":sql.name,"sha256":hashlib.sha256(sql.read_bytes()).hexdigest()},
                      "bots":{"file":tar.name,"sha256":hashlib.sha256(tar.read_bytes()).hexdigest()},
                      "replays":{"file":"replays.tar.gz","sha256":"none"}}
            path=root/"manifest.json";path.write_text(json.dumps(manifest))
            self.assertEqual(verify(path,sql)["database"],str(sql))
            with self.assertRaises(ValueError): verify(path,root/"other.sql.gz")
            tar.write_bytes(b"tampered")
            with self.assertRaises(ValueError): verify(path)

    def test_gzip_trailer_and_quota(self):
        compressed = gzip.compress(b"payload")
        check_gzip(io.BytesIO(compressed))
        with self.assertRaises((EOFError, OSError)):
            check_gzip(io.BytesIO(compressed[:-4]))
        with self.assertRaises(ValueError):
            check_gzip(io.BytesIO(compressed), max_expanded=1)

    def test_manifest_rejects_duplicate_keys(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / "manifest.json"
            path.write_text('{"database": {}, "database": {}}')
            with self.assertRaises(ValueError):
                verify(path)

    def test_restore_wrapper_is_read_only_and_checks_manifest(self):
        wrapper = Path(__file__).parent / "restore.sh"
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            sql = root / "agentrix_db_20260926_120000.sql.gz"
            sql.write_bytes(gzip.compress(b"SELECT 1;"))
            manifest = {
                "database": {"file": sql.name, "sha256": hashlib.sha256(sql.read_bytes()).hexdigest()},
                "bots": {"file": "bots.tar.gz", "sha256": "none"},
                "replays": {"file": "replays.tar.gz", "sha256": "none"},
            }
            path = root / "manifest_20260926_120000.json"
            path.write_text(json.dumps(manifest))
            result = subprocess.run(["bash", str(wrapper), "--verify-only", str(sql)], capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            result = subprocess.run(["bash", str(wrapper), str(sql)], capture_output=True)
            self.assertEqual(result.returncode, 2)
            (root / "bots.tar.gz").write_bytes(b"unverified")
            with self.assertRaises(ValueError):
                verify(path)
            result = subprocess.run(["bash", str(wrapper), "--verify-only", str(sql)], capture_output=True)
            self.assertNotEqual(result.returncode, 0)

    def test_matching_checksum_does_not_hide_truncated_gzip(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            sql = root / "db.sql.gz"
            sql.write_bytes(gzip.compress(b"SELECT 1;")[:-4])
            manifest = {
                "database": {"file": sql.name, "sha256": hashlib.sha256(sql.read_bytes()).hexdigest()},
                "bots": {"file": "bots.tar.gz", "sha256": "none"},
                "replays": {"file": "replays.tar.gz", "sha256": "none"},
            }
            path = root / "manifest.json"
            path.write_text(json.dumps(manifest))
            with self.assertRaises((EOFError, OSError)):
                verify(path)

if __name__ == "__main__": unittest.main()
