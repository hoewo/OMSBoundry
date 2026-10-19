#!/usr/bin/env python3
"""Publish and roll out the Workbench OA stage-navigation correction."""
import hashlib
import importlib.util
import json
import os
import subprocess
from pathlib import Path

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-admin-full-access-20261005a')
ARTIFACT = ROOT / 'stage-navigation-hotfix'
ENV = ROOT / 'candidate.env'
OVERRIDE = ROOT / 'compose.oa-public.json'
D1 = Path('/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.20_linux_amd64/deployments/d1/ops/d1')
VERSION = 'v0.1.0-oa.20261005.admin6'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
TAG = BASE + 'unified-workbench:' + VERSION

spec = importlib.util.spec_from_file_location('admin_release', ROOT / 'admin-full-access-release-ecs.py')
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
run, b = release.run, release.b


def save(path, value, private=False):
    data = value if isinstance(value, bytes) else (json.dumps(value, indent=2, sort_keys=True) + '\n').encode()
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.write_bytes(data)
    path.chmod(0o600 if private else 0o644)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def current_runtime():
    container = json.loads(run(['docker', 'inspect', 'omsbizfoundry-unified-workbench-1']))[0]
    image = json.loads(run(['docker', 'image', 'inspect', container['Image']]))[0]
    refs = [value for value in image.get('RepoDigests', []) if value.startswith(BASE + 'unified-workbench@sha256:')]
    assert len(refs) == 1
    return refs[0]


def publish():
    server = ARTIFACT / 'server'
    assert server.is_file()
    expected = sha(server)
    assert subprocess.run(['docker', 'manifest', 'inspect', TAG], capture_output=True, timeout=45).returncode != 0
    save(ARTIFACT / 'Dockerfile.release', (
        'ARG RUNTIME_IMAGE\nFROM ${RUNTIME_IMAGE}\nUSER root\n'
        'COPY --chown=oms:oms server /app/server\nUSER oms\n'
    ).encode())
    save(ROOT / 'private/stage-navigation-build.log', run([
        'docker', 'build', '--platform', 'linux/amd64', '--memory', '96m',
        '--cpu-period', '100000', '--cpu-quota', '12000',
        '--build-arg', 'RUNTIME_IMAGE=' + current_runtime(),
        '--label', 'oms.release=' + VERSION,
        '--label', 'oms.oa-stage-navigation=native-state-separated-v1',
        '-f', str(ARTIFACT / 'Dockerfile.release'), '-t', TAG, str(ARTIFACT),
    ]), True)
    save(ROOT / 'private/stage-navigation-push.log', run(['docker', 'push', TAG]), True)
    image = json.loads(run(['docker', 'image', 'inspect', TAG]))[0]
    refs = [value for value in image.get('RepoDigests', []) if value.startswith(BASE + 'unified-workbench@sha256:')]
    assert len(refs) == 1 and image['Architecture'] == 'amd64' and image['Config']['User'] == 'oms'
    name = 'oa_admin_stage_navigation_hotfix_admin6'
    run(['docker', 'create', '--name', name, '--network', 'none', '--memory', '64m', refs[0]])
    extracted = ROOT / 'private/stage-navigation-server'
    run(['docker', 'cp', name + ':/app/server', str(extracted)])
    assert sha(extracted) == expected
    result = {'passed': True, 'version': VERSION, 'reference': refs[0], 'server_sha256': expected}
    save(ROOT / 'stage-navigation-publication.json', result)
    return result


def facts():
    return {database: b.facts(database) for database in b.DBS if database != 'identity_access'}


def grants():
    return b.sql('identity_access', "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")


def rollout(reference):
    before, grant_digest = facts(), grants()
    save(ROOT / 'private/stage-navigation-facts-before.json', before, True)
    lines = ENV.read_text().splitlines()
    found = False
    for index, line in enumerate(lines):
        if line.startswith('D1_WORKBENCH_IMAGE='):
            lines[index] = 'D1_WORKBENCH_IMAGE=' + reference
            found = True
    assert found
    save(ROOT / 'candidate.before-stage-navigation.env', ENV.read_bytes(), True)
    save(ENV, ('\n'.join(lines) + '\n').encode(), True)
    save(ROOT / 'private/stage-navigation-up.log', run([
        'env', 'D1_ENV_FILE=' + str(ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(OVERRIDE), str(D1), 'up',
    ], timeout=1800), True)
    assert facts() == before and grants() == grant_digest
    rows = [json.loads(line) for line in run([
        'docker', 'ps', '--filter', 'name=^/omsbizfoundry-', '--format', '{{json .}}',
    ]).decode().splitlines()]
    assert len(rows) == 17 and all(row.get('State') == 'running' and 'healthy' in row.get('Status', '') for row in rows)
    current = json.loads(run(['docker', 'inspect', 'omsbizfoundry-unified-workbench-1']))[0]
    image = json.loads(run(['docker', 'image', 'inspect', current['Image']]))[0]
    assert reference in image.get('RepoDigests', [])
    result = {'passed': True, 'reference': reference, 'healthy': 17, 'business_facts_unchanged': True, 'grants_unchanged': True}
    save(ROOT / 'stage-navigation-rollout.json', result)
    return result


def main():
    assert os.geteuid() == 0
    publication = publish()
    result = rollout(publication['reference'])
    print(json.dumps(result))


if __name__ == '__main__':
    main()
