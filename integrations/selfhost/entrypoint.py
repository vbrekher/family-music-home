#!/usr/bin/env python3
"""Local self-host entrypoint. Never contacts storage or seeds user accounts."""
import os
from pathlib import Path
import secrets
import re
import sqlite3
import sys


def prepare(data):
    data.mkdir(mode=0o700, parents=True, exist_ok=True)
    database = data / 'navidrome.db'
    initialized = False
    if database.exists():
        with sqlite3.connect(database.resolve().as_uri() + '?mode=ro', uri=True) as db:
            if db.execute("SELECT 1 FROM sqlite_master WHERE type='table' AND name='user'").fetchone():
                initialized = db.execute('SELECT COUNT(*) FROM user').fetchone()[0] > 0
    token = data / 'setup-token'
    if initialized:
        token.unlink(missing_ok=True)
    elif not token.exists():
        value = os.environ.get('FMH_SETUP_TOKEN') or secrets.token_urlsafe(32)
        if not re.fullmatch(r'[A-Za-z0-9_-]{32,96}', value):
            raise ValueError('setup token must contain 32–96 URL-safe characters')
        # Exclusive creation and restrictive permissions; never print the value.
        fd = os.open(token, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, 'w') as stream:
            stream.write(value + '\n')
    if not initialized:
        print('First run: read /data/setup-token inside your container, then open the setup page.', flush=True)


if __name__ == '__main__':
    os.umask(0o077)
    if os.environ.get('FMH_INSTALL_MODE') != 'PUBLIC_SELF_HOST':
        raise SystemExit('This entrypoint requires PUBLIC_SELF_HOST mode')
    prepare(Path(os.environ.get('ND_DATAFOLDER', '/data')))
    command = sys.argv[1:] or ['/app/navidrome']
    os.execvp(command[0], command)
