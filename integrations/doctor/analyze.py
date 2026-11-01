#!/usr/bin/env python3
"""Network-free, read-only Docker audio investigation. JSON in, bounded facts out."""
import hashlib
import json
import math
import os
from pathlib import Path
import selectors
import subprocess
import sys
import time
import numpy as np

VERSION = 'family-doctor-local-v1'
MAX_INPUT = 1024 * 1024 * 1024
FORMATS = 'mp3,flac,wav,mov,ogg,aac,aiff,ape,wv,tta,asf'

class BudgetExceeded(Exception): pass

def bounded_command(args, deadline, maximum=65536):
    process = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    selector = selectors.DefaultSelector(); selector.register(process.stdout, selectors.EVENT_READ)
    data = bytearray()
    try:
        while True:
            if time.monotonic() >= deadline: raise BudgetExceeded()
            events = selector.select(min(.2, max(0, deadline-time.monotonic())))
            if not events:
                if process.poll() is not None: break
                continue
            block = os.read(process.stdout.fileno(), 16384)
            if not block: break
            data.extend(block)
            if len(data)>maximum: raise ValueError('bounded output exceeded')
        code=process.wait(timeout=max(.01,deadline-time.monotonic()))
        return code,bytes(data)
    finally:
        selector.close(); process.stdout.close()
        if process.poll() is None: process.kill()
        process.wait()

def probe(path, deadline):
    command=['ffprobe','-v','error','-protocol_whitelist','file,pipe','-format_whitelist',FORMATS,'-show_entries','format=format_name,duration:stream=codec_type,codec_name,sample_rate,channels','-of','json','-i',str(path)]
    code,data=bounded_command(command,min(deadline,time.monotonic()+10))
    if code: raise ValueError('unsupported or unreadable media')
    info=json.loads(data); audio=[s for s in info.get('streams',[]) if s.get('codec_type')=='audio']
    if len(audio)!=1: raise ValueError('one audio stream required')
    stream=audio[0]; rate=int(stream.get('sample_rate',0)); channels=int(stream.get('channels',0))
    if not 1<=channels<=8 or not 1<=rate<=768000: raise ValueError('unsupported audio layout')
    fmt=info.get('format',{}).get('format_name','').split(',')[0]
    if fmt not in FORMATS.split(','): raise ValueError('unsupported demuxer')
    return stream,fmt,rate,channels

