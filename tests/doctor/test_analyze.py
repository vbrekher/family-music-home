"""Runs exclusively in the local Doctor toolchain container."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
spec=importlib.util.spec_from_file_location('analyze',str(Path(__file__).resolve().parents[2]/'integrations/doctor/analyze.py'));analyze=importlib.util.module_from_spec(spec);spec.loader.exec_module(analyze)
class AnalyzeTest(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.root=Path(self.temp.name)
 def audio(self,name='sample.flac',duration=12):
  path=self.root/name
  # Changing partials make a reproducible non-copyright synthetic signal.
  expr='0.18*sin(2*PI*(220+60*sin(t*0.7))*t)+0.12*sin(2*PI*(440+100*sin(t*0.19))*t)+0.08*sin(2*PI*(900+300*sin(t*0.31))*t)'
  args=['ffmpeg','-v','error','-f','lavfi','-i','aevalsrc='+expr+':s=44100','-t',str(duration),'-ac','1']
  if name.endswith('.flac'):args+=['-c:a','flac']
  args+=[str(path)];subprocess.run(args,check=True);return path
 def test_full_decode_truthful_metrics_and_read_only_source(self):
  path=self.audio();before=path.read_bytes();result=analyze.analyze(path,{'quality':True,'fingerprint':False,'decodeSeconds':60})
  self.assertEqual(result['decode'],'decoded');self.assertGreater(result['samples'],0);self.assertAlmostEqual(result['duration'],12,places=1);self.assertEqual(result['measuredCodec'],'flac');self.assertEqual(path.read_bytes(),before);self.assertIn('not proof',result['bandwidthNote']);self.assertNotIn('qualityScore',result)
 def test_same_recording_across_encodings_has_actual_fingerprint_evidence(self):
  first=self.audio(duration=30);second=self.root/'encoded.mp3';subprocess.run(['ffmpeg','-v','error','-i',str(first),'-c:a','libmp3lame','-b:a','192k',str(second)],check=True)
  a=analyze.analyze(first,{'quality':True,'fingerprint':True,'decodeSeconds':90});b=analyze.analyze(second,{'quality':True,'fingerprint':True,'decodeSeconds':90})
  self.assertEqual(a['decode'],'decoded');self.assertEqual(b['decode'],'decoded');self.assertNotEqual(a['sha256'],b['sha256']);self.assertGreater(len(a.get('fingerprint',[])),20);self.assertGreater(len(b.get('fingerprint',[])),20)
  aa,bb=a['fingerprint'],b['fingerprint'];scores=[]
  for shift in range(-10,11):
   pairs=[(v,bb[i+shift]) for i,v in enumerate(aa) if 0<=i+shift<len(bb)]
   if len(pairs)>=20:scores.append(1-sum((x^y).bit_count() for x,y in pairs)/(32*len(pairs)))
  self.assertGreater(max(scores),.88)
 def test_checksum_only_does_not_claim_decode_or_provenance(self):
  path=self.root/'arbitrary';path.write_bytes(b'not an audio file');result=analyze.analyze(path,{'quality':False,'fingerprint':False,'decodeSeconds':10});self.assertEqual(result['decode'],'not-requested');self.assertEqual(result['samples'],0);self.assertNotIn('fingerprint',result)
 def test_truncated_audio_is_not_silently_verified(self):
  path=self.audio(duration=12);raw=path.read_bytes();path.write_bytes(raw[:len(raw)//2]);result=analyze.analyze(path,{'quality':True,'fingerprint':False,'decodeSeconds':60});self.assertNotEqual(result['decode'],'decoded');self.assertEqual(path.read_bytes(),raw[:len(raw)//2])
 def test_active_playlist_or_external_reference_is_not_a_supported_audio_format(self):
  path=self.root/'fake.m3u8';path.write_text('#EXTM3U\nhttps://invalid.example/music.ts\n');result=analyze.analyze(path,{'quality':True,'fingerprint':True,'decodeSeconds':10});self.assertEqual(result['decode'],'unsupported');self.assertNotIn('fingerprint',result)
if __name__=='__main__':unittest.main()
