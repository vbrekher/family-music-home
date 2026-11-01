from contextlib import closing
import io
import importlib
import os
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT=Path(__file__).resolve().parents[2]/'integrations'/'bunny'
sys.path.insert(0,str(ROOT))
runtime=importlib.import_module('runtime')

class RuntimeTest(unittest.TestCase):
    def test_missing_database_fails_closed(self):
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ,{'BUNNY_REQUIRE_BOOTSTRAP':'true'},clear=True):
            with self.assertRaises(ValueError):runtime.bootstrap(Path(folder))
            self.assertFalse((Path(folder)/'navidrome.db').exists())

    def test_explicit_first_bootstrap(self):
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ,{'BUNNY_REQUIRE_BOOTSTRAP':'false'}):
            runtime.bootstrap(Path(folder))
            self.assertFalse((Path(folder)/'navidrome.db').exists())

    def test_thresholds_and_backup_prefix(self):
        self.assertEqual([runtime.resource_level(n,100) for n in (69,70,85,95,110)],[0,70,85,95,95])
        for path in ('music/song.enc','backups/daily/../x.enc','backups/daily/x.db'):
            with self.assertRaises(ValueError):runtime.validate_backup_path(path)
        self.assertEqual(runtime.validate_backup_path('backups/daily/a.enc'),'backups/daily/a.enc')

    def test_authenticated_bootstrap(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);data=root/'data';data.mkdir()
            source=root/'source.db'
            with closing(sqlite3.connect(source)) as db, db:db.execute('CREATE TABLE proof(x)');db.execute('INSERT INTO proof VALUES (42)');db.execute('CREATE TABLE user(id TEXT, user_name TEXT, is_admin INTEGER)');db.executemany('INSERT INTO user(id,user_name,is_admin) VALUES (NULL,?,?)',[('admin',1),('member',0)])
            encrypted=root/'backup.enc';key=b'x'*32
            runtime.backup(source,encrypted,key)
            def fetch(client,remote,target):target.write_bytes(encrypted.read_bytes())
            with patch.dict(os.environ,{'BUNNY_BACKUP_KEY_HEX':key.hex(),'BUNNY_REQUIRE_BOOTSTRAP':'true','BUNNY_BOOTSTRAP_OBJECT':'backups/seed/test.enc'}),patch.object(runtime,'storage',return_value=object()),patch.object(runtime,'fetch',side_effect=fetch):
                runtime.bootstrap(data)
            with closing(sqlite3.connect(data/'navidrome.db')) as db, db:self.assertEqual(db.execute('SELECT x FROM proof').fetchone()[0],42)

    def test_listing_selects_latest_without_deletes(self):
        class Connection:
            def close(self):pass
        class Client:
            def request(self,method,path):
                assert method=='GET'
                response=io.BytesIO(b'[{"ObjectName":"navidrome-20261027T010000-aaaaaaaa.db.enc","IsDirectory":false}]')
                response.status=200
                return Connection(),response
        self.assertEqual(runtime.latest_backup(Client()),'backups/weekly/navidrome-20261027T010000-aaaaaaaa.db.enc')

    def test_retention_delete_allowlist_and_seed_protection(self):
        import json
        names = [f'navidrome-202610{i:02d}T010000-aaaaaaaa.db.enc' for i in range(1, 11)]
        malicious = ['../music/song.enc', 'navidrome-not-a-date.db.enc', '/music/x']
        class Connection:
            def close(self):pass
        class Client:
            def __init__(self):self.deletes=[]
            def request(self,method,path):
                if method=='GET':
                    entries=[{'ObjectName':n,'IsDirectory':False} for n in names+malicious] if path=='backups/daily/' else []
                    response=io.BytesIO(json.dumps(entries).encode());response.status=200
                else:
                    self.deletes.append(path);response=io.BytesIO(b'');response.status=204
                return Connection(),response
        client=Client()
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ,{'BUNNY_BOOTSTRAP_OBJECT':'backups/daily/'+names[0]}),patch.object(runtime,'fetch') as fetch,patch.object(runtime,'restore') as restore:
            runtime.prune_backups(Path(folder),client,b'x'*32)
            fetch.assert_called_once();restore.assert_called_once()
        self.assertEqual(client.deletes,['backups/daily/'+names[2],'backups/daily/'+names[1]])

    def test_retention_failed_restore_never_deletes(self):
        import json
        class Connection:
            def close(self):pass
        class Client:
            deletes=[]
            def request(self,method,path):
                if method=='DELETE':self.deletes.append(path)
                entries=[{'ObjectName':f'navidrome-202610{i:02d}T010000-aaaaaaaa.db.enc','IsDirectory':False} for i in range(1,10)]
                response=io.BytesIO(json.dumps(entries).encode());response.status=200
                return Connection(),response
        client=Client()
        with tempfile.TemporaryDirectory() as folder,patch.object(runtime,'fetch'),patch.object(runtime,'restore',side_effect=ValueError('bad key')):
            with self.assertRaises(ValueError):runtime.prune_backups(Path(folder),client,b'x'*32)
        self.assertEqual(client.deletes,[])

    def test_existing_empty_database_refuses_production_start(self):
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ,{'BUNNY_REQUIRE_BOOTSTRAP':'true'}):
            database=Path(folder)/'navidrome.db'
            with closing(sqlite3.connect(database)) as db, db: db.execute('CREATE TABLE user(id TEXT,user_name TEXT,is_admin INTEGER)')
            with self.assertRaises(ValueError): runtime.bootstrap(Path(folder))
            with closing(sqlite3.connect(database)) as db, db:
                db.executemany('INSERT INTO user(id,user_name,is_admin) VALUES (NULL,?,?)',[('admin',1),('member',1)])
            with self.assertRaises(ValueError): runtime.bootstrap(Path(folder))

    def test_first_bootstrap_flag_allows_empty_database(self):
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ,{'BUNNY_REQUIRE_BOOTSTRAP':'false'}):
            database=Path(folder)/'navidrome.db'
            with closing(sqlite3.connect(database)) as db, db: db.execute('CREATE TABLE user(id TEXT,user_name TEXT,is_admin INTEGER)')
            runtime.bootstrap(Path(folder))

    def test_retention_keeps_distinct_days_not_restart_count(self):
        import json
        names=[f'navidrome-202610{i:02d}T010000-aaaaaaaa.db.enc' for i in range(1,9)]
        duplicates=[f'navidrome-20261008T{h:02d}0000-bbbbbbbb.db.enc' for h in range(2,12)]
        class Connection:
            def close(self):pass
        class Client:
            def __init__(self):self.deletes=[]
            def request(self,method,path):
                entries=[{'ObjectName':n,'IsDirectory':False} for n in names+duplicates] if path=='backups/daily/' else []
                if method=='DELETE':self.deletes.append(path)
                response=io.BytesIO(json.dumps(entries).encode());response.status=200
                return Connection(),response
        client=Client()
        with tempfile.TemporaryDirectory() as folder,patch.object(runtime,'fetch'),patch.object(runtime,'restore'):
            runtime.prune_backups(Path(folder),client,b'x'*32)
        remaining={n for n in names+duplicates if 'backups/daily/'+n not in client.deletes}
        self.assertEqual(len(remaining),7)
        self.assertEqual({n[10:18] for n in remaining},{f'202610{i:02d}' for i in range(2,9)})
        self.assertIn('navidrome-20261008T110000-bbbbbbbb.db.enc',remaining)

    def test_retention_weekly_groups_iso_week(self):
        import json
        names=[f'navidrome-202610{i:02d}T010000-aaaaaaaa.db.enc' for i in range(1,29)]
        class Connection:
            def close(self):pass
        class Client:
            def __init__(self):self.deletes=[]
            def request(self,method,path):
                entries=[{'ObjectName':n,'IsDirectory':False} for n in names] if path=='backups/weekly/' else []
                if method=='DELETE':self.deletes.append(path)
                response=io.BytesIO(json.dumps(entries).encode());response.status=200
                return Connection(),response
        client=Client()
        with tempfile.TemporaryDirectory() as folder,patch.object(runtime,'fetch'),patch.object(runtime,'restore'):
            runtime.prune_backups(Path(folder),client,b'x'*32)
        remaining={n for n in names if 'backups/weekly/'+n not in client.deletes}
        self.assertEqual(len(remaining),4)
        periods={runtime.dt.datetime.strptime(n[10:18],'%Y%m%d').date().isocalendar()[:2] for n in remaining}
        self.assertEqual(len(periods),4)


