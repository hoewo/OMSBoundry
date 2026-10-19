#!/usr/bin/env python3
"""One real candidate /app/server at a time; not a Suite rollout."""
import importlib.util
import json
import os
from pathlib import Path
import secrets
import shlex
import subprocess
import time

ROOT = Path('/opt/omsbizfoundry/acceptance/material-candidate50-20261001')
WORK = ROOT / 'image-startup-a'
PG = 'production_material_source_auth58_20261001a_pg'
NETWORK = 'production_material_source_auth58_20261001a_isolated'
SOURCE = Path('/opt/omsbizfoundry/acceptance/production-material-source-20261001/production_material_source_auth58_20261001a')
READ_FIXTURE = Path('/opt/omsbizfoundry/acceptance/material-workload-20261001/material_workload_auth58_20261001c')
spec = importlib.util.spec_from_file_location('reads', READ_FIXTURE / 'verify-ecs.py')
reads = importlib.util.module_from_spec(spec); spec.loader.exec_module(reads)
run, inspect, save, snapshot = reads.run, reads.inspect, reads.save, reads.snapshot
sql, facts, request, wait_http = reads.sql, reads.facts, reads.request, reads.wait_http
TENANT = '11111111-1111-4111-8111-111111111111'
SPECS = {
    'project-workspace': ('44444444-4444-4444-8444-444444444444', '000009_results_cost_material_resource_links.sql', ['/api/v1/projects/aaaaaaaa-aaaa-4aaa-8aaa-000000000001/materials']),
    'production-operations': ('99999999-9999-4999-8999-999999999999', '000002_production_type_definitions.sql', ['/api/v1/references/production-work-items/aaaaaaaa-aaaa-4aaa-8aaa-000000000001/materials']),
    'approval-audit-results': ('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', '000001_approval_audit_results.sql', ['/api/v1/references/approval-audit-results/aaaaaaaa-aaaa-4aaa-8aaa-000000000001/materials']),
    'cost-outsourcing': ('cccccccc-cccc-4ccc-8ccc-cccccccccccc', '0002_payable_suggestions.sql', ['/api/v1/references/cost-entries/aaaaaaaa-aaaa-4aaa-8aaa-000000000001/materials', '/api/v1/references/outsourcing-engagements/aaaaaaaa-aaaa-4aaa-8aaa-000000000001/materials']),
}

