#!/usr/bin/env python3
"""Record only explicit compiled artifacts; never read deployment secrets."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys

folder, source = map(Path, sys.argv[1:])
assert folder.is_dir() and source.is_file()
apps = ('project-workspace', 'production-operations', 'approval-audit-results', 'cost-outsourcing')
values = {}
for app in apps:
    values[app] = {}
    for name in ('server', 'ops', 'workload-keyctl'):
        path = folder / app / name
        data = path.read_bytes()
        assert data[:6] == b'\x7fELF\x02\x01' and data[18:20] == b'\x3e\x00', 'not Linux AMD64 ELF'
        build = subprocess.check_output(['go', 'version', '-m', str(path)], text=True)
        for flag in ('CGO_ENABLED=0', 'GOOS=linux', 'GOARCH=amd64', '-trimpath=true'):
            assert flag in build, 'unexpected build flags'
        values[app][name] = {'sha256': hashlib.sha256(data).hexdigest(), 'go_build': build}
        os.chmod(path, 0o500)
recipe = Path(__file__).with_name('Dockerfile.verified-artifact')
shutil.copyfile(recipe, folder / 'Dockerfile.verified-artifact')
manifest = {'schema_version': '1', 'release': 'v0.1.0-test.50', 'source_archive_sha256': hashlib.sha256(source.read_bytes()).hexdigest(), 'toolchain': subprocess.check_output(['go', 'version'], text=True).strip(), 'recipe_sha256': hashlib.sha256(recipe.read_bytes()).hexdigest(), 'applications': values, 'buildkit_provenance': False}
target = folder / 'artifacts.json'
target.write_text(json.dumps(manifest, indent=2) + '\n')
os.chmod(target, 0o600)
print('PASS explicit Linux AMD64 artifacts: four applications / twelve binaries')
