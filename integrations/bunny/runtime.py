#!/usr/bin/env python3
"""Fail-closed bootstrap, verified daily snapshots and local resource monitoring."""
import datetime as dt
import json
import logging
import os
import re
from pathlib import Path
import signal
import sqlite3
from contextlib import closing
import subprocess
import sys
import tempfile
import threading
import time

from sqlite_backup import backup, restore, check
from upload_music import Storage, CHUNK

log = logging.getLogger('bunny-runtime')


def config_bool(name, default):
    value = os.environ.get(name, default).lower()
    if value not in ('true', 'false'):
        raise ValueError('invalid boolean setting')
    return value == 'true'


def backup_key():
    key = bytes.fromhex(os.environ.get('BUNNY_BACKUP_KEY_HEX', ''))
    if len(key) != 32:
        raise ValueError('backup key must be 32 bytes')
    return key


def storage():
    return Storage(os.environ.get('BUNNY_STORAGE_HOST', 'storage.bunnycdn.com'),
                   os.environ['BUNNY_STORAGE_ZONE'], os.environ['BUNNY_STORAGE_PASSWORD'])


def fetch(client, remote, local):
    conn, response = client.request('GET', remote)
    try:
        if response.status != 200:
            raise RuntimeError('authenticated backup read failed')
        with local.open('xb') as out:
            os.chmod(local, 0o600)
            while block := response.read(CHUNK):
                out.write(block)
            out.flush()
            os.fsync(out.fileno())
    finally:
        conn.close()


def validate_backup_path(path):
    from upload_music import safe_path
    safe_path(path)
    if not path.startswith(('backups/daily/', 'backups/weekly/', 'backups/manual/', 'backups/seed/')) or not path.endswith('.enc'):
        raise ValueError('backup object must be inside an approved backup prefix')
    return path


def latest_backup(client):
    # Listing is read-only. Immutable timestamped names avoid a mutable pointer race.
    candidates = []
    for prefix in ('backups/daily', 'backups/weekly'):
        conn, response = client.request('GET', prefix + '/')
        try:
            if response.status == 404:
                continue
            if response.status != 200:
                raise RuntimeError('backup listing failed')
            raw = response.read(4 * 1024 * 1024 + 1)
            if len(raw) > 4 * 1024 * 1024:
                raise RuntimeError('backup listing exceeds safety limit')
            entries = json.loads(raw)
            if not isinstance(entries, list):
                raise ValueError('invalid backup listing')
            for entry in entries:
                name = entry.get('ObjectName', '')
                if not entry.get('IsDirectory') and re.fullmatch(r'navidrome-[0-9]{8}T[0-9]{6}-[a-f0-9]{8}\.db\.enc', name):
                    candidate = validate_backup_path(prefix + '/' + name)
                    candidates.append((name, candidate))
        finally:
            conn.close()
    if not candidates:
        raise RuntimeError('no restore candidate exists')
    return max(candidates)[1]


def validate_family_database(database):
    """Keep the configured adults; any additional account must have explicit protected policy."""
    with closing(sqlite3.connect(database.resolve().as_uri() + '?mode=ro', uri=True)) as db:
        users = db.execute('SELECT id,user_name,is_admin FROM user').fetchall()
        if len({name for _,name,_ in users}) != len(users):
            raise ValueError('duplicate family identity')
        has_profiles = db.execute("SELECT 1 FROM sqlite_master WHERE type='table' AND name='family_child_profile'").fetchone()
        profiles = db.execute('SELECT user_id,parent_id,body FROM family_child_profile').fetchall() if has_profiles else []
        protected = {row[0] for row in profiles}
        adults = [(key,name,role) for key,name,role in users if key not in protected]
        admins = [(key,name) for key,name,role in adults if role == 1]
        members = [(key,name) for key,name,role in adults if role == 0]
        if len(adults) != 2 or len(admins) != 1 or len(members) != 1:
            raise ValueError('configured family adult roles must be preserved')
        owner_id, owner_name = admins[0]
        member_name = members[0][1]
        if (os.environ.get('BUNNY_OWNER_USERNAME', owner_name) != owner_name or
                os.environ.get('BUNNY_MEMBER_USERNAME', member_name) != member_name):
            raise ValueError('configured family adult identities must be preserved')
        expected_ids = {key for key,_,role in users if key in protected and role == 0}
        if len(profiles) > 10 or len(profiles) != len(protected) or protected != expected_ids:
            raise ValueError('only explicitly protected non-admin family accounts are supported')
        for key,parent,body in profiles:
            policy = json.loads(body)
            if parent != owner_id or not isinstance(policy,dict) or policy.get('interface') not in ('kids','full') or policy.get('density') not in ('large','comfortable') or type(policy.get('enabled')) is not bool or type(policy.get('allowFamilyMessages')) is not bool:
                raise ValueError('invalid protected family profile')


