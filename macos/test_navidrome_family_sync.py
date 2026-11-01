#!/usr/bin/env python3
"""Run: python3 -m unittest discover -s macos -p 'test_*.py' -v"""
import json
import os
from pathlib import Path
import sqlite3
import stat
import tempfile
import time
import unittest
from unittest import mock

import navidrome_family_sync as fs


class SyncTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.music = self.base / 'music'
        self.music.mkdir()
        self.state = self.base / 'state'
        self.cfg = dict(music_dir=str(self.music), server="https://music.example.invalid",
                        zone="synthetic-music-zone", host='storage.bunnycdn.com', owner='admin')
        self.uploads = []
        self.scans = []
        self.notifications = []
        self.now = time.time() + 1000

    def add(self, name='Artist/Album/Track.flac', payload=b'file bytes'):
        p = self.music / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_bytes(payload)
        os.utime(p, (self.now - 1000, self.now - 1000))
        return p

    def upload(self, path, rel):
        self.uploads.append((path, rel))
        return 'verified'

    def scan(self):
        self.scans.append(True)

    def run_agent(self, at=None, uploader=None, scanner=None):
        return fs.sync_once(self.cfg, self.state, uploader=uploader or self.upload,
                            scanner=scanner or self.scan, notifier=self.notifications.append,
                            now=self.now if at is None else at)

    def test_two_poll_stability_and_no_network_first_poll(self):
        self.add()
        self.assertEqual(self.run_agent()['pending'], 1)
        self.assertEqual(self.uploads, [])
        self.assertEqual(self.run_agent(self.now+299)['pending'], 1)
        self.assertEqual(self.uploads, [])
        r = self.run_agent(self.now+300)
        self.assertEqual(r['uploaded'], 1)
        self.assertEqual(self.uploads[0][1], 'Artist/Album/Track.flac')
        self.assertEqual(len(self.scans), 1)
        self.assertEqual(len(self.notifications), 1)

    def test_unchanged_makes_no_remote_requests_or_hash_reads(self):
        self.add()
        self.run_agent()
        self.run_agent(self.now+300)
        self.uploads.clear(); self.scans.clear()
        with mock.patch.object(fs, 'time') as clock:
            clock.time.return_value = self.now + 900
            r=self.run_agent(self.now+900, uploader=lambda *_: self.fail('network called'),
                             scanner=lambda: self.fail('scan called'))
        self.assertEqual(r['skipped'],1)
        self.assertEqual(self.uploads,[])

    def test_mac_deleted_files_never_deleted_from_remote(self):
        f=self.add(); self.run_agent();self.run_agent(self.now+300)
        f.unlink(); self.uploads.clear(); self.scans.clear()
        self.assertEqual(self.run_agent(self.now+600)['uploaded'],0)
        db=fs.Manifest(self.state/'manifest.sqlite')
        self.assertEqual(db.get('Artist/Album/Track.flac')[6], 'synced')
        db.close()
        self.assertEqual(self.scans,[])

    def test_replaced_file_never_overwritten(self):
        file=self.add(); self.run_agent();self.run_agent(self.now+300)
        file.write_bytes(b'changed content')
        os.utime(file,(self.now-1000,self.now-1000))
        self.run_agent(self.now+600)
        def conflict(*_): raise RuntimeError('name conflict; refusing overwrite')
        result=self.run_agent(self.now+900, uploader=conflict)
        self.assertEqual(result['conflicts'],1)
        self.assertEqual(fs.Manifest(self.state/'manifest.sqlite').get('Artist/Album/Track.flac')[6],'conflict')

    def test_verified_and_scan_obligation_commit_together(self):
        self.add(); self.run_agent()
        db=fs.Manifest(self.state/'manifest.sqlite')
        db.verified('Artist/Album/Track.flac',10,20,'a'*64,self.now)
        db.close()
        db=fs.Manifest(self.state/'manifest.sqlite')
        self.assertEqual(db.get_meta('scan_pending'),'1'); db.close()

    def test_unchanged_conflict_does_not_repeat_remote_hash(self):
        self.add(); self.run_agent()
        def conflict(*_): raise RuntimeError('name conflict')
        self.assertEqual(self.run_agent(self.now+300,uploader=conflict)['conflicts'],1)
        self.assertEqual(self.run_agent(self.now+600,uploader=lambda *_:self.fail('network retry'))['conflicts'],1)

    def test_idle_cli_never_reads_keychain(self):
        self.add(); self.run_agent(); self.run_agent(self.now+300)
        with mock.patch.object(fs,'APP_DIR',self.state), \
             mock.patch.object(fs,'read_config',return_value=self.cfg), \
             mock.patch.object(fs.sys,'argv',['sync','run-once']), \
             mock.patch.object(fs,'keychain_read',side_effect=AssertionError('idle credential access')):
            self.assertEqual(fs.main(),0)

    def test_ten_thousand_unchanged_files_no_network_or_reads(self):
        from hashlib import sha256
        db=fs.Manifest(self.state/'manifest.sqlite')
        for i in range(10000):
            f=self.add(f'benchmark/{i}.mp3'); st=f.stat();rel=f.relative_to(self.music).as_posix()
            db.observe(rel,st.st_size,st.st_mtime_ns,self.now)
            db.verified(rel,st.st_size,st.st_mtime_ns,sha256(b'file bytes').hexdigest(),self.now)
        db.set_meta('scan_pending','0');db.close()
        started=time.monotonic()
        original=Path.open
        def guard(path,*args,**kwargs):
            if path.suffix=='.mp3':self.fail('unchanged audio opened')
            return original(path,*args,**kwargs)
        with mock.patch.object(Path,'open',guard):
            result=self.run_agent(self.now+600,uploader=lambda *_:self.fail('network'),scanner=lambda:self.fail('scan'))
        self.assertEqual(result['skipped'],10000)
        print('10,000 unchanged metadata poll seconds:',round(time.monotonic()-started,3))

    def test_transient_error_retried(self):
        self.add(); self.run_agent()
        tries=[]
        def flaky(f,rel):
            tries.append(1)
            if len(tries)<3: raise OSError('temporary outage')
            return 'verified'
        with mock.patch.object(fs.time,'sleep'):
            r=self.run_agent(self.now+300, uploader=flaky)
        self.assertEqual(r['uploaded'],1)
        self.assertEqual(len(tries),3)

    def test_scanner_failure_keeps_pending_scan(self):
        self.add(); self.run_agent()
        r=self.run_agent(self.now+300, scanner=lambda: (_ for _ in ()).throw(RuntimeError('down')))
        self.assertEqual(r['failed'],1)
        db=fs.Manifest(self.state/'manifest.sqlite'); self.assertEqual(db.get_meta('scan_pending'),'1');db.close()
        self.scans.clear()
        self.run_agent(self.now+600)
        self.assertEqual(len(self.scans),1)
        db=fs.Manifest(self.state/'manifest.sqlite'); self.assertEqual(db.get_meta('scan_pending'),'0');db.close()

    def test_remote_identical_triggers_scan(self):
        self.add(); self.run_agent()
        r=self.run_agent(self.now+300, uploader=lambda *_:'identical')
        self.assertEqual(r['identical'],1)
        self.assertEqual(len(self.scans),1)

    def test_hidden_symlink_and_partial_skipped(self):
        self.add('.hidden.mp3'); self.add('download.part'); self.add('a.tmp');
        real=self.add('album/song.mp3')
        (self.music/'linked.mp3').symlink_to(real)
        self.assertEqual(self.run_agent()['pending'],1)
        self.assertEqual(self.run_agent(self.now+300)['uploaded'],1)
        self.assertEqual(len(self.uploads),1)

    def test_zero_byte_deferred(self):
        self.add('empty.flac',b'')
        self.assertEqual(self.run_agent()['pending'],1)
        self.assertEqual(self.run_agent(self.now+600)['pending'],1)
        self.assertEqual(self.uploads,[])

    def test_root_symlink_rejected(self):
        link=self.base/'link'; link.symlink_to(self.music, target_is_directory=True)
        self.cfg['music_dir']=str(link)
        with self.assertRaises(RuntimeError): self.run_agent()

    def test_manifest_private_mode_and_no_secret(self):
        self.add(); self.run_agent(); self.run_agent(self.now+300)
        path=self.state/'manifest.sqlite'
        self.assertEqual(stat.S_IMODE(path.stat().st_mode),0o600)
        self.assertEqual(stat.S_IMODE(self.state.stat().st_mode),0o700)
        self.assertNotIn('password', path.read_bytes().decode('latin-1').lower())

    def test_exact_unchanged_metadata_only_shortcut(self):
        f=self.add(); self.run_agent();self.run_agent(self.now+300)
        # Change mtime only: scan must observe again and revalidate, not blindly skip.
        os.utime(f,ns=(int((self.now-50)*1e9),int((self.now-50)*1e9)))
        self.assertEqual(self.run_agent(self.now+400)['pending'],1)

    def test_cant_create_unsafe_endpoint(self):
        for server in ('http://example.com','https://user:pass@example.com',
                       'https://example.com/path','https://example.com/?secret=1'):
            cfg={**self.cfg,'server':server}
            with self.assertRaises(ValueError): fs.valid_config(cfg)

    def test_single_instance_lock(self):
        path=self.state/'sync.lock'
        with fs.exclusive_lock(path) as outer:
            self.assertTrue(outer)
            with fs.exclusive_lock(path) as inner:
                self.assertFalse(inner)

    def test_pause_marker_has_no_delete(self):
        self.assertFalse((self.state/'paused').exists())
        source=Path(fs.__file__).read_text()
        self.assertNotIn('request("DELETE"',source)

    def test_status_counts_and_pending_metadata(self):
        self.add();self.run_agent()
        db=fs.Manifest(self.state/'manifest.sqlite')
        self.assertEqual(db.status(), {'pending':1})
        db.close()


if __name__ == '__main__':
    unittest.main()
