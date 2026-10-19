#!/usr/bin/env python3
"""Publish the target-owned personnel permission context and matching Workbench."""
import hashlib
import importlib.util
import json
import os
import re
import subprocess
from pathlib import Path

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-admin-full-access-20261005a')
ARTIFACT = ROOT / 'personnel-context-hotfix'
ENV = ROOT / 'candidate.env'
OVERRIDE = ROOT / 'compose.oa-public.json'
D1 = Path('/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.20_linux_amd64/deployments/d1/ops/d1')
VERSION = 'v0.1.0-oa.20261005.admin4'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
APPS = {
    'project-workspace': ('PROJECT', 'postgres', ('server', 'ops', 'workload-keyctl')),
    'unified-workbench': ('WORKBENCH', 'oms', ('server',)),
}

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


def runtime_reference(app):
    container = inspect('omsbizfoundry-' + app + '-1')
    image = json.loads(run(['docker', 'image', 'inspect', container['Image']]))[0]
    refs = [value for value in image.get('RepoDigests', []) if value.startswith(BASE + app + '@sha256:')]
    assert len(refs) == 1
    return refs[0]


def publish():
    result = {'passed': False, 'version': VERSION, 'images': {}, 'binaries': {}}
    for app, (_, user, names) in APPS.items():
        directory = ARTIFACT / app
        assert all((directory / name).is_file() for name in names)
        result['binaries'][app] = {name: sha(directory / name) for name in names}
        tag = BASE + app + ':' + VERSION
        probe = subprocess.run(['docker', 'manifest', 'inspect', tag], capture_output=True, timeout=45)
        if probe.returncode == 0:
            run(['docker', 'pull', tag])
        else:
            copies = '\n'.join('COPY --chown=' + user + ':' + user + ' ' + name + ' /app/' + name for name in names)
            (directory / 'Dockerfile.release').write_text(
                'ARG RUNTIME_IMAGE\nFROM ${RUNTIME_IMAGE}\nUSER root\n' + copies + '\nUSER ' + user + '\n',
                encoding='utf-8')
            build = run([
                'docker', 'build', '--platform', 'linux/amd64', '--memory', '96m',
                '--cpu-period', '100000', '--cpu-quota', '12000',
                '--build-arg', 'RUNTIME_IMAGE=' + runtime_reference(app),
                '--label', 'oms.release=' + VERSION,
                '--label', 'oms.permission-closure=oa-admin-target-final-v3',
                '-f', str(directory / 'Dockerfile.release'), '-t', tag, str(directory),
            ])
            save(ROOT / 'private' / (app + '-admin4-build.log'), build, True)
            save(ROOT / 'private' / (app + '-admin4-push.log'), run(['docker', 'push', tag]), True)
        image = json.loads(run(['docker', 'image', 'inspect', tag]))[0]
        refs = [value for value in image.get('RepoDigests', []) if value.startswith(BASE + app + '@sha256:')]
        assert len(refs) == 1 and image['Architecture'] == 'amd64' and image['Config']['User'] == user
        reference = refs[0]
        extract = 'oa_admin4_' + app.replace('-', '_')
        containers = run(['docker', 'ps', '-a', '--format', '{{.Names}}']).decode().splitlines()
        if extract not in containers:
            run(['docker', 'create', '--name', extract, '--network', 'none', '--memory', '64m', reference])
        for name in names:
            target = ROOT / 'published-admin4' / app / name
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            run(['docker', 'cp', extract + ':/app/' + name, str(target)])
            assert sha(target) == result['binaries'][app][name]
        result['images'][app] = reference
        save(ROOT / 'personnel-context-publication.json', result)
        print(json.dumps({'phase': 'published', 'application': app, 'reference': reference}), flush=True)
    result['passed'] = True
    save(ROOT / 'personnel-context-publication.json', result)


def facts():
    return {db: b.facts(db) for db in b.DBS if db != 'identity_access'}


def replace_env(images):
    replacements = {'D1_' + APPS[app][0] + '_IMAGE': reference for app, reference in images.items()}
    lines = ENV.read_text().splitlines()
    seen = set()
    for index, line in enumerate(lines):
        key = line.split('=', 1)[0]
        if key in replacements:
            lines[index] = key + '=' + replacements[key]
            seen.add(key)
    assert seen == set(replacements)
    backup = ROOT / 'candidate.before-personnel-context-hotfix.env'
    if not backup.exists():
        save(backup, ENV.read_bytes(), True)
    save(ENV, ('\n'.join(lines) + '\n').encode(), True)


def rollout():
    publication = json.loads((ROOT / 'personnel-context-publication.json').read_text())
    assert publication['passed']
    before = facts()
    grants = b.sql('identity_access', "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")
    save(ROOT / 'private' / 'personnel-context-facts-before.json', before, True)
    replace_env(publication['images'])
    output = run(['env', 'D1_ENV_FILE=' + str(ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(OVERRIDE), str(D1), 'up'], timeout=1800)
    save(ROOT / 'private' / 'personnel-context-up.log', output, True)
    assert facts() == before
    assert b.sql('identity_access', "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t") == grants
    rows = [json.loads(line) for line in run(['docker', 'ps', '--filter', 'name=^/omsbizfoundry-', '--format', '{{json .}}']).decode().splitlines()]
    assert len(rows) == 17 and all(row.get('State') == 'running' and 'healthy' in row.get('Status', '') for row in rows)
    for app, reference in publication['images'].items():
        current = inspect('omsbizfoundry-' + app + '-1')
        image = json.loads(run(['docker', 'image', 'inspect', current['Image']]))[0]
        assert reference in image.get('RepoDigests', [])
    save(ROOT / 'personnel-context-rollout.json', {
        'passed': True,
        'healthy': 17,
        'business_facts_unchanged': True,
        'grants_unchanged': True,
        'images': publication['images'],
    })
    print(json.dumps({'passed': True, 'healthy': 17, 'images': publication['images']}))


if __name__ == '__main__':
    assert os.geteuid() == 0
    if len(os.sys.argv) != 2 or os.sys.argv[1] not in {'publish', 'rollout'}:
        raise SystemExit('usage: script publish|rollout')
    {'publish': publish, 'rollout': rollout}[os.sys.argv[1]]()
