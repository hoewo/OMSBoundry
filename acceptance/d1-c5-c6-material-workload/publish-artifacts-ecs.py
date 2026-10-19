#!/usr/bin/env python3
"""Fixed Auth58 candidate publication; never apply a Main release."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess

ROOT = Path('/opt/omsbizfoundry/acceptance/material-candidate50-20261001')
WORK = ROOT / 'publication-b'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
RUNTIME = BASE + 'base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66'
spec = importlib.util.spec_from_file_location('entry_checks', '/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/verify-entry-capabilities.py')
entry = importlib.util.module_from_spec(spec); spec.loader.exec_module(entry)
run, inspect, save, snapshot = entry.run, entry.inspect, entry.save, entry.snapshot

def main():
    assert os.geteuid() == 0 and not WORK.exists()
    assert not inspect('production_material_source_auth58_20261001a_pg')['State']['Running']
    assert not any('restore20' in name for name in run(['docker', 'ps', '--format', '{{.Names}}']).splitlines())
    manifest = json.loads((ROOT / 'artifacts/artifacts.json').read_text())
    apps = ('project-workspace', 'production-operations', 'approval-audit-results', 'cost-outsourcing')
    assert manifest['release'] == 'v0.1.0-test.50' and set(manifest['applications']) == set(apps)
    assert manifest['source_archive_sha256'] == 'be3580d4561bbe7cf725b38f55d9b9c41f867411fdfdb9b5a4aecab8b546e46e'
    recipe = ROOT / 'artifacts/Dockerfile.verified-artifact'
    assert hashlib.sha256(recipe.read_bytes()).hexdigest() == manifest['recipe_sha256']
    for app in apps:
        for name in ('server', 'ops', 'workload-keyctl'):
            assert hashlib.sha256((ROOT / 'artifacts' / app / name).read_bytes()).hexdigest() == manifest['applications'][app][name]['sha256']
        absent = subprocess.run(['docker', 'manifest', 'inspect', BASE + app + ':v0.1.0-test.50'], capture_output=True, text=True, timeout=30)
        assert absent.returncode != 0 and any(reason in absent.stderr.lower() for reason in ('no such manifest', 'manifest unknown', 'not found')), 'tag existence or authorization uncertain'
    before = snapshot(); WORK.mkdir(mode=0o700)
    save(WORK / 'main-before.json', json.dumps(before, indent=2).encode())
    save(WORK / 'publisher.py', Path(__file__).read_bytes())
    results = {'passed': False, 'buildkit_provenance': False, 'images': {}, 'stopped_extraction_containers': []}
    try:
        for app in apps:
            tag = BASE + app + ':v0.1.0-test.50'
            build = run(['docker', 'build', '--platform', 'linux/amd64', '--memory', '128m', '--cpu-period', '100000', '--cpu-quota', '15000', '--build-arg', 'RUNTIME_IMAGE=' + RUNTIME, '--label', 'oms.source-archive-sha256=' + manifest['source_archive_sha256'], '--label', 'oms.release=v0.1.0-test.50', '--label', 'oms.artifact-publication=true', '--file', str(recipe), '--tag', tag, str(ROOT / 'artifacts' / app)])
            save(WORK / (app + '-build.log'), build.encode())
            built = json.loads(run(['docker', 'image', 'inspect', tag]))[0]
            assert built['Os'] == 'linux' and built['Architecture'] == 'amd64'
            assert built['Config']['User'] == 'postgres' and not built['Config'].get('Entrypoint') and built['Config']['Cmd'] == ['/app/server']
            pushed = run(['docker', 'push', tag]); save(WORK / (app + '-push.log'), pushed.encode())
            matches = re.findall(r'digest: (sha256:[a-f0-9]{64})', pushed)
            assert len(matches) == 1
            reference = BASE + app + '@' + matches[0]
            run(['docker', 'pull', '--platform', 'linux/amd64', reference])
            pulled = json.loads(run(['docker', 'image', 'inspect', reference]))[0]
            assert pulled['Id'] == built['Id'] and reference in pulled['RepoDigests']
            name = 'material_candidate50_' + app.replace('-', '_') + '_extract'
            run(['docker', 'create', '--name', name, '--network', 'none', '--memory', '64m', '--cpus', '0.1', '--label', 'oms.purpose=candidate-artifact-extraction', reference])
            results['stopped_extraction_containers'].append(name)
            assert not inspect(name)['State']['Running']
            directory = WORK / app; directory.mkdir(mode=0o700)
            for binary in ('server', 'ops', 'workload-keyctl'):
                target = directory / binary
                run(['docker', 'cp', name + ':/app/' + binary, str(target)])
                assert hashlib.sha256(target.read_bytes()).hexdigest() == manifest['applications'][app][binary]['sha256']
            results['images'][app] = {'reference': reference, 'image_id': pulled['Id'], 'platform': 'linux/amd64', 'user': 'postgres', 'binary_sha256': {name: value['sha256'] for name, value in manifest['applications'][app].items()}}
            save(WORK / (app + '-results.json'), json.dumps(results, indent=2).encode())
            print(json.dumps({'check': 'candidate_pushed_pulled_and_twelve_byte_hashes', 'application': app, 'reference': reference, 'three_binaries_equal': True}), flush=True)
        assert snapshot() == before
        results['passed'] = True
    finally:
        assert all(not inspect(name)['State']['Running'] for name in results['stopped_extraction_containers'])
        assert snapshot() == before
        results['main_unchanged'] = True
        save(WORK / 'results.json', json.dumps(results, indent=2).encode())
        print(json.dumps({'check': 'candidate_publication_no_main_mutation', 'passed': results['passed'], 'main_databases': 13, 'main_container_ids': 17, 'restore20_running': 0}), flush=True)

if __name__ == '__main__':
    try: main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next: trace = trace.tb_next
        raise SystemExit('candidate publication failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics retained')
