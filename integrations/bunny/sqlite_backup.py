#!/usr/bin/env python3
"""Consistent SQLite backup with authenticated streaming encryption; offline restore."""
import argparse
from contextlib import closing
import os
from pathlib import Path
import secrets
import sqlite3
import sys
import tempfile
import family_backup
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes

MAGIC = b'NDBACK01'
CHUNK = 1024 * 1024


def key_from_env():
    try:
        key = bytes.fromhex(os.environ.get('NAVIDROME_BACKUP_KEY_HEX', ''))
    except ValueError:
        raise ValueError('NAVIDROME_BACKUP_KEY_HEX must contain 64 hexadecimal characters') from None
    if len(key) != 32:
        raise ValueError('NAVIDROME_BACKUP_KEY_HEX must contain 64 hexadecimal characters')
    return key


def check(path):
    with closing(sqlite3.connect(path.resolve().as_uri() + '?mode=ro', uri=True)) as db:
        if db.execute('PRAGMA integrity_check').fetchall() != [('ok',)]:
            raise RuntimeError('SQLite integrity check failed')


def publish(temp, target):
    # Hard-link gives atomic no-clobber publication on the same filesystem.
    os.link(temp, target)
    with open(target, 'rb') as stream:
        os.fsync(stream.fileno())


def backup(source, target, key):
    if not source.is_file() or target.exists():
        raise ValueError('source must exist and destination must be new')
    with tempfile.TemporaryDirectory(prefix='.ndbackup-', dir=target.parent) as directory:
        os.chmod(directory, 0o700)
        snapshot, encrypted = Path(directory) / 'snapshot.db', Path(directory) / 'encrypted'
        # Backup API includes committed WAL data without copying a live DB file.
        with closing(sqlite3.connect(source.resolve().as_uri() + '?mode=ro', uri=True)) as src:
            with closing(sqlite3.connect(snapshot)) as dst:
                src.backup(dst, pages=256, sleep=0.1)
        check(snapshot)
        nonce = secrets.token_bytes(12)
        rows = family_backup.attachments(snapshot)
        header = (MAGIC if rows is None else family_backup.MAGIC) + nonce
        cipher = Cipher(algorithms.AES(key), modes.GCM(nonce)).encryptor()
        cipher.authenticate_additional_data(header)
        with encrypted.open('xb') as out:
            os.chmod(encrypted, 0o600)
            out.write(header)
            if rows is None:
                with snapshot.open('rb') as plain:
                    for block in iter(lambda: plain.read(CHUNK), b''):out.write(cipher.update(block))
            else:
                class Writer:
                    def write(self,block):out.write(cipher.update(block));return len(block)
                family_backup.pack(snapshot,source.parent,rows,Writer())
            out.write(cipher.finalize())
            out.write(cipher.tag)
            out.flush()
            os.fsync(out.fileno())
        publish(encrypted, target)


def restore(source, target, key):
    if target.exists():
        raise ValueError('restore destination must be new; stop Navidrome before swapping databases')
    with tempfile.TemporaryDirectory(prefix='.ndrestore-', dir=target.parent) as directory:
        os.chmod(directory, 0o700)
        plain = Path(directory) / 'restored.db'
        with source.open('rb') as encrypted:
            size = os.fstat(encrypted.fileno()).st_size
            header = encrypted.read(20)
            if len(header) != 20 or header[:8] not in (MAGIC,family_backup.MAGIC) or size < 36 or size>family_backup.MAX_DATABASE+family_backup.MAX_MEDIA+32*1024*1024:
                raise ValueError('invalid encrypted backup format')
            encrypted.seek(-16, 2)
            tag = encrypted.read(16)
            encrypted.seek(20)
            cipher = Cipher(algorithms.AES(key), modes.GCM(header[8:], tag)).decryptor()
            cipher.authenticate_additional_data(header)
            remaining = size - 36
            with plain.open('xb') as out:
                os.chmod(plain, 0o600)
                while remaining:
                    block = encrypted.read(min(CHUNK, remaining))
                    if not block:
                        raise ValueError('truncated encrypted backup')
                    remaining -= len(block)
                    out.write(cipher.update(block))
                out.write(cipher.finalize())  # Authentication must succeed before publishing.
                out.flush()
                os.fsync(out.fileno())
        media = None
        if header[:8]==family_backup.MAGIC:
            plain,media=family_backup.unpack(plain,Path(directory))
        check(plain)
        if header[:8]==MAGIC and family_backup.attachments(plain):
            raise ValueError('SQLite-only archive is missing referenced private family media')
        destination_media=target.parent/'family-media'
        if media is not None and media.exists():
            if destination_media.exists():raise ValueError('family media restore destination must be new')
            os.rename(media,destination_media)
        publish(plain, target)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('operation', choices=('backup', 'restore'))
    parser.add_argument('source', type=Path)
    parser.add_argument('destination', type=Path)
    args = parser.parse_args()
    if not args.destination.parent.is_dir():
        parser.error('destination parent directory must exist')
    {'backup': backup, 'restore': restore}[args.operation](args.source, args.destination, key_from_env())
    print(args.operation + ' complete; integrity verified')


if __name__ == '__main__':
    try:
        main()
    except Exception:
        print('Backup/restore failed; destination was not intentionally overwritten. Check paths, key, and database integrity.', file=sys.stderr)
        sys.exit(1)
