#!/usr/bin/env python3
"""Candidate Contract binary + its own empty PG database; no Main rollout."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import secrets
import subprocess
import time

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-contract-native-startup-20261003c')
DB = 'oa_contract_native_startup_20261003c'
NAME = 'oa_contract_native_startup_20261003c_server'
PG = 'production_material_source_auth58_20261001a_pg'
NETWORK = 'production_material_source_auth58_20261001a_isolated'
RUNTIME = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66'
FIXTURE = Path('/opt/omsbizfoundry/acceptance/material-workload-20261001/material_workload_auth58_20261001c')
spec = importlib.util.spec_from_file_location('reads', FIXTURE / 'verify-ecs.py')
reads = importlib.util.module_from_spec(spec); spec.loader.exec_module(reads)
run, inspect, save, snapshot, sql, facts, request, wait_http = reads.run, reads.inspect, reads.save, reads.snapshot, reads.sql, reads.facts, reads.request, reads.wait_http


def main():
    assert os.geteuid() == 0 and ROOT.is_dir() and not (ROOT / 'results.json').exists()
    pg = inspect(PG)
    assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings']
    assert pg['Config']['Image'] == RUNTIME and pg['HostConfig']['NanoCpus'] == 200000000
    assert set(pg['NetworkSettings']['Networks']) == {NETWORK}
    assert json.loads(run(['docker', 'network', 'inspect', NETWORK]))[0]['Internal']
    assert not any('restore20' in name for name in run(['docker', 'ps', '--format', '{{.Names}}']).splitlines())
    binary = ROOT / 'contract-server'; ops = ROOT / 'contract-ops'; assert binary.is_file() and ops.is_file()
    before = snapshot(); save(ROOT / 'main-before.json', json.dumps(before, indent=2).encode())
    results = {'passed': False, 'main_unchanged': False, 'candidate_only': True, 'public_or_full_oa': False}
    started = False; stage = 'empty-database'
    try:
        run(['docker', 'start', PG]); deadline = time.monotonic() + 40
        while True:
            try: sql('postgres', 'SELECT 1'); break
            except Exception:
                assert time.monotonic() < deadline; time.sleep(1)
        assert sql('postgres', "SELECT count(*) FROM pg_database WHERE datname='" + DB + "'") == '0'
        password = secrets.token_hex(24)
        sql('postgres', 'CREATE ROLE ' + DB + "_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '" + password + "';")
        sql('postgres', 'CREATE DATABASE ' + DB + ' OWNER ' + DB + '_owner;')
        save(ROOT / 'jwks.json', (FIXTURE / 'data/jwks.json').read_bytes(), 0o444)
        env = {'HTTP_ADDR': ':8080', 'DATABASE_URL': 'postgres://' + DB + '_owner:' + password + '@' + PG + ':5432/' + DB + '?sslmode=disable',
               'MIGRATION_MODE': 'verify', 'IDENTITY_PROVIDER': 'iam', 'IAM_ISSUER': 'https://material-workload-fixture.example.test/auth',
               'IAM_TENANT_ID': '11111111-1111-4111-8111-111111111111', 'IAM_PUBLIC_KEYS_PATH': '/probe/jwks.json',
               'EVIDENCE_STORAGE_DIR': '/data/evidence', 'GOMAXPROCS': '1'}
        save(ROOT / 'fixture.env', ''.join(k + '=' + v + '\n' for k, v in env.items()).encode())
        run(['chmod', '0555', str(binary)])
        run(['chmod', '0555', str(ops)])
        # Follow the application's declared installation workflow: ops migrates
        # the empty database, server verifies schema. Dev auto has a 15s startup
        # budget and is not the formal low-resource installation mechanism.
        stage = 'explicit-install-migration'
        run(['docker','run','--name',NAME + '_migration','--network',NETWORK,'--cpus=.2','--memory=160m',
             '--memory-swap=160m','--pids-limit=64','--read-only','--user=70:70',
             '--tmpfs','/data/evidence:rw,noexec,nosuid,size=16777216,uid=70,gid=70','--env-file',str(ROOT / 'fixture.env'),
             '-v',str(ops)+':/probe/ops:ro','--entrypoint','/probe/ops',RUNTIME,'migrate'])
        assert not inspect(NAME + '_migration')['State']['Running'] and inspect(NAME + '_migration')['State']['ExitCode'] == 0
        stage = 'server-startup'
        run(['docker', 'run', '--detach', '--name', NAME, '--label', 'oms.purpose=oa-contract-native-startup', '--network', NETWORK,
             '--cpus=.2', '--memory=160m', '--memory-swap=160m', '--pids-limit=64', '--read-only', '--user=70:70',
             '--tmpfs', '/data/evidence:rw,noexec,nosuid,size=16777216,uid=70,gid=70',
             '--env-file', str(ROOT / 'fixture.env'), '-v', str(binary) + ':/probe/server:ro',
             '-v', str(ROOT / 'jwks.json') + ':/probe/jwks.json:ro', '--entrypoint', '/probe/server', RUNTIME])
        started = True; info = inspect(NAME)
        assert not info['HostConfig']['PortBindings'] and info['Config']['User'] == '70:70' and info['HostConfig']['ReadonlyRootfs']
        base = 'http://' + info['NetworkSettings']['Networks'][NETWORK]['IPAddress'] + ':8080'; wait_http(base)
        assert sql(DB, 'SELECT version::text FROM schema_migrations ORDER BY version DESC LIMIT 1') == '000019_party_runtime_transitions.sql'
        assert sql(DB, 'SELECT count(*) FROM contracts') == '0'
        original = facts(DB)
        stage = 'anonymous-denial'
        cid = '33333333-3333-4333-8333-333333333333'
        paths = [('GET', '/api/v1/contracts/' + cid + '/workspace'), ('POST', '/api/v1/contracts')]
        paths += [('POST', '/api/v1/contracts/' + cid + p) for p in ('', '/cancel', '/activate', '/changes', '/settlements', '/parties', '/parties/44444444-4444-4444-8444-444444444444/remove')]
        for method, path in paths:
            request(base, method, path, body={} if method == 'POST' else None, expected=401)
        assert facts(DB) == original
        stage = 'restart'
        run(['docker', 'restart', '--time', '10', NAME])
        base = 'http://' + inspect(NAME)['NetworkSettings']['Networks'][NETWORK]['IPAddress'] + ':8080'; wait_http(base)
        assert facts(DB) == original
        results.update(passed=True, actual_server_startup=True, schema=19, restart_facts_unchanged=True,
                       anonymous_native_routes_denied=len(paths), server_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(),
                       ops_sha256=hashlib.sha256(ops.read_bytes()).hexdigest(), explicit_ops_migration=True, migration_mode='verify',
                       standalone_with_declared_pg_and_iam_keys=True, legacy_evidence_compatibility_empty_tmpfs=True)
    except Exception as error:
        results['failure'] = {'stage': stage, 'type': type(error).__name__}
        raise
    finally:
        if started:
            if inspect(NAME)['State']['Running']: run(['docker', 'stop', '--time', '15', NAME])
            logs = subprocess.run(['docker', 'logs', NAME], capture_output=True, timeout=20)
            save(ROOT / 'server.log', logs.stdout + logs.stderr)
            assert not inspect(NAME)['State']['Running'] and not inspect(NAME)['State']['OOMKilled']
        if inspect(PG)['State']['Running']: run(['docker', 'stop', '--time', '15', PG])
        after = snapshot(); save(ROOT / 'main-after.json', json.dumps(after, indent=2).encode())
        results['main_unchanged'] = before == after; results['probe_and_pg_stopped'] = True
        results['passed'] = results['passed'] and results['main_unchanged']
        save(ROOT / 'results.json', json.dumps(results, indent=2).encode()); assert results['main_unchanged']
    assert results['passed']; print(json.dumps(results), flush=True)


if __name__ == '__main__':
    try: main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next: trace = trace.tb_next
        raise SystemExit('contract startup failed: ' + type(error).__name__ + ' line ' + str(trace.tb_lineno) + '; private diagnostics retained')
