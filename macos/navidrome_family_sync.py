#!/usr/bin/env python3
"""Mac-only, add-only music uploader for your Family Music Home server.

No daemon, third-party dependencies, remote DELETE, or silent overwrite.  launchd
runs a bounded, independent scan every 300 seconds.  Secrets live in Keychain.
"""
from __future__ import annotations

import argparse
from contextlib import contextmanager
from datetime import datetime, timezone
import fcntl
import json
import os
from pathlib import Path
import plistlib
import shutil
import sqlite3
import subprocess
import sys
import time
from typing import Callable
from urllib.parse import urlsplit

AGENT_LABEL = "com.navidrome.family-sync"
BUNNY_ITEM = "navidrome-family-sync-bunny-upload"
OWNER_ITEM = "navidrome-family-sync-owner-password"
APP_DIR = Path.home() / "Library/Application Support/NavidromeFamilySync"
LAUNCH_PLIST = Path.home() / "Library/LaunchAgents" / (AGENT_LABEL + ".plist")
DEFAULT_MUSIC = Path.home() / "Music/Navidrome"
ALLOWED_EXT = frozenset(".mp3 .flac .m4a .aac .ogg .opus .wav .aif .aiff .alac .wma .wv .ape .jpg .jpeg .png .webp".split())
INCOMPLETE_EXT = frozenset(".part .partial .crdownload .download .tmp .temp .aria2 .torrent".split())
STABILITY_SECONDS = 300  # Must observe the same metadata on two separate polls.


def protected_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True, mode=0o700)
    if path.is_symlink():
        raise ValueError("state directory cannot be a symlink")
    path.chmod(0o700)


def atomic_private_write(path: Path, content: bytes) -> None:
    protected_dir(path.parent)
    tmp = path.with_name(path.name + ".tmp." + str(os.getpid()))
    fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        with os.fdopen(fd, "wb") as out:
            out.write(content)
            out.flush()
            os.fsync(out.fileno())
        os.replace(tmp, path)
        path.chmod(0o600)
    finally:
        tmp.unlink(missing_ok=True)


def valid_config(config: dict) -> dict:
    """Validate persisted values before ever accessing credentials or network."""
    url = urlsplit(config["server"])
    if url.scheme != "https" or not url.hostname or url.username or url.password or url.query or url.fragment or url.path not in ("", "/"):
        raise ValueError("server must be an HTTPS origin (no userinfo/path/query)")
    import re
    if not re.fullmatch(r"[A-Za-z0-9_-]+", config["zone"]):
        raise ValueError("invalid Bunny zone")
    if not re.fullmatch(r"(?:[a-z]{2,3}\.)?storage\.bunnycdn\.com", config["host"]):
        raise ValueError("invalid Bunny Storage hostname")
    root = Path(config["music_dir"]).expanduser()
    if not root.is_absolute() or root.is_symlink():
        raise ValueError("music directory must be an absolute nonsymlink path")
    if not config["owner"] or any(ord(x) < 32 for x in config["owner"]):
        raise ValueError("invalid owner username")
    return config


def config_path() -> Path:
    return APP_DIR / "config.json"


def read_config() -> dict:
    return valid_config(json.loads(config_path().read_text(encoding="utf-8")))


def keychain_read(service: str, account: str) -> str:
    # The password never enters command line arguments or files.  The Keychain
    # may prompt the signed-in Mac user to authorize this Python binary.
    cp = subprocess.run(["/usr/bin/security", "find-generic-password", "-w", "-s", service, "-a", account],
                        capture_output=True, text=True, check=False, timeout=20)
    if cp.returncode != 0 or not cp.stdout.rstrip("\n"):
        raise RuntimeError("Keychain item missing or locked: " + service)
    return cp.stdout.rstrip("\n")


