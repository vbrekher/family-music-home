#!/usr/bin/env python3
"""Creates two synthetic adults, plays MP3/FLAC, revokes access and restarts
an explicitly labelled fresh disposable loopback container. Credentials remain
in memory. Run only with the original six-album demo from create-demo-library.py.
The container and its volume are retained for inspection; no media is deleted.
"""
import concurrent.futures,json,pathlib,secrets,subprocess,time,urllib.request,urllib.error
import argparse
parser=argparse.ArgumentParser(description='Real first-run HTTP acceptance against a fresh, explicitly disposable local Docker container. Never use a personal database.')
parser.add_argument('--container', required=True)
parser.add_argument('--restore-check', action='store_true', help='Also stop, encrypt and restore this disposable data volume and verify both accounts in an isolated clone')
args=parser.parse_args()
container=args.container
info=json.loads(subprocess.check_output(['docker','inspect',container]))[0]
assert info['Config']['Labels'].get('net.family-music-home.disposable')=='true', 'explicit disposable label required'
bindings=info['HostConfig']['PortBindings']['4533/tcp']
assert len(bindings)==1 and bindings[0]['HostIp']=='127.0.0.1', 'loopback-only container required'
assert 'FMH_INSTALL_MODE=PUBLIC_SELF_HOST' in info['Config']['Env'], 'public self-host mode required'
base='http://127.0.0.1:'+bindings[0]['HostPort']; report={}; passwords={x:secrets.token_urlsafe(26) for x in ('river','brook')}
def call(path,body=None,token=None,headers=None,method=None):
 h={'Content-Type':'application/json',**(headers or {})}
 if token:h['X-ND-Authorization']='Bearer '+token
 q=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers=h,method=method or ('POST' if body is not None else 'GET'))
 try:
  with urllib.request.urlopen(q,timeout=30) as r:raw=r.read();status=r.status
 except urllib.error.HTTPError as e:status=e.code;raw=e.read()
 try:return status,json.loads(raw)
 except ValueError:return status,raw
for _ in range(60):
 try:
  if call('/ping')[0]==200:break
 except OSError:pass
 time.sleep(1)
setup=subprocess.check_output(['docker','exec',container,'cat','/data/setup-token'],text=True).strip()
assert call('/auth/createAdmin',{'username':'river','password':passwords['river']})[0]==403
assert call('/auth/createAdmin',{'username':'river','password':'short'},headers={'X-Family-Setup-Token':setup})[0]==403
report['bootstrap_missing_token_and_weak_password']='PASS'
def create(_):return call('/auth/createAdmin',{'username':'river','password':passwords['river']},headers={'X-Family-Setup-Token':setup})
with concurrent.futures.ThreadPoolExecutor(2) as pool:results=list(pool.map(create,range(2)))
assert sorted(c for c,_ in results)==[200,403],[(c,type(v).__name__) for c,v in results]
report['concurrent_first_admin_exactly_one']='PASS'
def login(name):
 c,v=call('/auth/login',{'username':name,'password':passwords[name]});assert c==200,c;return v['token']
admin=login('river');c,users=call('/api/user',token=admin);assert c==200 and len(users)==1
c,member=call('/api/user',{'userName':'brook','name':'Brook','password':passwords['brook'],'isAdmin':False},admin);assert c in (200,201),c
c,users=call('/api/user',token=admin);uid=next(u['id'] for u in users if u['userName']=='brook');member_token=login('brook')
c,libs=call('/api/user/'+uid+'/library',token=admin);assert c==200 and not libs,(c,libs)
c,albums=call('/api/album?_start=0&_end=20',token=member_token);assert c==200 and not albums
report['member_has_no_implicit_music_access']='PASS'
c,libs=call('/api/library',token=admin);assert c==200 and len(libs)==1
c,_=call('/api/user/'+uid+'/library',{'libraryIds':[libs[0]['id']]},admin,method='PUT');assert c==200,c
for _ in range(60):
 c,albums=call('/api/album?_start=0&_end=20',token=member_token)
 if c==200 and len(albums)==6:break
 time.sleep(1)
