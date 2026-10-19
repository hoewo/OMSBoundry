#!/usr/bin/env python3
"""Bounded BF-097 compatibility closure; no automatic grants or down migration."""
import copy
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

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-contributors-release-20261003a')
BACKUP = Path('/var/backups/omsbizfoundry/oa-contributors-20261003a')
PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-login-csrf-20261003b')
VERSION = 'v0.1.0-oa.20261003.3'
SCHEMA = '000011_commercial_contributors.sql'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
spec = importlib.util.spec_from_file_location('joint', '/opt/omsbizfoundry/acceptance/oa-bidding-material-release-20261003a/deploy-bidding-materials-v2-ecs.py')
b = importlib.util.module_from_spec(spec)
spec.loader.exec_module(b)
b.ROOT, b.BACKUP, b.WORKBENCH_ONLY, b.BIDDING_SLICE = ROOT, BACKUP, False, False
run, record = b.run, b.record


def publish():
    before = {k: v['Id'] for k, v in b.healthy().items()}
    manifest = json.loads((ROOT / 'artifacts/manifest.json').read_text())
    assert manifest['version'] == VERSION and manifest['official_release'] is False
    assert manifest['schema_change'] == 'Project 10 -> 11'
    assert hashlib.sha256((ROOT / 'source.tar.gz').read_bytes()).hexdigest() == manifest['source_archive_sha256']
    work = ROOT / 'publication'
    work.mkdir(mode=0o700, exist_ok=True)
    assert not (work / 'results.json').exists(), 'published outcome requires recovery, not overwrite'
    results = {'passed': False, 'official_release': False, 'images': {}}
    for app, runtime, user in (
        ('project-workspace', BASE + 'base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66', 'postgres'),
        ('unified-workbench', BASE + 'unified-workbench@sha256:76e3b9164bf3bf0511631d333b98c8ab900615cfa468b8292494ebd657d38fcb', 'oms'),
    ):
        directory = ROOT / 'artifacts' / app
        for name, digest in manifest['applications'][app].items():
            assert hashlib.sha256((directory / name).read_bytes()).hexdigest() == digest
        tag = BASE + app + ':' + VERSION
        absent = subprocess.run(['docker', 'manifest', 'inspect', tag], capture_output=True, text=True, timeout=40)
        assert absent.returncode and any(s in absent.stderr.lower() for s in ('no such manifest', 'manifest unknown', 'not found')), 'immutable new tag absence required'
        output = run(['docker', 'build', '--platform', 'linux/amd64', '--memory', '128m', '--cpu-period', '100000', '--cpu-quota', '15000', '--build-arg', 'RUNTIME_IMAGE=' + runtime, '--label', 'oms.release=' + VERSION, '--label', 'oms.official-release=false', '--label', 'oms.source-archive-sha256=' + manifest['source_archive_sha256'], '-t', tag, str(directory)])
        record(work / (app + '-build-private.log'), output, True)
        pushed = run(['docker', 'push', tag])
        record(work / (app + '-push-private.log'), pushed, True)
        digests = re.findall(r'digest: (sha256:[a-f0-9]{64})', pushed.decode())
        assert len(digests) == 1
        reference = BASE + app + '@' + digests[0]
        run(['docker', 'pull', '--platform', 'linux/amd64', reference])
        image = json.loads(run(['docker', 'image', 'inspect', reference]))[0]
        assert image['Architecture'] == 'amd64' and image['Config']['User'] == user
        name = 'oa_contributors_20261003a_' + app.replace('-', '_') + '_extract'
        run(['docker', 'create', '--name', name, '--network', 'none', '--memory', '64m', '--cpus', '.1', reference])
        extracted = work / app
        extracted.mkdir(mode=0o700)
        for binary, digest in manifest['applications'][app].items():
            target = extracted / binary
            run(['docker', 'cp', name + ':/app/' + binary, str(target)])
            assert hashlib.sha256(target.read_bytes()).hexdigest() == digest
        assert not b.inspect(name)['State']['Running']
        results['images'][app] = {'reference': reference, 'image_id': image['Id'], 'binary_sha256': manifest['applications'][app]}
        record(work / 'results.json', results)
        print(json.dumps({'phase': 'publication', 'application': app, 'reference': reference}), flush=True)
    assert {k: v['Id'] for k, v in b.healthy().items()} == before
    results.update(passed=True, main_unchanged=True)
    record(work / 'results.json', results)


