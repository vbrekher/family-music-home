#!/usr/bin/env python3
"""Download checksum-pinned upstream NDPs and derive least-privilege local packages.
No service/production mutation. Writes a staging directory, not /data/plugins.
"""
import argparse,hashlib,json,pathlib,urllib.request,zipfile,io
PINS=[('J0R6IT0/navidrome-lyrics-plugin','v8.1.0','nd-lyrics.ndp','29dd571ba0d4bb756e62c0d5c594a2d7e7c9ccec2950f3b027bd64c73c605715'),('navidrome/apple-music-plugin','v0.2.0','apple-music.ndp','3682751e72caa5cc46b3f7ab70de70ea026f0a38a47f780b2d2b4922ed0ad154'),('sunsetroute1/navidrome-coverartarchive-plugin','v1.0.0','coverartarchive.ndp','3b0ec9055ce7825ebecf70f129e69aafc2d3f1398376ce7a7d390e31e72046cb')]
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('destination',type=pathlib.Path);p.add_argument('--downloads',type=pathlib.Path);a=p.parse_args();a.destination.mkdir(parents=True,exist_ok=True);records=[]
 for repo,tag,name,sha in PINS:
  url=f'https://github.com/{repo}/releases/download/{tag}/{name}'
  raw=(a.downloads/name).read_bytes() if a.downloads else urllib.request.urlopen(url,timeout=60).read(32<<20)
  if hashlib.sha256(raw).hexdigest()!=sha:raise ValueError('Upstream checksum mismatch: '+name)
  with zipfile.ZipFile(io.BytesIO(raw)) as z:manifest=json.loads(z.read('manifest.json'));wasm=z.read('plugin.wasm')
  if len(wasm)>32<<20 or not wasm.startswith(b'\0asm'):raise ValueError('Invalid WASM')
  if name=='nd-lyrics.ndp':
   manifest['permissions']['http']['requiredHosts']=['lrclib.net']
   manifest['permissions']['library']={'reason':'Read library metadata only; no filesystem or sidecar access','filesystem':False}
   manifest['permissions']['kvstore']['maxSize']='20MB'
   schema=manifest['config']['schema']['properties'];schema['providersList']['default']=[{'provider':'lrclib','romanizationScript':'none','translationLanguage':'none'}]
   schema['writeLyrics']['default']=False
  if name=='coverartarchive.ndp':
   manifest['permissions']['http']['requiredHosts']=['musicbrainz.org','coverartarchive.org','archive.org','*.archive.org']
  output=a.destination/name
  with zipfile.ZipFile(output,'w',compression=zipfile.ZIP_DEFLATED) as z:
   for entry,data in [('manifest.json',json.dumps(manifest,sort_keys=True,separators=(',',':')).encode()),('plugin.wasm',wasm),('LICENSE',(pathlib.Path(__file__).resolve().parents[1]/'integrations/plugins/licenses'/(name.removesuffix('.ndp')+'.LICENSE')).read_bytes())]:
    info=zipfile.ZipInfo(entry,date_time=(2026,1,1,0,0,0));info.compress_type=zipfile.ZIP_DEFLATED;info.external_attr=0o100644<<16;z.writestr(info,data)
  records.append(dict(name=name,version=tag,source=url,upstreamSHA256=sha,stagedSHA256=hashlib.sha256(output.read_bytes()).hexdigest(),wasmSHA256=hashlib.sha256(wasm).hexdigest(),permissions=manifest.get('permissions',{})))
 (a.destination/'plugins.lock.json').write_text(json.dumps(records,indent=2)+'\n');print(json.dumps(records,indent=2))
if __name__=='__main__':main()
