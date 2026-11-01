from contextlib import closing
import importlib
import io
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'integrations/bunny'))
full = importlib.import_module('full_backup')
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes


class FullBackupTest(unittest.TestCase):
    def test_a_changed_inventory_is_not_published_as_a_complete_backup(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            source = self.fixture(root)
            addfile = tarfile.TarFile.addfile
            def change_during_pack(archive, member, stream=None):
                addfile(archive, member, stream)
                if member.name == 'navidrome.db':
                    (source / 'late-file').write_bytes(b'new state after enumeration')
            with patch.object(tarfile.TarFile, 'addfile', change_during_pack):
                with self.assertRaises(ValueError):
                    full.backup(source, root / 'changed.enc', os.urandom(32))
            self.assertFalse((root / 'changed.enc').exists())

    def fixture(self, root):
        source = root / 'source'
        source.mkdir()
        for path in (source / 'navidrome.db', source / 'plugins/example-plugin/kvstore.db'):
            path.parent.mkdir(parents=True, exist_ok=True)
            with closing(sqlite3.connect(path)) as db, db:
                db.execute('CREATE TABLE proof(value text)')
                db.execute('INSERT INTO proof VALUES(?)', ('synthetic persistent state',))
        (source / 'plugins/example.ndp').write_bytes(b'synthetic package bytes')
        (source / 'family-media').mkdir()
        (source / 'family-media/fictional-attachment').write_bytes(b'private synthetic attachment')
        (source / 'cache').mkdir()
        (source / 'cache/temporary').write_bytes(b'rebuildable cache')
        return source

    def test_full_roundtrip_preserves_separate_plugin_state_and_private_media(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            source = self.fixture(root)
            archive, restored = root / 'full.enc', root / 'restored'
            key = os.urandom(32)
            full.backup(source, archive, key)
            self.assertEqual(archive.read_bytes()[:8], b'FMHFULL1')
            self.assertNotIn(b'synthetic persistent state', archive.read_bytes())
            self.assertEqual(archive.stat().st_mode & 0o777, 0o600)
            full.restore(archive, restored, key)
            self.assertFalse((restored / 'cache').exists())
            for name in ('navidrome.db', 'plugins/example-plugin/kvstore.db', 'plugins/example.ndp', 'family-media/fictional-attachment'):
                self.assertEqual((restored / name).read_bytes(), (source / name).read_bytes())

    def test_cold_wal_commits_are_verified_without_writing_the_source(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            source = self.fixture(root)
            paths = (source / 'navidrome.db', source / 'plugins/example-plugin/kvstore.db')
            for path in paths:
                # Simulate a stopped/crashed writer with a committed WAL and no
                # reusable shared-memory index; committed rows are not in main DB.
                script = "import sqlite3,os,sys;c=sqlite3.connect(sys.argv[1]);c.execute('PRAGMA journal_mode=WAL');c.execute('PRAGMA wal_autocheckpoint=0');c.execute(\"INSERT INTO proof VALUES('wal-only-committed-state')\");c.commit();os._exit(0)"
                subprocess.run([sys.executable, '-c', script, str(path)], check=True)
                self.assertTrue(Path(str(path) + '-wal').is_file())
                Path(str(path) + '-shm').unlink(missing_ok=True)
            before = {str(p.relative_to(source)): p.read_bytes() for p in source.rglob('*') if p.is_file()}
            key = os.urandom(32)
            archive, restored = root / 'wal.enc', root / 'restored'
            full.backup(source, archive, key)
            self.assertEqual(before, {str(p.relative_to(source)): p.read_bytes() for p in source.rglob('*') if p.is_file()})
            full.restore(archive, restored, key)
            for path in paths:
                with closing(sqlite3.connect(restored / path.relative_to(source))) as db:
                    self.assertEqual(db.execute('SELECT value FROM proof ORDER BY rowid').fetchall(), [('synthetic persistent state',), ('wal-only-committed-state',)])

    def test_wrong_key_and_tamper_never_publish_a_restore(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            source, key = self.fixture(root), os.urandom(32)
            archive = root / 'full.enc'
            full.backup(source, archive, key)
            with self.assertRaises(Exception):
                full.restore(archive, root / 'wrong-key', os.urandom(32))
            self.assertFalse((root / 'wrong-key').exists())
            raw = bytearray(archive.read_bytes())
            raw[40] ^= 1
            archive.write_bytes(raw)
            with self.assertRaises(Exception):
                full.restore(archive, root / 'tampered', key)
            self.assertFalse((root / 'tampered').exists())

    def test_source_symlinks_and_existing_or_recursive_destinations_refused(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            source, key = self.fixture(root), os.urandom(32)
            with self.assertRaises(ValueError):
                full.backup(source, source / 'recursive.enc', key)
            archive = root / 'full.enc'
            archive.write_bytes(b'keep existing')
            with self.assertRaises(ValueError):
                full.backup(source, archive, key)
            self.assertEqual(archive.read_bytes(), b'keep existing')
            (source / 'link').symlink_to(source / 'navidrome.db')
            with self.assertRaises(ValueError):
                full.backup(source, root / 'unsafe.enc', key)
            self.assertFalse((root / 'unsafe.enc').exists())

    def test_authenticated_traversal_links_and_duplicates_are_rejected(self):
        for names, link in [(['../escape'], False), (['/absolute'], False), (['plugins/link'], True), (['same', 'same'], False)]:
            with self.subTest(names=names, link=link), tempfile.TemporaryDirectory() as folder:
                root, key = Path(folder), os.urandom(32)
                stream = io.BytesIO()
                with tarfile.open(fileobj=stream, mode='w') as archive:
                    for name in names:
                        member = tarfile.TarInfo(name)
                        if link:
                            member.type, member.linkname = tarfile.SYMTYPE, '/outside'
                        archive.addfile(member)
                header = full.MAGIC + os.urandom(12)
                cipher = Cipher(algorithms.AES(key), modes.GCM(header[8:])).encryptor()
                cipher.authenticate_additional_data(header)
                raw = header + cipher.update(stream.getvalue()) + cipher.finalize() + cipher.tag
                (root / 'unsafe.enc').write_bytes(raw)
                with self.assertRaises(ValueError):
                    full.restore(root / 'unsafe.enc', root / 'restored', key)
                self.assertFalse((root / 'restored').exists())
                self.assertFalse((root / 'escape').exists())