def parse_contracts(current):
    iam = current['IDENTITY']
    args = ['docker', 'create', '--name', 'oa_contributors_20261003a_iam_parser', '--network', 'none', '--read-only', '--memory', '96m', '--cpus', '.15', '--cap-drop', 'ALL']
    for m in iam['Mounts']:
        source = str(ROOT / 'artifacts/contracts/project-workspace-capabilities.json') if m['Destination'] == '/config/project-workspace-capabilities.json' else m['Source']
        args += ['--mount', 'type=bind,src=' + source + ',dst=' + m['Destination'] + ',readonly']
    for value in iam['Config']['Env']: args += ['--env', value]
    run(args + [iam['Image'], '/app/ops', 'validate-contracts'])
    assert json.loads(run(['docker', 'start', '--attach', 'oa_contributors_20261003a_iam_parser'])) == {'service': 'identity-access', 'status': 'contracts-valid'}
    state = b.inspect('oa_contributors_20261003a_iam_parser')['State']
    assert state['ExitCode'] == 0 and not state['OOMKilled']


def prepare():
    assert not BACKUP.exists() and b.ENV.stat().st_mode & 0o077 == 0
    current = b.healthy()
    published = json.loads((ROOT / 'publication/results.json').read_text())
    assert published['passed'] and published['main_unchanged']
    assert b.sql('project_workspace', 'SELECT max(version) FROM schema_migrations') == '000010_initial_dispatch_draft.sql'
    catalog = ROOT / 'artifacts/contracts/project-workspace-capabilities.json'
    catalog.chmod(0o444)
    expected = json.loads((ROOT / 'artifacts/manifest.json').read_text())['contracts']['project-workspace-capabilities.json']
    assert hashlib.sha256(catalog.read_bytes()).hexdigest() == expected
    parse_contracts(current)
    original = b.ENV.read_bytes()
    replacements = {'D1_PROJECT_IMAGE': published['images']['project-workspace']['reference'], 'D1_WORKBENCH_IMAGE': published['images']['unified-workbench']['reference']}
    lines = original.decode().splitlines()
    assert all(sum(line.startswith(k + '=') for line in lines) == 1 for k in replacements)
    candidate = ('\n'.join(k + '=' + replacements[k] if (k := line.split('=', 1)[0]) in replacements else line for line in lines) + '\n').encode()
    override = json.loads((PREVIOUS / 'compose.oa-public.json').read_text())
    iam = override.setdefault('services', {}).setdefault('identity-access', {})
    volumes = iam.setdefault('volumes', [])
    destination = ':/config/project-workspace-capabilities.json:ro'
    assert sum(v.endswith(destination) for v in volumes) == 1
    iam['volumes'] = [str(catalog) + destination if v.endswith(destination) else v for v in volumes]
    record(ROOT / 'compose.oa-public.json', override)
    BACKUP.mkdir(mode=0o700)
    record(BACKUP / 'original.env', original, True)
    record(BACKUP / 'candidate.env', candidate, True)
    record(BACKUP / 'containers-before.json', {k: {'id': v['Id'], 'image': v['Image']} for k, v in current.items()})
    record(BACKUP / 'preflight.json', {'passed': True, 'closure': ['Project server/migration 11', 'Workbench original personnel components', 'IAM same binary/new catalog 0.7'], 'gateway_manifest_unchanged': True, 'selected_assignment_policies_unchanged': True, 'grant_writes': 0, 'schema_change': '10 -> 11', 'old_binary_direct_rollback_forbidden': True, 'official_release': False})
    print(json.dumps({'phase': 'preflight', 'passed': True, 'grants_unchanged': True}), flush=True)


