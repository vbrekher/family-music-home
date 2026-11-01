#!/bin/sh
set -eu
umask 077
exec python3 /app/bunny/runtime.py "$@"
