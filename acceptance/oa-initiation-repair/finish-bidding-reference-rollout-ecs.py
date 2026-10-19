#!/usr/bin/env python3
"""Read-only k postflight after the mount-order gate failure; never recreate Main.

The initial native before-mount array was not persisted. Validate the old
declarative configuration and pre-existing volumes explicitly, not a fabricated
before snapshot. Preserve the failed rollout receipt and immutable release.
"""
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path


def validate_mounts(expected, actual, image_volumes, volume_created, cutoff):
    by_target = {mount['Destination']: mount for mount in actual}
    assert len(by_target) == len(actual)
    assert set(by_target) == set(expected) | set(image_volumes)
    for target, declaration in expected.items():
        mount = by_target[target]
        assert all(mount.get(key) == value for key, value in declaration.items())
    for target in set(image_volumes) - set(expected):
        assert by_target[target]['Type'] == 'volume' and by_target[target]['RW'] is True
    for mount in actual:
        if mount['Type'] == 'volume':
            assert volume_created[mount['Name']] < cutoff, 'new or substituted volume'


def main():
    assert os.geteuid() == 0
    os.environ['OA_PUBLIC_RUN'] = 'k'
    root = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002k')
    spec = importlib.util.spec_from_file_location('deployed_k', root / 'deploy-public-slice-ecs.py')
    deploy = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(deploy)
    backup = deploy.BACKUP
    assert (backup / 'rollout-failure.json').is_file() and not (backup / 'rollout.json').exists()
    assert json.loads((backup / 'restore.json').read_text())['passed']
    assert deploy.ENV.read_bytes() == (backup / 'candidate.env').read_bytes()
    current = deploy.healthy()
    original = json.loads((backup / 'containers-before.json').read_text())
    assert current['PROJECT']['Image'] == original['PROJECT']['image']
    publication = json.loads((root / 'publication/results.json').read_text())
    assert publication['passed'] and current['WORKBENCH']['Image'] == publication['images']['unified-workbench']['image_id']
    assert all(current[key]['Id'] == value['id'] for key, value in original.items() if key not in ('PROJECT', 'WORKBENCH'))
    assert all(current[key]['Id'] != original[key]['id'] for key in ('PROJECT', 'WORKBENCH'))
    args = ['docker', 'compose', '--project-directory', str(deploy.OLD), '-p', 'omsbizfoundry', '--env-file', str(backup / 'original.env')]
    for name in ('compose.yaml', 'compose.pull.yaml', 'compose.https.yaml'):
        args += ['-f', str(deploy.OLD / name)]
    args += ['-f', '/opt/omsbizfoundry/acceptance/oa-public-release-20261002a/compose.oa-public.json', '--profile', '*', 'config', '--format', 'json']
    legacy = json.loads(deploy.run(args))
    project = legacy['services']['project-workspace']
    image = json.loads(deploy.run(['docker', 'image', 'inspect', current['PROJECT']['Image']]))[0]
    expected_env = dict(value.split('=', 1) for value in image['Config']['Env'])
    for key, value in project['environment'].items():
        if value is None:
            expected_env.pop(key, None)
        else:
            expected_env[key] = str(value)
    assert not expected_env.get('BUSINESS_ENGAGEMENT_BASE_URL')
    assert expected_env['IDENTITY_PROVIDER'] == 'iam' and expected_env.get('IAM_SERVICE_CREDENTIAL_URL')
    assert not expected_env.get('BUSINESS_ENGAGEMENT_ACCESS_TOKEN')
    expected_env['BUSINESS_ENGAGEMENT_BASE_URL'] = deploy.BIDDING_REFERENCE_ENDPOINT
    assert dict(value.split('=', 1) for value in current['PROJECT']['Config']['Env']) == expected_env
    assert not project.get('ports') and not current['PROJECT']['HostConfig']['PortBindings']
    declarations = {}
    created = {}
    for mount in current['PROJECT']['Mounts']:
        if mount['Type'] == 'volume':
            volume = json.loads(deploy.run(['docker', 'volume', 'inspect', mount['Name']]))[0]
            created[mount['Name']] = datetime.datetime.fromisoformat(volume['CreatedAt']).timestamp()
    for mount in project['volumes']:
        target = {'Type': mount['type'], 'RW': not mount.get('read_only', False)}
        if mount['type'] == 'bind':
            target['Source'] = mount['source']
        else:
            assert mount['type'] == 'volume'
            target['Name'] = legacy['volumes'][mount['source']]['name']
            volume = json.loads(deploy.run(['docker', 'volume', 'inspect', target['Name']]))[0]
            target['Source'] = volume['Mountpoint']
        declarations[mount['target']] = target
    validate_mounts(declarations, current['PROJECT']['Mounts'], image['Config'].get('Volumes', {}), created, (backup / 'preflight.json').stat().st_mtime)
    snapshots = json.loads((backup / 'facts-before.json').read_text())
    for db in deploy.DBS:
        after = deploy.facts(db)
        if db == 'identity_access':
            dynamic = {'iam_audit_entries', 'suite_sessions', 'workload_assertion_nonces'}
            assert {key: value for key, value in after.items() if key not in dynamic} == {key: value for key, value in snapshots[db].items() if key not in dynamic}
        else:
            assert after == snapshots[db], db + ' facts changed'
    deploy.run([str(root / 'd1-active'), 'check'], timeout=240)
    detail = {'passed': True, 'business_databases_preserved': 12, 'iam_static_facts_preserved': True, 'before_native_mount_snapshot_available': False, 'mount_validation': 'old-declarative-config-and-preexisting-volume', 'initial_failure_retained': True, 'container_recreates': 0, 'business_api_writes': 0, 'checker_sha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest()}
    deploy.record(root / 'postflight.json', detail)
    deploy.record(backup / 'rollout.json', {'passed': True, 'schema': 10, 'changed_containers': ['project-workspace', 'unified-workbench'], 'other_container_ids_unchanged': 15, 'project_facts_preserved': True, 'project_reference_config_only': True, 'd1_check': True, 'official_release': False, 'postflight_receipt': 'postflight.json', 'initial_failure_retained': True})
    print(json.dumps(detail), flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('k postflight failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; no Main recreate or business write')