def decode(path, fmt, rate, channels, deadline):
    command=['ffmpeg','-v','error','-nostdin','-xerror','-threads','1','-filter_threads','1','-err_detect','explode','-protocol_whitelist','file,pipe','-format_whitelist',FORMATS,'-f',fmt]
    if fmt=='mov': command+=['-enable_drefs','0','-use_absolute_path','0']
    command+=['-i',str(path),'-map','0:a:0','-vn','-sn','-dn','-c:a','pcm_f32le','-f','f32le','pipe:1']
    process=subprocess.Popen(command,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
    selector=selectors.DefaultSelector();selector.register(process.stdout,selectors.EVENT_READ)
    total=near=0;peak=0.;carry=b'';fft_samples=[];spectral=np.zeros(16,dtype=np.float64);spectral_windows=0
    upper_energy=total_energy=0.
    try:
        while True:
            if time.monotonic()>=deadline:raise BudgetExceeded()
            events=selector.select(min(.2,max(0,deadline-time.monotonic())))
            if not events:
                if process.poll() is not None:break
                continue
            block=os.read(process.stdout.fileno(),65536)
            if not block:break
            raw=carry+block;unit=4*channels;size=len(raw)//unit*unit;carry=raw[size:]
            samples=np.frombuffer(raw[:size],dtype='<f4')
            if not samples.size:continue
            if not np.isfinite(samples).all():raise ValueError('non-finite PCM measurements')
            absolute=np.abs(samples);total+=int(samples.size);near+=int(np.count_nonzero(absolute>=.9995));peak=max(peak,float(absolute.max()))
            if spectral_windows<8:
                mono=samples[::channels];fft_samples.extend(mono.tolist())
                while len(fft_samples)>=16384 and spectral_windows<8:
                    window=np.asarray(fft_samples[:16384],dtype=np.float64);del fft_samples[:16384]
                    power=np.abs(np.fft.rfft(window*np.hanning(window.size)))**2
                    bins=np.array_split(power,16);spectral+=np.array([float(x.sum()) for x in bins]);spectral_windows+=1
                    frequencies=np.fft.rfftfreq(window.size,1/rate)
                    upper_energy+=float(power[frequencies>=16000].sum());total_energy+=float(power.sum())
                if spectral_windows>=8:fft_samples=[]
            if total>rate*channels*24*3600:raise BudgetExceeded()
        code=process.wait(timeout=max(.01,deadline-time.monotonic()))
        if code or carry or total==0:raise ValueError('strict decoder reported an error')
        if peak>100:raise ValueError('out-of-range PCM measurements')
        bands=(spectral/spectral.sum()).tolist() if spectral.sum()>0 else []
        ratio=upper_energy/total_energy if rate>=44100 and total_energy>0 and spectral_windows else None
        return {'decode':'decoded','duration':total/(rate*channels),'samples':total,'nearFullScale':near,'peak':peak,'spectralBands':bands,'upperBandRatio':ratio,'bandwidthNote':'First channel, up to eight initial 16384-sample windows; energy above 16 kHz is a limited bandwidth heuristic, not proof of lossy ancestry or superior mastering.'}
    finally:
        selector.close();process.stdout.close()
        if process.poll() is None:process.kill()
        process.wait()

def analyze(path,settings):
    if path.is_symlink() or not path.is_file() or path.stat().st_size>MAX_INPUT:raise ValueError('invalid bounded input')
    digest=hashlib.sha256()
    with path.open('rb') as source:
        for data in iter(lambda:source.read(1024*1024),b''):digest.update(data)
    result={'worker':VERSION,'sha256':digest.hexdigest(),'decode':'not-requested','duration':0,'samples':0,'nearFullScale':0,'peak':0}
    seconds=int(settings.get('decodeSeconds',120))
    if not 10<=seconds<=600:raise ValueError('invalid analysis deadline')
    deadline=time.monotonic()+seconds
    if not settings.get('quality') and not settings.get('fingerprint'):return result
    try:
        stream,fmt,rate,channels=probe(path,deadline)
        result.update({'measuredCodec':stream.get('codec_name',''),'measuredSampleRate':rate,'measuredChannels':channels})
    except BudgetExceeded:
        if settings.get('quality'):result['decode']='budget'
        result['note']='Probe deadline exceeded; no integrity or provenance verdict.';return result
    except (ValueError,KeyError,TypeError,json.JSONDecodeError):
        if settings.get('quality'):result['decode']='unsupported'
        result['note']='This bounded investigation requires one supported audio stream. Unsupported format/layout is not proof of corruption.';return result
    if settings.get('quality'):
        try:result.update(decode(path,fmt,rate,channels,deadline))
        except (BudgetExceeded,subprocess.TimeoutExpired):
            result['decode']='budget';result['note']='Explicit local analysis deadline reached. Integrity is unknown; prior successful evidence must be retained.'
        except ValueError:
            result['decode']='error';result['note']='Strict local decoding failed. Inspect manually; the original has not been changed.'
    if settings.get('fingerprint') and result['decode'] not in ('error','budget'):
        try:
            code,raw=bounded_command(['fpcalc','-json','-raw','-length','120',str(path)],deadline)
            parsed=json.loads(raw);fingerprint=parsed.get('fingerprint',[])
            if not isinstance(fingerprint,list) or len(fingerprint)>4096 or any(not isinstance(x,int) or not 0<=x<2**32 for x in fingerprint):raise ValueError('invalid fingerprint')
            duration=float(parsed.get('duration',0))
            # Some decoders return a nonzero code at clean EOF. Only a valid,
            # nonempty bounded JSON fingerprint is accepted, never stderr text.
            if not fingerprint or not math.isfinite(duration) or duration<=0:raise ValueError('fingerprint unavailable')
            result['fingerprint']=fingerprint;result['fingerprintDuration']=min(duration,120.)
        except (ValueError,KeyError,TypeError,json.JSONDecodeError):result['note']='Acoustical fingerprint unavailable; identical titles are not duplicate evidence.'
        except (BudgetExceeded,subprocess.TimeoutExpired):result['note']='Fingerprint deadline reached; no acoustical-match claim.'
    return result

def main():
    if len(sys.argv)!=2:raise ValueError('one private input path required')
    settings=json.loads(sys.stdin.buffer.read(65537))
    result=analyze(Path(sys.argv[1]),settings)
    raw=json.dumps(result,ensure_ascii=True,allow_nan=False,separators=(',',':'))
    if len(raw)>16384:raise ValueError('result too large')
    print(raw)
if __name__=='__main__':
    try:main()
    except Exception:
        print('Local investigation failed within its private sandbox; no original was modified.',file=sys.stderr)
        sys.exit(1)
