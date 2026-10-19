#!/usr/bin/env python3
"""Publish the Workbench-only OA administrator/profile UI correction."""
import hashlib
import importlib.util
import json
import os
import re
import subprocess
from pathlib import Path

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-admin-full-access-20261005a')
ARTIFACT = ROOT / 'workbench-profile-hotfix'
ENV = ROOT / 'candidate.env'
OVERRIDE = ROOT / 'compose.oa-public.json'
D1 = Path('/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.20_linux_amd64/deployments/d1/ops/d1')
VERSION = 'v0.1.0-oa.20261005.admin3'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
TAG = BASE + 'unified-workbench:' + VERSION

spec = importlib.util.spec_from_file_location('admin_release', ROOT / 'admin-full-access-release-ecs.py')
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
run, b = release.run, release.b

def save(path, value, private=False):
    data = value if isinstance(value, bytes) else (json.dumps(value, indent=2, sort_keys=True) + '\n').encode()
    path.write_bytes(data)
    path.chmod(0o600 if private else 0o644)

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def image_reference(container_name):
    container = json.loads(run(['docker', 'inspect', container_name]))[0]
    image = json.loads(run(['docker', 'image', 'inspect', container['Image']]))[0]
    refs = [value for value in image.get('RepoDigests', []) if value.startswith(BASE + 'unified-workbench@sha256:')]
    assert len(refs) == 1
    return refs[0]

def publish():
    server = ARTIFACT / 'server'
    assert server.is_file()
    expected = sha(server)
    probe = subprocess.run(['docker', 'manifest', 'inspect', TAG], capture_output=True, timeout=45)
    if probe.returncode == 0:
        run(['docker', 'pull', TAG])
    else:
        runtime = image_reference('omsbizfoundry-unified-workbench-1')
        (ARTIFACT / 'Dockerfile.release').write_text(
            'ARG RUNTIME_IMAGE\nFROM ${RUNTIME_IMAGE}\nUSER root\n'
            'COPY --chown=oms:oms server /app/server\nUSER oms\n', encoding='utf-8')
        run(['docker', 'build', '--platform', 'linux/amd64', '--memory', '96m',
             '--cpu-period', '100000', '--cpu-quota', '12000',
             '--build-arg', 'RUNTIME_IMAGE=' + runtime,
             '--label', 'oms.release=' + VERSION,
             '--label', 'oms.permission-closure=oa-admin-full-access-v2',
             '-f', str(ARTIFACT / 'Dockerfile.release'), '-t', TAG, str(ARTIFACT)])
        save(ROOT / 'private' / 'workbench-hotfix-push.log', run(['docker', 'push', TAG]), True)
    image = json.loads(run(['docker', 'image', 'inspect', TAG]))[0]
    refs = [value for value in image.get('RepoDigests', []) if value.startswith(BASE + 'unified-workbench@sha256:')]
    assert len(refs) == 1 and image['Architecture'] == 'amd64' and image['Config']['User'] == 'oms'
    name = 'oa_admin_full_access_workbench_hotfix_admin3'
    if name not in run(['docker', 'ps', '-a', '--format', '{{.Names}}']).decode().splitlines():
        run(['docker', 'create', '--name', name, '--network', 'none', '--memory', '64m', refs[0]])
    extracted = ROOT / 'private' / 'workbench-hotfix-server'
    run(['docker', 'cp', name + ':/app/server', str(extracted)])
    assert sha(extracted) == expected
    save(ROOT / 'workbench-hotfix-publication.json', {'passed': True, 'reference': refs[0], 'server_sha256': expected})
    return refs[0]

def replace_env(reference):
    lines = ENV.read_text().splitlines()
    key = 'D1_WORKBENCH_IMAGE'
    found = False
    for index, line in enumerate(lines):
        if line.startswith(key + '='):
            lines[index] = key + '=' + reference
            found = True
    assert found
    backup = ROOT / 'candidate.before-workbench-hotfix.env'
    if not backup.exists():
        save(backup, ENV.read_bytes(), True)
    save(ENV, ('\n'.join(lines) + '\n').encode(), True)

def facts():
    return {db: b.facts(db) for db in b.DBS if db != 'identity_access'}

def main():
    assert os.geteuid() == 0
    reference = publish()
    replace_env(reference)
    before = facts()
    grants = b.sql('identity_access', "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")
    save(ROOT / 'private' / 'workbench-hotfix-facts-before.json', before, True)
    output = run(['env', 'D1_ENV_FILE=' + str(ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(OVERRIDE), str(D1), 'up'], timeout=1800)
    save(ROOT / 'private' / 'workbench-hotfix-up.log', output, True)
    assert facts() == before
    assert b.sql('identity_access', "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t") == grants
    rows = [json.loads(line) for line in run(['docker', 'ps', '--filter', 'name=^/omsbizfoundry-', '--format', '{{json .}}']).decode().splitlines()]
    assert len(rows) == 17 and all(row.get('State') == 'running' and 'healthy' in row.get('Status', '') for row in rows)
    environment = dict(value.split('=', 1) for value in json.loads(run(['docker', 'inspect', 'omsbizfoundry-unified-workbench-1']))[0]['Config']['Env'] if '=' in value)
    assert environment['REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE'] == 'reference-oa-production-personnel-v1'
    assert environment['REFERENCE_OA_PRODUCTION_STAGE_PROFILE'] == 'reference-oa-stage-materials-v1'
    assert environment['REFERENCE_OA_BONUS_PROFILE'] == 'reference-oa-bonus-v3'
    save(ROOT / 'workbench-hotfix-rollout.json', {
        'passed': True, 'reference': reference, 'healthy': 17,
        'business_facts_unchanged': True, 'grants_unchanged': True,
        'profiles': {key: environment[key] for key in (
            'REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE',
            'REFERENCE_OA_PRODUCTION_STAGE_PROFILE', 'REFERENCE_OA_BONUS_PROFILE')},
    })
    print(json.dumps({'passed': True, 'reference': reference, 'healthy': 17}))

if __name__ == '__main__':
    main()
