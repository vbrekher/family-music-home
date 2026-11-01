#!/usr/bin/env python3
"""Explicit Mac-only Doctor companion. Credentials stay in Keychain; analysis in local Docker."""
from contextlib import contextmanager
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import selectors
import stat
import subprocess
import sys
import tempfile
import time
import urllib.request
import urllib.error
from urllib.parse import urlsplit

KEYCHAIN_SERVICE='navidrome-family-sync-owner-password'
SYNC_CONFIG=Path.home()/'Library/Application Support/NavidromeFamilySync/config.json'
APP_DIR=Path.home()/'Library/Application Support/NavidromeFamilyDoctor'
IDENTIFIER=re.compile(r'[A-Za-z0-9_-]{8,80}')
class StopRequested(Exception):pass
class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs):raise ValueError('redirect refused; credentials remain at configured origin')

class Client:
    def __init__(self,origin):
        url=urlsplit(origin)
        if url.scheme not in ('http','https') or not url.hostname or url.username or url.password or url.path not in ('','/') or url.query or url.fragment:raise ValueError('invalid server origin')
        self.loopback=url.hostname in ('127.0.0.1','localhost','::1')
        if url.scheme!='https' and not self.loopback:raise ValueError('HTTPS required')
        self.origin=origin.rstrip('/');self.opener=urllib.request.build_opener(NoRedirect());self.token=''
    def open(self,path,body=None,headers=None,timeout=30):
        if not path.startswith('/') or path.startswith('//'):raise ValueError('fixed origin path required')
        headers=dict(headers or {})
        if self.token:headers['X-ND-Authorization']='Bearer '+self.token
        if body is not None:headers['Content-Type']='application/json'
        request=urllib.request.Request(self.origin+path,data=json.dumps(body).encode() if body is not None else None,headers=headers)
        return self.opener.open(request,timeout=timeout)
    def json(self,path,body=None):
        with self.open(path,body) as response:
            raw=response.read(1024*1024+1)
            if len(raw)>1024*1024:raise ValueError('bounded response exceeded')
            return json.loads(raw)
    def family(self,path,body=None):return self.json('/api/family/doctor'+path,body)
    def login(self,username,password):
        result=self.json('/auth/login',{'username':username,'password':password});self.token=result['token']
        if not self.json('/api/family/me').get('admin'):raise ValueError('Administrator account required')

def existing_credentials(args,client):
    if args.test_credentials:
        if not client.loopback:raise ValueError('credential file is permitted only for a disposable loopback acceptance clone')
        path=Path(args.test_credentials)
        if stat.S_IMODE(path.stat().st_mode)!=0o600 or path.is_symlink():raise ValueError('test credentials must be an existing private file')
        return args.owner,json.loads(path.read_text())[args.owner]['password']
    if sys.platform!='darwin':raise ValueError('macOS Keychain required')
    command=['/usr/bin/security','find-generic-password','-w','-s',KEYCHAIN_SERVICE,'-a',args.owner]
    result=subprocess.run(command,capture_output=True,text=True,timeout=20)
    if result.returncode or not result.stdout.rstrip('\n'):raise ValueError('existing administrator Keychain item is unavailable')
    return args.owner,result.stdout.rstrip('\n')

def local_docker():
    endpoint=subprocess.check_output(['docker','context','inspect','desktop-linux','--format','{{.Endpoints.docker.Host}}'],text=True,stderr=subprocess.DEVNULL).strip()
    if not endpoint.startswith('unix://'):raise ValueError('local Docker Desktop Unix socket required')

