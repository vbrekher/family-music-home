#!/usr/bin/env python3
"""Authenticated complete data-volume archive. Stop the application first.

Unlike the online NDBACK02 database/media snapshot, this archive also preserves
installed plugin packages and their separate SQLite KV stores. Root cache is
rebuildable and excluded. Music mounted outside /data is never included.
"""
import argparse
from contextlib import closing
import getpass
import os
from pathlib import Path, PurePosixPath
import secrets
import shutil
import sqlite3
import sys
import tarfile
import tempfile

from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes

MAGIC = b'FMHFULL1'
MAX_BYTES = 5_000_000_000
MAX_FILES = 200000
CHUNK = 1024 * 1024


def check_database(path):
    # SQLite may need to create/rebuild a WAL index or recover a hot journal,
    # even for an otherwise read-only integrity check. Verify a private copy
    # with all persistent sidecars; never write to the stopped source volume.
    with tempfile.TemporaryDirectory(prefix='.full-db-check-') as folder:
        target = Path(folder) / 'database'
        for suffix in ('', '-wal', '-journal'):
            source = Path(str(path) + suffix)
            if source.is_symlink():
                raise ValueError('database symlinks are not supported')
            if not source.exists():
                if not suffix:
                    raise ValueError('application database missing')
                continue
            if not source.is_file():
                raise ValueError('unsupported database sidecar')
            copied = Path(str(target) + suffix)
            shutil.copyfile(source, copied)
            copied.chmod(0o600)
        with closing(sqlite3.connect(target)) as db:
            if db.execute('PRAGMA integrity_check').fetchall() != [('ok',)]:
                raise ValueError('database integrity check failed')


def check_databases(root):
    if not (root / 'navidrome.db').is_file():
        raise ValueError('application database missing')
    check_database(root / 'navidrome.db')
    for path in (root / 'plugins').glob('*/kvstore.db'):
        check_database(path)


def safe_name(name):
    p = PurePosixPath(name)
    return not p.is_absolute() and p.parts and str(p) == name and all(x not in ('..', '.') for x in p.parts) and '\\' not in name


def backup(source, target, key):
    source = source.resolve()
    if target.exists() or target.is_symlink() or source == target.resolve() or source in target.resolve().parents:
        raise ValueError('backup destination must be new and outside the data volume')
    entries = []
    total = 0
    for path in sorted(source.rglob('*')):
        name = path.relative_to(source).as_posix()
        if name == 'cache' or name.startswith('cache/'):
            continue
        if path.is_symlink():
            raise ValueError('data-volume symlinks are not supported')
        if path.is_dir():
            continue
        info = path.stat()
        if not path.is_file() or not safe_name(name):
            raise ValueError('unsupported data-volume entry')
        total += info.st_size
        entries.append((path, name, info))
        if total > MAX_BYTES or len(entries) > MAX_FILES:
            raise ValueError('data-volume archive limit exceeded')
    check_databases(source)
    with tempfile.TemporaryDirectory(prefix='.full-backup-', dir=target.parent) as folder:
        temp = Path(folder) / 'encrypted'
        header = MAGIC + secrets.token_bytes(12)
        cipher = Cipher(algorithms.AES(key), modes.GCM(header[8:])).encryptor()
        cipher.authenticate_additional_data(header)
        with temp.open('xb') as output:
            os.fchmod(output.fileno(), 0o600)
            output.write(header)
            class Writer:
                def write(self, block):
                    output.write(cipher.update(block))
                    return len(block)
            with tarfile.open(fileobj=Writer(), mode='w|', format=tarfile.PAX_FORMAT) as archive:
                for path, name, before in entries:
                    with path.open('rb') as stream:
                        now = os.fstat(stream.fileno())
                        if not os.path.samestat(before, now) or (before.st_size, before.st_mtime_ns) != (now.st_size, now.st_mtime_ns):
                            raise ValueError('source changed; stop the application before backup')
                        member = tarfile.TarInfo(name)
                        member.size = now.st_size
                        member.mode = 0o600
                        archive.addfile(member, stream)
                        after = os.fstat(stream.fileno())
                        if (after.st_size, after.st_mtime_ns) != (now.st_size, now.st_mtime_ns):
                            raise ValueError('source changed during backup')
            current = {p.relative_to(source).as_posix() for p in source.rglob('*') if p.is_file() and p.relative_to(source).parts[0] != 'cache'}
            if current != {name for _, name, _ in entries}:
                raise ValueError('source inventory changed during backup')
            for path, _, before in entries:
                after = path.lstat()
                if not os.path.samestat(before, after) or (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns):
                    raise ValueError('source changed before archive publication')
            output.write(cipher.finalize())
            output.write(cipher.tag)
            output.flush()
            os.fsync(output.fileno())
        os.link(temp, target)


