#!/usr/bin/env python3
"""Auth58 bounded login-only repair. Private state stays on ECS, not in receipts."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import pwd
import re
import secrets
import subprocess
import sys
import tarfile
import time

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-login-csrf-20261003b')
BACKUP = Path('/var/backups/omsbizfoundry/oa-login-csrf-20261003b')
PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-bidding-material-release-20261003a')
VERSION = 'v0.1.0-oa.20261003.2'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
RUNTIME = BASE + 'unified-workbench@sha256:c53999c753e8363e7b0f8ec9c24548933094e5fb49202751da0aa4ffe24ce688'
RESTORE_PREFIX = 'login_20261003b_'
# A single bounded follow-up reuses the verified no-migration delivery steps.
# Arbitrary roots, images or rollout scopes are deliberately not accepted.
REPAIR = os.environ.get('OA_WORKBENCH_REPAIR', '')
assert REPAIR in ('', 'personnel-route', 'personnel-refresh', 'projection-visibility', 'commercial-routes', 'commercial-actions', 'commercial-links', 'cost-modal')
if REPAIR == 'personnel-route':
    ROOT = Path('/opt/omsbizfoundry/acceptance/oa-personnel-route-20261003a')
    BACKUP = Path('/var/backups/omsbizfoundry/oa-personnel-route-20261003a')
    PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-contributors-release-20261003a')
    VERSION = 'v0.1.0-oa.20261003.4'
    RUNTIME = BASE + 'unified-workbench@sha256:453ad7502b04a598a7d5c14ef7230273db519b99029acaf0b3f883402290d385'
    RESTORE_PREFIX = 'personnel_route_20261003a_'
elif REPAIR == 'personnel-refresh':
    ROOT = Path('/opt/omsbizfoundry/acceptance/oa-personnel-refresh-20261003a')
    BACKUP = Path('/var/backups/omsbizfoundry/oa-personnel-refresh-20261003a')
    PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-personnel-route-20261003a')
    VERSION = 'v0.1.0-oa.20261003.5'
    RUNTIME = BASE + 'unified-workbench@sha256:4fb8c8fd78ba024c6ec02a11eb6f4ae03eed86fc4445fddcde0418c0ebcef690'
    RESTORE_PREFIX = 'personnel_refresh_20261003a_'
elif REPAIR == 'projection-visibility':
    ROOT = Path('/opt/omsbizfoundry/acceptance/oa-projection-visibility-20261003a')
    BACKUP = Path('/var/backups/omsbizfoundry/oa-projection-visibility-20261003a')
    PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-personnel-refresh-20261003a')
    VERSION = 'v0.1.0-oa.20261003.6'
    RUNTIME = BASE + 'unified-workbench@sha256:b0e77b861167c7f3822670d7729aa03634e1565e438a1d37453068763cd9a1bf'
    RESTORE_PREFIX = 'projection_visibility_20261003a_'
elif REPAIR == 'commercial-routes':
    ROOT = Path('/opt/omsbizfoundry/acceptance/oa-commercial-routes-20261004a')
    BACKUP = Path('/var/backups/omsbizfoundry/oa-commercial-routes-20261004a')
    PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-commercial-release-20261004a')
    VERSION = 'v0.1.0-oa.20261004.2'
    RUNTIME = BASE + 'unified-workbench@sha256:53db5592e44af931ea9dc50ccd2d567868d3d756bd98d853b02d228005131530'
    RESTORE_PREFIX = 'commercial_routes_20261004a_'
elif REPAIR == 'commercial-actions':
    ROOT = Path('/opt/omsbizfoundry/acceptance/oa-commercial-actions-20261004a')
    BACKUP = Path('/var/backups/omsbizfoundry/oa-commercial-actions-20261004a')
    PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-commercial-routes-20261004a')
    VERSION = 'v0.1.0-oa.20261004.3'
    RUNTIME = BASE + 'unified-workbench@sha256:70d773681fa097429ce07ad169eb9d2b0a7f48ae41878d8007af3ba1455bd8ac'
    RESTORE_PREFIX = 'commercial_actions_20261004a_'
elif REPAIR == 'commercial-links':
    ROOT = Path('/opt/omsbizfoundry/acceptance/oa-commercial-links-20261004a')
    BACKUP = Path('/var/backups/omsbizfoundry/oa-commercial-links-20261004a')
    PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-commercial-actions-20261004a')
    VERSION = 'v0.1.0-oa.20261004.4'
    RUNTIME = BASE + 'unified-workbench@sha256:f51dd655de78add9b37e895db64c0de7554383491f2afce161eddf7be6c27635'
    RESTORE_PREFIX = 'commercial_links_20261004a_'
elif REPAIR == 'cost-modal':
    ROOT = Path('/opt/omsbizfoundry/acceptance/oa-cost-modal-20261004a')
    BACKUP = Path('/var/backups/omsbizfoundry/oa-cost-modal-20261004a')
    PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a')
    VERSION = 'v0.1.0-oa.20261004.6'
    RUNTIME = json.loads((PREVIOUS / 'publication/results.json').read_text())['images']['unified-workbench']['reference']
    assert re.fullmatch(re.escape(BASE) + r'unified-workbench@sha256:[a-f0-9]{64}', RUNTIME)
    RESTORE_PREFIX = 'cost_modal_20261004a_'
spec = importlib.util.spec_from_file_location('joint_backup', '/opt/omsbizfoundry/acceptance/oa-bidding-material-release-20261003a/deploy-bidding-materials-v2-ecs.py')
b = importlib.util.module_from_spec(spec)
spec.loader.exec_module(b)
b.ROOT, b.BACKUP = ROOT, BACKUP
run, record = b.run, b.record
if REPAIR in ('commercial-routes','commercial-actions','commercial-links','cost-modal'):
    spec = importlib.util.spec_from_file_location('commercial_backup', '/opt/omsbizfoundry/acceptance/oa-commercial-release-20261004a/commercial-release-ecs.py')
    commercial = importlib.util.module_from_spec(spec); spec.loader.exec_module(commercial)
    commercial.ROOT, commercial.BACKUP = ROOT, BACKUP
    b = commercial.b
    b.ROOT, b.BACKUP = ROOT, BACKUP
    run, record = b.run, b.record


def publish():
    before = {k: v['Id'] for k, v in b.healthy().items()}
    manifest = json.loads((ROOT / 'artifacts/manifest.json').read_text())
    assert manifest['version'] == VERSION and manifest['official_release'] is False
    assert manifest['closure'] == ['unified-workbench'] and manifest['schema_change'] is False
    assert hashlib.sha256((ROOT / 'source.tar.gz').read_bytes()).hexdigest() == manifest['source_archive_sha256']
    directory, work = ROOT / 'artifacts/unified-workbench', ROOT / 'publication'
    work.mkdir(mode=0o700, exist_ok=True)
    for name, digest in manifest['applications']['unified-workbench'].items():
        assert hashlib.sha256((directory / name).read_bytes()).hexdigest() == digest
    tag = BASE + 'unified-workbench:' + VERSION
    absent = subprocess.run(['docker', 'manifest', 'inspect', tag], capture_output=True, text=True, timeout=40)
    assert absent.returncode != 0 and any(s in absent.stderr.lower() for s in ('no such manifest', 'manifest unknown', 'not found')), 'tag absent proof required'
    output = run(['docker', 'build', '--platform', 'linux/amd64', '--memory', '128m', '--cpu-period', '100000', '--cpu-quota', '15000', '--build-arg', 'RUNTIME_IMAGE=' + RUNTIME, '--label', 'oms.release=' + VERSION, '--label', 'oms.source-archive-sha256=' + manifest['source_archive_sha256'], '--label', 'oms.official-release=false', '-t', tag, str(directory)])
    record(work / 'build-private.log', output, True)
    pushed = run(['docker', 'push', tag])
    record(work / 'push-private.log', pushed, True)
    digests = re.findall(r'digest: (sha256:[a-f0-9]{64})', pushed.decode())
    assert len(digests) == 1
    reference = BASE + 'unified-workbench@' + digests[0]
    run(['docker', 'pull', '--platform', 'linux/amd64', reference])
    image = json.loads(run(['docker', 'image', 'inspect', reference]))[0]
    assert image['Architecture'] == 'amd64' and image['Config']['User'] == 'oms'
    name = ROOT.name.replace('-', '_') + '_extract'
    run(['docker', 'create', '--name', name, '--network', 'none', '--memory', '64m', '--cpus', '.1', reference])
    target = work / 'server-extracted'
    run(['docker', 'cp', name + ':/app/server', str(target)])
    assert hashlib.sha256(target.read_bytes()).hexdigest() == manifest['applications']['unified-workbench']['server']
    assert not b.inspect(name)['State']['Running']
    assert {k: v['Id'] for k, v in b.healthy().items()} == before
    record(work / 'results.json', {'passed': True, 'official_release': False, 'main_unchanged': True, 'images': {'unified-workbench': {'reference': reference, 'binary_sha256': manifest['applications']['unified-workbench']['server']}}})
    print(json.dumps({'phase': 'publication', 'passed': True, 'main_unchanged': True, 'reference': reference}), flush=True)


def prepare():
    assert not BACKUP.exists() and b.ENV.stat().st_mode & 0o077 == 0
    current = b.healthy()
    publication = json.loads((ROOT / 'publication/results.json').read_text())
    assert publication['passed'] and publication['main_unchanged']
    original = b.ENV.read_bytes()
    replacement = publication['images']['unified-workbench']['reference']
    lines = original.decode().splitlines()
    assert sum(line.startswith('D1_WORKBENCH_IMAGE=') for line in lines) == 1
    candidate = ('\n'.join('D1_WORKBENCH_IMAGE=' + replacement if line.startswith('D1_WORKBENCH_IMAGE=') else line for line in lines) + '\n').encode()
    override = (PREVIOUS / 'compose.oa-public.json').read_bytes()
    record(ROOT / 'compose.oa-public.json', override, True)
    BACKUP.mkdir(mode=0o700)
    record(BACKUP / 'original.env', original, True)
    record(BACKUP / 'candidate.env', candidate, True)
    record(BACKUP / 'containers-before.json', {k: {'id': v['Id'], 'image': v['Image']} for k, v in current.items()})
    record(BACKUP / 'preflight.json', {'passed': True, 'closure': ['unified-workbench'], 'schema_change': False, 'iam_gateway_manifest_policy_unchanged': True, 'compose_override_sha256': hashlib.sha256(override).hexdigest(), 'project_schema': b.sql('project_workspace', 'SELECT max(version) FROM schema_migrations'), 'be_schema': b.sql('business_engagement', 'SELECT max(version) FROM schema_migrations'), 'official_release': False})
    print(json.dumps({'phase': 'preflight', 'passed': True, 'closure': ['unified-workbench'], 'schema_change': False}), flush=True)


def restore():
    b.healthy()
    assert not (BACKUP / 'restore.json').exists()
    receipt = json.loads((BACKUP / 'cold-backup.json').read_text())
    for name, digest in receipt['checksums'].items():
        assert hashlib.sha256((BACKUP / name).read_bytes()).hexdigest() == digest
    before = json.loads((BACKUP / 'facts-before.json').read_text())
    pg = b.inspect(b.PRIVATE_PG)
    assert not pg['HostConfig']['PortBindings'] and pg['HostConfig']['NanoCpus'] == 200000000
    run(['docker', 'start', b.PRIVATE_PG])
    try:
        deadline = time.monotonic() + 45
        while True:
            try:
                b.sql('postgres', 'SELECT 1', True)
                break
            except RuntimeError:
                assert time.monotonic() < deadline
                time.sleep(1)
        for db in b.DBS:
            target = RESTORE_PREFIX + db
            owner = target + '_owner'
            assert b.sql('postgres', "SELECT count(*) FROM pg_database WHERE datname='" + target + "'", True) == '0'
            b.sql('postgres', 'CREATE ROLE ' + owner + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '" + secrets.token_hex(24) + "';", True)
            b.sql('postgres', 'CREATE DATABASE ' + target + ' OWNER ' + owner, True)
            with (BACKUP / (db + '.dump')).open('rb') as source:
                result = subprocess.run(['docker', 'exec', '-i', b.PRIVATE_PG, 'pg_restore', '-U', 'probe_admin', '--role', owner, '-d', target, '--single-transaction', '--exit-on-error', '--no-owner', '--no-privileges'], stdin=source, capture_output=True, timeout=120)
                assert result.returncode == 0
            assert b.facts(target, True) == before[db]
            print(json.dumps({'phase': 'empty-target-restore', 'database': db, 'facts_and_sequences_equal': True}), flush=True)
        expected = json.loads((BACKUP / 'assets-before.json').read_text())
        for key in (('EVIDENCE', 'IDENTITY', 'CONTRACT') if REPAIR in ('commercial-routes','commercial-actions','commercial-links','cost-modal') else ('EVIDENCE', 'IDENTITY')):
            directory = BACKUP / ('empty-restored-' + key.lower())
            directory.mkdir(mode=0o700)
            with tarfile.open(BACKUP / (key.lower() + '-files.tar.gz')) as archive:
                for member in archive.getmembers():
                    path = Path(member.name)
                    assert path.parts[0] == 'data' and not path.is_absolute() and '..' not in path.parts
                    assert member.isdir() or member.isfile()
                    target = directory / path
                    if member.isdir(): target.mkdir(mode=0o700, parents=True, exist_ok=True)
                    else:
                        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
                        record(target, archive.extractfile(member).read(), True)
            actual = {str(path.relative_to(directory / 'data')): hashlib.sha256(path.read_bytes()).hexdigest() for path in (directory / 'data').rglob('*') if path.is_file()}
            assert actual == expected[key]
        record(BACKUP / 'restore.json', {'passed': True, 'empty_databases': 13, 'facts_and_sequences_equal': True, 'files_and_iam_key_bytes_equal': True, 'file_trees': 3 if REPAIR in ('commercial-routes','commercial-actions','commercial-links','cost-modal') else 2, 'migration_run': False, 'schema_change': False, 'full_suite_restore_runtime': False})
    finally:
        if b.inspect(b.PRIVATE_PG)['State']['Running']: run(['docker', 'stop', '--time', '15', b.PRIVATE_PG])
    print(json.dumps({'phase': 'restore', 'passed': True, 'schema_change': False}), flush=True)


def export_backup():
    assert json.loads((BACKUP / 'restore.json').read_text())['passed']
    target = Path('/home/ecs-user/' + ROOT.name + '-backup.tar.gz')
    assert not target.exists()
    with tarfile.open(target, 'x:gz') as archive: archive.add(BACKUP, arcname=BACKUP.name)
    target.chmod(0o600)
    account = pwd.getpwnam('ecs-user')
    os.chown(target, account.pw_uid, account.pw_gid)
    value = {'archive_sha256': hashlib.sha256(target.read_bytes()).hexdigest(), 'bytes': target.stat().st_size, 'local_receipt_pending': True}
    record(ROOT / 'offhost-export.json', value)
    print(json.dumps({'phase': 'backup-export', **value}), flush=True)


def apply():
    current = b.healthy()
    assert json.loads((BACKUP / 'restore.json').read_text())['passed']
    assert json.loads((ROOT / 'offhost-confirmed.json').read_text())['passed']
    assert json.loads((ROOT / 'offhost-confirmed.json').read_text())['archive_sha256'] == json.loads((ROOT / 'offhost-export.json').read_text())['archive_sha256']
    assert b.ENV.read_bytes() == (BACKUP / 'original.env').read_bytes()
    assert (ROOT / 'compose.oa-public.json').read_bytes() == (PREVIOUS / 'compose.oa-public.json').read_bytes()
    before = {db: b.facts(db) for db in b.DBS if db != 'identity_access'}
    record(BACKUP / 'business-before.json', before)
    try:
        record(b.ENV, (BACKUP / 'candidate.env').read_bytes(), True)
        b.compose('up', '-d', '--no-deps', '--no-build', '--pull', 'never', '--wait', '--wait-timeout', '100', 'unified-workbench')
        after = b.healthy()
        assert after['WORKBENCH']['Id'] != current['WORKBENCH']['Id']
        assert all(after[k]['Id'] == v['Id'] for k, v in current.items() if k != 'WORKBENCH')
        for db in before: assert b.facts(db) == before[db], db + ' changed during login-only rollout'
        run(['env', 'D1_ENV_FILE=' + str(b.ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(ROOT / 'compose.oa-public.json'), str(b.OLD / 'ops/d1'), 'check'], timeout=240)
        record(BACKUP / 'rollout.json', {'passed': True, 'healthy': 17, 'changed_containers': ['unified-workbench'], 'other_container_ids_unchanged': 16, 'all_business_facts_unchanged': True, 'iam_gateway_policies_unchanged': True, 'schema_change': False, 'd1_check': True, 'official_release': False})
        wrapper = '#!/bin/sh\nexport D1_ENV_FILE=' + str(b.ENV) + '\nexport D1_COMPOSE_OVERRIDE_FILE=' + str(ROOT / 'compose.oa-public.json') + '\nexec ' + str(b.OLD / 'ops/d1') + ' "$@"\n'
        record(ROOT / 'd1-active', wrapper.encode(), True); (ROOT / 'd1-active').chmod(0o700)
    except Exception:
        # This repair has no migration. Only the previous Workbench image may be restored.
        record(b.ENV, (BACKUP / 'original.env').read_bytes(), True)
        b.compose('up', '-d', '--no-deps', '--no-build', '--pull', 'never', '--wait', '--wait-timeout', '100', 'unified-workbench')
        record(BACKUP / 'rollout-failure.json', {'workbench_rollback_attempted': True, 'schema_change': False, 'data_and_backup_retained': True})
        raise
    print(json.dumps({'phase': 'main-rollout', 'passed': True, 'healthy': 17, 'other_container_ids_unchanged': 16, 'schema_change': False}), flush=True)


if __name__ == '__main__':
    try:
        assert os.geteuid() == 0 and len(sys.argv) == 2 and sys.argv[1] in ('publish', 'prepare', 'backup', 'restore', 'export_backup', 'apply')
        if sys.argv[1] == 'backup':
            if REPAIR in ('commercial-routes','commercial-actions','commercial-links','cost-modal'): commercial.backup()
            else: b.backup()
        else: globals()[sys.argv[1]]()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next: trace = trace.tb_next
        raise SystemExit('login-only repair failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics retained')