def bootstrap(data):
    database = data / 'navidrome.db'
    if database.exists():
        check(database)
        if config_bool('BUNNY_REQUIRE_BOOTSTRAP', 'true'):
            validate_family_database(database)
        return
    if not config_bool('BUNNY_REQUIRE_BOOTSTRAP', 'true'):
        log.warning('Explicit first-bootstrap mode enabled; Navidrome may initialize a new database')
        return
    key = backup_key()
    client = storage()
    remote = os.environ.get('BUNNY_BOOTSTRAP_OBJECT') or latest_backup(client)
    validate_backup_path(remote)
    with tempfile.TemporaryDirectory(prefix='.bootstrap-', dir=data) as folder:
        os.chmod(folder, 0o700)
        encrypted = Path(folder) / 'snapshot.enc'
        fetch(client, remote, encrypted)
        candidate = Path(folder) / 'candidate.db'
        restore(encrypted, candidate, key)
        validate_family_database(candidate)
        media=Path(folder)/'family-media'
        if media.exists():
            if (data/'family-media').exists():raise ValueError('bootstrap family media destination must be new')
            os.rename(media,data/'family-media')
        os.link(candidate, database)
    log.info('Authenticated database/private-media bootstrap and integrity check succeeded')


def directory_bytes(root):
    total = 0
    for parent, dirs, files in os.walk(root, followlinks=False):
        dirs[:] = [name for name in dirs if not (Path(parent) / name).is_symlink()]
        for name in files:
            path = Path(parent) / name
            if not path.is_symlink():
                total += path.stat().st_size
    return total


def resource_level(used, cap):
    if cap <= 0:
        raise ValueError('data cap must be positive')
    ratio = used / cap
    return 95 if ratio >= .95 else 85 if ratio >= .85 else 70 if ratio >= .70 else 0


def monitor(data):
    cap = int(os.environ.get('BUNNY_DATA_CAP_BYTES', str(5_000_000_000)))
    used = directory_bytes(data)
    threshold = resource_level(used, cap)
    if threshold:
        log.warning('Data usage threshold=%s%% used_bytes=%s cap_bytes=%s; operator action required', threshold, used, cap)
    else:
        log.info('Data usage used_bytes=%s cap_bytes=%s', used, cap)
    # Cache deletions need app-specific ownership proof; never remove arbitrary files.
    return threshold


def prune_backups(data, client, key):
    """Delete only exact backup names after a fresh authenticated restore proof."""
    pattern = r'navidrome-[0-9]{8}T[0-9]{6}-[a-f0-9]{8}\.db\.enc'
    groups = []
    for prefix, keep in (('backups/daily', 7), ('backups/weekly', 4)):
        conn, response = client.request('GET', prefix + '/')
        try:
            if response.status == 404:
                continue
            if response.status != 200:
                raise RuntimeError('retention listing failed')
            raw = response.read(4 * 1024 * 1024 + 1)
            if len(raw) > 4 * 1024 * 1024:
                raise RuntimeError('retention listing exceeds safety limit')
            entries = json.loads(raw)
            if not isinstance(entries, list):
                raise ValueError('invalid retention listing')
            names = sorted(set(entry.get('ObjectName', '') for entry in entries
                               if not entry.get('IsDirectory')
                               and re.fullmatch(pattern, entry.get('ObjectName', ''))), reverse=True)
            # Restart-triggered snapshots must not consume all seven retained days.
            representatives = {}
            for name in names:
                try:
                    date = dt.datetime.strptime(name[10:18], '%Y%m%d').date()
                except ValueError:
                    continue
                period = date.isoformat() if prefix == 'backups/daily' else date.isocalendar()[:2]
                representatives.setdefault(period, name)  # names are newest first
            retained = set(list(representatives.values())[:keep])
            # Include same-period duplicates, but never mutate an invalid date name.
            obsolete = []
            for name in names:
                try:
                    dt.datetime.strptime(name[10:18], '%Y%m%d')
                except ValueError:
                    continue
                if name not in retained:
                    obsolete.append(name)
            if obsolete and retained:
                newest = next(name for name in names if name in retained)
                groups.append((prefix, newest, obsolete))
        finally:
            conn.close()
    # Prove every group's newest before issuing the first DELETE anywhere.
    for prefix, newest, obsolete in groups:
        with tempfile.TemporaryDirectory(prefix='.retention-proof-', dir=data) as folder:
            os.chmod(folder, 0o700)
            encrypted = Path(folder) / 'proof.enc'
            fetch(client, prefix + '/' + newest, encrypted)
            restore(encrypted, Path(folder) / 'proof.db', key)
    bootstrap_object = os.environ.get('BUNNY_BOOTSTRAP_OBJECT', '')
    for prefix, newest, obsolete in groups:
        for name in obsolete:
            remote = prefix + '/' + name
            if remote == bootstrap_object:
                continue
            # Revalidate at the mutation boundary; no input path can change prefix.
            if prefix not in ('backups/daily', 'backups/weekly') or not re.fullmatch(pattern, name):
                raise ValueError('unsafe retention target')
            conn, response = client.request('DELETE', remote)
            try:
                if response.status not in (200, 204, 404):
                    raise RuntimeError('backup retention deletion failed')
                response.read(CHUNK)
            finally:
                conn.close()
    if groups:
        log.info('Backup-only retention complete after authenticated restore proof')


