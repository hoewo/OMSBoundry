#!/usr/bin/env python3
"""Independent BE native binary startup against an owned empty database.

Use existing IAM public/workload configuration, not a bypass provider. Anonymous
probes invoke no reference dependency and write zero facts. Main is unchanged.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import secrets
import subprocess
import time

RUN = os.environ.get('OA_BIDDING_STARTUP_RUN', 'h')
assert RUN in ('g','h','i')
ROOT = Path('/opt/omsbizfoundry/acceptance/oa-bidding-startup-20261002' + RUN)
PG = 'production_material_source_auth58_20261001a_pg'
NETWORK = 'production_material_source_auth58_20261001a_isolated'
RUNTIME = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66'
FIXTURE = Path('/opt/omsbizfoundry/acceptance/material-workload-20261001/material_workload_auth58_20261001c')
spec = importlib.util.spec_from_file_location('reads', FIXTURE / 'verify-ecs.py')
reads = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reads)
run, inspect, save, snapshot, sql, facts, request, wait_http = reads.run, reads.inspect, reads.save, reads.snapshot, reads.sql, reads.facts, reads.request, reads.wait_http
NAME = 'oa_bidding_startup_20261002' + RUN + '_server'
DB = 'oa_bidding_startup_20261002' + RUN


def main():
    assert os.geteuid() == 0 and ROOT.is_dir() and not (ROOT / 'results.json').exists()
    pg = inspect(PG)
    assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings']
    assert pg['Config']['Image'] == RUNTIME and pg['HostConfig']['NanoCpus'] == 200000000
    assert set(pg['NetworkSettings']['Networks']) == {NETWORK}
    assert json.loads(run(['docker', 'network', 'inspect', NETWORK]))[0]['Internal']
    assert not any('restore20' in name for name in run(['docker', 'ps', '--format', '{{.Names}}']).splitlines())
    binary = ROOT / 'business-server'
    assert binary.is_file()
    before = snapshot()
    save(ROOT / 'main-before.json', json.dumps(before).encode())
    result = {'passed': False, 'main_unchanged': False, 'full_suite_or_business_journey': False}
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
        source = inspect('omsbizfoundry-business-engagement-1')
        env = dict(item.split('=', 1) for item in source['Config']['Env'])
        assert env['IDENTITY_PROVIDER'] == 'iam'
        env.update(HTTP_ADDR=':8080', MIGRATION_MODE='auto', GOMAXPROCS='1', DATABASE_URL='postgres://' + DB + '_owner:' + password + '@' + PG + ':5432/' + DB + '?sslmode=disable')
        save(ROOT / 'fixture.env', ''.join(key + '=' + value + '\n' for key, value in env.items()).encode())
        run(['chmod', '0555', str(binary)])
        args = ['docker', 'run', '--detach', '--name', NAME, '--label', 'oms.purpose=oa-bidding-startup', '--network', NETWORK,
                '--cpus=.2', '--memory=160m', '--memory-swap=160m', '--pids-limit=64', '--read-only', '--user=70:70',
                '--env-file', str(ROOT / 'fixture.env'), '-v', str(binary) + ':/probe/server:ro']
        for mount in source['Mounts']:
            if mount['Destination'] not in ('/public','/workload-private'):
                continue  # no Main backup directory or runtime data volume
            assert mount['Type'] in ('bind','volume') and not mount['RW']
            args += ['--mount', 'type=bind,src=' + mount['Source'] + ',dst=' + mount['Destination'] + ',readonly']
        assert {m['Destination'] for m in source['Mounts']} >= {'/public','/workload-private'}
        run(args + ['--entrypoint', '/probe/server', RUNTIME])
        started = True
        info = inspect(NAME)
        assert not info['HostConfig']['PortBindings'] and info['HostConfig']['ReadonlyRootfs']
        base = 'http://' + info['NetworkSettings']['Networks'][NETWORK]['IPAddress'] + ':8080'
        wait_http(base)
        schema = sql(DB, 'SELECT version::text FROM schema_migrations ORDER BY version DESC LIMIT 1')
        assert schema == ('000002_material_collections.sql' if RUN == 'i' else '000001_business_engagement.sql')
        assert sql(DB, 'SELECT count(*) FROM business_engagements') == '0'
        original = facts(DB)
        paths = [('GET', '/api/v1/business-engagements'), ('GET', '/api/v1/business-engagements?project_id=33333333-3333-4333-8333-333333333333&limit=2'),
                 ('POST', '/api/v1/business-engagements'), ('POST', '/api/v1/business-engagements/33333333-3333-4333-8333-333333333333/method'),
                 ('POST', '/api/v1/business-engagements/33333333-3333-4333-8333-333333333333/applicability')]
        if RUN == 'i':
            paths += [('POST', '/api/v1/business-engagements/33333333-3333-4333-8333-333333333333/materials'),
                      ('GET', '/api/v1/references/business-engagements/33333333-3333-4333-8333-333333333333/materials')]
        for method, path in paths:
            request(base, method, path, body={} if method == 'POST' else None, expected=401)
        assert facts(DB) == original
        run(['docker', 'restart', '--time', '10', NAME])
        base = 'http://' + inspect(NAME)['NetworkSettings']['Networks'][NETWORK]['IPAddress'] + ':8080'
        wait_http(base)
        assert facts(DB) == original
        result.update(passed=True, actual_server_startup=True, identity_provider='iam', schema=schema, anonymous_native_routes_denied=len(paths),
                      restart_facts_unchanged=True, server_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(), private_owned_database=DB,
                      max_total_cpu=.4, max_total_memory_mib=352)
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
        save(ROOT / 'main-after.json', json.dumps(after).encode())
        result['main_unchanged'] = before == after
        result['passed'] = result['passed'] and result['main_unchanged']
        save(ROOT / 'results.json', json.dumps(result).encode())
    assert result['passed']
    print(json.dumps(result), flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('bidding startup failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics retained')