def keychain_setup(config: dict) -> None:
    if sys.platform != "darwin" or not sys.stdin.isatty():
        raise RuntimeError("Keychain setup requires an interactive macOS Terminal")
    print("Bunny Storage upload password (NOT Bunny account API key):")
    # Apple's SecurityTool explicitly documents putting -w LAST without an
    # argument, so it prompts instead of leaking the password in process args.
    for service, account in ((BUNNY_ITEM, config["zone"]), (OWNER_ITEM, config["owner"])):
        print("Set Keychain item:", service)
        subprocess.run(["/usr/bin/security", "add-generic-password", "-U", "-s", service,
                        "-a", account, "-w"], check=True)
    print("Keychain setup complete; no secret values were written to config or Git.")


@contextmanager
def exclusive_lock(path: Path):
    protected_dir(path.parent)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            yield False
        else:
            yield True
    finally:
        os.close(fd)


class Manifest:
    def __init__(self, db_path: Path):
        protected_dir(db_path.parent)
        self.db = sqlite3.connect(db_path, timeout=10)
        os.chmod(db_path, 0o600)
        self.db.execute("PRAGMA busy_timeout=10000")
        self.db.execute("PRAGMA journal_mode=WAL")
        self.db.executescript("""
        CREATE TABLE IF NOT EXISTS files (
          path TEXT PRIMARY KEY,
          size INTEGER NOT NULL,
          mtime_ns INTEGER NOT NULL,
          observed_at REAL NOT NULL,
          verified_size INTEGER,
          verified_mtime_ns INTEGER,
          verified_sha256 TEXT,
          status TEXT NOT NULL DEFAULT 'pending',
          last_error TEXT,
          verified_at REAL
        );
        CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
        """)
        self.db.commit()

    def close(self):
        self.db.close()

    def get(self, path: str):
        return self.db.execute("SELECT size,mtime_ns,observed_at,verified_size,verified_mtime_ns,"
                               "verified_sha256,status FROM files WHERE path=?", (path,)).fetchone()

    def observe(self, path: str, size: int, mtime_ns: int, now: float):
        self.db.execute("""INSERT INTO files(path,size,mtime_ns,observed_at,status)
                          VALUES(?,?,?,?,'pending') ON CONFLICT(path) DO UPDATE SET
                          size=excluded.size,mtime_ns=excluded.mtime_ns,
                          observed_at=excluded.observed_at,status='pending',last_error=NULL""",
                        (path, size, mtime_ns, now))
        self.db.commit()

    def verified(self, path: str, size: int, mtime_ns: int, sha: str, now: float):
        self.db.execute("""UPDATE files SET verified_size=?,verified_mtime_ns=?,verified_sha256=?,
                          status='synced',last_error=NULL,verified_at=? WHERE path=?""",
                        (size, mtime_ns, sha, now, path))
        # Persist upload verification and the scan obligation atomically. A crash
        # after this commit must never strand uploaded files outside the catalog.
        self.db.execute("INSERT INTO metadata(key,value) VALUES('scan_pending','1') "
                        "ON CONFLICT(key) DO UPDATE SET value='1'")
        self.db.commit()

    def failure(self, path: str, state: str, msg: str):
        self.db.execute("UPDATE files SET status=?,last_error=? WHERE path=?", (state, msg[:240], path))
        self.db.commit()

    def set_meta(self, key: str, value: str):
        self.db.execute("INSERT INTO metadata(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", (key, value))
        self.db.commit()

    def get_meta(self, key: str, default: str = "") -> str:
        row = self.db.execute("SELECT value FROM metadata WHERE key=?", (key,)).fetchone()
        return row[0] if row else default

    def status(self):
        return dict(self.db.execute("SELECT status,COUNT(*) FROM files GROUP BY status"))


def iter_sources(root: Path):
    if not root.is_dir() or root.is_symlink():
        raise RuntimeError("music directory is missing or symlinked")
    for base, dirs, names in os.walk(root, followlinks=False):
        dirs[:] = sorted(d for d in dirs if not d.startswith(".") and not (Path(base) / d).is_symlink())
        for name in sorted(names):
            f = Path(base) / name
            if name.startswith(".") or f.is_symlink() or not f.is_file() or f.suffix.lower() not in ALLOWED_EXT:
                continue
            if f.suffix.lower() in INCOMPLETE_EXT:
                continue
            yield f, f.relative_to(root).as_posix()


