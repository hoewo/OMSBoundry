#!/usr/bin/env python3
"""Auth58 test slice: preflight -> cold joint backup -> empty restore -> forward rollout.

No down migration, volume deletion, credential disclosure or old-binary rollback.
This is a sparse development/test deployment, not an official clean-HEAD release.
"""
import hashlib
import copy
import importlib.util
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import tarfile
import time
import urllib.parse

RUN = os.environ.get('OA_PUBLIC_RUN', 'a')
assert RUN in ('a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k')
NO_SCHEMA_CHANGE = RUN in ('b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k')
WORKBENCH_ONLY = RUN in ('c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k')  # no Project image/migration replacement
BIDDING_SLICE = RUN in ('g','h')
PROJECT_REFERENCE_CONFIG = RUN == 'k'
BIDDING_REFERENCE_ENDPOINT = 'http://business-engagement:8089'
ROOT = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002' + RUN)
BACKUP = Path('/var/backups/omsbizfoundry/oa-public-20261002' + RUN)
ENV = Path('/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.https.env')
OLD = Path('/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.20_linux_amd64/deployments/d1')
PG = 'omsbizfoundry-postgres-1'
PRIVATE_PG = 'production_material_source_auth58_20261001a_pg'
NETWORK = 'production_material_source_auth58_20261001a_isolated'
DBS = ('identity_access', 'evidence_service', 'project_workspace', 'people_groups', 'party_directory', 'contract_ledger', 'money_flow', 'business_engagement', 'billing_invoice', 'design_operations', 'approval_audit_results', 'cost_outsourcing', 'bonus_distribution')
SERVICES = {'POSTGRES': 'postgres', 'IDENTITY': 'identity-access', 'CONTRACT': 'contract-ledger', 'MONEY': 'money-flow-ledger', 'PEOPLE': 'people-groups', 'PARTY': 'party-directory', 'PROJECT': 'project-workspace', 'EVIDENCE': 'evidence-service', 'BUSINESS_ENGAGEMENT': 'business-engagement', 'BILLING': 'billing-invoice', 'PRODUCTION': 'production-operations', 'APPROVAL_AUDIT_RESULTS': 'approval-audit-results', 'COST_OUTSOURCING': 'cost-outsourcing', 'BONUS_DISTRIBUTION': 'bonus-distribution', 'WORKBENCH': 'unified-workbench', 'GATEWAY': 'gateway', 'NGINX': 'nginx'}


def with_bidding_reference_endpoint(override):
    """Repair only the declared D1 reference edge, not images or IAM policy."""
    value = copy.deepcopy(override)
    project = value.setdefault('services', {}).setdefault('project-workspace', {})
    environment = project.setdefault('environment', {})
    assert environment.get('BUSINESS_ENGAGEMENT_BASE_URL') in (None, '', BIDDING_REFERENCE_ENDPOINT), 'unexpected reference target'
    assert not environment.get('BUSINESS_ENGAGEMENT_ACCESS_TOKEN'), 'no static credential injection'
    environment['BUSINESS_ENGAGEMENT_BASE_URL'] = BIDDING_REFERENCE_ENDPOINT
    return value


def validate_project_reference_repair(before, after):
    """Same Project binary, schema and credentials; exactly one endpoint added."""
    assert before['Image'] == after['Image']
    original = dict(item.split('=', 1) for item in before['Config']['Env'])
    expected = {**original, 'BUSINESS_ENGAGEMENT_BASE_URL': BIDDING_REFERENCE_ENDPOINT}
    assert dict(item.split('=', 1) for item in after['Config']['Env']) == expected
    assert original['IDENTITY_PROVIDER'] == 'iam' and original.get('IAM_SERVICE_CREDENTIAL_URL')
    assert not original.get('BUSINESS_ENGAGEMENT_ACCESS_TOKEN')
    # Docker inspect enumerates a mount map; order is not a data boundary.
    assert sorted(before['Mounts'], key=lambda mount: mount['Destination']) == sorted(after['Mounts'], key=lambda mount: mount['Destination'])
    assert before['HostConfig']['PortBindings'] == after['HostConfig']['PortBindings']


