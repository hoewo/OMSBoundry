#!/usr/bin/env python3
"""Deploy the current native permission closure without collapsing authorization layers."""
import hashlib
import importlib.util
import json
import os
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-admin-full-access-20261005a')
ARTIFACTS = ROOT
CURRENT_ENV = Path('/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.https.env')
CURRENT_OVERRIDE = Path('/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a/compose.oa-public.json')
CANDIDATE_ENV = ROOT / 'candidate.env'
CANDIDATE_OVERRIDE = ROOT / 'compose.oa-public.json'
D1 = Path('/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.20_linux_amd64/deployments/d1/ops/d1')
VERSION = 'v0.1.0-oa.20261005.admin1'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
APPS = {
    'project-workspace': ('PROJECT', 'postgres'),
    'production-operations': ('PRODUCTION', 'postgres'),
    'approval-audit-results': ('APPROVAL_AUDIT_RESULTS', 'postgres'),
    'bonus-distribution': ('BONUS_DISTRIBUTION', 'postgres'),
    'unified-workbench': ('WORKBENCH', 'oms'),
}
SOURCE_DIRECTORIES = {'production-operations': 'design-operations'}
PROFILES = {
    'REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE': 'reference-oa-production-personnel-v1',
    'REFERENCE_OA_PRODUCTION_STAGE_PROFILE': 'reference-oa-stage-materials-v1',
    'REFERENCE_OA_BONUS_PROFILE': 'reference-oa-bonus-v3',
}
SCHEMAS = {
    'project_workspace': '000012_reference_oa_production_personnel.sql',
    'design_operations': '000006_change_completion_definition.sql',
    'bonus_distribution': '0004_reference_oa_professional_allocations.sql',
}
COMMAND_DRIVEN_SERVICES = [
    'postgres-provision', 'project-workload-key-permissions', 'project-workload-keys',
    'business-engagement-workload-key-permissions', 'business-engagement-workload-keys',
    'production-workload-key-permissions', 'production-workload-keys',
    'approval-audit-results-workload-key-permissions', 'approval-audit-results-workload-keys',
    'cost-outsourcing-workload-key-permissions', 'cost-outsourcing-workload-keys',
    'bonus-distribution-workload-key-permissions', 'bonus-distribution-workload-keys',
    'billing-workload-key-permissions', 'billing-workload-keys',
    'contract-workload-key-permissions', 'contract-workload-keys',
    'money-workload-key-permissions', 'money-workload-keys',
    'identity-migrate', 'identity-access', 'public-keys-permissions', 'public-keys',
    'contract-migrate', 'contract-ledger', 'money-migrate', 'money-flow-ledger',
    'people-migrate', 'people-groups', 'party-migrate', 'party-directory',
    'project-migrate', 'project-workspace', 'evidence-migrate', 'evidence-service',
    'business-engagement-migrate', 'business-engagement', 'production-migrate',
    'production-operations', 'approval-audit-results-migrate', 'approval-audit-results',
    'cost-outsourcing-migrate', 'cost-outsourcing', 'bonus-distribution-migrate',
    'bonus-distribution', 'billing-migrate', 'billing-invoice',
]

spec = importlib.util.spec_from_file_location('cost_release', '/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a/cost-release-ecs.py')
cost = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cost)
b = cost.b
run = b.run

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def save(path, value, private=False):
    data = value if isinstance(value, bytes) else (json.dumps(value, indent=2, sort_keys=True) + '\n').encode()
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.write_bytes(data)
    path.chmod(0o600 if private else 0o644)

def inspect(name):
    return json.loads(run(['docker', 'inspect', name]))[0]

def image_reference(container):
    image = json.loads(run(['docker', 'image', 'inspect', container['Image']]))[0]
    refs = [value for value in image.get('RepoDigests', []) if value.startswith(BASE)]
    assert len(refs) == 1 and re.fullmatch(re.escape(BASE) + r'[^@]+@sha256:[a-f0-9]{64}', refs[0])
    return refs[0]

