#!/usr/bin/env python3
"""Request an authenticated Navidrome scan. Credentials never enter command arguments."""
import argparse
import getpass
import hashlib
import json
import os
import secrets
import sys
from urllib.parse import urlencode, urlsplit
from urllib.request import Request, HTTPRedirectHandler, build_opener


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError("redirect refused")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--server', required=True)
    parser.add_argument('--user', default='admin')
    args = parser.parse_args()
    origin = urlsplit(args.server)
    if origin.scheme != 'https' or not origin.hostname or origin.username or origin.password or origin.query or origin.fragment:
        parser.error('server must be an HTTPS URL without credentials or query')
    password = os.environ.get('NAVIDROME_SYNC_PASSWORD') or getpass.getpass('Navidrome password: ')
    salt = secrets.token_hex(16)
    token = hashlib.md5((password + salt).encode(), usedforsecurity=False).hexdigest()
    fields = dict(u=args.user, t=token, s=salt, v='1.16.1', c='bunny-family-sync', f='json')
    request = Request(args.server.rstrip('/') + '/rest/startScan.view',
                      data=urlencode(fields).encode(), method='POST')
    with build_opener(NoRedirect()).open(request, timeout=30) as response:
        result = json.load(response)['subsonic-response']
    if result.get('status') != 'ok':
        raise RuntimeError('server rejected scan; check account permissions')
    print('Authenticated library scan requested')


if __name__ == '__main__':
    try:
        main()
    except Exception:
        print('Scan request failed; check HTTPS connectivity and account permissions', file=sys.stderr)
        sys.exit(1)