def run(args, data=None, timeout=180):
    result = subprocess.run(args, input=data, capture_output=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError('command failed: ' + ' '.join(args[:2]))
    return result.stdout


def inspect(name):
    return json.loads(run(['docker', 'inspect', name]))[0]


def record(path, value, binary=False):
    tmp = path.with_name(path.name + '.next')
    with os.fdopen(os.open(tmp, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), 'wb') as output:
        output.write(value if binary else (json.dumps(value, indent=2) + '\n').encode())
    os.replace(tmp, path)


def sql(db, statement, private=False):
    return run(['docker', 'exec', '-i', PRIVATE_PG if private else PG, 'psql', '-X', '-q', '-A', '-t', '-v', 'ON_ERROR_STOP=1', '-U', 'probe_admin' if private else 'oms_admin', '-d', db], statement.encode()).decode().strip()


def facts(db, private=False, exclude=()):
    tables = json.loads(sql(db, "SELECT coalesce(json_agg(tablename ORDER BY tablename),'[]') FROM pg_tables WHERE schemaname='public';", private))
    result = {}
    for table in tables:
        if table not in exclude:
            quoted = '"' + table.replace('"', '""') + '"'
            result[table] = sql(db, "SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public." + quoted + ' t', private)
    sequences = json.loads(sql(db, "SELECT coalesce(json_agg(sequencename ORDER BY sequencename),'[]') FROM pg_sequences WHERE schemaname='public';", private))
    for sequence in sequences:
        quoted = '"' + sequence.replace('"', '""') + '"'
        result['sequence:' + sequence] = sql(db, 'SELECT last_value::text || \':\' || is_called::text FROM public.' + quoted, private)
    return result


def healthy():
    current = {key: inspect('omsbizfoundry-' + name + '-1') for key, name in SERVICES.items()}
    assert all(value['State'].get('Health', {}).get('Status') == 'healthy' for value in current.values())
    assert not inspect(PRIVATE_PG)['State']['Running']
    assert not any('restore20' in name for name in run(['docker', 'ps', '--format', '{{.Names}}']).decode().splitlines())
    return current


def compose(*command):
    args = ['docker', 'compose', '--project-directory', str(OLD), '-p', 'omsbizfoundry', '--env-file', str(ENV)]
    for file in ('compose.yaml', 'compose.pull.yaml', 'compose.https.yaml'):
        args += ['-f', str(OLD / file)]
    args += ['-f', str(ROOT / 'compose.oa-public.json'), '--profile', '*']
    return run(args + list(command), timeout=240)


def parser(current):
    contracts = ROOT / 'artifacts/contracts'
    iam = current['IDENTITY']
    env = dict(item.split('=', 1) for item in iam['Config']['Env'])
    # Explicitly select v1.2; do not mutate the legacy policy document.
    env['PROJECT_ASSIGNMENT_POLICY_PATHS'] = env['PROJECT_ASSIGNMENT_POLICY_PATHS'].replace('/config/project-assignment-policy.json', '/config/project-assignment-policy-v1.2.json')
    args = ['docker', 'create', '--name', 'oa_public_20261002a_iam_parser', '--network', 'none', '--read-only', '--memory', '96m', '--cpus', '.15', '--cap-drop', 'ALL']
    for mount in iam['Mounts']:
        source, destination = mount['Source'], mount['Destination']
        if destination == '/config/project-workspace-capabilities.json':
            source = str(contracts / 'project-workspace-capabilities.json')
        if destination == '/config/project-assignment-policy.json':
            source, destination = str(contracts / 'project-assignment-policy-v1.2.json'), '/config/project-assignment-policy-v1.2.json'
        args += ['--mount', 'type=bind,src=' + source + ',dst=' + destination + ',readonly']
    for key, value in env.items():
        args += ['--env', key + '=' + value]
    run(args + [iam['Image'], '/app/ops', 'validate-contracts'])
    result = run(['docker', 'start', '--attach', 'oa_public_20261002a_iam_parser'])
    assert json.loads(result) == {'service': 'identity-access', 'status': 'contracts-valid'}
    state = inspect('oa_public_20261002a_iam_parser')['State']
    assert state['ExitCode'] == 0 and not state['OOMKilled']
    return env['PROJECT_ASSIGNMENT_POLICY_PATHS']


def prepare():
    assert not BACKUP.exists() and ENV.stat().st_mode & 0o077 == 0
    current = healthy()
    published = json.loads((ROOT / 'publication/results.json').read_text())
    assert published['passed'] and published['main_unchanged']
    manifest = json.loads((ROOT / 'artifacts/manifest.json').read_text())
    assert hashlib.sha256((ROOT / 'source.tar.gz').read_bytes()).hexdigest() == manifest['source_archive_sha256']
    for name, digest in manifest['contracts'].items():
        path = ROOT / 'artifacts/contracts' / name
        assert hashlib.sha256(path.read_bytes()).hexdigest() == digest
        path.chmod(0o444)
    if NO_SCHEMA_CHANGE:
        assert sql('project_workspace', 'SELECT max(version) FROM schema_migrations') == '000010_initial_dispatch_draft.sql'
        previous = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002a')
        for name, digest in manifest['contracts'].items():
            assert hashlib.sha256((previous / 'artifacts/contracts' / name).read_bytes()).hexdigest() == digest
        iam_env = dict(item.split('=', 1) for item in current['IDENTITY']['Config']['Env'])
        policy_paths = iam_env['PROJECT_ASSIGNMENT_POLICY_PATHS']
        assert policy_paths.startswith('/config/project-assignment-policy-v1.2.json,')
    else:
        policy_paths = parser(current)
    original = ENV.read_bytes()
    images = {}
    for key, container in current.items():
        info = json.loads(run(['docker', 'image', 'inspect', container['Image']]))[0]
        refs = [ref for ref in info['RepoDigests'] if '.aliyuncs.com/omsbizfoundry/' in ref]
        assert len(refs) == 1
        images['D1_' + key + '_IMAGE'] = refs[0]
    if not WORKBENCH_ONLY:
        images['D1_PROJECT_IMAGE'] = published['images']['project-workspace']['reference']
    images['D1_WORKBENCH_IMAGE'] = published['images']['unified-workbench']['reference']
    if BIDDING_SLICE:
        assert sql('business_engagement', 'SELECT max(version) FROM schema_migrations') == '000001_business_engagement.sql'
        images['D1_BUSINESS_ENGAGEMENT_IMAGE'] = published['images']['business-engagement']['reference']
    target_values = {'D1_IMAGE_MODE': 'pull', **images}
    lines = []
    for line in original.decode().splitlines():
        key = line.split('=', 1)[0]
        lines.append(key + '=' + target_values.pop(key) if key in target_values else line)
    lines += [key + '=' + value for key, value in target_values.items()]
    candidate = ('\n'.join(lines) + '\n').encode()
    override = {'services': {'identity-access': {'environment': {'PROJECT_ASSIGNMENT_POLICY_PATHS': policy_paths}, 'volumes': [str(ROOT / 'artifacts/contracts/project-workspace-capabilities.json') + ':/config/project-workspace-capabilities.json:ro', str(ROOT / 'artifacts/contracts/project-assignment-policy-v1.2.json') + ':/config/project-assignment-policy-v1.2.json:ro']}}}
    if NO_SCHEMA_CHANGE:
        override = json.loads((previous / 'compose.oa-public.json').read_text())
    if PROJECT_REFERENCE_CONFIG:
        project_env = dict(item.split('=', 1) for item in current['PROJECT']['Config']['Env'])
        assert not project_env.get('BUSINESS_ENGAGEMENT_BASE_URL'), 'diagnosed missing endpoint no longer holds'
        assert project_env['IDENTITY_PROVIDER'] == 'iam' and project_env.get('IAM_SERVICE_CREDENTIAL_URL')
        assert not project_env.get('BUSINESS_ENGAGEMENT_ACCESS_TOKEN')
        override = with_bidding_reference_endpoint(override)
    record(ROOT / 'compose.oa-public.json', override)
    BACKUP.mkdir(mode=0o700)
    if PROJECT_REFERENCE_CONFIG:
        # Private only: contains runtime Env. Never include in safe JSON exports.
        record(BACKUP / 'project-reference-before-private.json', {key: current['PROJECT'][key] for key in ('Image', 'Config', 'Mounts', 'HostConfig')})
    record(BACKUP / 'original.env', original, True)
    record(BACKUP / 'candidate.env', candidate, True)
    record(BACKUP / 'containers-before.json', {key: {'id': value['Id'], 'image': value['Image']} for key, value in current.items()})
    closure = ['Workbench'] if WORKBENCH_ONLY else ['Project server', 'Workbench'] if NO_SCHEMA_CHANGE else ['Project server/migration', 'Workbench', 'IAM catalog 0.6 and selected policy 1.2']
    if BIDDING_SLICE:
        closure = ['Business Engagement read API (schema unchanged)', 'Workbench B03 method/applicability connector']
    if PROJECT_REFERENCE_CONFIG:
        closure = ['Project declared BE endpoint configuration (same image/schema/IAM)', 'Workbench attachment-status text']
    record(BACKUP / 'preflight.json', {'passed': True, 'iam_contracts_unchanged': NO_SCHEMA_CHANGE, 'project_schema_before': sql('project_workspace', 'SELECT max(version) FROM schema_migrations'), 'closure': closure, 'gateway_suite_manifest_unchanged': True, 'other_business_images_unchanged': True, 'official_release': False})
    print(json.dumps({'phase': 'preflight', 'passed': True, 'closure': closure}), flush=True)


def backup():
    current = healthy()
    assert not (BACKUP / 'cold-backup.json').exists() and ENV.read_bytes() == (BACKUP / 'original.env').read_bytes()
    stopped = [value['Name'].lstrip('/') for key, value in current.items() if key not in ('POSTGRES', 'GATEWAY', 'WORKBENCH', 'NGINX')]
    run(['docker', 'stop', '--time', '30', *stopped])
    try:
        snapshot = {}
        for db in DBS:
            snapshot[db] = facts(db)
            path = BACKUP / (db + '.dump')
            with os.fdopen(os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), 'wb') as output:
                completed = subprocess.run(['docker', 'exec', PG, 'pg_dump', '-U', 'oms_admin', '-d', db, '-Fc', '--no-owner', '--no-privileges'], stdout=output, stderr=subprocess.PIPE, timeout=120)
                assert completed.returncode == 0
        assets = {}
        for key, destination in (('EVIDENCE', '/data/evidence'), ('IDENTITY', '/data/iam')):
            matches = [mount for mount in current[key]['Mounts'] if mount['Destination'] == destination]
            assert len(matches) == 1
            source = Path(matches[0]['Source'])
            assert source.is_dir() and '/var/lib/docker/volumes/' in str(source)
            archive = BACKUP / (key.lower() + '-files.tar.gz')
            with tarfile.open(archive, 'x:gz') as target:
                target.add(source, arcname='data', recursive=True)
            archive.chmod(0o600)
            assets[key] = {str(path.relative_to(source)): hashlib.sha256(path.read_bytes()).hexdigest() for path in source.rglob('*') if path.is_file()}
        record(BACKUP / 'facts-before.json', snapshot)
        record(BACKUP / 'assets-before.json', assets)
        checksums = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in BACKUP.iterdir() if path.is_file()}
        record(BACKUP / 'cold-backup.json', {'passed': True, 'databases': 13, 'checksums': checksums, 'evidence_provider': 'local', 'iam_keys_preserved': True})
        print(json.dumps({'phase': 'cold-joint-backup', 'passed': True, 'databases': 13, 'file_trees': 2}), flush=True)
    finally:
        run(['docker', 'start', *stopped])
        deadline = time.monotonic() + 90
        while True:
            try:
                healthy()
                break
            except AssertionError:
                assert time.monotonic() < deadline
                time.sleep(2)


