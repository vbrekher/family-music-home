#!/usr/bin/env python3
"""Add-only Bunny Storage uploader. Never sends DELETE or follows redirects."""
import argparse
import hashlib
import http.client
import os
from pathlib import Path, PurePosixPath
import re
import sys
from urllib.parse import quote

CHUNK = 1024 * 1024


def digest(stream):
    result = hashlib.sha256()
    for block in iter(lambda: stream.read(CHUNK), b''):
        result.update(block)
    return result.hexdigest()


def safe_path(value):
    if not value or value.startswith('/') or any(p in ('', '.', '..') for p in value.split('/')):
        raise ValueError('remote path must be relative without empty or traversal components')
    if any(ord(c) < 32 for c in value) or '\\' in value:
        raise ValueError('invalid remote path')
    return value


class Storage:
    def __init__(self, host, zone, key):
        if not re.fullmatch(r'(?:[a-z]{2,3}\.)?storage\.bunnycdn\.com', host):
            raise ValueError('host must be a Bunny Storage HTTPS hostname')
        if not re.fullmatch(r'[A-Za-z0-9_-]+', zone) or not key:
            raise ValueError('valid zone and BUNNY_STORAGE_PASSWORD are required')
        self.host, self.zone, self.key = host, zone, key

    def request(self, method, path, body=None, headers=None):
        conn = http.client.HTTPSConnection(self.host, timeout=120)
        try:
            validated = safe_path(path[:-1]) + '/' if path.endswith('/') else safe_path(path)
            conn.request(method, '/' + quote(self.zone + '/' + validated, safe='/'),
                         body=body, headers={'AccessKey': self.key, **(headers or {})})
            response = conn.getresponse()
            return conn, response
        except BaseException:
            conn.close()
            raise

    def remote_hash(self, path):
        conn, response = self.request('GET', path)
        try:
            if response.status == 404:
                return None
            if response.status != 200:
                raise RuntimeError('Storage read failed: HTTP ' + str(response.status))
            return digest(response)
        finally:
            conn.close()

    def upload(self, source, path, dry_run=False):
        # A streamed GET also checks existence; HEAD cannot establish content identity.
        with source.open('rb') as stream:
            before = os.fstat(stream.fileno())
            local_hash = digest(stream)
            remote_hash = self.remote_hash(path)
            if remote_hash == local_hash:
                return 'identical'
            if remote_hash is not None:
                raise RuntimeError('name conflict; refusing overwrite: ' + path)
            if dry_run:
                return 'would-upload'
            stream.seek(0)
            conn, response = self.request('PUT', path, stream, {
                'Content-Length': str(before.st_size), 'Content-Type': 'application/octet-stream',
                'Checksum': local_hash.upper(),
            })
            try:
                if response.status not in (200, 201):
                    raise RuntimeError('Storage upload failed: HTTP ' + str(response.status))
                response.read(CHUNK)
            finally:
                conn.close()
            after = os.fstat(stream.fileno())
            if (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns):
                raise RuntimeError('source changed during upload')
        if self.remote_hash(path) != local_hash:
            raise RuntimeError('remote SHA256 verification failed: ' + path)
        return 'verified'


def files(root):
    for directory, dirs, names in os.walk(root, followlinks=False):
        dirs[:] = sorted(d for d in dirs if not d.startswith('.') and not (Path(directory) / d).is_symlink())
        for name in sorted(names):
            file = Path(directory) / name
            if not name.startswith('.') and not file.is_symlink() and file.is_file():
                yield file


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path)
    parser.add_argument('--zone', required=True)
    parser.add_argument('--host', default='storage.bunnycdn.com')
    parser.add_argument('--prefix', default='music')
    parser.add_argument('--dry-run', action='store_true')
    args = parser.parse_args()
    safe_path(args.prefix)
    if not args.directory.exists() or args.directory.is_symlink():
        parser.error('source must be a real file or directory')
    client = Storage(args.host, args.zone, os.environ.get('BUNNY_STORAGE_PASSWORD', ''))
    sources = files(args.directory) if args.directory.is_dir() else [args.directory]
    for index, file in enumerate(sources, 1):
        relative = file.relative_to(args.directory).as_posix() if args.directory.is_dir() else file.name
        remote = args.prefix + '/' + relative
        print(f'[{index}] checking {remote} ({file.stat().st_size} bytes)', flush=True)
        print(client.upload(file, remote, args.dry_run), remote, flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as exc:
        # Never print headers, keys, or remote response bodies.
        print('Upload stopped: ' + str(exc), file=sys.stderr)
        sys.exit(1)
