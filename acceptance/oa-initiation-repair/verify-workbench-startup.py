#!/usr/bin/env python3
"""Private standalone binary/asset startup, never a public or IAM journey."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import time
from urllib.request import urlopen


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--manifest', required=True)
    parser.add_argument('--source-root')
    args = parser.parse_args()
    binary, manifest = Path(args.binary).resolve(), Path(args.manifest).resolve()
    assert binary.is_file() and manifest.is_file()
    assert binary.name == 'server' and binary.parent.name.startswith('oa-material-workbench.')
    assert binary.parent.parent == Path('/tmp').resolve()
    source = Path(__file__).resolve().parents[2] / 'frontend/oms-workbench/internal/server'
    if args.source_root:
        source_root=Path(args.source_root).resolve()
        assert source_root.parent == Path('/tmp').resolve() and source_root.name.startswith(('oa-commercial-link-source.', 'oa-cost-release-source.'))
        source=source_root / 'frontend/oms-workbench/internal/server'
    assets = {
        '/assets/reference-oa-architecture-lens.js': 'reference_oa_architecture_lens.js',
        '/assets/reference-oa-bidding-runtime.js': 'reference_oa_bidding_runtime.js',
        '/assets/reference-oa-initiation-runtime.js': 'reference_oa_initiation_runtime.js',
        '/assets/reference-oa-business-runtime.js': 'reference_oa_business_runtime.js',
        '/assets/reference-oa-react-bidding.js': 'reference_oa_react_bidding.js',
    }
    expected = {route: hashlib.sha256((source / filename).read_bytes()).hexdigest() for route, filename in assets.items()}
    passes = []
    for cycle in range(2):
        with socket.socket() as reservation:
            reservation.bind(('127.0.0.1', 0))
            port = reservation.getsockname()[1]
        process = subprocess.Popen([str(binary)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, env={
            'PATH': os.environ.get('PATH', '/usr/bin:/bin'), 'GOMAXPROCS': '2',
            'HTTP_ADDR': '127.0.0.1:' + str(port), 'SUITE_MANIFEST_PATH': str(manifest),
        })
        base = 'http://127.0.0.1:' + str(port)
        try:
            deadline = time.monotonic() + 15
            while True:
                assert process.poll() is None, 'candidate failed before readiness'
                try:
                    with urlopen(base + '/health/ready', timeout=1) as response:
                        assert response.status == 200 and json.load(response)['status'] == 'ready'
                    break
                except OSError:
                    assert time.monotonic() < deadline, 'startup readiness timeout'
                    time.sleep(.1)
            with urlopen(base + '/', timeout=3) as response:
                page = response.read().decode()
                assert response.status == 200 and 'IAM' in page
                assert 'data-architecture-lens-open' not in page
                assert 'data-initiation-workspace' not in page
            for route, digest in expected.items():
                with urlopen(base + route, timeout=3) as response:
                    assert response.status == 200
                    assert response.headers['Content-Type'].startswith(('text/javascript', 'application/javascript'))
                    assert hashlib.sha256(response.read()).hexdigest() == digest, route + ' differs from candidate source'
            with urlopen(base + '/assets/reference-oa-business.css', timeout=3) as response:
                assert response.status == 200 and b'.bidding-file-slot' in response.read()
            with urlopen(base + '/assets/reference-oa-react-bidding.css', timeout=3) as response:
                assert response.status == 200 and response.headers['Content-Type'].startswith('text/css')
                assert hashlib.sha256(response.read()).hexdigest() == hashlib.sha256((source / 'reference_oa_react_bidding.css').read_bytes()).hexdigest()
            passes.append({'cycle': cycle + 1, 'readiness': True, 'anonymous_login_only': True, 'five_embedded_runtimes_match': True, 'original_react_css_matches': True, 'reference_css': True})
        finally:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
            assert process.poll() is not None
    print(json.dumps({'passed': True, 'cycles': passes, 'probe_stopped': True, 'assets_sha256': expected,
                      'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
                      'scope': 'Native standalone process with declared example Suite; no identity headers, protected user APIs, DB writes, Docker, Main deployment or public browser journey.'}, indent=2))


if __name__ == '__main__':
    main()