def restore():
    b.healthy()
    assert not (BACKUP / 'restore.json').exists()
    receipt = json.loads((BACKUP / 'cold-backup.json').read_text())
    for name, digest in receipt['checksums'].items(): assert hashlib.sha256((BACKUP / name).read_bytes()).hexdigest() == digest
    before = json.loads((BACKUP / 'facts-before.json').read_text())
    pg = b.inspect(b.PRIVATE_PG)
    assert not pg['HostConfig']['PortBindings'] and pg['HostConfig']['NanoCpus'] == 200000000
    run(['docker', 'start', b.PRIVATE_PG])
    try:
        deadline = time.monotonic() + 45
        while True:
            try: b.sql('postgres', 'SELECT 1', True); break
            except RuntimeError: assert time.monotonic() < deadline; time.sleep(1)
        for db in b.DBS:
            target = 'contributors_20261003a_' + db
            owner, password = target + '_owner', secrets.token_hex(24)
            assert b.sql('postgres', "SELECT count(*) FROM pg_database WHERE datname='" + target + "'", True) == '0'
            b.sql('postgres', 'CREATE ROLE ' + owner + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '" + password + "';", True)
            b.sql('postgres', 'CREATE DATABASE ' + target + ' OWNER ' + owner, True)
            with (BACKUP / (db + '.dump')).open('rb') as source:
                completed = subprocess.run(['docker', 'exec', '-i', b.PRIVATE_PG, 'pg_restore', '-U', 'probe_admin', '--role', owner, '-d', target, '--single-transaction', '--exit-on-error', '--no-owner', '--no-privileges'], stdin=source, capture_output=True, timeout=120)
                assert completed.returncode == 0
            assert b.facts(target, True) == before[db]
            if db == 'project_workspace': record(BACKUP / 'restore-project-url', ('postgres://' + owner + ':' + password + '@' + b.PRIVATE_PG + ':5432/' + target + '?sslmode=disable').encode(), True)
            print(json.dumps({'phase': 'empty-target-restore', 'database': db, 'equal': True}), flush=True)
        for key in ('EVIDENCE', 'IDENTITY'):
            directory = BACKUP / ('empty-restored-' + key.lower())
            directory.mkdir(mode=0o700)
            with tarfile.open(BACKUP / (key.lower() + '-files.tar.gz')) as archive:
                for member in archive.getmembers():
                    path = Path(member.name)
                    assert path.parts[0] == 'data' and not path.is_absolute() and '..' not in path.parts and (member.isdir() or member.isfile())
                    target = directory / path
                    if member.isdir(): target.mkdir(mode=0o700, parents=True, exist_ok=True)
                    else:
                        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
                        record(target, archive.extractfile(member).read(), True)
            actual = {str(p.relative_to(directory / 'data')): hashlib.sha256(p.read_bytes()).hexdigest() for p in (directory / 'data').rglob('*') if p.is_file()}
            assert actual == json.loads((BACKUP / 'assets-before.json').read_text())[key]
        target = 'contributors_20261003a_project_workspace'
        old = b.facts(target, True, ('schema_migrations',))
        b.project_ops(True, 'migrate')
        assert b.project_ops(True, 'check') == SCHEMA
        assert b.facts(target, True, ('schema_migrations',)) == old
        b.project_ops(True, 'migrate')
        assert b.facts(target, True, ('schema_migrations',)) == old
        record(BACKUP / 'restore.json', {'passed': True, 'empty_databases': 13, 'all_old_facts_and_sequences_equal': True, 'evidence_and_iam_key_bytes_equal': True, 'project_10_to11_twice_preserves_facts': True, 'full_suite_restore_runtime': False, 'rollback_requires_empty_restore_original_release': True})
    finally:
        if b.inspect(b.PRIVATE_PG)['State']['Running']: run(['docker', 'stop', '--time', '15', b.PRIVATE_PG])
    print(json.dumps({'phase': 'restore-migration', 'passed': True}), flush=True)


def export_backup():
    assert json.loads((BACKUP / 'restore.json').read_text())['passed']
    target = Path('/home/ecs-user/oa-contributors-20261003a-backup.tar.gz')
    assert not target.exists()
    with tarfile.open(target, 'x:gz') as archive: archive.add(BACKUP, arcname=BACKUP.name)
    target.chmod(0o600)
    account = pwd.getpwnam('ecs-user'); os.chown(target, account.pw_uid, account.pw_gid)
    value = {'archive_sha256': hashlib.sha256(target.read_bytes()).hexdigest(), 'bytes': target.stat().st_size}
    record(ROOT / 'offhost-export.json', value)
    print(json.dumps({'phase': 'offhost-export', **value}), flush=True)


def apply():
    current = b.healthy()
    assert json.loads((BACKUP / 'restore.json').read_text())['passed']
    assert json.loads((ROOT / 'offhost-confirmed.json').read_text())['passed']
    assert json.loads((ROOT / 'offhost-confirmed.json').read_text())['archive_sha256'] == json.loads((ROOT / 'offhost-export.json').read_text())['archive_sha256']
    assert b.ENV.read_bytes() == (BACKUP / 'original.env').read_bytes()
    assert b.facts('project_workspace') == json.loads((BACKUP / 'facts-before.json').read_text())['project_workspace']
    original = {db: b.facts(db, exclude=('schema_migrations',) if db == 'project_workspace' else ()) for db in b.DBS if db != 'identity_access'}
    grants = b.sql('identity_access', "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")
    run(['docker', 'stop', '--time', '30', 'omsbizfoundry-project-workspace-1'])
    try:
        b.project_ops(False, 'migrate')
        assert b.project_ops(False, 'check') == SCHEMA
        assert b.facts('project_workspace', exclude=('schema_migrations',)) == original['project_workspace']
        record(b.ENV, (BACKUP / 'candidate.env').read_bytes(), True)
        configuration = json.loads(b.compose('config', '--format', 'json'))
        assert not configuration['services']['project-workspace'].get('ports')
        assert configuration['services']['project-workspace']['environment']['BUSINESS_ENGAGEMENT_BASE_URL'] == 'http://business-engagement:8089'
        b.compose('up', '-d', '--no-deps', '--no-build', '--pull', 'never', '--wait', '--wait-timeout', '100', 'identity-access', 'project-workspace', 'unified-workbench')
        after = b.healthy()
        changed = ('IDENTITY', 'PROJECT', 'WORKBENCH')
        assert all(after[k]['Id'] != current[k]['Id'] for k in changed)
        assert all(after[k]['Id'] == v['Id'] for k, v in current.items() if k not in changed)
        for db, value in original.items(): assert b.facts(db, exclude=('schema_migrations',) if db == 'project_workspace' else ()) == value, db
        assert b.sql('identity_access', "SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t") == grants
        run(['env', 'D1_ENV_FILE=' + str(b.ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(ROOT / 'compose.oa-public.json'), str(b.OLD / 'ops/d1'), 'check'], timeout=240)
        wrapper = '#!/bin/sh\nexport D1_ENV_FILE=' + str(b.ENV) + '\nexport D1_COMPOSE_OVERRIDE_FILE=' + str(ROOT / 'compose.oa-public.json') + '\nexec ' + str(b.OLD / 'ops/d1') + ' "$@"\n'
        record(ROOT / 'd1-active', wrapper.encode(), True); (ROOT / 'd1-active').chmod(0o700)
        record(BACKUP / 'rollout.json', {'passed': True, 'version': VERSION, 'project_schema': 11, 'healthy': 17, 'changed_containers': ['identity-access', 'project-workspace', 'unified-workbench'], 'other_14_container_ids_unchanged': True, 'all_old_business_facts_preserved': True, 'grant_writes': 0, 'assignment_policies_unchanged': True, 'd1_check': True, 'official_release': False, 'full_oa_accepted': False})
        print(json.dumps({'phase': 'main-rollout', 'passed': True, 'healthy': 17, 'schema': 11, 'grants_unchanged': True}), flush=True)
    except Exception:
        migrated = b.sql('project_workspace', 'SELECT max(version) FROM schema_migrations') == SCHEMA
        if not migrated: run(['docker', 'start', 'omsbizfoundry-project-workspace-1'])
        record(BACKUP / 'rollout-failure.json', {'migration_applied': migrated, 'old_binary_rollback_forbidden': migrated, 'backup_volumes_retained': True})
        raise


if __name__ == '__main__':
    try:
        assert os.geteuid() == 0 and len(sys.argv) == 2
        {'publish': publish, 'prepare': prepare, 'backup': b.backup, 'restore': restore, 'export': export_backup, 'apply': apply}[sys.argv[1]]()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next: trace = trace.tb_next
        raise SystemExit('contributors release failed: ' + type(error).__name__ + ' line ' + str(trace.tb_lineno) + '; private diagnostics retained')