assert len(albums)==6,len(albums)
report['explicit_grant_six_albums']='PASS'
c,tracks=call('/api/song?_start=0&_end=50',token=member_token);assert c==200 and len(tracks)==12,(c,len(tracks))
# Subsonic uses token authentication, generated fresh per request; credentials stay in memory.
import hashlib,urllib.parse
for suffix in ('mp3','flac'):
 track=next(t for t in tracks if t['suffix'].lower()==suffix);salt=secrets.token_hex(12)
 query=urllib.parse.urlencode({'u':'brook','t':hashlib.md5((passwords['brook']+salt).encode()).hexdigest(),'s':salt,'v':'1.16.1','c':'family-clean-acceptance','id':track['id'],'format':'raw'})
 c,raw=call('/rest/stream?'+query,headers={'Range':'bytes=0-1023'});assert c==206 and len(raw)==1024,(suffix,c,len(raw));report[suffix+'_authorized_range']='PASS'
c,_=call('/api/user/'+uid+'/library',{'libraryIds':[]},admin,method='PUT');assert c==200
c,albums=call('/api/album?_start=0&_end=20',token=member_token);assert c==200 and not albums
report['library_revocation']='PASS'
subprocess.run(['docker','restart',container],check=True,stdout=subprocess.DEVNULL)
for _ in range(60):
 try:
  admin=login('river');login('brook');break
 except (AssertionError,OSError):time.sleep(1)
else:raise AssertionError('restart login')
c,users=call('/api/user',token=admin);assert c==200 and len(users)==2
assert subprocess.run(['docker','exec',container,'test','!','-e','/data/setup-token']).returncode==0
report['restart_both_logins_bootstrap_removed']='PASS'
if args.restore_check:
 import tempfile,os
 backup_key=secrets.token_hex(32).encode()+b'\n'
 image=info['Image'];clone='family-restore-check-'+secrets.token_hex(5)
 stopped=False;created=False;original_base=base
 with tempfile.TemporaryDirectory(prefix='family-clean-restore-') as folder:
  folder=pathlib.Path(folder);folder.chmod(0o700)
  def docker(command,**kwargs):
   return subprocess.run(['docker',*command],check=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,**kwargs)
  try:
   docker(['stop',container]);stopped=True
   docker(['run','--rm','--pull','never','--platform','linux/amd64','--user',info['Config']['User'] or '0:0','-i','--volumes-from',container+':ro','-v',str(folder)+':/backup','--entrypoint','python3',image,'/app/bunny/full_backup.py','backup','/data','/backup/full.enc','--key-stdin'],input=backup_key)
   docker(['start',container]);stopped=False
   docker(['run','--rm','--pull','never','--platform','linux/amd64','--user',info['Config']['User'] or '0:0','-i','-v',str(folder)+':/backup','--entrypoint','python3',image,'/app/bunny/full_backup.py','restore','/backup/full.enc','/backup/restored','--key-stdin'],input=backup_key)
   envfile=folder/'runtime.env';envfile.write_text('\n'.join(info['Config']['Env'])+'\n');envfile.chmod(0o600)
   command=['run','-d','--pull','never','--platform','linux/amd64','--name',clone,'--label','net.family-music-home.disposable=true','--user',info['Config']['User'] or '0:0','--cpus','2','--memory','1g','--env-file',str(envfile),'-p','127.0.0.1::4533','-v',str(folder/'restored')+':/data']
   for mount in info['Mounts']:
    if mount['Destination']=='/data':continue
    assert mount['Destination']=='/music' and not mount['RW'], 'only read-only demo music is supported'
    command+=['-v',mount['Source']+':/music:ro']
   docker([*command,image]);created=True
   restored=json.loads(subprocess.check_output(['docker','inspect',clone]))[0]
   base='http://127.0.0.1:'+restored['NetworkSettings']['Ports']['4533/tcp'][0]['HostPort']
   for attempt in range(60):
    try:
     restored_admin=login('river');login('brook');break
    except (AssertionError,OSError):time.sleep(1)
   else:raise AssertionError('restored logins unavailable')
   c,users=call('/api/user',token=restored_admin);assert c==200 and len(users)==2
   report['full_encrypted_backup_restore_both_logins']='PASS'
  finally:
   base=original_base
   if stopped:docker(['start',container])
   if created:docker(['rm','-f',clone])
print(json.dumps(report,indent=2))
