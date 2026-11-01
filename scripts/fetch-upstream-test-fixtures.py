#!/usr/bin/env python3
"""Fetch hash-pinned upstream regression fixtures, never personal runtime data.

The source release does not redistribute upstream sample audio/artwork. This
explicit development-only step retrieves the original fixtures from the public
Navidrome revision identified in NOTICE.md, with every byte checked first.
"""
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import tempfile
import urllib.request
import zipfile


def fetch(repo):
    lock = json.loads((repo / 'tests/upstream-fixtures.lock.json').read_text())
    revision = lock['revision']
    if len(revision) != 40 or any(c not in '0123456789abcdef' for c in revision):
        raise ValueError('invalid pinned revision')
    destination = repo / 'tests/fixtures'
    def matches():
        if destination.is_symlink():
            return False
        for name, item in lock['files'].items():
            p = destination / name
            if item['mode'] == '120000':
                if not p.is_symlink() or os.readlink(p) != item['target']:
                    return False
                raw = os.readlink(p).encode()
            else:
                if p.is_symlink() or not p.is_file():
                    return False
                raw = p.read_bytes()
            if hashlib.sha256(raw).hexdigest() != item['sha256']:
                return False
        return True
    if destination.exists():
        if not matches():
            raise ValueError('existing fixture directory differs; refusing overwrite')
        print('Existing upstream fixtures match the reviewed lock.')
        return
    url = 'https://codeload.github.com/navidrome/navidrome/zip/' + revision
    with urllib.request.urlopen(url, timeout=120) as response:
        raw = response.read((50 << 20) + 1)
    if len(raw) > 50 << 20:
        raise ValueError('upstream source archive exceeds download limit')
    stage = Path(tempfile.mkdtemp(prefix='.upstream-fixtures-', dir=repo / 'tests'))
    try:
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            if len(archive.infolist()) > 10000:
                raise ValueError('too many archive entries')
            prefix = 'navidrome-' + revision + '/tests/fixtures/'
            contents = {}
            for name, item in lock['files'].items():
                rel = PurePosixPath(name)
                if rel.is_absolute() or '..' in rel.parts or str(rel) != name:
                    raise ValueError('unsafe fixture path')
                info = archive.getinfo(prefix + name)
                if info.file_size > 2 << 20:
                    raise ValueError('oversized fixture')
                value = archive.read(info)
                if hashlib.sha256(value).hexdigest() != item['sha256']:
                    raise ValueError('upstream fixture checksum mismatch: ' + name)
                contents[name] = value
            # Validate all bytes before creating anything, including symlinks.
            for name, value in contents.items():
                p = stage / name
                p.parent.mkdir(parents=True, exist_ok=True)
                item = lock['files'][name]
                if item['mode'] == '120000':
                    target = item['target']
                    if value.decode() != target or not (p.parent / target).resolve().is_relative_to(stage.resolve()):
                        raise ValueError('unsafe fixture symlink')
                    p.symlink_to(target)
                else:
                    p.write_bytes(value)
                    p.chmod(0o644)
        stage.rename(destination)
        print('Fetched and verified upstream development fixtures; do not publish them as demo media.')
    finally:
        if stage.exists():
            shutil.rmtree(stage)


def fetch_tls(repo):
    lock = json.loads((repo / 'tests/upstream-fixtures.lock.json').read_text())
    for name, item in lock['tlsFiles'].items():
        if not name.startswith('server/testdata/') or PurePosixPath(name).name not in (
            'test_cert.pem', 'test_cert_encrypted.pem', 'test_key.pem',
            'test_key_encrypted.pem', 'test_key_encrypted_legacy.pem',
        ):
            raise ValueError('unexpected upstream TLS fixture path')
        p = repo / name
        if p.exists():
            if p.is_symlink() or hashlib.sha256(p.read_bytes()).hexdigest() != item['sha256']:
                raise ValueError('existing TLS fixture differs; refusing overwrite')
            continue
        url = 'https://raw.githubusercontent.com/navidrome/navidrome/' + lock['revision'] + '/' + name
        with urllib.request.urlopen(url, timeout=30) as response:
            raw = response.read(16385)
        if len(raw) > 16384 or hashlib.sha256(raw).hexdigest() != item['sha256']:
            raise ValueError('upstream TLS fixture checksum mismatch')
        p.parent.mkdir(parents=True, exist_ok=True)
        with p.open('xb') as stream:
            stream.write(raw)
        p.chmod(0o600)
    print('Upstream public TLS regression fixtures verified; never use these keys on a server.')


if __name__ == '__main__':
    root = Path(__file__).resolve().parents[1]
    fetch(root)
    fetch_tls(root)
