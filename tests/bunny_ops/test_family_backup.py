from contextlib import closing
import hashlib
import importlib
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest
sys.path.insert(0,str(Path(__file__).resolve().parents[2]/'integrations'/'bunny'))
backup=importlib.import_module('sqlite_backup')
family=importlib.import_module('family_backup')
class FamilyBackupTest(unittest.TestCase):
 def fixture(self,root):
  source=root/'source';source.mkdir();media=source/'family-media';media.mkdir();data=b'private immutable bytes';key='syntheticphoto001';(media/key).write_bytes(data)
  with closing(sqlite3.connect(source/'navidrome.db')) as db,db:
   db.execute('CREATE TABLE family_attachment(id TEXT,size INTEGER,sha256 TEXT,state TEXT)');db.execute('INSERT INTO family_attachment VALUES(?,?,?,?)',(key,len(data),hashlib.sha256(data).hexdigest(),'ready'))
  return source,key,data
 def test_encrypted_complete_roundtrip_and_no_overwrite(self):
  with tempfile.TemporaryDirectory() as folder:
   root=Path(folder);source,key,data=self.fixture(root);encrypted=root/'private.enc';backup.backup(source/'navidrome.db',encrypted,b'x'*32);self.assertEqual(encrypted.read_bytes()[:8],family.MAGIC);self.assertNotIn(data,encrypted.read_bytes())
   target=root/'restored';target.mkdir();backup.restore(encrypted,target/'navidrome.db',b'x'*32);self.assertEqual((target/'family-media'/key).read_bytes(),data);self.assertEqual(family.attachments(source/'navidrome.db'),family.attachments(target/'navidrome.db'))
   with self.assertRaises(ValueError):backup.restore(encrypted,target/'other.db',b'x'*32)
   self.assertFalse((target/'other.db').exists())
 def test_missing_or_tampered_ready_blob_prevents_backup(self):
  with tempfile.TemporaryDirectory() as folder:
   root=Path(folder);source,key,data=self.fixture(root);(source/'family-media'/key).write_bytes(b'changed')
   with self.assertRaises(ValueError):backup.backup(source/'navidrome.db',root/'bad.enc',b'x'*32)
   self.assertFalse((root/'bad.enc').exists())
 def test_authenticated_tamper_never_publishes_database_or_media(self):
  with tempfile.TemporaryDirectory() as folder:
   root=Path(folder);source,key,data=self.fixture(root);encrypted=root/'private.enc';backup.backup(source/'navidrome.db',encrypted,b'x'*32);raw=bytearray(encrypted.read_bytes());raw[-1]^=1;encrypted.write_bytes(raw);target=root/'restore';target.mkdir()
   with self.assertRaises(Exception):backup.restore(encrypted,target/'navidrome.db',b'x'*32)
   self.assertEqual(list(target.iterdir()),[])
 def test_traversal_and_links_rejected(self):
  import tarfile,io
  for name,kind in [('../escape',tarfile.REGTYPE),('family-media/../escape',tarfile.REGTYPE),('navidrome.db',tarfile.SYMTYPE)]:
   with self.subTest(name=name),tempfile.TemporaryDirectory() as folder:
    root=Path(folder);archive=root/'unsafe.tar'
    with tarfile.open(archive,'w') as tar:
     info=tarfile.TarInfo(name);info.type=kind;info.size=0;tar.addfile(info,io.BytesIO())
    with self.assertRaises(ValueError):family.unpack(archive,root)
 def test_runtime_bootstrap_restores_database_and_private_blobs(self):
  from unittest.mock import patch
  import os
  runtime=importlib.import_module('runtime')
  with tempfile.TemporaryDirectory() as folder:
   root=Path(folder);source,key,data=self.fixture(root)
   with closing(sqlite3.connect(source/'navidrome.db')) as db,db:db.execute('CREATE TABLE user(id TEXT,user_name TEXT,is_admin INTEGER)');db.executemany('INSERT INTO user(id,user_name,is_admin) VALUES(NULL,?,?)',[('admin',1),('member',0)])
   encrypted=root/'private.enc';secret=b'x'*32;backup.backup(source/'navidrome.db',encrypted,secret);target=root/'bootstrap';target.mkdir()
   def fetch(client,remote,path):path.write_bytes(encrypted.read_bytes())
   with patch.dict(os.environ,{'BUNNY_BACKUP_KEY_HEX':secret.hex(),'BUNNY_REQUIRE_BOOTSTRAP':'true','BUNNY_BOOTSTRAP_OBJECT':'backups/seed/private.enc'}),patch.object(runtime,'storage',return_value=object()),patch.object(runtime,'fetch',side_effect=fetch):runtime.bootstrap(target)
   self.assertTrue((target/'navidrome.db').exists());self.assertEqual((target/'family-media'/key).read_bytes(),data)
 def test_legacy_sqlite_only_copy_with_ready_media_fails_closed(self):
  from unittest.mock import patch
  with tempfile.TemporaryDirectory() as folder:
   root=Path(folder);source,key,data=self.fixture(root);encrypted=root/'incomplete.enc'
   with patch.object(family,'attachments',return_value=None):backup.backup(source/'navidrome.db',encrypted,b'x'*32)
   target=root/'restore';target.mkdir()
   with self.assertRaises(ValueError):backup.restore(encrypted,target/'navidrome.db',b'x'*32)
   self.assertEqual(list(target.iterdir()),[])
