#!/usr/bin/env python3
"""Private Billing HTTP PG test only. Never publish a candidate into Main."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import time
import secrets
from urllib.parse import quote

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-invoice-candidate-20261004b')
PG = 'production_material_source_auth58_20261001a_pg'
NETWORK = 'production_material_source_auth58_20261001a_isolated'
NAME = 'oa_invoice_candidate_20261004b_test'
RUNTIME = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66'
FIXTURE = Path('/opt/omsbizfoundry/acceptance/material-workload-20261001/material_workload_auth58_20261001c')
spec = importlib.util.spec_from_file_location('reads', FIXTURE / 'verify-ecs.py')
reads = importlib.util.module_from_spec(spec); spec.loader.exec_module(reads)
run, inspect, save, snapshot, sql = reads.run, reads.inspect, reads.save, reads.snapshot, reads.sql

def main():
    assert os.geteuid() == 0 and ROOT.is_dir() and not (ROOT / 'results.json').exists()
    binary = ROOT / 'billing.test'; assert binary.is_file()
    pg = inspect(PG)
    assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings']
    assert pg['Config']['Image'] == RUNTIME and pg['HostConfig']['NanoCpus'] == 200000000
    assert set(pg['NetworkSettings']['Networks']) == {NETWORK}
    assert json.loads(run(['docker', 'network', 'inspect', NETWORK]))[0]['Internal']
    assert not any('restore20' in name for name in run(['docker','ps','--format','{{.Names}}']).splitlines())
    before=snapshot();save(ROOT/'main-before.json',json.dumps(before,indent=2).encode())
    result={'passed':False,'candidate_only':True,'public_ui_verified':False,'schema':1,'iam_provider':'real_verifier_test_issuer','original_components_verified':False}
    try:
        run(['docker','start',PG]);deadline=time.monotonic()+40
        while True:
            try:sql('postgres','SELECT 1');break
            except Exception:
                assert time.monotonic()<deadline;time.sleep(1)
        original=sql('postgres',"SELECT datname FROM pg_database WHERE datname LIKE 'money_flow_test_%' OR datname LIKE 'contract_ledger_system_%' ORDER BY datname")
        db='oa_invoice_candidate_20261004b'; owner=db+'_owner'; password=secrets.token_hex(24)
        assert sql('postgres',"SELECT count(*) FROM pg_database WHERE datname='"+db+"'")=='0'
        sql('postgres', 'CREATE ROLE '+owner+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '"+password+"';")
        sql('postgres','CREATE DATABASE '+db+' OWNER '+owner+';')
        connection='postgres://'+owner+':'+quote(password,safe='')+'@'+PG+':5432/'+db+'?sslmode=disable'
        save(ROOT/'private-test.env',('TEST_DATABASE_URL='+connection+'\nGOMAXPROCS=1\n').encode())
        result['tests']=[]
        for filename,testname,suffix in [('billing.test','TestProtectedHTTPWithNativeIAMPostgresAndTypedResolver','billing')]:
            candidate=ROOT/filename
            assert candidate.is_file()
            run(['chmod','0555',str(candidate)])
            process=subprocess.run(['docker','run','--name',NAME+'_'+suffix,'--network',NETWORK,'--cpus=.2','--memory=192m','--memory-swap=192m','--pids-limit=64','--read-only','--user=70:70',
                '--tmpfs','/tmp:rw,noexec,nosuid,size=16777216,uid=70,gid=70','--env-file',str(ROOT/'private-test.env'),'-v',str(candidate)+':/probe/test:ro','--entrypoint','/probe/test',RUNTIME,
                '-test.v','-test.run','^'+testname+'$','-test.timeout','270s'],capture_output=True,timeout=300)
            save(ROOT/(suffix+'-test.log'),process.stdout+process.stderr)
            output=process.stdout.decode(errors='replace')
            record={'test':testname,'exit_code':process.returncode,'started':('=== RUN   '+testname) in output,'passed':('--- PASS: '+testname) in output,'skipped':'SKIP' in output,'binary_sha256':hashlib.sha256(candidate.read_bytes()).hexdigest()}
            result['tests'].append(record)
            assert process.returncode==0 and record['started'] and record['passed'] and not record['skipped']
        assert sql('postgres',"SELECT datname FROM pg_database WHERE datname LIKE 'money_flow_test_%' OR datname LIKE 'contract_ledger_system_%' ORDER BY datname")==original
        assert sql(db,"SELECT count(*) FROM information_schema.schemata WHERE schema_name LIKE 'billing_http_%'")=='0'
        result['own_database']=db; result['test_schemas_cleaned']=True
        result['passed']=True
    except Exception as error:
        result['failure_type']=type(error).__name__;raise
    finally:
        if inspect(PG)['State']['Running']:run(['docker','stop','--time','15',PG])
        after=snapshot();save(ROOT/'main-after.json',json.dumps(after,indent=2).encode())
        result['main_unchanged']=before==after;result['private_pg_stopped']=not inspect(PG)['State']['Running']
        result['passed']=result['passed'] and result['main_unchanged'] and result['private_pg_stopped']
        save(ROOT/'results.json',json.dumps(result,indent=2).encode())
    assert result['passed'];print(json.dumps(result),flush=True)

if __name__=='__main__':
    try:main()
    except Exception as error:
        trace=error.__traceback__
        while trace.tb_next:trace=trace.tb_next
        raise SystemExit('invoice native candidate failed: '+type(error).__name__+' line '+str(trace.tb_lineno)+'; private diagnostics retained')