def notify(message: str) -> None:
    if sys.platform != "darwin":
        return
    safe = message.replace('\\', '\\\\').replace('"', '\\"').replace('\n', ' ')[:180]
    try:
        subprocess.run(["/usr/bin/osascript", "-e", f'display notification "{safe}" with title "Navidrome Family Sync"'],
                       capture_output=True, timeout=8, check=False)
    except (OSError, subprocess.TimeoutExpired):
        pass  # Notifications are optional and must never change upload outcome.


def direct_upload(file: Path, path: str, config: dict, password: str) -> str:
    # Reuse the existing security-reviewed uploader: no overwrite, complete
    # remote SHA256 verification on a first upload only.
    from upload_music import Storage
    storage = Storage(config["host"], config["zone"], password)
    return storage.upload(file, "music/" + path)


def direct_scan(config: dict, password: str) -> None:
    # Reuse existing Subsonic scanner.  Only transient child env holds password,
    # never command arguments, .env, plist, manifest, or logs.
    helper = Path(__file__).with_name("sync_library.py")
    env = dict(os.environ)
    env["NAVIDROME_SYNC_PASSWORD"] = password
    cp = subprocess.run([sys.executable, str(helper), "--server", config["server"], "--user", config["owner"]],
                        env=env, capture_output=True, text=True, check=False, timeout=45)
    if cp.returncode:
        raise RuntimeError("Authenticated Navidrome library scan failed")


def sync_once(config: dict, state_dir: Path, *, uploader: Callable, scanner: Callable,
              notifier: Callable = notify, now: float | None = None) -> dict:
    """One bounded add-only pass. The caller owns the single-instance flock."""
    root = Path(config["music_dir"]).expanduser()
    now = time.time() if now is None else now
    manifest = Manifest(state_dir / "manifest.sqlite")
    totals = dict(uploaded=0, identical=0, skipped=0, pending=0, failed=0, conflicts=0)
    try:
        for file, rel in iter_sources(root):
            try:
                st = file.stat()
                if not file.is_file() or not st.st_size:
                    totals["pending"] += 1
                    continue
                row = manifest.get(rel)
                if row and row[3] == st.st_size and row[4] == st.st_mtime_ns and row[6] == "synced":
                    totals["skipped"] += 1
                    continue  # No network access and no rehash of unchanged files.
                if not row or (row[0], row[1]) != (st.st_size, st.st_mtime_ns):
                    manifest.observe(rel, st.st_size, st.st_mtime_ns, now)
                    totals["pending"] += 1
                    continue
                if row[6] == "conflict":
                    totals["conflicts"] += 1
                    continue  # Explicit conflict remains until local metadata changes.
                if now - row[2] < STABILITY_SECONDS or now - st.st_mtime < 15:
                    totals["pending"] += 1
                    continue
                # A previously verified path that changed is NEVER overwritten.
                # Let the original uploader perform conflict detection with Bunny.
                from hashlib import sha256
                h = sha256()
                with file.open("rb") as src:
                    stat_start = os.fstat(src.fileno())
                    for buf in iter(lambda: src.read(1024 * 1024), b""):
                        h.update(buf)
                    stat_end = os.fstat(src.fileno())
                if (stat_start.st_size, stat_start.st_mtime_ns) != (st.st_size, st.st_mtime_ns) or \
                   (stat_end.st_size, stat_end.st_mtime_ns) != (st.st_size, st.st_mtime_ns):
                    manifest.observe(rel, stat_end.st_size, stat_end.st_mtime_ns, now)
                    totals["pending"] += 1
                    continue
                # Bounded retries for outages/429/5xx/timeout.  A retry calls
                # the existing uploader again: it re-verifies remote bytes,
                # so a successful PUT followed by lost response is safe.
                for attempt in range(3):
                    try:
                        result = uploader(file, rel)
                        break
                    except (OSError, RuntimeError) as error:
                        if "conflict" in str(error).lower() or attempt == 2:
                            raise
                        time.sleep(2 ** attempt)
                if result not in ("verified", "identical"):
                    raise RuntimeError("unverified uploader result")
                st2 = file.stat()
                if (st2.st_size, st2.st_mtime_ns) != (st.st_size, st.st_mtime_ns):
                    manifest.observe(rel, st2.st_size, st2.st_mtime_ns, now)
                    raise RuntimeError("file changed during transfer; revalidate next run")
                manifest.verified(rel, st.st_size, st.st_mtime_ns, h.hexdigest(), now)
                if result == "verified":
                    totals["uploaded"] += 1
                else:
                    totals["identical"] += 1
            except (OSError, RuntimeError, ValueError) as exc:
                # Never write secrets or remote exception bodies into persistent logs.
                conflict = "conflict" if "conflict" in str(exc).lower() else "failed"
                manifest.failure(rel, conflict, "Upload refused or failed; check local source and destination")
                totals["conflicts" if conflict == "conflict" else "failed"] += 1
                print("Upload error for:", repr(rel[:120]), "(status:", conflict + ")", file=sys.stderr)
        if manifest.get_meta("scan_pending") == "1":
            try:
                scanner()
            except Exception:
                totals["failed"] += 1
                print("Navidrome scan failed; will retry next interval", file=sys.stderr)
            else:
                manifest.set_meta("scan_pending", "0")
                manifest.set_meta("last_scan", datetime.now(timezone.utc).isoformat())
        manifest.set_meta("last_run", datetime.now(timezone.utc).isoformat())
        manifest.set_meta("last_stats", json.dumps(totals))
    finally:
        manifest.close()
    if totals["uploaded"]:
        notifier(f"Uploaded {totals['uploaded']} file(s) to Bunny Storage")
    if totals["failed"] or totals["conflicts"]:
        notifier(f"Sync needs attention: {totals['failed']} failed, {totals['conflicts']} conflicts")
    return totals


