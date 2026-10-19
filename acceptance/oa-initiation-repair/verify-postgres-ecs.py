#!/usr/bin/env python3
"""Sequential Project tests in the existing private PG18, never the Main DB.

Each Go integration test creates and cleans up its own randomly named test DB.
Keep diagnostic artifacts and containers, stop the runner and PG on every exit.
"""
import importlib.util
import json
import os
from pathlib import Path
import shlex
import subprocess
import time

RUN = os.environ.get('OA_REPAIR_RUN', 'a')
assert len(RUN) == 1 and RUN in 'abcdefghijklmnopqrstuvwxyz'
APPLICATION = os.environ.get('OA_REPAIR_APPLICATION', 'project-workspace')
assert APPLICATION in ('project-workspace', 'business-engagement', 'contract-ledger', 'cost-outsourcing')
GROUPS = os.environ.get('OA_REPAIR_GROUPS', 'projects,database,httpserver').split(',')
TEST_TIMEOUT_SECONDS = int(os.environ.get('OA_REPAIR_TEST_TIMEOUT_SECONDS', '150'))
assert 60 <= TEST_TIMEOUT_SECONDS <= 600
allowed_groups = {'project-workspace':['projects','database','httpserver'],'business-engagement':['engagement','database','httpserver'],'contract-ledger':['contract','database','httpserver'],'cost-outsourcing':['referencecost','costoutsourcing','httpserver']}[APPLICATION]
assert GROUPS and len(GROUPS) == len(set(GROUPS)) and all(group in allowed_groups for group in GROUPS)
PREFIX = {'project-workspace':'oa-initiation-repair-20261002','business-engagement':'oa-bidding-validation-20261002','contract-ledger':'oa-contract-native-20261003','cost-outsourcing':'oa-cost-receipts-20261004'}[APPLICATION]
ROOT = Path('/opt/omsbizfoundry/acceptance/' + PREFIX + RUN)
TEST_REGEX = os.environ.get('OA_REPAIR_TEST_REGEX', '')
assert not TEST_REGEX or (APPLICATION == 'contract-ledger' and TEST_REGEX == '^TestWorkspaceNativePostgresExactMoneyHistoryAndReplay$') or (APPLICATION == 'cost-outsourcing' and GROUPS == ['httpserver'] and TEST_REGEX == '^(TestReferenceCostNativeHTTPExactIAMSideScopesReplayHistoryAndRedaction|TestProtectedHTTPWithIAMExactScopesReferencesFailureRecoveryAndRedaction)$')
PG = 'production_material_source_auth58_20261001a_pg'
NETWORK = 'production_material_source_auth58_20261001a_isolated'
SOURCE = Path('/opt/omsbizfoundry/acceptance/production-material-source-20261001/production_material_source_auth58_20261001a')
RUNTIME = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66'
spec = importlib.util.spec_from_file_location('entry', '/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/verify-entry-capabilities.py')
entry = importlib.util.module_from_spec(spec)
spec.loader.exec_module(entry)
inspect, save, snapshot = entry.inspect, entry.save, entry.snapshot


def run(args, data=None, timeout=180):
    result = subprocess.run(args, input=data, capture_output=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError('command failed: ' + ' '.join(args[:2]))
    return result.stdout.decode().strip()


def main():
    assert os.geteuid() == 0 and ROOT.is_dir() and not (ROOT / 'results.json').exists()
    pg = inspect(PG)
    assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings']
    assert pg['Config']['Labels']['oms.purpose'] == 'production-material-source'
    assert pg['Config']['Image'] == RUNTIME and pg['HostConfig']['NanoCpus'] == 200000000
    assert set(pg['NetworkSettings']['Networks']) == {NETWORK}
    assert json.loads(run(['docker', 'network', 'inspect', NETWORK]))[0]['Internal']
    assert not any('restore20' in name for name in run(['docker', 'ps', '--format', '{{.Names}}']).splitlines())
    fixture = dict(line.split('=', 1) for line in (SOURCE / 'config/fixture.env').read_text().splitlines())
    database_url = shlex.split(fixture['TEST_DATABASE_URL'])[0]
    assert '@' + PG + ':5432/postgres?' in database_url
    before = snapshot()
    save(ROOT / 'main-before.json', json.dumps(before, indent=2).encode())
    result = {'passed': False, 'application': APPLICATION, 'checks': [], 'main_unchanged': False, 'public_browser_or_full_oa': False, 'focused_test_regex': TEST_REGEX or None}
    names = []
    try:
        run(['docker', 'start', PG])
        deadline = time.monotonic() + 45
        while True:
            try:
                run(['docker', 'exec', PG, 'pg_isready', '-U', 'probe_admin'])
                break
            except (RuntimeError, subprocess.TimeoutExpired):
                assert time.monotonic() < deadline
                time.sleep(1)
        for group in GROUPS:
            binary = ROOT / (group + '.test')
            assert binary.is_file()
            name = PREFIX.replace('-', '_') + RUN + '_' + group
            names.append(name)
            args = ['docker', 'run', '--name', name,
                          '--label', 'oms.purpose=oa-initiation-repair',
                          '--cpus=.25', '--memory=256m', '--memory-swap=256m', '--pids-limit=96',
                          '--read-only', '--tmpfs', '/tmp:rw,noexec,size=32m',
                          '--network', NETWORK, '--user=0:0',
                          '-v', str(binary) + ':/probe/test:ro',
                          '-e', 'GOMAXPROCS=2', '-e', 'TEST_DATABASE_URL=' + database_url,
                          '--entrypoint', '/probe/test', RUNTIME,
                          '-test.v', '-test.timeout=' + str(TEST_TIMEOUT_SECONDS) + 's', '-test.parallel=1']
            if TEST_REGEX: args += ['-test.run=' + TEST_REGEX]
            try:
                output = run(args, timeout=TEST_TIMEOUT_SECONDS + 60)
            except (RuntimeError, subprocess.TimeoutExpired):
                # Keep a failed test's diagnostics as well as the unchanged
                # Main receipt; do not lose them when docker exits nonzero.
                save(ROOT / (group + '.log'), run(['docker', 'logs', name]).encode())
                result['failed_group'] = group
                raise
            save(ROOT / (group + '.log'), output.encode())
            state = inspect(name)['State']
            assert state['ExitCode'] == 0 and not state['OOMKilled'] and not state['Running']
            assert '\n--- SKIP:' not in output and output.rstrip().endswith('PASS')
            result['checks'].append({'group': group, 'passed': True, 'tests': output.count('--- PASS:')})
            print(json.dumps(result['checks'][-1]), flush=True)
        result['passed'] = True
    finally:
        for name in names:
            try:
                if inspect(name)['State']['Running']:
                    run(['docker', 'stop', '-t', '10', name])
            except RuntimeError:
                pass
        run(['docker', 'stop', '-t', '20', PG])
        after = snapshot()
        save(ROOT / 'main-after.json', json.dumps(after, indent=2).encode())
        result['main_unchanged'] = before == after
        result['passed'] = result['passed'] and result['main_unchanged']
        save(ROOT / 'results.json', json.dumps(result, indent=2).encode())
        assert result['main_unchanged'], 'Main facts or container identities changed'
    assert result['passed']
    print(json.dumps({'passed': True, 'main_unchanged': True, 'probe_stopped': True}), flush=True)


if __name__ == '__main__':
    main()
