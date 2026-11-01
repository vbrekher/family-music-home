import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
spec=importlib.util.spec_from_file_location('companion',str(Path(__file__).resolve().parents[2]/'macos/navidrome_family_doctor.py'));companion=importlib.util.module_from_spec(spec);spec.loader.exec_module(companion)
class CompanionTest(unittest.TestCase):
 def test_auth_origin_validation_and_no_credential_redirects(self):
  for url in ['http://remote.example','https://user:password@example.com','https://example.com/?token=x','https://example.com/private','file:///private']:
   with self.assertRaises(ValueError):companion.Client(url)
  client=companion.Client('http://127.0.0.1:45536');self.assertTrue(client.loopback)
  with self.assertRaises(ValueError):companion.NoRedirect().redirect_request(None,None,None,None,None,None)
 def test_keychain_secret_is_captured_in_memory_and_not_in_argv(self):
  from types import SimpleNamespace
  args=SimpleNamespace(test_credentials=None,owner='admin')
  result=SimpleNamespace(returncode=0,stdout='synthetic-secret\n')
  with patch.object(companion.sys,'platform','darwin'),patch.object(companion.subprocess,'run',return_value=result) as call:
   username,password=companion.existing_credentials(args,companion.Client('https://music.example.invalid'))
  self.assertEqual(username,'admin');self.assertEqual(password,'synthetic-secret');self.assertNotIn('synthetic-secret',str(call.call_args));self.assertIn(companion.KEYCHAIN_SERVICE,str(call.call_args))
 def test_test_credential_file_is_for_loopback_only(self):
  from types import SimpleNamespace
  with tempfile.TemporaryDirectory() as folder:
   path=Path(folder)/'creds';path.write_text('{"admin":{"password":"synthetic-secret"}}');path.chmod(0o600);args=SimpleNamespace(test_credentials=str(path),owner='admin')
   with self.assertRaises(ValueError):companion.existing_credentials(args,companion.Client('https://music.example.invalid'))
   self.assertEqual(companion.existing_credentials(args,companion.Client('http://127.0.0.1:45536'))[1],'synthetic-secret')
if __name__=='__main__':unittest.main()
