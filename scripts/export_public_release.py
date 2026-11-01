#!/usr/bin/env python3
"""Export an explicitly reviewed, hash-locked source snapshot; never copy Git history.

The manifest is an object mapping every permitted repository-relative path to
its SHA-256. It is reviewed and stored privately; adding a file requires an
explicit manifest change. Sensitive deny values are supplied from a private
0600 JSON array and are never included in errors, exported files, or reports.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import tempfile

FORBIDDEN_PARTS = {'.git', '.private', '.docker-auth', 'node_modules', '__pycache__'}
FORBIDDEN_SUFFIXES = {'.db', '.sqlite', '.sqlite3', '.enc', '.pem', '.key', '.log', '.bundle'}
FORBIDDEN_PREFIXES = ('docs/operations/', 'work/', 'outputs/', '.github/workflows/')
PATTERNS = [
    rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',
    rb'\b(?:ghp_|github_pat_|gho_|dckr_pat_)[A-Za-z0-9_]{20,}',
    rb'https?://mc-[a-z0-9]+\.bunny\.run',
    rb'https?://[^\s/@:]+:[^\s/@]+@',
]


def safe_path(name):
    path = PurePosixPath(name)
    return (not path.is_absolute() and str(path) == name and
            not any(x in FORBIDDEN_PARTS or x in ('..', '.') for x in path.parts) and
            not name.startswith(FORBIDDEN_PREFIXES) and
            not name.endswith(('-wal', '-shm')) and
            path.suffix.lower() not in FORBIDDEN_SUFFIXES and
            (not path.name.startswith('.env') or path.name == '.env.example'))


def export(repo, revision, manifest, deny, destination, reviewed_synthetic_urls=None):
    reviewed_synthetic_urls = reviewed_synthetic_urls or {}
    reviewed_url_count = 0
    if destination.exists():
        raise ValueError('destination must not already exist')
    if not manifest or not all(isinstance(k, str) and re.fullmatch('[a-f0-9]{64}', v or '') for k, v in manifest.items()):
        raise ValueError('invalid or empty reviewed manifest')
    revision = subprocess.check_output(['git', '-C', str(repo), 'rev-parse', '--verify', revision + '^{commit}'], text=True).strip()
    tree = subprocess.check_output(['git', '-C', str(repo), 'ls-tree', '-rz', '--full-tree', revision])
    entries = {}
    for item in tree.split(b'\0'):
        if item:
            meta, rawname = item.split(b'\t', 1)
            mode, kind, oid = meta.decode().split()
            entries[rawname.decode('utf-8')] = (mode, kind, oid)
    # Read exact committed objects in one Git invocation. No checkout, filters,
    # working-tree files or private history enter the exported source.
    ordered = sorted(manifest)
    for name in ordered:
        if not safe_path(name) or name not in entries:
            raise ValueError('forbidden or missing manifest path: ' + name)
        if entries[name][1] != 'blob' or entries[name][0] not in ('100644', '100755'):
            raise ValueError('only ordinary source files allowed: ' + name)
    raw = subprocess.check_output(
        ['git', '-C', str(repo), 'cat-file', '--batch'],
        input=''.join(entries[name][2] + '\n' for name in ordered).encode())
    contents = {}
    offset = 0
    for name in ordered:
        end = raw.find(b'\n', offset)
        header = raw[offset:end].decode().split() if end >= 0 else []
        if len(header) != 3 or header[:2] != [entries[name][2], 'blob'] or not header[2].isdigit():
            raise ValueError('invalid Git blob response')
        size = int(header[2])
        offset = end + 1
        content = raw[offset:offset + size]
        if len(content) != size or raw[offset + size:offset + size + 1] != b'\n':
            raise ValueError('truncated Git blob response')
        contents[name] = content
        offset += size + 1
    if offset != len(raw):
        raise ValueError('unexpected Git blob response')
    destination.parent.mkdir(parents=True, exist_ok=True)
    staging = Path(tempfile.mkdtemp(prefix='.public-export-', dir=destination.parent))
    try:
        for name, expected in sorted(manifest.items()):
            if not safe_path(name) or name not in entries:
                raise ValueError('forbidden or missing manifest path: ' + name)
            mode, kind, oid = entries[name]
            if kind != 'blob' or mode not in ('100644', '100755'):
                raise ValueError('only ordinary source files allowed: ' + name)
            content = contents[name]
            if hashlib.sha256(content).hexdigest() != expected:
                raise ValueError('reviewed file changed: ' + name)
            # Binary assets are scanned bytewise too. Image metadata must also
            # receive the documented manual review before manifest approval.
            lowered = content.lower()
            if any(value.lower() in lowered for value in deny):
                raise ValueError('privacy scan rejected file: ' + name)
            for index, pattern in enumerate(PATTERNS):
                if not re.search(pattern, content):
                    continue
                # Negative authorization tests deliberately include synthetic
                # URL userinfo. Permit only this rule, only in test source, and
                # only after an explicit separate review of the exact file hash.
                # Private deny values, keys and token patterns are never exempt.
                is_test = name.endswith('_test.go') or (Path(name).name.startswith('test_') and name.endswith('.py'))
                if index == 3 and is_test and reviewed_synthetic_urls.get(name) == expected:
                    reviewed_url_count += 1
                    continue
                raise ValueError('privacy scan rejected file: ' + name)
            output = staging / name
            output.parent.mkdir(parents=True, exist_ok=True)
            output.write_bytes(content)
            output.chmod(0o755 if mode == '100755' else 0o644)
        # The public source digest does not disclose the private Git revision.
        digest = hashlib.sha256(json.dumps(manifest, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        (staging / 'SOURCE_MANIFEST.json').write_text(json.dumps({'schema': 1, 'sourceDigest': digest, 'files': manifest}, indent=2, sort_keys=True) + '\n')
        staging.rename(destination)
        return {'status': 'EXPORTED_NOT_PUBLISHED', 'files': len(manifest), 'sourceDigest': digest, 'gitHistoryCopied': False, 'reviewedSyntheticURLTestFiles': reviewed_url_count}
    finally:
        if staging.exists():
            shutil.rmtree(staging)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, required=True)
    parser.add_argument('--revision', required=True)
    parser.add_argument('--manifest', type=Path, required=True)
    parser.add_argument('--private-deny-values', type=Path, required=True)
    parser.add_argument('--destination', type=Path, required=True)
    parser.add_argument('--reviewed-synthetic-url-tests', type=Path, help='Separate reviewed path/SHA-256 allowlist for negative URL-userinfo tests only')
    args = parser.parse_args()
    info = args.private_deny_values.lstat()
    if not args.private_deny_values.is_file() or args.private_deny_values.is_symlink() or info.st_mode & 0o077:
        parser.error('private deny list must be an ordinary 0600 file')
    raw = json.loads(args.private_deny_values.read_text())
    if not isinstance(raw, list) or not raw or not all(isinstance(x, str) and len(x) >= 4 for x in raw):
        parser.error('private deny values must be a nonempty string array')
    reviewed = json.loads(args.reviewed_synthetic_url_tests.read_text()) if args.reviewed_synthetic_url_tests else {}
    if not isinstance(reviewed, dict) or not all(isinstance(k, str) and isinstance(v, str) and re.fullmatch('[a-f0-9]{64}', v) for k, v in reviewed.items()):
        parser.error('reviewed synthetic URL test allowlist must map paths to SHA-256')
    print(json.dumps(export(args.repo, args.revision, json.loads(args.manifest.read_text()), [x.encode() for x in raw], args.destination, reviewed)))


if __name__ == '__main__':
    main()