def restore(source, destination, key):
    if destination.exists() or destination.is_symlink():
        raise ValueError('restore destination must not exist')
    with tempfile.TemporaryDirectory(prefix='.full-restore-', dir=destination.parent) as folder:
        root = Path(folder)
        plain = root / 'archive.tar'
        with source.open('rb') as encrypted:
            size = os.fstat(encrypted.fileno()).st_size
            header = encrypted.read(20)
            if size < 36 or size > MAX_BYTES + MAX_FILES * 4096 or header[:8] != MAGIC:
                raise ValueError('invalid full backup')
            encrypted.seek(-16, 2)
            tag = encrypted.read(16)
            encrypted.seek(20)
            cipher = Cipher(algorithms.AES(key), modes.GCM(header[8:], tag)).decryptor()
            cipher.authenticate_additional_data(header)
            remaining = size - 36
            with plain.open('xb') as output:
                os.fchmod(output.fileno(), 0o600)
                while remaining:
                    block = encrypted.read(min(CHUNK, remaining))
                    if not block:
                        raise ValueError('truncated archive')
                    remaining -= len(block)
                    output.write(cipher.update(block))
                output.write(cipher.finalize())
        # No archive member is processed until GCM authentication succeeds.
        staged = root / 'data'
        staged.mkdir(mode=0o700)
        total, seen = 0, set()
        with tarfile.open(plain, mode='r:') as archive:
            for member in archive:
                if not member.isfile() or not safe_name(member.name) or member.name in seen or member.size < 0:
                    raise ValueError('unsafe archive member')
                total += member.size
                seen.add(member.name)
                if total > MAX_BYTES or len(seen) > MAX_FILES:
                    raise ValueError('archive quota exceeded')
                path = staged / member.name
                path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
                with archive.extractfile(member) as content, path.open('xb') as output:
                    os.fchmod(output.fileno(), 0o600)
                    remaining = member.size
                    while remaining:
                        block = content.read(min(CHUNK, remaining))
                        if not block:
                            raise ValueError('truncated member')
                        output.write(block)
                        remaining -= len(block)
                    output.flush()
                    os.fsync(output.fileno())
        check_databases(staged)
        staged.rename(destination)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('operation', choices=('backup', 'restore'))
    parser.add_argument('source', type=Path)
    parser.add_argument('destination', type=Path)
    parser.add_argument('--key-stdin', action='store_true', help='read 64 hexadecimal characters from standard input instead of a hidden prompt')
    args = parser.parse_args()
    os.umask(0o077)
    raw = sys.stdin.readline().strip() if args.key_stdin else getpass.getpass('Backup key (64 hexadecimal characters): ')
    key = bytes.fromhex(raw)
    if len(key) != 32:
        raise ValueError('backup key must be 32 bytes')
    {'backup': backup, 'restore': restore}[args.operation](args.source, args.destination, key)
    print('Full data-volume ' + args.operation + ' completed. Original music was not modified.')


if __name__ == '__main__':
    try:
        main()
    except Exception:
        print('Full backup/restore refused. Check stopped state, source, destination, key and integrity; no existing destination was replaced.', file=sys.stderr)
        sys.exit(1)