def publish():
    result = {'passed': False, 'version': VERSION, 'images': {}, 'binaries': {}}
    for app, (_, user) in APPS.items():
        directory = ARTIFACTS / SOURCE_DIRECTORIES.get(app, app)
        names = ['server'] if app == 'unified-workbench' else ['server', 'ops', 'workload-keyctl']
        assert all((directory / name).is_file() for name in names)
        result['binaries'][app] = {name: sha(directory / name) for name in names}
        current = inspect('omsbizfoundry-' + app + '-1')
        runtime = image_reference(current)
        tag = BASE + app + ':' + VERSION
        probe = subprocess.run(['docker', 'manifest', 'inspect', tag], capture_output=True, timeout=45)
        if probe.returncode == 0:
            run(['docker', 'pull', tag])
            tagged = json.loads(run(['docker', 'image', 'inspect', tag]))[0]
            assert tagged['Config']['Labels']['oms.release'] == VERSION and tagged['Config']['Labels']['oms.permission-closure'] == 'oa-admin-full-access-v1'
            refs = [value for value in tagged.get('RepoDigests', []) if value.startswith(BASE + app + '@sha256:')]
            assert len(refs) == 1
            reference = refs[0]
        else:
            dockerfile = directory / 'Dockerfile.release'
            copies = '\n'.join('COPY --chown=' + user + ':' + user + ' ' + name + ' /app/' + name for name in names)
            dockerfile.write_text('ARG RUNTIME_IMAGE\nFROM ${RUNTIME_IMAGE}\nUSER root\n' + copies + '\nUSER ' + user + '\n', encoding='utf-8')
            build = run(['docker', 'build', '--platform', 'linux/amd64', '--memory', '96m', '--cpu-period', '100000', '--cpu-quota', '12000', '--build-arg', 'RUNTIME_IMAGE=' + runtime, '--label', 'oms.release=' + VERSION, '--label', 'oms.permission-closure=oa-admin-full-access-v1', '-f', str(dockerfile), '-t', tag, str(directory)])
            save(ROOT / 'private' / (app + '-build.log'), build, True)
            pushed = run(['docker', 'push', tag])
            save(ROOT / 'private' / (app + '-push.log'), pushed, True)
            matches = re.findall(rb'digest: (sha256:[a-f0-9]{64})', pushed)
            assert len(matches) == 1
            reference = BASE + app + '@' + matches[0].decode()
        run(['docker', 'pull', reference])
        candidate = json.loads(run(['docker', 'image', 'inspect', reference]))[0]
        assert candidate['Architecture'] == 'amd64' and candidate['Config']['User'] == user
        extract = 'oa_admin_full_access_' + app.replace('-', '_')
        if extract not in run(['docker', 'ps', '-a', '--format', '{{.Names}}']).decode().splitlines():
            run(['docker', 'create', '--name', extract, '--network', 'none', '--memory', '64m', reference])
        for name in names:
            target = ROOT / 'published' / app / name
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            run(['docker', 'cp', extract + ':/app/' + name, str(target)])
            assert sha(target) == result['binaries'][app][name]
        result['images'][app] = reference
        save(ROOT / 'publication.json', result)
        print(json.dumps({'phase': 'published', 'application': app, 'reference': reference}), flush=True)
    result['passed'] = True
    save(ROOT / 'publication.json', result)

def replace_env(data, replacements):
    lines = data.decode().splitlines()
    keys = {line.split('=', 1)[0] for line in lines if '=' in line}
    assert all(key in keys for key in replacements if key.startswith('D1_'))
    updated = [(line.split('=', 1)[0] + '=' + replacements[line.split('=', 1)[0]]) if line.split('=', 1)[0] in replacements else line for line in lines]
    updated.extend(key + '=' + value for key, value in replacements.items() if key not in keys)
    return ('\n'.join(updated) + '\n').encode()

def mount(service, destination, source, override):
    values = override.setdefault('services', {}).setdefault(service, {}).setdefault('volumes', [])
    suffix = ':' + destination + ':ro'
    override['services'][service]['volumes'] = [value for value in values if not value.endswith(suffix)] + [str(source) + suffix]

def facts():
    return {db: b.facts(db) for db in b.DBS if db != 'identity_access'}