def project_ops(private, command):
    project = inspect('omsbizfoundry-project-workspace-1')
    values = dict(item.split('=', 1) for item in project['Config']['Env'])
    if private:
        values['DATABASE_URL'] = (BACKUP / 'restore-project-url').read_text()
    name = 'oa_public_20261002' + RUN + '_' + ('restore' if private else 'main') + '_' + command + '_' + secrets.token_hex(3)
    image = (project['Image'] if WORKBENCH_ONLY else json.loads((ROOT / 'publication/results.json').read_text())['images']['project-workspace']['reference'])
    args = ['docker', 'create', '--name', name, '--network', NETWORK if private else 'omsbizfoundry_data', '--read-only', '--memory', '160m', '--cpus', '.2', '--cap-drop', 'ALL']
    for key, value in values.items():
        args += ['--env', key + '=' + value]
    for mount in project['Mounts']:
        if mount['Destination'] in ('/public', '/workload-private'):
            args += ['--mount', 'type=bind,src=' + mount['Source'] + ',dst=' + mount['Destination'] + ',readonly']
    run(args + [image, '/app/ops', command])
    output = run(['docker', 'start', '--attach', name])
    state = inspect(name)['State']
    assert state['ExitCode'] == 0 and not state['OOMKilled']
    return output.decode().strip()