def main():
    assert os.geteuid() == 0 and not WORK.exists()
    published = json.loads((ROOT / 'publication-b/results.json').read_text())
    assert published['passed'] and published['main_unchanged'] and set(published['images']) == set(SPECS)
    pg = inspect(PG)
    assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings'] and pg['HostConfig']['NanoCpus'] == 200000000
    assert pg['Config']['Labels']['oms.purpose'] == 'production-material-source'
    assert not any('restore20' in name for name in run(['docker', 'ps', '--format', '{{.Names}}']).splitlines())
    before = snapshot(); WORK.mkdir(mode=0o700)
    save(WORK / 'main-before.json', json.dumps(before, indent=2).encode())
    save(WORK / 'verifier.py', Path(__file__).read_bytes())
    fixture = dict(line.split('=', 1) for line in (SOURCE / 'config/fixture.env').read_text().splitlines())
    admin_url = shlex.split(fixture['TEST_DATABASE_URL'])[0]
    assert '@' + PG + ':5432/postgres?' in admin_url
    outcomes = {'passed': False, 'applications': [], 'full_suite_or_business_journey': False}
    names = []
    try:
        run(['docker', 'start', PG])
        deadline = time.monotonic() + 40
        while True:
            try: sql('postgres', 'SELECT 1'); break
            except Exception: assert time.monotonic() < deadline; time.sleep(1)
        for index, (app, (subject, schema, paths)) in enumerate(SPECS.items()):
            image = published['images'][app]['reference']
            folder = WORK / app; folder.mkdir(mode=0o700)
            data = folder / 'data'; data.mkdir(mode=0o700)
            save(data / 'jwks.json', (READ_FIXTURE / 'data/jwks.json').read_bytes())
            db = 'material_image50_' + str(index) + '_a'
            password = secrets.token_hex(24)
            sql('postgres', "CREATE ROLE " + db + "_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '" + password + "'; CREATE DATABASE " + db + ' OWNER ' + db + '_owner;')
            env = {'HTTP_ADDR': ':8080', 'DATABASE_URL': 'postgres://' + db + '_owner:' + password + '@' + PG + ':5432/' + db + '?sslmode=disable', 'MIGRATION_MODE': 'auto', 'IDENTITY_PROVIDER': 'iam', 'IAM_ISSUER': 'https://material-workload-fixture.example.test/auth', 'IAM_TENANT_ID': TENANT, 'IAM_PUBLIC_KEYS_PATH': '/probe/jwks.json', 'IAM_SERVICE_CREDENTIAL_URL': 'http://reference-unavailable.example.test:9/api/v1/service-credentials', 'WORKLOAD_ID': app, 'WORKLOAD_SUBJECT_ID': subject, 'WORKLOAD_KEY_FILE': '/probe/private.json', 'WORKLOAD_PUBLIC_KEY_FILE': '/probe/public.json', 'GOMAXPROCS': '1'}
            for neighbor in ('PROJECT', 'PEOPLE', 'PARTY', 'CONTRACT', 'EVIDENCE'):
                env[neighbor + '_BASE_URL'] = 'http://reference-unavailable.example.test:9'
            save(folder / 'fixture.env', ''.join(key + '=' + value + '\n' for key, value in env.items()).encode())
            run(['chown', '-R', '70:70', str(folder)])
            key_name = 'material_image50_' + str(index) + '_key_a'
            run(['docker', 'run', '--name', key_name, '--network', 'none', '--memory', '64m', '--cpus', '0.1', '--env-file', str(folder / 'fixture.env'), '--mount', 'type=bind,src=' + str(data) + ',dst=/probe', '--entrypoint', '/app/workload-keyctl', image, 'init'])
            assert not inspect(key_name)['State']['Running'] and inspect(key_name)['State']['ExitCode'] == 0
            name = 'material_image50_' + str(index) + '_server_a'; names.append(name)
            run(['docker', 'run', '--detach', '--name', name, '--network', NETWORK, '--memory', '160m', '--cpus', '0.2', '--pids-limit', '64', '--label', 'oms.purpose=candidate-minimal-startup', '--env-file', str(folder / 'fixture.env'), '--mount', 'type=bind,src=' + str(data) + ',dst=/probe,readonly', image])
            try:
                info = inspect(name)
                assert not info['HostConfig']['PortBindings'] and info['Config']['User'] == 'postgres' and set(info['NetworkSettings']['Networks']) == {NETWORK}
                base = 'http://' + info['NetworkSettings']['Networks'][NETWORK]['IPAddress'] + ':8080'
                wait_http(base)
                assert run(['docker', 'exec', name, '/app/ops', 'check']) == schema
                owned = facts(db)
                for path in paths: request(base, 'GET', path, expected=401)
                assert facts(db) == owned
                outcomes['applications'].append({'application': app, 'image': image, 'schema': schema, 'owned_canonical': owned, 'native_material_routes_anonymous_401': len(paths), 'actual_server_startup': True, 'workload_keyctl': True, 'ops_check': True})
                print(json.dumps({'check': 'one_actual_image_server_ready_schema_and_native_anonymous', 'application': app, 'schema': schema, 'material_routes': len(paths)}), flush=True)
            finally:
                if inspect(name)['State']['Running']: run(['docker', 'stop', '--time', '15', name])
                logs = subprocess.run(['docker', 'logs', name], capture_output=True, timeout=20)
                save(folder / 'server.log', logs.stdout + logs.stderr)
                assert not inspect(name)['State']['Running'] and not inspect(name)['State']['OOMKilled']
        assert len(outcomes['applications']) == 4 and snapshot() == before
        outcomes['passed'] = True
    finally:
        for name in names:
            if inspect(name)['State']['Running']: run(['docker', 'stop', '--time', '15', name])
        if inspect(PG)['State']['Running']: run(['docker', 'stop', '--time', '15', PG])
        assert not inspect(PG)['State']['Running'] and snapshot() == before
        outcomes['main_unchanged'] = True
        save(WORK / 'results.json', json.dumps(outcomes, indent=2).encode())
        print(json.dumps({'check': 'four_image_startups_stopped_main_unchanged', 'passed': outcomes['passed'], 'max_total_cpu': 0.4, 'memory_limits_mb': 352, 'restore20_running': 0}), flush=True)

if __name__ == '__main__':
    try: main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next: trace = trace.tb_next
        raise SystemExit('candidate startup failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics retained')
