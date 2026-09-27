import gzip
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import uuid
import zipfile

from backup_bundle import create_bundle
from restore_bundle import new_target, publish_new, restore_bundle


class RestoreBundleTests(unittest.TestCase):
    def test_existing_destination_is_never_replaced(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, destination = root / "staged", root / "live"
            source.mkdir(); destination.mkdir()
            with self.assertRaises(ValueError): new_target(destination)
            with self.assertRaises(OSError): publish_new(source, destination)
            self.assertTrue(source.is_dir())
            self.assertTrue(destination.is_dir())

    def test_trusted_sql_acknowledgment_precedes_all_io(self):
        with self.assertRaises(ValueError):
            restore_bundle("missing", "new_db", "missing", "missing", "missing", [])

    @unittest.skipUnless(os.environ.get("AGENTRIX_TEST_RECOVERY"), "requires isolated PostgreSQL and compiled verifier")
    def test_real_database_100_mib_bot_and_replay_recovery(self):
        repo = Path(__file__).resolve().parent.parent
        connection = ["-h", os.environ.get("AGENTRIX_TEST_PG_HOST", "127.0.0.1"),
                      "-p", os.environ.get("AGENTRIX_TEST_PG_PORT", "55439"),
                      "-U", os.environ.get("AGENTRIX_TEST_PG_USER", "f4nk1")]
        source_db = "agentrix_source_" + uuid.uuid4().hex
        restored_db = "agentrix_restored_" + uuid.uuid4().hex
        verifier = os.environ["AGENTRIX_TEST_RECOVERY_VERIFIER"]
        def sql(database, text):
            return subprocess.check_output(["psql", "-X", *connection, "-d", database, "-v", "ON_ERROR_STOP=1", "-At", "-c", text], text=True).strip()
        def quote(value): return "'" + str(value).replace("'", "''") + "'"
        subprocess.run(["createdb", *connection, source_db], check=True)
        restored_created = False
        try:
            for migration in sorted((repo / "backend/migrations").glob("*.up.sql")):
                subprocess.run(["psql", "-X", *connection, "-d", source_db, "-v", "ON_ERROR_STOP=1", "-f", str(migration)],
                               check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                bots, replays, backups = (root / name for name in ("source-bots", "source-replays", "backups"))
                for path in (bots, replays, backups): path.mkdir()
                tree = bots / "version"; tree.mkdir()
                original = bots / "version.zip"
                example = repo / "bots/heuristic_bot"
                def write_zip(padding):
                    with zipfile.ZipFile(original, "w", compression=zipfile.ZIP_STORED) as archive:
                        for name in ("agent.py", "agentrix.json"):
                            info = zipfile.ZipInfo(name)
                            info.external_attr = (0o100755 if name == "agent.py" else 0o100644) << 16
                            archive.writestr(info, (example / name).read_bytes())
                        info = zipfile.ZipInfo("padding.bin"); info.external_attr = 0o100644 << 16
                        with archive.open(info, "w") as output:
                            while padding:
                                count = min(1024**2, padding); output.write(b"\0" * count); padding -= count
                write_zip(0); write_zip(100 * 1024**2 - original.stat().st_size)
                self.assertEqual(original.stat().st_size, 100 * 1024**2)
                with zipfile.ZipFile(original) as archive:
                    for info in archive.infolist():
                        with archive.open(info) as source, (tree / info.filename).open("wb") as destination:
                            while chunk := source.read(1024**2): destination.write(chunk)
                        (tree / info.filename).chmod((info.external_attr >> 16) & 0o777)
                digest = hashlib.sha256()
                for path in sorted(tree.iterdir()):
                    name = path.name.encode(); size = path.stat().st_size
                    digest.update(struct.pack(">Q", len(name))); digest.update(name)
                    digest.update(struct.pack(">I", path.stat().st_mode & 0o111)); digest.update(b"f")
                    digest.update(struct.pack(">Q", size))
                    with path.open("rb") as stream:
                        while chunk := stream.read(1024**2): digest.update(chunk)
                with original.open("rb") as stream: zip_hash = hashlib.file_digest(stream, "sha256").hexdigest()
                replay = replays / "fixture.json.gz"; replay.write_bytes(gzip.compress(b'{"fixture":true}'))
                replay_hash = hashlib.sha256(replay.read_bytes()).hexdigest()
                sql(source_db, "INSERT INTO users(username,email,password_hash) VALUES ('fixture','fixture@example.test','not-login'); "
                    "INSERT INTO teams(name,owner_id) VALUES ('fixture',1); INSERT INTO arenas(slug,name) VALUES ('fixture','fixture'); "
                    "INSERT INTO agent_versions(team_id,arena_id,name,artifact_path,sha256,artifact_sha256) VALUES (1,1,'display name',"+
                    quote(tree)+","+quote(zip_hash)+","+quote(digest.hexdigest())+"); "
                    "INSERT INTO ladder_entries(arena_id,agent_version_id,display_rating,matches_played) VALUES (1,1,1700,3); "
                    "INSERT INTO matches(arena_id,seed,status) VALUES (1,42,'finished'); "
                    "INSERT INTO replays(match_id,file_path,sha256,size_bytes) VALUES (1,"+quote(replay)+","+quote(replay_hash)+","+str(replay.stat().st_size)+");")
                manifest = create_bundle(backups,bots,replays,[*connection,"-d",source_db],True)
                final_bots,final_replays = root/"recovered-bots",root/"recovered-replays"
                environment = {"PGHOST":connection[1],"PGPORT":connection[3],"PGUSER":connection[5]}
                with patch.dict(os.environ,environment):
                    try:
                        receipt = restore_bundle(manifest,restored_db,final_bots,final_replays,verifier,connection,True)
                    finally:
                        restored_created = sql("postgres", "SELECT count(*) FROM pg_database WHERE datname="+quote(restored_db)) == "1"
                self.assertEqual(json.loads(receipt.read_text())["status"],"complete")
                self.assertEqual(sql(restored_db,"SELECT artifact_path FROM agent_versions"),str(final_bots/"version"))
                self.assertEqual(sql(restored_db,"SELECT file_path FROM replays"),str(final_replays/replay.name))
                self.assertEqual(sql(restored_db,"SELECT display_rating,matches_played FROM ladder_entries"),"1700|3")
                self.assertEqual((final_bots/"version/agent.py").stat().st_mode & 0o111,0o111)
                with (final_bots/"version.zip").open("rb") as stream:
                    self.assertEqual(hashlib.file_digest(stream,"sha256").hexdigest(),zip_hash)
                self.assertEqual(sql(source_db,"SELECT artifact_path FROM agent_versions"),str(tree))
                if os.environ.get("AGENTRIX_TEST_RECOVERY_SANDBOX"):
                    subprocess.run([verifier,"--check-bot",str(final_bots/"version"),"--arbiter",
                                    os.environ["AGENTRIX_TEST_ARBITER_PATH"]],check=True)
                # A checksum-valid bundle can still contain inconsistent DB
                # provenance. The agent UPDATE happens before replay validation:
                # a bad replay must roll the whole remapping transaction back.
                bad_db = "agentrix_bad_recovery_" + uuid.uuid4().hex
                sql(source_db,"UPDATE replays SET sha256=" + quote("0" * 64))
                bad_manifest = create_bundle(backups,bots,replays,[*connection,"-d",source_db],True)
                bad_bots,bad_replays = root/"bad-bots",root/"bad-replays"
                try:
                    with patch.dict(os.environ,environment), self.assertRaises(subprocess.CalledProcessError):
                        restore_bundle(bad_manifest,bad_db,bad_bots,bad_replays,verifier,connection,True)
                    self.assertEqual(sql(bad_db,"SELECT artifact_path FROM agent_versions"),str(tree))
                    self.assertFalse(bad_bots.exists())
                    self.assertFalse(bad_replays.exists())
                    self.assertFalse((root/("agentrix-recovery-"+bad_db+".json")).exists())
                finally:
                    if sql("postgres", "SELECT count(*) FROM pg_database WHERE datname="+quote(bad_db)) == "1":
                        subprocess.run(["dropdb",*connection,bad_db],check=True)
        finally:
            if restored_created: subprocess.run(["dropdb",*connection,restored_db],check=True)
            subprocess.run(["dropdb",*connection,source_db],check=True)


if __name__ == "__main__": unittest.main()
