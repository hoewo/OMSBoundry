"""Metadata must not accept a build still running or a missing program."""
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

script = Path(__file__).with_name('prepare-public-artifacts.py').resolve()


class ArtifactCompletenessTest(unittest.TestCase):
    def test_missing_workbench_binary_fails_before_manifest(self):
        with tempfile.TemporaryDirectory(prefix='oa-public-release.', dir='/tmp') as directory:
            root=Path(directory)
            (root/'artifacts/unified-workbench').mkdir(parents=True)
            (root/'source.tar.gz').write_bytes(b'source')
            (root/'artifacts/unified-workbench/Dockerfile').write_text('FROM runtime')
            result=subprocess.run(['python3',str(script),str(root),'v0.1.0-oa.20261002.11'],capture_output=True)
            self.assertNotEqual(result.returncode,0)
            self.assertIn(b'compiled artifact set incomplete',result.stderr)
            self.assertFalse((root/'artifacts/manifest.json').exists())

    def test_complete_repair_is_workbench_only_with_three_assets(self):
        with tempfile.TemporaryDirectory(prefix='oa-public-release.', dir='/tmp') as directory:
            root=Path(directory)
            (root/'artifacts/unified-workbench').mkdir(parents=True)
            (root/'artifacts/contracts').mkdir()
            (root/'source.tar.gz').write_bytes(b'source')
            for name in ('server','Dockerfile'): (root/'artifacts/unified-workbench'/name).write_bytes(b'artifact')
            result=subprocess.run(['python3',str(script),str(root),'v0.1.0-oa.20261002.11'],capture_output=True)
            self.assertEqual(result.returncode,0,result.stderr.decode())
            manifest=json.loads((root/'artifacts/manifest.json').read_text())
            self.assertEqual(set(manifest['applications']),{'unified-workbench'})
            self.assertEqual(set(manifest['applications']['unified-workbench']),{'server','Dockerfile'})
            self.assertEqual(len(manifest['assets']),3)
            self.assertFalse(manifest['official_release'])


if __name__ == '__main__': unittest.main()
