#!/usr/bin/env python3
"""One new Project binary, its own empty DB, no Suite or Main rollout."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import secrets
import subprocess
import time

RUN = os.environ.get('OA_REPAIR_RUN', 'd')
assert len(RUN) == 1 and RUN in 'abcdefghijklmnopqrstuvwxyz'
SCHEMA = int(os.environ.get('OA_REPAIR_SCHEMA', '10'))
assert SCHEMA in (10, 11)
ROOT = Path('/opt/omsbizfoundry/acceptance/oa-initiation-repair-20261002' + RUN)
PG = 'production_material_source_auth58_20261001a_pg'
NETWORK = 'production_material_source_auth58_20261001a_isolated'
RUNTIME = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66'
FIXTURE = Path('/opt/omsbizfoundry/acceptance/material-workload-20261001/material_workload_auth58_20261001c')
spec = importlib.util.spec_from_file_location('reads', FIXTURE / 'verify-ecs.py')
reads = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reads)
run, inspect, save, snapshot, sql, facts, request, wait_http = reads.run, reads.inspect, reads.save, reads.snapshot, reads.sql, reads.facts, reads.request, reads.wait_http
NAME = 'oa_initiation_repair_20261002' + RUN + '_server'
DB = 'oa_initiation_startup_20261002' + RUN


def main():
    assert os.geteuid() == 0 and ROOT.is_dir() and not (ROOT / 'results.json').exists()
    pg = inspect(PG)
    assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings']
    assert pg['Config']['Image'] == RUNTIME and pg['Config']['Labels']['oms.purpose'] == 'production-material-source'
    assert pg['HostConfig']['NanoCpus'] == 200000000 and set(pg['NetworkSettings']['Networks']) == {NETWORK}
    assert json.loads(run(['docker', 'network', 'inspect', NETWORK]))[0]['Internal']
    assert not any('restore20' in name for name in run(['docker', 'ps', '--format', '{{.Names}}']).splitlines())
    assert (ROOT / 'project-server').is_file()
    before = snapshot()
    save(ROOT / 'main-before.json', json.dumps(before, indent=2).encode())
    results = {'passed': False, 'main_unchanged': False, 'full_suite_or_business_journey': False}
    started = False
    try:
        run(['docker', 'start', PG])
        deadline = time.monotonic() + 40
        while True:
            try:
                sql('postgres', 'SELECT 1')
                break
            except Exception:
                assert time.monotonic() < deadline
                time.sleep(1)
        assert sql('postgres', "SELECT count(*) FROM pg_database WHERE datname='" + DB + "'") == '0'
        password = secrets.token_hex(24)
        sql('postgres', 'CREATE ROLE ' + DB + "_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '" + password + "';")
        sql('postgres', 'CREATE DATABASE ' + DB + ' OWNER ' + DB + '_owner;')
        save(ROOT / 'jwks.json', (FIXTURE / 'data/jwks.json').read_bytes(), 0o444)
        env = {'HTTP_ADDR': ':8080', 'DATABASE_URL': 'postgres://' + DB + '_owner:' + password + '@' + PG + ':5432/' + DB + '?sslmode=disable',
               'MIGRATION_MODE': 'auto', 'IDENTITY_PROVIDER': 'iam', 'IAM_ISSUER': 'https://material-workload-fixture.example.test/auth',
               'IAM_TENANT_ID': '11111111-1111-4111-8111-111111111111', 'IAM_PUBLIC_KEYS_PATH': '/probe/jwks.json', 'GOMAXPROCS': '1'}
        save(ROOT / 'fixture.env', ''.join(key + '=' + value + '\n' for key, value in env.items()).encode())
        run(['chmod', '0555', str(ROOT / 'project-server')])
        run(['docker', 'run', '--detach', '--name', NAME, '--label', 'oms.purpose=oa-initiation-startup', '--network', NETWORK,
             '--cpus=.2', '--memory=160m', '--memory-swap=160m', '--pids-limit=64', '--read-only', '--user=70:70',
             '--env-file', str(ROOT / 'fixture.env'), '-v', str(ROOT / 'project-server') + ':/probe/server:ro',
             '-v', str(ROOT / 'jwks.json') + ':/probe/jwks.json:ro', '--entrypoint', '/probe/server', RUNTIME])
        started = True
        info = inspect(NAME)
        assert not info['HostConfig']['PortBindings'] and info['Config']['User'] == '70:70' and info['HostConfig']['ReadonlyRootfs']
        base = 'http://' + info['NetworkSettings']['Networks'][NETWORK]['IPAddress'] + ':8080'
        wait_http(base)
        schema = sql(DB, 'SELECT version::text FROM schema_migrations ORDER BY version DESC LIMIT 1')
        assert schema == {10: '000010_initial_dispatch_draft.sql', 11: '000011_commercial_contributors.sql'}[SCHEMA]
        assert sql(DB, 'SELECT count(*) FROM projects') == '0'
        original = facts(DB)
        request(base, 'POST', '/api/v1/initiation-dispatches', body={}, expected=401)
        request(base, 'POST', '/api/v1/projects/33333333-3333-4333-8333-333333333333/profile-corrections', body={}, expected=401)
        request(base, 'POST', '/api/v1/projects/33333333-3333-4333-8333-333333333333/basic-info-changes', body={}, expected=401)
        request(base, 'POST', '/api/v1/projects/33333333-3333-4333-8333-333333333333/initiation-info-changes', body={}, expected=401)
        request(base, 'GET', '/api/v1/projects/33333333-3333-4333-8333-333333333333/counterparty-context', expected=401)
        request(base, 'POST', '/api/v1/projects/33333333-3333-4333-8333-333333333333/counterparty-context', body={}, expected=401)
        request(base, 'GET', '/api/v1/projects', expected=401)
        request(base, 'GET', '/api/v1/project-directory', expected=401)
        request(base, 'GET', '/api/v1/counterparty-context/name-suggestions?q=test', expected=401)
        request(base, 'POST', '/api/v1/projects/33333333-3333-4333-8333-333333333333/assignments', body={}, expected=401)
        assert facts(DB) == original
        run(['docker', 'restart', '--time', '10', NAME])
        base = 'http://' + inspect(NAME)['NetworkSettings']['Networks'][NETWORK]['IPAddress'] + ':8080'
        wait_http(base)
        assert facts(DB) == original
        results.update({'passed': True, 'schema': SCHEMA, 'actual_server_startup': True, 'restart_facts_unchanged': True,
                        'anonymous_native_routes_denied': 10, 'server_sha256': hashlib.sha256((ROOT / 'project-server').read_bytes()).hexdigest(),
                        'private_owned_database': DB, 'max_total_cpu': 0.4, 'max_total_memory_mib': 352})
    finally:
        if started:
            if inspect(NAME)['State']['Running']:
                run(['docker', 'stop', '--time', '15', NAME])
            logs = subprocess.run(['docker', 'logs', NAME], capture_output=True, timeout=20)
            save(ROOT / 'server.log', logs.stdout + logs.stderr)
            assert not inspect(NAME)['State']['Running'] and not inspect(NAME)['State']['OOMKilled']
        if inspect(PG)['State']['Running']:
            run(['docker', 'stop', '--time', '15', PG])
        after = snapshot()
        save(ROOT / 'main-after.json', json.dumps(after, indent=2).encode())
        results['main_unchanged'] = before == after
        results['passed'] = results['passed'] and results['main_unchanged']
        save(ROOT / 'results.json', json.dumps(results, indent=2).encode())
        assert results['main_unchanged']
    assert results['passed']
    print(json.dumps(results), flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('startup failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics retained')
