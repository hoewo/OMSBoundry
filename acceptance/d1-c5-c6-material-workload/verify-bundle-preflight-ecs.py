#!/usr/bin/env python3
"""Parse the candidate D1 bundle without starting a Suite or touching Main."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tarfile

ROOT = Path('/opt/omsbizfoundry/acceptance/material-candidate50-20261001')
WORK = ROOT / 'bundle-preflight-a'
ARCHIVE = ROOT / 'omsbizfoundry-d1_v0.1.0-test.50_linux_amd64.tar.gz'
ARCHIVE_SHA = 'c1839328f01a5d068e78acb7352cbd2c6721684432fda327786a4cc1ba910b72'
BUNDLE_NAME = 'omsbizfoundry-d1_v0.1.0-test.50_linux_amd64'
CONTAINER = 'material_bundle50_iam_preflight_a'
spec = importlib.util.spec_from_file_location('entry_checks', '/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/verify-entry-capabilities.py')
entry = importlib.util.module_from_spec(spec)
spec.loader.exec_module(entry)
run, inspect, save, snapshot = entry.run, entry.inspect, entry.save, entry.snapshot


def main():
    assert os.geteuid() == 0 and not WORK.exists()
    assert CONTAINER not in run(['docker', 'ps', '-a', '--format', '{{.Names}}']).splitlines()
    assert not any('restore20' in name for name in run(['docker', 'ps', '--format', '{{.Names}}']).splitlines())
    assert not inspect('production_material_source_auth58_20261001a_pg')['State']['Running']
    assert hashlib.sha256(ARCHIVE.read_bytes()).hexdigest() == ARCHIVE_SHA
    before = snapshot()
    WORK.mkdir(mode=0o700)
    save(WORK / 'main-before.json', json.dumps(before, indent=2).encode())
    save(WORK / 'driver.py', Path(__file__).read_bytes())
    results = {'passed': False, 'full_d1_runtime': False, 'official_release': False, 'archive_sha256': ARCHIVE_SHA}
    created = False
    try:
        extracted = WORK / 'extracted'
        extracted.mkdir(mode=0o755)
        with tarfile.open(ARCHIVE) as archive:
            for member in archive.getmembers():
                path = Path(member.name)
                assert path.parts[0] == BUNDLE_NAME and not path.is_absolute() and '..' not in path.parts
                assert member.isfile() or member.isdir()
                assert not path.name.endswith(('.key', '.pem')) and path.name not in ('.env', '.runtime')
            archive.extractall(extracted)
        bundle = extracted / BUNDLE_NAME
        for directory in (bundle, *[path for path in bundle.rglob('*') if path.is_dir()]):
            directory.chmod(0o755)
        verified = subprocess.run(['sha256sum', '-c', 'RELEASE-CHECKSUMS.sha256'], cwd=bundle, capture_output=True, text=True, timeout=30)
        save(WORK / 'inner-checksums.log', verified.stdout.encode())
        assert verified.returncode == 0
        metadata = dict(line.split('=', 1) for line in (bundle / 'RELEASE-METADATA').read_text().splitlines())
        assert metadata['commit'] == 'unknown' and metadata['target'] == 'linux/amd64'
        d1 = bundle / 'deployments/d1'
        example = (d1 / '.env.example').read_text()
        original = dict(line.split('=', 1) for line in example.splitlines() if line and not line.startswith('#') and '=' in line)
        images = {key: value for key, value in original.items() if key.startswith('D1_') and key.endswith('_IMAGE')}
        assert len(images) == 17 and all('@sha256:' in image for image in images.values())
        published = json.loads((ROOT / 'results-publication50.json').read_text())
        mapping = {'project-workspace': 'PROJECT', 'production-operations': 'PRODUCTION', 'approval-audit-results': 'APPROVAL_AUDIT_RESULTS', 'cost-outsourcing': 'COST_OUTSOURCING'}
        for app, key in mapping.items():
            assert images['D1_' + key + '_IMAGE'] == published['images'][app]['reference']
        values = dict(original)
        for key in values:
            if key.endswith('_PASSWORD'):
                values[key] = 'Fixture_' + hashlib.sha256(key.encode()).hexdigest()[:24]
            elif key.startswith('ENABLE_'):
                values[key] = 'true'
        values.update({'PARTY_CURSOR_HMAC_KEY': 'Fixture_Cursor_Key_0123456789012345678901234', 'EVIDENCE_LOCAL_SIGNING_KEY': 'YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE', 'PROJECT_PRODUCTION_BASE_URL': 'http://production-operations:8087', 'PROJECT_COST_BASE_URL': 'http://cost-outsourcing:8093'})
        fixture_env = WORK / 'fixture.env'
        save(fixture_env, ('\n'.join(key + '=' + value for key, value in values.items()) + '\n').encode())
        command = ['env', 'D1_ENV_FILE=' + str(fixture_env), 'D1_PROJECT_NAME=material_bundle50_a', str(d1 / 'ops/d1')]
        configuration = json.loads(run(command + ['config', '--format', 'json']))
        save(WORK / 'config-fixture.json', json.dumps(configuration, indent=2).encode())
        resolved_images = set(run(command + ['images']).splitlines())
        assert resolved_images == set(images.values())
        project = configuration['services']['project-workspace']
        assert project['environment']['PRODUCTION_BASE_URL'] == values['PROJECT_PRODUCTION_BASE_URL']
        assert project['environment']['COST_BASE_URL'] == values['PROJECT_COST_BASE_URL']
        assert all(not service.get('ports') for name, service in configuration['services'].items() if name != 'nginx')
        run(['env', *[key + '=' + value for key, value in values.items() if key.startswith('ENABLE_') or key == 'TENANT_ID'], str(d1 / 'ops/render-suite-manifest'), str(WORK / 'suite-manifest.json')])
        manifest = json.loads((WORK / 'suite-manifest.json').read_text())
        assert all(app['enabled'] for app in manifest['applications'])
        evidence = next(app for app in manifest['applications'] if app['id'] == 'evidence-service')
        assert evidence['entry_actions'] == ['evidence-service.view', 'evidence-service.metadata.read', 'evidence-service.upload', 'evidence-service.download']
        assert not evidence['show_in_workbench']
        contracts = bundle / 'contracts/access/v1/examples'
        capabilities = sorted(path.name for path in contracts.glob('*-capabilities.json'))
        assert len(capabilities) == 12
        policy_names = ('project-assignment-policy.json', 'business-engagement-project-assignment-policy.json', 'production-operations-project-assignment-policy.json')
        environment = {'DATABASE_URL': 'postgres://fixture:fixture@unavailable.invalid/fixture?sslmode=disable', 'TENANT_ID': values['TENANT_ID'], 'TENANT_NAME': 'Isolated candidate parser preflight', 'PUBLIC_ISSUER': 'https://fixture.invalid/auth', 'CAPABILITY_MANIFEST_PATHS': ','.join('/contracts/' + name for name in capabilities), 'PROJECT_POLICY_BASE_URL': 'http://unavailable-project.invalid', 'PEOPLE_POLICY_BASE_URL': 'http://unavailable-people.invalid', 'POLICY_SERVICE_SUBJECT_ID': values['POLICY_SERVICE_SUBJECT_ID'], 'PROJECT_ASSIGNMENT_POLICY_PATHS': ','.join('/contracts/' + name for name in policy_names), 'WORKLOAD_IDENTITY_POLICY_PATH': '/contracts/workload-identity-policy.json'}
        args = ['docker', 'create', '--name', CONTAINER, '--network', 'none', '--read-only', '--memory', '96m', '--cpus', '0.15', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges', '--label', 'oms.purpose=candidate-parser-preflight', '--mount', 'type=bind,src=' + str(contracts) + ',dst=/contracts,readonly']
        for key, value in environment.items():
            args += ['--env', key + '=' + value]
        run(args + [images['D1_IDENTITY_IMAGE'], '/app/ops', 'validate-contracts'])
        created = True
        output = run(['docker', 'start', '--attach', CONTAINER])
        save(WORK / 'iam-validate.log', output.encode())
        state = inspect(CONTAINER)['State']
        assert not state['Running'] and state['ExitCode'] == 0 and not state['OOMKilled']
        assert json.loads(output) == {'service': 'identity-access', 'status': 'contracts-valid'}
        assert snapshot() == before
        results.update({'passed': True, 'immutable_images': len(resolved_images), 'enabled_applications': len(manifest['applications']), 'capability_manifests': len(capabilities), 'assignment_policies': len(policy_names), 'workload_policy': True, 'iam8_parsers': True, 'evidence_native_entry_actions': True, 'configured_neighbor_endpoints': True, 'no_public_runtime': True})
    finally:
        if created and inspect(CONTAINER)['State']['Running']:
            run(['docker', 'stop', '--time', '5', CONTAINER])
        assert snapshot() == before
        results['main_unchanged'] = True
        save(WORK / 'results.json', json.dumps(results, indent=2).encode())
        print(json.dumps(results), flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('candidate bundle preflight failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics retained')
