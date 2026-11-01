"""Private family blob payload for authenticated SQLite backups (no music files)."""
from contextlib import closing
import hashlib
import os
from pathlib import Path
import re
import sqlite3
import tarfile

MAGIC = b'NDBACK02'
MAX_MEDIA = 128 * 1024 * 1024
MAX_FILE = 5 * 1024 * 1024
MAX_DATABASE = 5_000_000_000
ID = re.compile(r'[A-Za-z0-9_-]{8,80}')

def attachments(database):
    with closing(sqlite3.connect(database.resolve().as_uri()+'?mode=ro',uri=True)) as db:
        if not db.execute("SELECT 1 FROM sqlite_master WHERE name='family_attachment'").fetchone():return None
        rows=db.execute("SELECT id,size,sha256 FROM family_attachment WHERE state='ready' ORDER BY id").fetchall()
    if len(rows)>20000 or sum(row[1] for row in rows)>MAX_MEDIA:raise ValueError('family media quota exceeded')
    for key,size,digest in rows:
        if not ID.fullmatch(key) or not 0<size<=MAX_FILE or not re.fullmatch(r'[a-f0-9]{64}',digest):raise ValueError('invalid private media manifest')
    return rows

def verify(path,size,digest):
    if path.is_symlink() or not path.is_file() or path.stat().st_size!=size:raise ValueError('private media missing or invalid')
    h=hashlib.sha256()
    with path.open('rb') as f:
        for b in iter(lambda:f.read(1024*1024),b''):h.update(b)
    if h.hexdigest()!=digest:raise ValueError('private media hash mismatch')

def pack(snapshot,source_root,rows,writer):
    # Ready blobs are immutable; validate against the *snapshot*, not live rows.
    for key,size,digest in rows:verify(source_root/'family-media'/key,size,digest)
    with tarfile.open(fileobj=writer,mode='w|',format=tarfile.USTAR_FORMAT) as tar:
        def add(path,name):
            info=tarfile.TarInfo(name);info.size=path.stat().st_size;info.mode=0o600;info.mtime=0
            with path.open('rb') as f:tar.addfile(info,f)
        add(snapshot,'navidrome.db')
        for key,_,_ in rows:add(source_root/'family-media'/key,'family-media/'+key)

def unpack(archive,directory):
    root=directory/'payload';root.mkdir(mode=0o700)
    seen=set();total=0;count=0
    with tarfile.open(archive,mode='r:') as tar:
        for member in tar:
            count+=1
            if count>20001 or not member.isfile() or member.name in seen:raise ValueError('invalid family backup member')
            name=member.name
            if name=='navidrome.db':limit=MAX_DATABASE
            elif name.startswith('family-media/') and ID.fullmatch(name[len('family-media/'):]):limit=MAX_FILE
            else:raise ValueError('unsafe family backup path')
            if member.size<0 or member.size>limit:raise ValueError('family backup member too large')
            total+=member.size
            if total>MAX_DATABASE+MAX_MEDIA:raise ValueError('family backup too large')
            seen.add(name);path=root/name;path.parent.mkdir(mode=0o700,exist_ok=True)
            with tar.extractfile(member) as source,path.open('xb') as out:
                os.chmod(path,0o600);remaining=member.size
                while remaining:
                    b=source.read(min(1024*1024,remaining))
                    if not b:raise ValueError('truncated family backup member')
                    out.write(b);remaining-=len(b)
                out.flush();os.fsync(out.fileno())
    database=root/'navidrome.db'
    if not database.is_file():raise ValueError('family backup database missing')
    rows=attachments(database)
    if rows is None:raise ValueError('family schema missing')
    expected={'navidrome.db'}|{'family-media/'+key for key,_,_ in rows}
    if seen!=expected:raise ValueError('family backup does not match database manifest')
    for key,size,digest in rows:verify(root/'family-media'/key,size,digest)
    return database,root/'family-media'