def prepare():
    publication = json.loads((ROOT / 'publication.json').read_text())
    assert publication['passed'] and CURRENT_ENV.stat().st_mode & 0o077 == 0 and CURRENT_OVERRIDE.stat().st_mode & 0o077 == 0
    replacements = {'D1_' + APPS[app][0] + '_IMAGE': reference for app, reference in publication['images'].items()}
    replacements.update(PROFILES)
    save(ROOT / 'original.env', CURRENT_ENV.read_bytes(), True)
    save(CANDIDATE_ENV, replace_env(CURRENT_ENV.read_bytes(), replacements), True)
    override = json.loads(CURRENT_OVERRIDE.read_text())
    # The currently installed D1 bundle predates these explicit profile
    # variables.  Keep the immutable candidate self-contained instead of
    # assuming a newer base compose file is already installed on the host.
    profile_environments = {
        'unified-workbench': {
            'REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE': PROFILES['REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE'],
            'REFERENCE_OA_PRODUCTION_STAGE_PROFILE': PROFILES['REFERENCE_OA_PRODUCTION_STAGE_PROFILE'],
            'REFERENCE_OA_BONUS_PROFILE': PROFILES['REFERENCE_OA_BONUS_PROFILE'],
        },
        'project-workspace': {
            'REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE': PROFILES['REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE'],
        },
        'production-operations': {
            'REFERENCE_STAGE_MATERIAL_PROFILE': PROFILES['REFERENCE_OA_PRODUCTION_STAGE_PROFILE'],
        },
        'bonus-distribution': {
            'REFERENCE_OA_BONUS_PROFILE': PROFILES['REFERENCE_OA_BONUS_PROFILE'],
            'REFERENCE_OA_PRODUCTION_STAGE_PROFILE': PROFILES['REFERENCE_OA_PRODUCTION_STAGE_PROFILE'],
        },
    }
    for service, environment in profile_environments.items():
        override.setdefault('services', {}).setdefault(service, {}).setdefault('environment', {}).update(environment)
    for service in COMMAND_DRIVEN_SERVICES:
        override.setdefault('services', {}).setdefault(service, {})['entrypoint'] = []
    ROOT.chmod(0o711)
    (ARTIFACTS / 'contracts').chmod(0o755)
    for name in ('project-workspace', 'production-operations', 'bonus-distribution'):
        source = ARTIFACTS / 'contracts' / (name + '-capabilities.json')
        assert source.is_file()
        source.chmod(0o444)
        for service in ('identity-access', 'identity-migrate'):
            mount(service, '/config/' + name + '-capabilities.json', source, override)
    save(CANDIDATE_OVERRIDE, override, True)
    before = facts()
    save(ROOT / 'facts-before.json', before, True)
    grants = b.sql('identity_access', "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")
    save(ROOT / 'grants-before.txt', (grants + '\n').encode(), True)
    d1('preflight-upgrade')
    print(json.dumps({'phase': 'prepared', 'applications': list(APPS), 'profiles': PROFILES, 'business_databases': len(before)}), flush=True)

def d1(command):
    return run(['env', 'D1_ENV_FILE=' + str(CANDIDATE_ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(CANDIDATE_OVERRIDE), str(D1), command], timeout=1800)

def verify_rollout():
    assert (ROOT / 'facts-before.json').is_file()
    before = json.loads((ROOT / 'facts-before.json').read_text())
    after = facts()
    for db, old in before.items():
        for key, value in old.items():
            if key == 'schema_migrations':
                continue
            assert after[db].get(key) == value, db + ':' + key
    for db, expected in SCHEMAS.items():
        assert b.sql(db, 'SELECT max(version) FROM schema_migrations') == expected
    assert b.sql('identity_access', "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t") == (ROOT / 'grants-before.txt').read_text().strip()
    running = [json.loads(line) for line in run(['docker', 'ps', '--filter', 'name=^/omsbizfoundry-', '--format', '{{json .}}']).decode().splitlines()]
    healthy = [row for row in running if row.get('State') == 'running' and 'healthy' in row.get('Status', '')]
    assert len(healthy) == 17
    identity = inspect('omsbizfoundry-identity-access-1')
    env = dict(value.split('=', 1) for value in identity['Config']['Env'] if '=' in value)
    manifests = []
    for path in env['CAPABILITY_MANIFEST_PATHS'].split(','):
        data = json.loads(run(['docker', 'exec', 'omsbizfoundry-identity-access-1', 'cat', path]))
        manifests.append({'application_id': data['application_id'], 'version': data['application_version'], 'count': len(data['capabilities'])})
    assert sum(value['count'] for value in manifests) == 145
    wrapper = '#!/bin/sh\nexport D1_ENV_FILE=' + str(CANDIDATE_ENV) + '\nexport D1_COMPOSE_OVERRIDE_FILE=' + str(CANDIDATE_OVERRIDE) + '\nexec ' + str(D1) + ' "$@"\n'
    active = ROOT / 'd1-active'
    active.write_text(wrapper)
    active.chmod(0o700)
    save(ROOT / 'rollout.json', {'passed': True, 'healthy': 17, 'manifests': manifests, 'catalog_capabilities': 145, 'schemas': SCHEMAS, 'old_business_facts_preserved': True, 'grants_unchanged_before_reconciliation': True})
    print(json.dumps({'phase': 'deployed', 'healthy': 17, 'catalog_capabilities': 145}), flush=True)

def apply():
    output = d1('upgrade')
    save(ROOT / 'private' / 'upgrade.log', output, True)
    verify_rollout()

def resume():
    output = d1('up')
    save(ROOT / 'private' / 'resume.log', output, True)
    verify_rollout()

if __name__ == '__main__':
    assert os.geteuid() == 0 and len(sys.argv) == 2
    {'publish': publish, 'prepare': prepare, 'apply': apply, 'resume': resume}[sys.argv[1]]()
