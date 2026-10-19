#!/usr/bin/env python3
"""Generate metadata for precompiled, explicit development/test artifacts."""
import hashlib
import json
from pathlib import Path
import sys

root = Path(sys.argv[1]).resolve()
version = sys.argv[2] if len(sys.argv) > 2 else 'v0.1.0-oa.20261002.1'
assert version in tuple('v0.1.0-oa.20261002.'+str(n) for n in range(1,12))
assert root.name.startswith('oa-public-release.') and root.parent == Path('/tmp').resolve()
artifacts = root / 'artifacts'
manifest = {
    'version': version, 'platform': 'linux/amd64',
    'official_release': False, 'commit': 'unknown',
    'source_archive_sha256': hashlib.sha256((root / 'source.tar.gz').read_bytes()).hexdigest(),
    'applications': {}, 'contracts': {},
}
for app in (('business-engagement','unified-workbench') if version.endswith(('.7','.8')) else ('unified-workbench',) if version.endswith(('.3', '.4', '.5', '.6', '.9', '.10', '.11')) else ('project-workspace', 'unified-workbench')):
    required = {'server', 'Dockerfile'} if app == 'unified-workbench' else {'server', 'ops', 'workload-keyctl', 'Dockerfile'}
    assert required <= {path.name for path in (artifacts / app).iterdir() if path.is_file()}, 'compiled artifact set incomplete'
    manifest['applications'][app] = {
        path.name: hashlib.sha256(path.read_bytes()).hexdigest()
        for path in sorted((artifacts / app).iterdir()) if path.is_file()
    }
for path in sorted((artifacts / 'contracts').iterdir()):
    manifest['contracts'][path.name] = hashlib.sha256(path.read_bytes()).hexdigest()
if version.endswith(('.3', '.4', '.5', '.6', '.7', '.8', '.9', '.10', '.11')):
    source = Path(__file__).resolve().parents[2]
    manifest['assets'] = {'reference-oa-initiation-runtime.js': hashlib.sha256((source / 'frontend/oms-workbench/internal/server/reference_oa_initiation_runtime.js').read_bytes()).hexdigest()}
    if version.endswith(('.6', '.7', '.8', '.9', '.10', '.11')):
        manifest['assets']['reference-oa-business-runtime.js'] = hashlib.sha256((source / 'frontend/oms-workbench/internal/server/reference_oa_business_runtime.js').read_bytes()).hexdigest()
    if version.endswith(('.7','.8','.9','.10','.11')):
        manifest['assets']['reference-oa-bidding-runtime.js'] = hashlib.sha256((source / 'frontend/oms-workbench/internal/server/reference_oa_bidding_runtime.js').read_bytes()).hexdigest()
target = artifacts / 'manifest.json'
assert not target.exists()
target.write_text(json.dumps(manifest, indent=2) + '\n')
print(json.dumps(manifest))