def install(config: dict) -> None:
    if sys.platform != "darwin":
        raise RuntimeError("install is supported only on macOS")
    valid_config(config)
    root = Path(config["music_dir"])
    root.mkdir(parents=True, exist_ok=True)
    if root.is_symlink():
        raise RuntimeError("music root must not be a symlink")
    protected_dir(APP_DIR)
    for source in ("navidrome_family_sync.py", "upload_music.py", "sync_library.py"):
        src = (Path(__file__).with_name(source) if source == "navidrome_family_sync.py"
               else Path(__file__).resolve().parents[1] / "integrations" / "bunny" / source)
        if not src.is_file():
            raise RuntimeError("missing bundled file: " + source)
        atomic_private_write(APP_DIR / source, src.read_bytes())
    atomic_private_write(config_path(), json.dumps(config, indent=2).encode() + b"\n")
    protected_dir(LAUNCH_PLIST.parent)
    agent = {"Label": AGENT_LABEL, "ProgramArguments": [sys.executable, str(APP_DIR / "navidrome_family_sync.py"), "run-once"],
             "StartInterval": 300, "RunAtLoad": True, "WorkingDirectory": str(APP_DIR),
             "StandardOutPath": str(APP_DIR / "agent.log"), "StandardErrorPath": str(APP_DIR / "agent.err.log"),
             "ProcessType": "Background",
             # Python.org Python does not automatically use the macOS trust store.
             # Use the OS-maintained CA bundle; never disable TLS verification.
             "EnvironmentVariables": {"SSL_CERT_FILE": "/etc/ssl/cert.pem"}}
    subprocess.run(["launchctl", "bootout", f"gui/{os.getuid()}/{AGENT_LABEL}"],
                   capture_output=True, check=False)
    atomic_private_write(LAUNCH_PLIST, plistlib.dumps(agent))
    subprocess.run(["launchctl", "bootstrap", f"gui/{os.getuid()}", str(LAUNCH_PLIST)], check=True)
    print("Installed per-user LaunchAgent; runs at login and every 300 seconds.")
    print("To check: python3 macos/navidrome_family_sync.py status")


