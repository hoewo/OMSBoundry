#!/usr/bin/env python3
"""Export explicitly selected non-secret receipts, never runtime/DB/key backups."""
import hashlib
import json
import os
from pathlib import Path
import pwd
import sys
import tarfile

batch = sys.argv[1]
assert os.geteuid() == 0 and batch in ('j-after', 'k-final')
run = batch[0]
root = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002' + run)
backup = Path('/var/backups/omsbizfoundry/oa-public-20261002' + run)
checks = ('baseline', 'completed', 'comparison') if run == 'j' else ('baseline', 'completed', 'comparison', 'negotiation', 'tender', 'not-applicable', 'restored', 'final')
checker_root = 'bidding-browser-v2' if run == 'j' else 'bidding-browser'
selected = [(root / checker_root / (phase + '.json'), run + '/' + checker_root + '/' + phase + '.json') for phase in checks]
if run == 'k':
    assert all(json.loads(path.read_text())['passed'] for path, _ in selected)
    selected += [(root / path, run + '/' + path) for path in ('artifacts/manifest.json', 'publication/results.json', 'offhost-export.json', 'compose.oa-public.json', 'postflight.json', 'mount-order-observation.json')]
    selected += [(backup / (name + '.json'), run + '/backup/' + name + '.json') for name in ('preflight', 'cold-backup', 'restore', 'rollout')]
    selected += [(backup / 'rollout-failure.json', run + '/backup/rollout-failure.json')]
target = Path('/home/ecs-user/oa-bidding-safe-evidence-20261002' + batch + '.tar.gz')
assert not target.exists() and all(path.is_file() for path, _ in selected)
with tarfile.open(target, 'x:gz') as archive:
    for path, name in selected:
        json.loads(path.read_text())
        archive.add(path, arcname=name, recursive=False)
target.chmod(0o600)
account = pwd.getpwnam('ecs-user')
os.chown(target, account.pw_uid, account.pw_gid)
print(json.dumps({'selected_json_files': len(selected), 'sha256': hashlib.sha256(target.read_bytes()).hexdigest(), 'bytes': target.stat().st_size, 'contains_private_backup': False}), flush=True)
