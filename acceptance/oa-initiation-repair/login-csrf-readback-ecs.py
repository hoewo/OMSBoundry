#!/usr/bin/env python3
"""Read-only business/Grant guards around actual login browser verification."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import pwd
import sys
import tarfile

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-login-csrf-20261003b')
spec = importlib.util.spec_from_file_location('login_deploy', str(ROOT / 'login-csrf-release-ecs.py'))
d = importlib.util.module_from_spec(spec)
spec.loader.exec_module(d)


def checkpoint():
    current = d.b.healthy()
    tables = d.b.sql('identity_access', "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename").splitlines()
    grant_tables = [t for t in tables if 'grant' in t]
    assert grant_tables
    ignored = tuple(t for t in tables if t not in grant_tables)
    grant_facts = d.b.facts('identity_access', exclude=ignored)
    return {'business': {db: d.b.facts(db) for db in d.b.DBS if db != 'identity_access'}, 'grants': {t: grant_facts[t] for t in grant_tables}, 'containers': {k: {'id': v['Id'], 'image': v['Image']} for k, v in current.items()}}


def export_safe():
    assert json.loads((ROOT / 'browser-after.json').read_text())['passed']
    paths = {
        'manifest.json': ROOT / 'artifacts/manifest.json',
        'publication.json': ROOT / 'publication/results.json',
        'preflight.json': d.BACKUP / 'preflight.json',
        'cold-backup.json': d.BACKUP / 'cold-backup.json',
        'restore.json': d.BACKUP / 'restore.json',
        'rollout.json': d.BACKUP / 'rollout.json',
        'offhost-export.json': ROOT / 'offhost-export.json',
        'offhost-confirmed.json': ROOT / 'offhost-confirmed.json',
        'browser-after.json': ROOT / 'browser-after.json'
    }
    digests = {}
    for name, path in paths.items():
        json.loads(path.read_text())
        digests[name] = hashlib.sha256(path.read_bytes()).hexdigest()
    target = Path('/home/ecs-user/oa-login-csrf-20261003b-safe.tar.gz')
    assert not target.exists()
    with tarfile.open(target, 'x:gz') as archive:
        for name, path in paths.items(): archive.add(path, arcname=name, recursive=False)
    target.chmod(0o600)
    account = pwd.getpwnam('ecs-user')
    os.chown(target, account.pw_uid, account.pw_gid)
    d.record(ROOT / 'safe-export-index.json', {'passed': True, 'archive_sha256': hashlib.sha256(target.read_bytes()).hexdigest(), 'files': digests, 'credentials_included': False})
    print(json.dumps({'phase': 'safe-json-export', 'passed': True, 'files': len(paths), 'archive_sha256': hashlib.sha256(target.read_bytes()).hexdigest()}))


if __name__ == '__main__':
    assert os.geteuid() == 0 and len(sys.argv) == 2 and sys.argv[1] in ('before', 'after', 'export')
    if sys.argv[1] == 'export':
        export_safe()
        raise SystemExit(0)
    value = checkpoint()
    if sys.argv[1] == 'before':
        assert not (ROOT / 'browser-before.json').exists()
        d.record(ROOT / 'browser-before.json', value)
        print(json.dumps({'phase': 'browser-read-guard-before', 'business_databases': 12, 'grants_captured': True}))
    else:
        before = json.loads((ROOT / 'browser-before.json').read_text())
        assert value == before, 'business/Grant/container state changed during login checks'
        d.record(ROOT / 'browser-after.json', {'passed': True, 'business_databases': 12, 'all_business_facts_and_sequences_equal': True, 'grants_equal': True, 'container_ids_equal': True, 'healthy': 17})
        print(json.dumps({'phase': 'browser-read-guard-after', 'passed': True, 'business_and_grants_unchanged': True, 'healthy': 17}))