def remove_agent():
    if sys.platform != "darwin":
        raise RuntimeError("macOS only")
    subprocess.run(["launchctl", "bootout", f"gui/{os.getuid()}/{AGENT_LABEL}"], capture_output=True, check=False)
    LAUNCH_PLIST.unlink(missing_ok=True)
    print("LaunchAgent disabled; music, manifest, and Keychain entries retained.")


def main() -> int:
    if sys.platform == "darwin":
        os.environ.setdefault("SSL_CERT_FILE", "/etc/ssl/cert.pem")
    parser = argparse.ArgumentParser(description="Navidrome Bunny Family: private Mac add-only sync")
    sub = parser.add_subparsers(dest="action", required=True)
    pi = sub.add_parser("install")
    pi.add_argument("--music-dir", default=str(DEFAULT_MUSIC))
    pi.add_argument("--server", required=True)
    pi.add_argument("--zone", required=True)
    pi.add_argument("--host", default="storage.bunnycdn.com")
    pi.add_argument("--owner", default="admin")
    sub.add_parser("setup-keychain")
    sub.add_parser("run-once")
    sub.add_parser("sync-now")
    sub.add_parser("status")
    sub.add_parser("pause")
    sub.add_parser("resume")
    sub.add_parser("uninstall")
    pl = sub.add_parser("logs")
    pl.add_argument("--lines", type=int, default=40)
    args = parser.parse_args()
    os.umask(0o077)
    try:
        if args.action == "install":
            install(dict(music_dir=str(Path(args.music_dir).expanduser().absolute()),server=args.server,
                         zone=args.zone,host=args.host,owner=args.owner))
        elif args.action == "uninstall":
            remove_agent()
        elif args.action == "setup-keychain":
            keychain_setup(read_config())
        elif args.action in ("pause", "resume"):
            protected_dir(APP_DIR)
            if args.action == "pause":
                atomic_private_write(APP_DIR / "paused", b"1\n")
            else:
                (APP_DIR / "paused").unlink(missing_ok=True)
            print(args.action + "d")
        elif args.action == "status":
            data = {"installed": LAUNCH_PLIST.exists(), "paused": (APP_DIR / "paused").exists(),
                    "interval_seconds": 300, "music_dir": read_config()["music_dir"]}
            if (APP_DIR / "manifest.sqlite").exists():
                db = Manifest(APP_DIR / "manifest.sqlite")
                try:
                    data.update(counts=db.status(), last_run=db.get_meta("last_run"),
                                last_scan=db.get_meta("last_scan"), scan_pending=db.get_meta("scan_pending"))
                finally:
                    db.close()
            print(json.dumps(data, indent=2))
        elif args.action == "logs":
            for path in (APP_DIR / "agent.log", APP_DIR / "agent.err.log"):
                print("===", path.name, "===")
                if path.exists():
                    print("".join(path.read_text(errors="replace").splitlines(keepends=True)[-max(1,min(args.lines,200)):]))
        else:
            if (APP_DIR / "paused").exists():
                print("Sync paused; no network actions taken")
                return 0
            config = read_config()
            with exclusive_lock(APP_DIR / "sync.lock") as acquired:
                if not acquired:
                    print("Another sync is already running; safely skipping")
                    return 0
                bunny = None
                def upload(file, path):
                    nonlocal bunny
                    if bunny is None:
                        bunny = keychain_read(BUNNY_ITEM, config["zone"])
                    return direct_upload(file, path, config, bunny)
                # Read Owner password only when a scan is needed, not on each idle poll.
                totals = sync_once(config, APP_DIR,
                                   uploader=upload,
                                   scanner=lambda: direct_scan(config, keychain_read(OWNER_ITEM, config["owner"])))
                print("Sync completed:", json.dumps(totals, sort_keys=True))
                if totals["failed"] or totals["conflicts"]:
                    return 2
        return 0
    except (OSError, RuntimeError, ValueError, KeyError, sqlite3.Error, subprocess.CalledProcessError, subprocess.TimeoutExpired) as exc:
        # Avoid logging raw exception objects from external tools: they may
        # contain URLs/credentials.  Operator can inspect source and status.
        print("Sync operation failed safely:", type(exc).__name__, file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