def restore():
    healthy()
    assert not (BACKUP / 'restore.json').exists()
    receipt = json.loads((BACKUP / 'cold-backup.json').read_text())
    for name, digest in receipt['checksums'].items():
        assert hashlib.sha256((BACKUP / name).read_bytes()).hexdigest() == digest
    before = json.loads((BACKUP / 'facts-before.json').read_text())
    pg = inspect(PRIVATE_PG)
    assert not pg['HostConfig']['PortBindings'] and pg['HostConfig']['NanoCpus'] == 200000000
    run(['docker', 'start', PRIVATE_PG])
    try:
        deadline = time.monotonic() + 45
        while True:
            try:
                sql('postgres', 'SELECT 1', True)
                break
            except RuntimeError:
                assert time.monotonic() < deadline
                time.sleep(1)
        for db in DBS:
            target = 'oa_pub_20261002' + RUN + '_' + db
            owner, password = target + '_owner', secrets.token_hex(24)
            assert sql('postgres', "SELECT count(*) FROM pg_database WHERE datname='" + target + "'", True) == '0'
            sql('postgres', 'CREATE ROLE ' + owner + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '" + password + "';", True)
            sql('postgres', 'CREATE DATABASE ' + target + ' OWNER ' + owner, True)
            with (BACKUP / (db + '.dump')).open('rb') as source:
                completed = subprocess.run(['docker', 'exec', '-i', PRIVATE_PG, 'pg_restore', '-U', 'probe_admin', '--role', owner, '-d', target, '--single-transaction', '--exit-on-error', '--no-owner', '--no-privileges'], stdin=source, capture_output=True, timeout=120)
                assert completed.returncode == 0
            assert facts(target, True) == before[db]
            if db == 'project_workspace':
                record(BACKUP / 'restore-project-url', ('postgres://' + owner + ':' + password + '@' + PRIVATE_PG + ':5432/' + target + '?sslmode=disable').encode(), True)
            print(json.dumps({'phase': 'empty-restore', 'database': db, 'facts_and_sequences_equal': True}), flush=True)
        for key in ('EVIDENCE', 'IDENTITY'):
            directory = BACKUP / ('empty-restored-' + key.lower())
            directory.mkdir(mode=0o700)
            with tarfile.open(BACKUP / (key.lower() + '-files.tar.gz')) as archive:
                for member in archive.getmembers():
                    relative = Path(member.name)
                    assert relative.parts[0] == 'data' and not relative.is_absolute() and '..' not in relative.parts
                    assert member.isdir() or member.isfile()
                    target = directory / relative
                    if member.isdir():
                        target.mkdir(mode=0o700, parents=True, exist_ok=True)
                    else:
                        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
                        record(target, archive.extractfile(member).read(), True)
            restored = {str(path.relative_to(directory / 'data')): hashlib.sha256(path.read_bytes()).hexdigest() for path in (directory / 'data').rglob('*') if path.is_file()}
            assert restored == json.loads((BACKUP / 'assets-before.json').read_text())[key]
        target = 'oa_pub_20261002' + RUN + '_project_workspace'
        project_before = facts(target, True, ('schema_migrations',))
        project_ops(True, 'migrate')
        assert project_ops(True, 'check') == '000010_initial_dispatch_draft.sql'
        assert facts(target, True, ('schema_migrations',)) == project_before
        project_ops(True, 'migrate')  # idempotence, fresh one-shot is required below
        record(BACKUP / 'restore.json', {'passed': True, 'empty_databases': 13, 'facts_and_sequences_equal': True, 'files_and_iam_key_bytes_equal': True, 'schema_7_to_10': not NO_SCHEMA_CHANGE, 'schema_10_unchanged': NO_SCHEMA_CHANGE, 'existing_project_facts_unchanged': True, 'full_suite_restore_runtime': False})
    finally:
        if inspect(PRIVATE_PG)['State']['Running']:
            run(['docker', 'stop', '--time', '15', PRIVATE_PG])
    print(json.dumps({'phase': 'restore-and-migration-preflight', 'passed': True, 'schema': 10, 'full_suite_restore_runtime': False}), flush=True)


def apply():
    current = healthy()
    assert json.loads((BACKUP / 'restore.json').read_text())['passed']
    assert ENV.read_bytes() == (BACKUP / 'original.env').read_bytes()
    assert facts('project_workspace') == json.loads((BACKUP / 'facts-before.json').read_text())['project_workspace'], 'project changed since joint backup; re-backup before migration'
    if BIDDING_SLICE:
        assert facts('business_engagement') == json.loads((BACKUP / 'facts-before.json').read_text())['business_engagement']
    if not WORKBENCH_ONLY:
        run(['docker', 'stop', '--time', '30', 'omsbizfoundry-project-workspace-1'])
    migrated = False
    try:
        project_before = facts('project_workspace', exclude=('schema_migrations',))
        if not WORKBENCH_ONLY:
            project_ops(False, 'migrate')
            migrated = True
            assert project_ops(False, 'check') == '000010_initial_dispatch_draft.sql'
        assert facts('project_workspace', exclude=('schema_migrations',)) == project_before
        record(ENV, (BACKUP / 'candidate.env').read_bytes(), True)
        configuration = json.loads(compose('config', '--format', 'json'))
        assert not configuration['services']['project-workspace'].get('ports')
        assert configuration['services']['identity-access']['environment']['PROJECT_ASSIGNMENT_POLICY_PATHS'].startswith('/config/project-assignment-policy-v1.2.json,')
        changed_services = ['unified-workbench'] if WORKBENCH_ONLY else ['project-workspace', 'unified-workbench'] if NO_SCHEMA_CHANGE else ['identity-access', 'project-workspace', 'unified-workbench']
        changed_keys = ('WORKBENCH',) if WORKBENCH_ONLY else ('PROJECT', 'WORKBENCH') if NO_SCHEMA_CHANGE else ('IDENTITY', 'PROJECT', 'WORKBENCH')
        if BIDDING_SLICE:
            changed_services, changed_keys = ['business-engagement','unified-workbench'], ('BUSINESS_ENGAGEMENT','WORKBENCH')
        if PROJECT_REFERENCE_CONFIG:
            assert configuration['services']['project-workspace']['environment']['BUSINESS_ENGAGEMENT_BASE_URL'] == BIDDING_REFERENCE_ENDPOINT
            changed_services, changed_keys = ['project-workspace','unified-workbench'], ('PROJECT','WORKBENCH')
        compose('up', '-d', '--no-deps', '--no-build', '--pull', 'never', '--wait', '--wait-timeout', '100', *changed_services)
        after = healthy()
        if PROJECT_REFERENCE_CONFIG:
            validate_project_reference_repair(current['PROJECT'], after['PROJECT'])
        assert all(after[key]['Id'] == value['Id'] for key, value in current.items() if key not in changed_keys)
        assert facts('project_workspace', exclude=('schema_migrations',)) == project_before
        if BIDDING_SLICE:
            assert facts('business_engagement') == json.loads((BACKUP / 'facts-before.json').read_text())['business_engagement']
        run(['env', 'D1_ENV_FILE=' + str(ENV), 'D1_COMPOSE_OVERRIDE_FILE=' + str(ROOT / 'compose.oa-public.json'), str(OLD / 'ops/d1'), 'check'], timeout=240)
        record(BACKUP / 'rollout.json', {'passed': True, 'schema': 10, 'changed_containers': changed_services, 'other_container_ids_unchanged': 17-len(changed_services), 'project_facts_preserved': True, 'project_reference_config_only': PROJECT_REFERENCE_CONFIG, 'd1_check': True, 'official_release': False})
        print(json.dumps({'phase': 'main-rollout', 'passed': True, 'schema': 10, 'healthy': 17, 'other_container_ids_unchanged': 17-len(changed_services)}), flush=True)
    except Exception:
        # A successful forward migration permanently forbids starting the old Project binary.
        if not migrated and not WORKBENCH_ONLY:
            run(['docker', 'start', 'omsbizfoundry-project-workspace-1'])
        record(BACKUP / 'rollout-failure.json', {'migration_applied': migrated, 'old_binary_rollback_forbidden': migrated, 'data_volumes_and_joint_backup_retained': True})
        raise


def export_backup():
    assert json.loads((BACKUP / 'restore.json').read_text())['passed']
    import pwd
    target = Path('/home/ecs-user/oa-public-20261002' + RUN + '-backup.tar.gz')
    assert not target.exists()
    with tarfile.open(target, 'x:gz') as archive:
        archive.add(BACKUP, arcname=BACKUP.name)
    target.chmod(0o600)
    account = pwd.getpwnam('ecs-user')
    os.chown(target, account.pw_uid, account.pw_gid)
    digest = hashlib.sha256(target.read_bytes()).hexdigest()
    record(ROOT / 'offhost-export.json', {'archive_sha256': digest, 'bytes': target.stat().st_size, 'local_receipt_pending': True})
    print(json.dumps({'phase': 'backup-export', 'sha256': digest, 'bytes': target.stat().st_size}), flush=True)


if __name__ == '__main__':
    try:
        assert os.geteuid() == 0 and len(sys.argv) == 2 and sys.argv[1] in ('prepare', 'backup', 'restore', 'apply', 'export_backup')
        globals()[sys.argv[1]]()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('slice failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics retained')
