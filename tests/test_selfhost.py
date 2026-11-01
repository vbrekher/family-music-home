import importlib.util
import os
from pathlib import Path
import sqlite3
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('selfhost', Path(__file__).resolve().parents[1] / 'integrations/selfhost/entrypoint.py')
selfhost = importlib.util.module_from_spec(spec)
spec.loader.exec_module(selfhost)


class FirstRunTest(unittest.TestCase):
    def test_remote_operator_can_supply_a_private_setup_secret(self):
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ, {'FMH_SETUP_TOKEN': 'synthetic_setup_value_with_more_than_32_chars'}):
            data = Path(folder)
            selfhost.prepare(data)
            self.assertEqual((data / 'setup-token').read_text().strip(), os.environ['FMH_SETUP_TOKEN'])
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ, {'FMH_SETUP_TOKEN': 'too-short'}):
            with self.assertRaises(ValueError):
                selfhost.prepare(Path(folder))
            self.assertFalse((Path(folder) / 'setup-token').exists())

    def test_token_is_private_stable_and_removed_after_registration(self):
        with tempfile.TemporaryDirectory() as folder:
            data = Path(folder)
            selfhost.prepare(data)
            token = data / 'setup-token'
            original = token.read_bytes()
            self.assertGreaterEqual(len(original), 43)
            self.assertEqual(token.stat().st_mode & 0o777, 0o600)
            selfhost.prepare(data)
            self.assertEqual(token.read_bytes(), original)
            with sqlite3.connect(data / 'navidrome.db') as db:
                db.execute('CREATE TABLE user(user_name text)')
                db.execute('INSERT INTO user VALUES(?)', ('arbitrary-admin',))
            selfhost.prepare(data)
            self.assertFalse(token.exists())
            with sqlite3.connect(data / 'navidrome.db') as db:
                self.assertEqual(db.execute('SELECT user_name FROM user').fetchall(), [('arbitrary-admin',)])


if __name__ == '__main__':
    unittest.main()
