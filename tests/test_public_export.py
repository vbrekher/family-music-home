import hashlib
import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('exporter', Path(__file__).resolve().parents[1] / 'scripts/export_public_release.py')
exporter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(exporter)


class PublicExportTest(unittest.TestCase):
    def fixture(self, root):
        repo = root / 'private'
        repo.mkdir()
        subprocess.run(['git', 'init', '-q', str(repo)], check=True)
        (repo / 'main.go').write_text('package main\n')
        (repo / 'private-report.txt').write_text('private-canary-never-export')
        subprocess.run(['git', '-C', str(repo), 'add', '.'], check=True)
        subprocess.run(['git', '-C', str(repo), '-c', 'user.name=Synthetic', '-c', 'user.email=synthetic@example.invalid', 'commit', '-qm', 'private history'], check=True)
        return repo, {'main.go': hashlib.sha256((repo / 'main.go').read_bytes()).hexdigest()}

    def test_only_reviewed_committed_blobs_export_without_history(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            repo, manifest = self.fixture(root)
            (repo / 'main.go').write_text('unreviewed worktree change')
            report = exporter.export(repo, 'HEAD', manifest, [b'private-canary-never-export'], root / 'public')
            self.assertFalse(report['gitHistoryCopied'])
            self.assertEqual((root / 'public/main.go').read_text(), 'package main\n')
            self.assertEqual(sorted(x.name for x in (root / 'public').iterdir()), ['SOURCE_MANIFEST.json', 'main.go'])

    def test_secret_or_manifest_drift_fails_without_partial_destination(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            repo, manifest = self.fixture(root)
            for content, deny in [('0' * 64, [b'canary']), (manifest['main.go'], [b'package main'])]:
                with self.assertRaises(ValueError):
                    exporter.export(repo, 'HEAD', {'main.go': content}, deny, root / 'public')
                self.assertFalse((root / 'public').exists())
                self.assertFalse(list(root.glob('.public-export-*')))

    def test_reviewed_synthetic_url_exception_cannot_bypass_secret_or_token_rules(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            repo, _ = self.fixture(root)
            name = 'url_test.go'
            content = 'package main\n// negative test: https://synthetic:placeholder@example.invalid\n'
            (repo / name).write_text(content)
            subprocess.run(['git', '-C', str(repo), 'add', name], check=True)
            subprocess.run(['git', '-C', str(repo), '-c', 'user.name=Synthetic', '-c', 'user.email=synthetic@example.invalid', 'commit', '-qm', 'negative fixture'], check=True)
            manifest = {name: hashlib.sha256(content.encode()).hexdigest()}
            for review, deny in [({}, [b'private-canary']), ({name: '0' * 64}, [b'private-canary']), (manifest, [b'placeholder'])]:
                with self.assertRaises(ValueError):
                    exporter.export(repo, 'HEAD', manifest, deny, root / 'public', review)
                self.assertFalse((root / 'public').exists())
            report = exporter.export(repo, 'HEAD', manifest, [b'private-canary'], root / 'public', manifest)
            self.assertEqual(report['reviewedSyntheticURLTestFiles'], 1)
            # A reviewed URL fixture containing a token still fails.
            content += '// ghp_' + 'a' * 30 + '\n'
            (repo / name).write_text(content)
            subprocess.run(['git', '-C', str(repo), 'add', name], check=True)
            subprocess.run(['git', '-C', str(repo), '-c', 'user.name=Synthetic', '-c', 'user.email=synthetic@example.invalid', 'commit', '-qm', 'token canary'], check=True)
            manifest = {name: hashlib.sha256(content.encode()).hexdigest()}
            with self.assertRaises(ValueError):
                exporter.export(repo, 'HEAD', manifest, [b'private-canary'], root / 'rejected', manifest)

    def test_batch_reads_binary_blobs_with_embedded_newlines(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            repo, manifest = self.fixture(root)
            binary = b'\x00header\n123 blob 999\n\xff\x00'
            (repo / 'art.bin').write_bytes(binary)
            subprocess.run(['git', '-C', str(repo), 'add', 'art.bin'], check=True)
            subprocess.run(['git', '-C', str(repo), '-c', 'user.name=Synthetic', '-c', 'user.email=synthetic@example.invalid', 'commit', '-qm', 'binary fixture'], check=True)
            manifest['art.bin'] = hashlib.sha256(binary).hexdigest()
            exporter.export(repo, 'HEAD', manifest, [b'private-canary'], root / 'public')
            self.assertEqual((root / 'public/art.bin').read_bytes(), binary)
            self.assertEqual((root / 'public/main.go').read_text(), 'package main\n')

    def test_runtime_files_history_workflows_and_traversal_are_never_allowed(self):
        for name in ('../main.go', '/main.go', '.git/config', '.env', 'a/data.db', 'a/data.db-wal', 'docs/operations/acceptance/report.json', '.github/workflows/build.yml', 'work/secrets/key'):
            self.assertFalse(exporter.safe_path(name), name)
        self.assertTrue(exporter.safe_path('core/family/store.go'))


if __name__ == '__main__':
    unittest.main()
