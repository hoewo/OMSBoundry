#!/usr/bin/env python3
"""Bounded publication of test artifacts; never changes Main containers."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess

RUN = 'a'
ROOT = Path('/opt/omsbizfoundry/acceptance/oa-bidding-material-release-20261003' + RUN)
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
VERSION = 'v0.1.0-oa.20261003.1'
spec = importlib.util.spec_from_file_location('main_reads', '/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/verify-entry-capabilities.py')
reads = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reads)
run, inspect, save, snapshot = reads.run, reads.inspect, reads.save, reads.snapshot


def record(path, value):
    temporary = path.with_suffix('.next')
    with os.fdopen(os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w') as target:
        json.dump(value, target, indent=2)
    os.replace(temporary, path)


def main():
    assert os.geteuid() == 0
    assert not any('restore20' in n for n in run(['docker', 'ps', '--format', '{{.Names}}']).splitlines())
    manifest = json.loads((ROOT / 'artifacts/manifest.json').read_text())
    assert manifest['version'] == VERSION and manifest['official_release'] is False
    before = snapshot()
    work = ROOT / 'publication'
    if not work.exists():
        work.mkdir(mode=0o700)
        save(work / 'main-before.json', json.dumps(before).encode())
    results = {'passed': False, 'official_release': False, 'images': {}}
    try:
        for app, runtime, user in (
            ('project-workspace', BASE + 'base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66', 'postgres'),
            ('business-engagement', BASE + 'base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66', 'postgres'),
            ('unified-workbench', BASE + 'unified-workbench@sha256:61ff89daceb8d7313a994acf9388dbf0d1ecb1e4fbebad5f0652b55abeffcc96', 'oms'),
        ):
            if app == 'project-workspace':
                continue
            directory = ROOT / 'artifacts' / app
            for name, digest in manifest['applications'][app].items():
                assert hashlib.sha256((directory / name).read_bytes()).hexdigest() == digest
            tag = BASE + app + ':' + VERSION
            absent = subprocess.run(['docker', 'manifest', 'inspect', tag], capture_output=True, text=True, timeout=40)
            if absent.returncode == 0:
                # Recover only this exact source-labelled publication after an interrupted driver.
                run(['docker', 'pull', '--platform', 'linux/amd64', tag])
                existing = json.loads(run(['docker', 'image', 'inspect', tag]))[0]
                assert existing['Config']['Labels']['oms.source-archive-sha256'] == manifest['source_archive_sha256']
                refs = [ref for ref in existing['RepoDigests'] if ref.startswith(BASE + app + '@')]
                assert len(refs) == 1
                reference = refs[0]
            else:
                assert any(s in absent.stderr.lower() for s in ('no such manifest', 'manifest unknown', 'not found')), 'tag existence or authorization uncertain'
                output = run(['docker', 'build', '--platform', 'linux/amd64', '--memory', '128m', '--cpu-period', '100000', '--cpu-quota', '15000', '--build-arg', 'RUNTIME_IMAGE=' + runtime, '--label', 'oms.release=' + VERSION, '--label', 'oms.source-archive-sha256=' + manifest['source_archive_sha256'], '--label', 'oms.official-release=false', '-f', str(directory / 'Dockerfile'), '-t', tag, str(directory)])
                if not (work / (app + '-build.log')).exists():
                    save(work / (app + '-build.log'), output.encode())
                pushed = run(['docker', 'push', tag])
                if not (work / (app + '-push.log')).exists():
                    save(work / (app + '-push.log'), pushed.encode())
                matches = re.findall(r'digest: (sha256:[a-f0-9]{64})', pushed)
                assert len(matches) == 1
                reference = BASE + app + '@' + matches[0]
            run(['docker', 'pull', '--platform', 'linux/amd64', reference])
            image = json.loads(run(['docker', 'image', 'inspect', reference]))[0]
            assert image['Architecture'] == 'amd64' and image['Config']['User'] == user
            name = 'oa_material_20261003' + RUN + '_' + app.replace('-', '_') + '_extract'
            if name not in run(['docker', 'ps', '-a', '--format', '{{.Names}}']).splitlines():
                run(['docker', 'create', '--name', name, '--network', 'none', '--memory', '64m', '--cpus', '.1', reference])
            extracted = work / app
            extracted.mkdir(mode=0o700, exist_ok=True)
            hashes = {}
            for binary in ('server', 'ops', 'workload-keyctl') if app in ('project-workspace', 'business-engagement') else ('server',):
                target = extracted / binary
                run(['docker', 'cp', name + ':/app/' + binary, str(target)])
                hashes[binary] = hashlib.sha256(target.read_bytes()).hexdigest()
                assert hashes[binary] == manifest['applications'][app][binary]
            assert not inspect(name)['State']['Running']
            results['images'][app] = {'reference': reference, 'image_id': image['Id'], 'binary_sha256': hashes}
            record(work / 'results.json', results)
            print(json.dumps({'phase': 'published-and-byte-verified', 'application': app, 'reference': reference}), flush=True)
        assert snapshot() == before
        results.update(passed=True, main_unchanged=True)
    finally:
        record(work / 'results.json', results)
    assert results['passed']
    print(json.dumps({'phase': 'publication-complete', 'main_unchanged': True, 'images': len(results['images'])}), flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('publication failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics retained')