def snapshot(data, client, key, now=None):
    now = now or dt.datetime.now(dt.timezone.utc)
    stamp = now.strftime('%Y%m%dT%H%M%S') + '-' + os.urandom(4).hex()
    with tempfile.TemporaryDirectory(prefix='.backup-', dir=data) as folder:
        os.chmod(folder, 0o700)
        encrypted = Path(folder) / 'snapshot.enc'
        backup(data / 'navidrome.db', encrypted, key)
        daily = 'backups/daily/navidrome-' + stamp + '.db.enc'
        if client.upload(encrypted, daily) != 'verified':
            raise RuntimeError('new daily snapshot was not verified')
        if now.weekday() == 6:
            weekly = 'backups/weekly/navidrome-' + stamp + '.db.enc'
            if client.upload(encrypted, weekly) != 'verified':
                raise RuntimeError('new weekly snapshot was not verified')
    log.info('Encrypted daily backup uploaded and independently verified')
    prune_backups(data, client, key)


def worker(data, stop):
    # The first verified snapshot is taken after the running database appears.
    next_backup = 0.0
    while not stop.is_set():
        try:
            threshold = monitor(data)
            if time.monotonic() >= next_backup and (data / 'navidrome.db').exists():
                if threshold >= 95:
                    log.error('Backup deferred at critical data usage; preserve database and free operator-reviewed cache space')
                else:
                    snapshot(data, storage(), backup_key())
                    next_backup = time.monotonic() + 86400
        except Exception as exc:
            # Exception bodies may contain credentials or response data; class only.
            log.error('Runtime maintenance failed (%s); retry required', type(exc).__name__)
        stop.wait(300)


def main():
    os.umask(0o077)
    logging.basicConfig(level=logging.INFO, format='%(asctime)s %(levelname)s %(message)s')
    data = Path(os.environ.get('ND_DATAFOLDER', '/data'))
    data.mkdir(mode=0o700, parents=True, exist_ok=True)
    mode = os.environ.get('FMH_INSTALL_MODE', 'FAMILY_EXISTING_LOCKED')
    if mode not in ('PUBLIC_SELF_HOST', 'FAMILY_EXISTING_LOCKED'):
        raise ValueError('unknown installation mode')
    command = sys.argv[1:] or ['/app/navidrome']
    if mode == 'PUBLIC_SELF_HOST':
        if config_bool('BUNNY_FAMILY_MODE', 'false'):
            raise ValueError('public setup and locked family policy cannot be combined')
        # Public installations need no Bunny account. A token protects signup;
        # encrypted Bunny backups are enabled only by explicit configuration.
        sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
        from selfhost.entrypoint import prepare
        prepare(data)
        if not config_bool('FMH_BUNNY_BACKUPS', 'false'):
            return os.execvp(command[0], command)
    # Validate backup configuration before launching, including existing-volume startup.
    backup_key()
    storage()
    if mode == 'FAMILY_EXISTING_LOCKED':
        bootstrap(data)
    stop = threading.Event()
    child = subprocess.Popen(command)
    def terminate(signum, frame):
        stop.set()
        child.send_signal(signum)
    signal.signal(signal.SIGTERM, terminate)
    signal.signal(signal.SIGINT, terminate)
    thread = threading.Thread(target=worker, args=(data, stop), daemon=True)
    thread.start()
    code = child.wait()
    stop.set()
    # A maintenance request can be in flight for up to the HTTPS timeout.
    thread.join(timeout=5)
    return code


if __name__ == '__main__':
    try:
        sys.exit(main())
    except Exception as exc:
        logging.error('Startup refused (%s); database was not initialized', type(exc).__name__)
        sys.exit(1)
