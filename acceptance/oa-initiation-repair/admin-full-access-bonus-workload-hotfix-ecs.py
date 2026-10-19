#!/usr/bin/env python3
"""Install the current workload policy so Bonus v3 can read native candidates."""
import importlib.util
import json
import os
from pathlib import Path


ROOT = Path('/opt/omsbizfoundry/acceptance/oa-admin-full-access-20261005a')
ENV = ROOT / 'candidate.env'
OVERRIDE = ROOT / 'compose.oa-public.json'
POLICY = ROOT / 'contracts/workload-identity-policy-admin-full-access-v1.json'
STAGED_POLICY = Path('/tmp/workload-identity-policy-admin-full-access-v1.json')
D1 = Path('/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.20_linux_amd64/deployments/d1/ops/d1')

spec = importlib.util.spec_from_file_location('admin_release', ROOT / 'admin-full-access-release-ecs.py')
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
run, b = release.run, release.b


def save(path, value, private=False):
    data = value if isinstance(value, bytes) else (json.dumps(value, indent=2, sort_keys=True) + '\n').encode()
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.write_bytes(data)
    path.chmod(0o600 if private else 0o644)


def business_facts():
    return {database: b.facts(database) for database in b.DBS if database != 'identity_access'}


def grant_digest():
    return b.sql(
        'identity_access',
        "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t",
    )


def load_and_validate_policy(path):
    value = json.loads(path.read_text())
    assert value['schema_version'] == '1'
    identities = value['identities']
    assert len({item['id'] for item in identities}) == len(identities)
    bonus = next(item for item in identities if item['id'] == 'bonus-distribution')
    permissions = {(item['audience'], item['action'], tuple(item['resource_types'])) for item in bonus['permissions']}
    assert ('project-workspace', 'project-workspace.read', ('project',)) in permissions
    assert ('project-workspace', 'project-workspace.reference-oa-personnel.read', ('project',)) in permissions
    return value


def prepare():
    value = load_and_validate_policy(STAGED_POLICY)
    save(POLICY, value)
    override = json.loads(OVERRIDE.read_text())
    mount = str(POLICY) + ':/config/workload-identity-policy.json:ro'
    service = override['services']['identity-access']
    volumes = [item for item in service.setdefault('volumes', []) if not item.endswith(':/config/workload-identity-policy.json:ro')]
    volumes.append(mount)
    service['volumes'] = volumes
    backup = ROOT / 'compose.before-bonus-workload-hotfix.json'
    if not backup.exists():
        save(backup, OVERRIDE.read_bytes(), True)
    save(OVERRIDE, override)


def verify_identity_mount():
    inspected = json.loads(run(['docker', 'inspect', 'omsbizfoundry-identity-access-1']))[0]
    mounts = [item for item in inspected['Mounts'] if item['Destination'] == '/config/workload-identity-policy.json']
    assert len(mounts) == 1 and mounts[0]['Source'] == str(POLICY)
    inside = json.loads(run(['docker', 'exec', 'omsbizfoundry-identity-access-1', 'cat', '/config/workload-identity-policy.json']))
    bonus = next(item for item in inside['identities'] if item['id'] == 'bonus-distribution')
    assert any(
        item['audience'] == 'project-workspace'
        and item['action'] == 'project-workspace.reference-oa-personnel.read'
        and item['resource_types'] == ['project']
        for item in bonus['permissions']
    )


def main():
    assert os.geteuid() == 0
    before = business_facts()
    grants = grant_digest()
    save(ROOT / 'private/bonus-workload-facts-before.json', before, True)
    prepare()
    output = run(
        ['env', 'D1_ENV_FILE=' + str(ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(OVERRIDE), str(D1), 'up'],
        timeout=1800,
    )
    save(ROOT / 'private/bonus-workload-up.log', output, True)
    assert business_facts() == before
    assert grant_digest() == grants
    rows = [json.loads(line) for line in run([
        'docker', 'ps', '--filter', 'name=^/omsbizfoundry-', '--format', '{{json .}}',
    ]).decode().splitlines()]
    assert len(rows) == 17
    assert all(row.get('State') == 'running' and 'healthy' in row.get('Status', '') for row in rows)
    verify_identity_mount()
    result = {
        'passed': True,
        'healthy': 17,
        'business_facts_unchanged': True,
        'grants_unchanged': True,
        'policy': str(POLICY),
        'bonus_professional_candidate_permission': True,
    }
    save(ROOT / 'bonus-workload-hotfix-rollout.json', result)
    print(json.dumps(result))


if __name__ == '__main__':
    main()
