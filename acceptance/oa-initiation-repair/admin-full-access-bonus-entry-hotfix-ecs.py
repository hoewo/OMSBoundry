#!/usr/bin/env python3
"""Install the Bonus v3 Gateway admission manifest without changing grants or facts."""
import importlib.util
import json
import os
from pathlib import Path


ROOT = Path('/opt/omsbizfoundry/acceptance/oa-admin-full-access-20261005a')
ENV = ROOT / 'candidate.env'
OVERRIDE = ROOT / 'compose.oa-public.json'
RELEASE = Path('/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.20_linux_amd64')
D1 = RELEASE / 'deployments/d1/ops/d1'
RUNTIME_MANIFEST = RELEASE / 'deployments/d1/.runtime/suite-manifest.json'
MANIFEST = ROOT / 'contracts/suite-manifest-admin-full-access-v1.json'

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


def prepare_manifest():
    value = json.loads(RUNTIME_MANIFEST.read_text())
    bonus = next(item for item in value['applications'] if item['id'] == 'bonus-distribution')
    assert bonus['enabled'] is True
    assert bonus['audience'] == 'bonus-distribution'
    assert bonus['route_prefix'] == '/bonus'
    for key in ('capability_namespace', 'entry_action', 'capability_namespaces', 'entry_actions'):
        bonus.pop(key, None)
    bonus['capability_namespaces'] = ['bonus-pool', 'bonus-manual-allocation']
    bonus['entry_actions'] = ['bonus-pool.read', 'bonus-manual-allocation.read']
    save(MANIFEST, value)

    override = json.loads(OVERRIDE.read_text())
    mount = str(MANIFEST) + ':/config/suite-manifest.json:ro'
    for service in ('gateway', 'unified-workbench'):
        service_value = override['services'].setdefault(service, {})
        volumes = service_value.setdefault('volumes', [])
        volumes = [item for item in volumes if not item.endswith(':/config/suite-manifest.json:ro')]
        volumes.append(mount)
        service_value['volumes'] = volumes
    backup = ROOT / 'compose.before-bonus-entry-hotfix.json'
    if not backup.exists():
        save(backup, OVERRIDE.read_bytes(), True)
    save(OVERRIDE, override)


def verify_mount(container):
    inspected = json.loads(run(['docker', 'inspect', container]))[0]
    mounts = [item for item in inspected['Mounts'] if item['Destination'] == '/config/suite-manifest.json']
    assert len(mounts) == 1
    assert mounts[0]['Source'] == str(MANIFEST)
    inside = json.loads(run(['docker', 'exec', container, 'cat', '/config/suite-manifest.json']))
    bonus = next(item for item in inside['applications'] if item['id'] == 'bonus-distribution')
    assert bonus['capability_namespaces'] == ['bonus-pool', 'bonus-manual-allocation']
    assert bonus['entry_actions'] == ['bonus-pool.read', 'bonus-manual-allocation.read']
    assert 'capability_namespace' not in bonus and 'entry_action' not in bonus


def main():
    assert os.geteuid() == 0
    before = business_facts()
    grants = grant_digest()
    save(ROOT / 'private/bonus-entry-facts-before.json', before, True)
    prepare_manifest()
    output = run(
        ['env', 'D1_ENV_FILE=' + str(ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(OVERRIDE), str(D1), 'up'],
        timeout=1800,
    )
    save(ROOT / 'private/bonus-entry-up.log', output, True)
    assert business_facts() == before
    assert grant_digest() == grants
    rows = [json.loads(line) for line in run([
        'docker', 'ps', '--filter', 'name=^/omsbizfoundry-', '--format', '{{json .}}',
    ]).decode().splitlines()]
    assert len(rows) == 17
    assert all(row.get('State') == 'running' and 'healthy' in row.get('Status', '') for row in rows)
    verify_mount('omsbizfoundry-gateway-1')
    verify_mount('omsbizfoundry-unified-workbench-1')
    result = {
        'passed': True,
        'healthy': 17,
        'business_facts_unchanged': True,
        'grants_unchanged': True,
        'manifest': str(MANIFEST),
        'bonus_capability_namespaces': ['bonus-pool', 'bonus-manual-allocation'],
        'bonus_entry_actions': ['bonus-pool.read', 'bonus-manual-allocation.read'],
    }
    save(ROOT / 'bonus-entry-hotfix-rollout.json', result)
    print(json.dumps(result))


if __name__ == '__main__':
    main()