class PublicRuntimeTest(unittest.TestCase):
    def test_public_local_start_needs_no_bunny_configuration(self):
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ, {'FMH_INSTALL_MODE':'PUBLIC_SELF_HOST','ND_DATAFOLDER':folder}, clear=True), patch.object(runtime.os,'execvp',return_value=0) as execute, patch.object(runtime,'storage',side_effect=AssertionError('Bunny must not be accessed')), patch.object(runtime,'backup_key',side_effect=AssertionError('No backup key needed')):
            with patch.object(sys,'argv',['runtime.py','/app/navidrome']):
                self.assertEqual(runtime.main(),0)
            self.assertTrue((Path(folder)/'setup-token').is_file())
            self.assertFalse((Path(folder)/'navidrome.db').exists())
            execute.assert_called_once_with('/app/navidrome',['/app/navidrome'])

    def test_public_mode_refuses_conflicting_locked_policy(self):
        with tempfile.TemporaryDirectory() as folder, patch.dict(os.environ, {'FMH_INSTALL_MODE':'PUBLIC_SELF_HOST','BUNNY_FAMILY_MODE':'true','ND_DATAFOLDER':folder}, clear=True):
            with self.assertRaises(ValueError):runtime.main()
            self.assertFalse((Path(folder)/'setup-token').exists())
