from contextlib import closing
import importlib.util
import io
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2] / 'integrations' / 'bunny'
sys.path.insert(0,str(ROOT))

def load(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

upload = load('upload_music')
backup = load('sqlite_backup')

class ToolsTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.addCleanup(self.temp.cleanup)

    def test_unicode_skip_and_conflict(self):
        file = self.root / 'Музика 🎵.flac'
        file.write_bytes(b'audio')
        client = upload.Storage('storage.bunnycdn.com', 'test', 'secret')
        with patch.object(client, 'remote_hash', return_value=upload.digest(io.BytesIO(b'audio'))):
            self.assertEqual(client.upload(file, 'music/' + file.name), 'identical')
        with patch.object(client, 'remote_hash', return_value='different'), patch.object(client, 'request') as request:
            with self.assertRaises(RuntimeError):
                client.upload(file, 'music/' + file.name)
            request.assert_not_called()

    def test_https_path_encoding_and_fail_closed(self):
        client = upload.Storage('storage.bunnycdn.com', 'test-zone', 'secret')
        with patch.object(upload.http.client, 'HTTPSConnection') as connection:
            conn, _ = client.request('GET', 'music/Музика space/#track.flac')
            request = connection.return_value.request.call_args
            self.assertEqual(request.args[1], '/test-zone/music/%D0%9C%D1%83%D0%B7%D0%B8%D0%BA%D0%B0%20space/%23track.flac')
            self.assertEqual(request.kwargs['headers']['AccessKey'], 'secret')
            connection.assert_called_once_with('storage.bunnycdn.com', timeout=120)
        class Response:
            status = 302
        class Connection:
            closed = False
            def close(self): self.closed = True
        conn = Connection()
        with patch.object(client, 'request', return_value=(conn, Response())):
            with self.assertRaisesRegex(RuntimeError, 'HTTP 302'):
                client.remote_hash('music/track')
        self.assertTrue(conn.closed)

    def test_symlinks_hidden_and_traversal(self):
        (self.root / 'track').write_bytes(b'x')
        (self.root / 'link').symlink_to(self.root / 'track')
        (self.root / '.DS_Store').write_bytes(b'x')
        self.assertEqual([p.name for p in upload.files(self.root)], ['track'])
        for path in ('../music', '/music', 'a//b', 'a/./b'):
            with self.assertRaises(ValueError):
                upload.safe_path(path)
        with self.assertRaises(ValueError):
            upload.Storage('attacker.example', 'test', 'secret')

    def test_upload_verification_and_dry_run(self):
        file = self.root / 'track'
        file.write_bytes(b'audio')
        client = upload.Storage('storage.bunnycdn.com', 'test', 'secret')
        response = io.BytesIO(b'')
        response.status = 201
        class Connection:
            def close(self): pass
        with patch.object(client, 'remote_hash', side_effect=[None, 'corrupt']), patch.object(client, 'request', return_value=(Connection(), response)):
            with self.assertRaisesRegex(RuntimeError, 'SHA256'):
                client.upload(file, 'music/track')
        with patch.object(client, 'remote_hash', return_value=None), patch.object(client, 'request') as request:
            self.assertEqual(client.upload(file, 'music/track', True), 'would-upload')
            request.assert_not_called()

    def test_wal_backup_roundtrip_and_tamper(self):
        source, encrypted, restored = [self.root / n for n in ('live.db', 'backup.enc', 'restored.db')]
        key = bytes(range(32))
        with closing(sqlite3.connect(source)) as db, db:
            db.execute('PRAGMA journal_mode=WAL')
            db.execute('CREATE TABLE tracks(name TEXT)')
            db.execute('INSERT INTO tracks VALUES (?)', ('Музика',))
            db.commit()
            backup.backup(source, encrypted, key)
        backup.restore(encrypted, restored, key)
        with closing(sqlite3.connect(restored)) as db, db:
            self.assertEqual(db.execute('SELECT name FROM tracks').fetchone()[0], 'Музика')
        with self.assertRaises(ValueError):
            backup.restore(encrypted, restored, key)
        tampered = bytearray(encrypted.read_bytes())
        tampered[25] ^= 1
        encrypted.write_bytes(tampered)
        failed = self.root / 'failed.db'
        with self.assertRaises(Exception):
            backup.restore(encrypted, failed, key)
        self.assertFalse(failed.exists())
        self.assertFalse(list(self.root.glob('.ndrestore-*')))

    def test_wrong_key_and_invalid_db(self):
        source, encrypted, restored = [self.root / n for n in ('live.db', 'backup.enc', 'restored.db')]
        with closing(sqlite3.connect(source)) as db, db:
            db.execute('CREATE TABLE x(y)')
        backup.backup(source, encrypted, b'a' * 32)
        with self.assertRaises(Exception):
            backup.restore(encrypted, restored, b'b' * 32)
        self.assertFalse(restored.exists())

if __name__ == '__main__':
    unittest.main()