def download(client,job,item,path):
    track=item['track'];size=int(track['size'])
    if not 0<size<=1024**3:raise ValueError('invalid bounded payload')
    digest=hashlib.sha256();read=0;start=time.monotonic();checked=start
    with client.open(f"/api/family/doctor/jobs/{job}/items/{track['id']}/content",headers={'X-ND-Doctor-Lease':item['lease']}) as response:
        if response.status!=200 or int(response.headers.get('Content-Length','-1'))!=size:raise ValueError('unexpected bounded response')
        fd=os.open(path,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
        with os.fdopen(fd,'wb') as target:
            while read<size:
                if time.monotonic()-start>300:raise TimeoutError('transfer deadline reached')
                if time.monotonic()-checked>=1:
                    if client.family('/jobs/'+job)['state']!='active':raise StopRequested()
                    checked=time.monotonic()
                block=response.read(min(65536,size-read))
                if not block:raise ValueError('incomplete source transfer')
                read+=len(block);digest.update(block);target.write(block)
            target.flush();os.fsync(target.fileno())
    return digest.hexdigest()

def sandbox(client,job,path,settings,image):
    name='navidrome-doctor-'+os.urandom(8).hex()
    args=['docker','--context','desktop-linux','run','--rm','--pull','never','-i','--name',name,'--platform','linux/amd64','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--pids-limit','64','--cpus','2','--memory','512m','--user',str(os.getuid())+':'+str(os.getgid()),'--tmpfs','/tmp:rw,nosuid,nodev,noexec,size=16m','-e','OPENBLAS_NUM_THREADS=1','-e','OMP_NUM_THREADS=1','-v',str(path.parent.resolve())+':/work:ro',image,'/work/'+path.name]
    # No URL, password, bearer token, original music mount or source credential
    # enters this sandbox. Its only input file is a private downloaded copy.
    process=subprocess.Popen(args,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
    process.stdin.write(json.dumps(settings).encode());process.stdin.close();start=time.monotonic();checked=start
    selector=selectors.DefaultSelector();selector.register(process.stdout,selectors.EVENT_READ);raw=bytearray();eof=False
    try:
        while not eof or process.poll() is None:
            if time.monotonic()-start>int(settings['decodeSeconds'])+60:raise TimeoutError('sandbox deadline reached')
            if time.monotonic()-checked>=1:
                if client.family('/jobs/'+job)['state']!='active':raise StopRequested()
                checked=time.monotonic()
            if eof:time.sleep(.1);continue
            if selector.select(.1):
                block=os.read(process.stdout.fileno(),4096)
                if not block:eof=True;selector.unregister(process.stdout)
                raw.extend(block)
                if len(raw)>16385:raise ValueError('bounded sandbox output exceeded')
        if process.returncode or len(raw.strip())>16384:raise ValueError('bounded sandbox investigation failed')
        return json.loads(raw)
    finally:
        selector.close()
        if process.poll() is None:
            subprocess.run(['docker','--context','desktop-linux','kill',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
            process.wait(timeout=10)
        process.stdout.close()

def run(client,job,image,resume=False,retry=False):
    if not IDENTIFIER.fullmatch(job):raise ValueError('invalid job ID')
    current=client.family('/jobs/'+job)
    if current['mode']!='deep':raise ValueError('deep job required')
    if current['state']=='cancelled':raise ValueError('cancelled job cannot be resurrected')
    if resume or retry:
        if current['state']=='active':client.family('/jobs/'+job+'/control',{'action':'pause'})
        current=client.family('/jobs/'+job+'/control',{'action':'resume','retry':retry})
    elif current['state']=='paused':raise ValueError('explicit --resume required')
    if current['state']=='completed':print('Run already ended; inspect the saved findings.');return 0
    while True:
        current=client.family('/jobs/'+job)
        if current['state'] not in ('queued','active'):return 0 if current['state']=='completed' else 2
        item=client.family('/jobs/'+job+'/claim',{})
        if item is None:
            current=client.family('/jobs/'+job)
            if current['state']=='completed':
                print('Local run ended; inspect findings and evidence. No original music was changed.')
                return 2 if any(current['counts'].get(s,0) for s in ('failed','unavailable','changed','budget')) else 0
            print('Another companion holds the current lease; no parallel audio read started.');return 2
        if item['state']=='budget':print('Source payload budget exhausted; explicit increase is required.');continue
        if not IDENTIFIER.fullmatch(item['track']['id']) or not IDENTIFIER.fullmatch(item['lease']):raise ValueError('invalid private lease')
        with tempfile.TemporaryDirectory(prefix='investigation-',dir=APP_DIR) as folder:
            os.chmod(folder,0o700);path=Path(folder)/'input.audio'
            try:
                digest=download(client,job,item,path)
                result=sandbox(client,job,path,current['settings'],image)
                if result.get('sha256')!=digest:raise ValueError('download and sandbox hashes differ')
                body={'lease':item['lease'],'result':result}
                # Last-byte delivery can precede the server's transactional hash
                # publication by a few milliseconds. Same-lease retries are safe.
                for attempt in range(4):
                    try:client.family(f"/jobs/{job}/items/{item['track']['id']}/result",body);break
                    except urllib.error.HTTPError as error:
                        if error.code!=409 or attempt==3:raise
                        time.sleep(.2)
                print('Recorded bounded local analysis for '+item['track']['id'])
            except StopRequested:raise
            except (OSError,ValueError,TimeoutError,subprocess.SubprocessError,urllib.error.URLError):
                print('Investigation interrupted safely; prior evidence and reserved I/O remain recorded. Use --resume after reviewing the job.',file=sys.stderr)
                client.family('/jobs/'+job+'/control',{'action':'pause'})
                return 2

@contextmanager
def lock():
    APP_DIR.mkdir(parents=True,exist_ok=True,mode=0o700)
    if APP_DIR.is_symlink():raise ValueError('private state directory cannot be a symlink')
    APP_DIR.chmod(0o700);fd=os.open(APP_DIR/'companion.lock',os.O_CREAT|os.O_RDWR,0o600)
    try:
        fcntl.flock(fd,fcntl.LOCK_EX|fcntl.LOCK_NB);yield
    finally:os.close(fd)

def main():
    config={}
    if SYNC_CONFIG.is_file():config=json.loads(SYNC_CONFIG.read_text())
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--job',required=True);parser.add_argument('--server',default=config.get('server'));parser.add_argument('--owner',default=config.get('owner','admin'));parser.add_argument('--image',default='navidrome-family-doctor:local');parser.add_argument('--resume',action='store_true');parser.add_argument('--retry',action='store_true');parser.add_argument('--test-credentials')
    args=parser.parse_args()
    if not args.server:parser.error('--server is required when no existing sync configuration is available')
    client=Client(args.server)
    if not re.fullmatch(r'[a-z0-9][a-zA-Z0-9./:_@-]{0,199}',args.image):raise ValueError('invalid local image reference')
    if sys.platform!='darwin' and not (args.test_credentials and client.loopback):raise ValueError('Mac-only companion')
    local_docker()
    labels=json.loads(subprocess.check_output(['docker','--context','desktop-linux','image','inspect',args.image,'--format','{{json .Config.Labels}}'],text=True,stderr=subprocess.DEVNULL))
    if labels.get('org.opencontainers.image.source')!='https://github.com/aitishnyk/family-music-home' or labels.get('net.navidrome.family.role')!='doctor-companion':raise ValueError('locally built project Doctor image required')
    username,password=existing_credentials(args,client);client.login(username,password);password=None
    def stop(*_):raise StopRequested()
    signal.signal(signal.SIGINT,stop);signal.signal(signal.SIGTERM,stop)
    with lock():
        try:return run(client,args.job,args.image,args.resume,args.retry)
        except StopRequested:
            try:client.family('/jobs/'+args.job+'/control',{'action':'pause'})
            except Exception:pass
            print('Paused. Durable progress is kept in the existing server SQLite.');return 2
if __name__=='__main__':
    try:sys.exit(main())
    except Exception as error:
        print('Doctor companion failed safely ('+type(error).__name__+'); inspect job state and local Docker/Keychain access. No credentials were printed.',file=sys.stderr)
        sys.exit(1)
