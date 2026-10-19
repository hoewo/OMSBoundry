#!/usr/bin/env python3
"""Publish Project empty production-candidate discovery semantics and roll it out."""
import hashlib
import importlib.util
import json
import os
import subprocess
from pathlib import Path


ROOT = Path('/opt/omsbizfoundry/acceptance/oa-admin-full-access-20261005a')
ARTIFACT = ROOT / 'empty-production-candidates-hotfix/project-workspace'
ENV = ROOT / 'candidate.env'
OVERRIDE = ROOT / 'compose.oa-public.json'
D1 = Path('/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.20_linux_amd64/deployments/d1/ops/d1')
VERSION = 'v0.1.0-oa.20261005.admin5'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
APP = 'project-workspace'
BINARIES = ('server', 'ops', 'workload-keyctl')

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


def inspect(name):
    return json.loads(run(['docker', 'inspect', name]))[0]


def runtime_reference():
    container = inspect('omsbizfoundry-' + APP + '-1')
    image = json.loads(run(['docker', 'image', 'inspect', container['Image']]))[0]
    refs = [value for value in image.get('RepoDigests', []) if value.startswith(BASE + APP + '@sha256:')]
    assert len(refs) == 1
    return refs[0]


def publish():
    assert all((ARTIFACT / name).is_file() for name in BINARIES)
    binary_digests = {name: sha(ARTIFACT / name) for name in BINARIES}
    tag = BASE + APP + ':' + VERSION
    assert subprocess.run(['docker', 'manifest', 'inspect', tag], capture_output=True, timeout=45).returncode != 0
    copies = '\n'.join('COPY --chown=postgres:postgres ' + name + ' /app/' + name for name in BINARIES)
    (ARTIFACT / 'Dockerfile.release').write_text(
        'ARG RUNTIME_IMAGE\nFROM ${RUNTIME_IMAGE}\nUSER root\n' + copies + '\nUSER postgres\n',
        encoding='utf-8',
    )
    build = run([
        'docker', 'build', '--platform', 'linux/amd64', '--memory', '96m',
        '--cpu-period', '100000', '--cpu-quota', '12000',
        '--build-arg', 'RUNTIME_IMAGE=' + runtime_reference(),
        '--label', 'oms.release=' + VERSION,
        '--label', 'oms.project-empty-production-candidates=200-v1',
        '-f', str(ARTIFACT / 'Dockerfile.release'), '-t', tag, str(ARTIFACT),
    ])
    save(ROOT / 'private/project-admin5-build.log', build, True)
    save(ROOT / 'private/project-admin5-push.log', run(['docker', 'push', tag]), True)
    image = json.loads(run(['docker', 'image', 'inspect', tag]))[0]
    refs = [value for value in image.get('RepoDigests', []) if value.startswith(BASE + APP + '@sha256:')]
    assert len(refs) == 1 and image['Architecture'] == 'amd64' and image['Config']['User'] == 'postgres'
    result = {'passed': True, 'version': VERSION, 'image': refs[0], 'binaries': binary_digests}
    save(ROOT / 'empty-production-candidates-publication.json', result)
    print(json.dumps(result))


def facts():
    return {database: b.facts(database) for database in b.DBS if database != 'identity_access'}


def grant_digest():
    return b.sql(
        'identity_access',
        "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t",
    )


def rollout():
    publication = json.loads((ROOT / 'empty-production-candidates-publication.json').read_text())
    assert publication['passed']
    before = facts()
    grants = grant_digest()
    save(ROOT / 'private/empty-production-candidates-facts-before.json', before, True)
    lines = ENV.read_text().splitlines()
    replaced = False
    for index, line in enumerate(lines):
        if line.startswith('D1_PROJECT_IMAGE='):
            lines[index] = 'D1_PROJECT_IMAGE=' + publication['image']
            replaced = True
    assert replaced
    backup = ROOT / 'candidate.before-empty-production-candidates.env'
    if not backup.exists():
        save(backup, ENV.read_bytes(), True)
    save(ENV, ('\n'.join(lines) + '\n').encode(), True)
    output = run(
        ['env', 'D1_ENV_FILE=' + str(ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(OVERRIDE), str(D1), 'up'],
        timeout=1800,
    )
    save(ROOT / 'private/empty-production-candidates-up.log', output, True)
    assert facts() == before
    assert grant_digest() == grants
    rows = [json.loads(line) for line in run([
        'docker', 'ps', '--filter', 'name=^/omsbizfoundry-', '--format', '{{json .}}',
    ]).decode().splitlines()]
    assert len(rows) == 17
    assert all(row.get('State') == 'running' and 'healthy' in row.get('Status', '') for row in rows)
    current = inspect('omsbizfoundry-' + APP + '-1')
    image = json.loads(run(['docker', 'image', 'inspect', current['Image']]))[0]
    assert publication['image'] in image.get('RepoDigests', [])
    result = {
        'passed': True,
        'healthy': 17,
        'business_facts_unchanged': True,
        'grants_unchanged': True,
        'image': publication['image'],
    }
    save(ROOT / 'empty-production-candidates-rollout.json', result)
    print(json.dumps(result))


if __name__ == '__main__':
    assert os.geteuid() == 0
    if len(os.sys.argv) != 2 or os.sys.argv[1] not in {'publish', 'rollout'}:
        raise SystemExit('usage: script publish|rollout')
    {'publish': publish, 'rollout': rollout}[os.sys.argv[1]]()
