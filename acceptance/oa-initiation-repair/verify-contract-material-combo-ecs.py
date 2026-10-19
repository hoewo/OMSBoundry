#!/usr/bin/env python3
"""Real private IAM + Contract + Evidence upload/ref/access and cold restore.
No Main database/grant or user-project write. No Nginx/OSS/full-OA claim.
"""
import base64
import fcntl
import hashlib
import http.cookiejar
import importlib.util
import ipaddress
import json
import os
from pathlib import Path
import secrets
import shlex
import time
import urllib.error
import urllib.request
import uuid

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-contract-material-combo-20261004m')
NAME = 'oa_contract_material_combo_20261004m'
PG = 'production_material_source_auth58_20261001a_pg'
NETWORK = 'production_material_source_auth58_20261001a_isolated'
RUNTIME = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66'
SOURCE = Path('/opt/omsbizfoundry/acceptance/production-material-source-20261001/production_material_source_auth58_20261001a')
PREVIOUS = Path('/opt/omsbizfoundry/acceptance/material-workload-20261001')
TENANT = '11111111-1111-4111-8111-111111111111'
spec = importlib.util.spec_from_file_location('private_reads', PREVIOUS/'material_workload_auth58_20261001c/verify-ecs.py')
reads = importlib.util.module_from_spec(spec); spec.loader.exec_module(reads)
run, inspect, save, snapshot, sql, facts = reads.run, reads.inspect, reads.save, reads.snapshot, reads.sql, reads.facts
events = []

def mark(name, **values):
    events.append({'check':name,'passed':True,**values}); print(json.dumps(events[-1]),flush=True)

def request(base, method, path, body=None, token=None, opener=None, csrf=None, expected=200, extra=None):
    headers={'Content-Type':'application/json',**(extra or {})}
    if token: headers['Authorization']='Bearer '+token
    if csrf: headers['X-CSRF-Token']=csrf
    req=urllib.request.Request(base+path,data=json.dumps(body).encode() if body is not None else None,headers=headers,method=method)
    try: response=(opener.open(req,timeout=15) if opener else urllib.request.urlopen(req,timeout=15))
    except urllib.error.HTTPError as error: response=error
    assert response.status==expected,method+' '+path+' status='+str(response.status)+' expected='+str(expected)
    with response: content=response.read(1<<20)
    return json.loads(content) if content else None

def ready(base):
    deadline=time.monotonic()+90
    while True:
        try: request(base,'GET','/health/ready'); return
        except (OSError,AssertionError,ValueError):
            assert time.monotonic()<deadline,'private readiness timeout'
            time.sleep(1)

def main():
    assert os.geteuid()==0 and ROOT.is_dir() and not (ROOT/'results.json').exists()
    lock=(ROOT/'run.lock').open('a')
    fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
    pg=inspect(PG)
    assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings'] and pg['Config']['Image']==RUNTIME
    assert set(pg['NetworkSettings']['Networks'])=={NETWORK} and json.loads(run(['docker','network','inspect',NETWORK]))[0]['Internal']
    assert not any('restore20' in n for n in run(['docker','ps','--format','{{.Names}}']).splitlines())
    before=snapshot(); save(ROOT/'main-before.json',json.dumps(before).encode())
    result={'passed':False,'candidate_only':True,'real_iam':False,'real_contract_process':False,'real_evidence_process':False,'postgresql18':True,'external_evidence_provider':'local','public_entry':False,'oss_verified':False,'full_oa':False}
    for subdir in ('bin','config','data'): (ROOT/subdir).mkdir(mode=0o700,exist_ok=True)
    for name in ('objects','legacy'): (ROOT/'data'/name).mkdir(mode=0o700)
    original=json.loads((SOURCE/'binaries.json').read_text())
    assert hashlib.sha256((SOURCE/'bin/iam').read_bytes()).hexdigest()==original['sha256']['iam']
    save(ROOT/'bin/iam',(SOURCE/'bin/iam').read_bytes(),0o500)
    save(ROOT/'bin/keyctl',(PREVIOUS/'material-workload-keyctl').read_bytes(),0o500)
    for name in ('contract','contract-ops','contract-backup','evidence','evidence-ops','evidence-backup'): assert (ROOT/'bin'/name).is_file(); os.chmod(ROOT/'bin'/name,0o500)
    identity={'id':'contract-reference-oa','subject_id':'44444444-4444-4444-8444-444444444444','display_name':'private Contract reference workload','public_keys_path':'/probe/data/workload-public.json','permissions':[{'audience':'evidence-service','action':'evidence-service.metadata.read','resource_types':['file']}]}
    # Reuse the validated policy shape, not assumptions about its field names.
    model=json.loads((PREVIOUS/'workload-identity-policy.json').read_text())['identities'][0]
    identity={**model,**identity}
    save(ROOT/'config/workload-policy.json',json.dumps({'schema_version':'1','identities':[identity]}).encode())
    fixture=dict(line.split('=',1) for line in (SOURCE/'config/fixture.env').read_text().splitlines())
    admin_url=shlex.split(fixture['TEST_DATABASE_URL'])[0]; assert '@'+PG+':5432/postgres?' in admin_url
    cfg={'COMBO_ISSUER':'https://contract-material-private.example.test/auth','IAM_ADMIN_PASSWORD':secrets.token_hex(24),'LOCAL_SIGNING_KEY':base64.urlsafe_b64encode(secrets.token_bytes(32)).decode().rstrip('=')}
    databases={name:'oa_material_combo_'+name+'_20261004m' for name in ('iam','contract','evidence','restored_contract','restored_evidence')}
    passwords={name:secrets.token_hex(24) for name in databases}
    for name,db in databases.items(): cfg[name.upper()+'_DATABASE_URL']='postgres://'+db+'_owner:'+passwords[name]+'@'+PG+':5432/'+db+'?sslmode=disable'
    save(ROOT/'config/fixture.env',''.join(k+'='+shlex.quote(v)+'\n' for k,v in cfg.items()).encode())
    result['binary_sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in (ROOT/'bin').iterdir()}
    run(['chown','-R','70:70',str(ROOT)])
    started=False
    try:
        run(['docker','start',PG]); deadline=time.monotonic()+40
        while True:
            try: sql('postgres','SELECT 1'); break
            except Exception: assert time.monotonic()<deadline; time.sleep(1)
        for name,db in databases.items(): sql('postgres',"CREATE ROLE "+db+"_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '"+passwords[name]+"'; CREATE DATABASE "+db+' OWNER '+db+'_owner;')
        run(['docker','run','--detach','--name',NAME,'--network',NETWORK,'--memory=320m','--memory-swap=320m','--cpus=.35','--pids-limit=112','--read-only','--user=70:70','--tmpfs','/tmp:rw,noexec,nosuid,size=16777216,uid=70,gid=70','--mount','type=bind,src='+str(ROOT/'bin')+',dst=/probe/bin,readonly','--mount','type=bind,src='+str(ROOT/'config')+',dst=/probe/config,readonly','--mount','type=bind,src='+str(ROOT/'data')+',dst=/probe/data','--mount','type=bind,src='+str(ROOT/'start.sh')+',dst=/probe/start.sh,readonly','--entrypoint','sh',RUNTIME,'-c','while :; do sleep 2; done']); started=True
        state=inspect(NAME); assert not state['HostConfig']['PortBindings']
        ip=state['NetworkSettings']['Networks'][NETWORK]['IPAddress']; assert ipaddress.ip_address(ip).is_private
        iam,contract,evidence='http://'+ip+':8080','http://'+ip+':8081','http://'+ip+':8085'
        def execute(script): return run(['docker','exec',NAME,'sh','-c','. /probe/config/fixture.env; '+script])
        run(['docker','exec','-d',NAME,'sh','-c','exec sh /probe/start.sh iam >/probe/data/iam.log 2>&1']); ready(iam); result['real_iam']=True
        def launch(mode): run(['docker','exec','-d','--env','COMBO_EVIDENCE_PUBLIC_URL='+evidence,NAME,'sh','-c','exec sh /probe/start.sh '+mode+' >/probe/data/pair-'+mode+'.log 2>&1']); ready(contract); ready(evidence)
        launch('source'); result['real_contract_process']=True; result['real_evidence_process']=True; mark('real_private_processes_ready',published_ports=0,total_cpu_limit=.55)
        opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        csrf=request(iam,'GET','/api/v1/csrf',opener=opener)['csrf_token']
        request(iam,'POST','/api/v1/login',{'username':'fixture-admin','password':cfg['IAM_ADMIN_PASSWORD']},opener=opener,csrf=csrf)
        subject=request(iam,'GET','/api/v1/session',opener=opener)['principal']['subject_id']
        tenant_scope={'kind':'tenant','resources':[]}
        scope=lambda kind,id:{'kind':'resources','resources':[{'type':kind,'id':id}]}
        def grant(audience,action,value): request(iam,'POST','/api/v1/admin/grants',{'subject_id':subject,'audience':audience,'action':action,'scope':value},opener=opener,csrf=csrf,expected=204)
        def credential(audience,value): return request(iam,'POST','/api/v1/credentials',{'audience':audience,'scope':value},opener=opener,csrf=csrf)['token']
        for action in ('contract-ledger.view','contract-ledger.draft.manage'): grant('contract-ledger',action,tenant_scope)
        grant('evidence-service','evidence-service.upload',tenant_scope)
        create_token=credential('contract-ledger',tenant_scope)
        cid=request(contract,'POST','/api/v1/contracts',{'command_id':str(uuid.uuid4()),'number_namespace':'private-combo','contract_number':'REFERENCE-001','title':'private real file reference fixture','currency':'CNY','total_amount':'123.45','signed_on':'2026-10-04'},token=create_token)['receipt']['contract_id']
        for action in ('contract-ledger.view','contract-ledger.draft.manage','contract-ledger.evidence.manage'): grant('contract-ledger',action,scope('contract',cid))
        token=credential('contract-ledger',scope('contract',cid)); base='/api/v1/contracts/'+cid
        amend_id=str(uuid.uuid4())
        draft={'command_id':str(uuid.uuid4()),'expected_revision':0,'expected_draft_version':1,'project_id':'22222222-2222-4222-8222-222222222222','chain_type':'client_to_qualification','fees':{'design_fee':'123.45','survey_fee':'0','consultation_fee':'0'},'rate_bps':0,'settlement_follows_contract_amount':True,'settlement_amount':None,'amendments':[{'id':amend_id,'amendment_number':'SUP-001','signed_on':'2026-10-04','change_type':'no_amount_change','fees':{'design_fee':'0','survey_fee':'0','consultation_fee':'0'},'cancelled':False}]}
        request(contract,'POST',base+'/reference-oa-draft',draft,token=token)
        upload_token=credential('evidence-service',tenant_scope); commands=[]; versions=[]; blob=b'%PDF-1.7\nreal private contract Evidence file\n'; digest=hashlib.sha256(blob).hexdigest()
        for rev,kind in enumerate(('contract','amendment','finalization')):
            s=request(evidence,'POST','/v1/upload-sessions',{'file_name':kind+'.pdf','size_bytes':len(blob),'media_type':'application/pdf','sha256':digest},token=upload_token,expected=201,extra={'Idempotency-Key':str(uuid.uuid4())})
            with urllib.request.urlopen(urllib.request.Request(s['authorization']['upload_url'],data=blob,method='PUT',headers={'Content-Type':'application/pdf'}),timeout=15) as response: assert response.status==204
            version=request(evidence,'POST','/v1/upload-sessions/'+s['session_id']+'/complete',{'observed_size_bytes':len(blob),'observed_sha256':digest},token=upload_token,extra={'Idempotency-Key':str(uuid.uuid4())}); assert version['status']=='available'; versions.append(version)
            cmd={'command_id':str(uuid.uuid4()),'expected_revision':rev,'operation':'append','kind':kind,'file_id':version['file_id'],'version':version['version']}
            if kind=='amendment': cmd['amendment_id']=amend_id
            value=request(contract,'POST',base+'/reference-oa-materials',cmd,token=token); assert value['result']['materials']['revision']==rev+1; commands.append(cmd)
            assert request(contract,'POST',base+'/reference-oa-materials',cmd,token=token)['result']['replayed']
        saved=request(contract,'GET',base+'/workspace',token=token); assert len(saved['reference_oa_materials']['items'])==3
        assert all(row['file']['sha256']==digest for row in saved['reference_oa_materials']['items'])
        mark('three_real_upload_complete_owner_references_and_replay',material_revision=3,files=3)
        fid=versions[0]['file_id']
        for action in ('evidence-service.metadata.read','evidence-service.download'): grant('evidence-service',action,scope('file',fid))
        file_token=credential('evidence-service',scope('file',fid))
        def download():
            access=request(evidence,'POST','/v1/files/'+fid+'/versions/1/access-grants',{'disposition':'attachment','delivery_mode':'direct'},token=file_token)
            with urllib.request.urlopen(access['url'],timeout=15) as response: assert response.read()==blob
        download(); mark('real_exact_file_access_grant_and_byte_digest',sha256=digest)
        owned={kind:facts(databases[kind]) for kind in ('contract','evidence')}
        request(contract,'POST',base+'/reference-oa-materials',commands[0],expected=401)
        mark('anonymous_material_command_rejected',status=401)
        neighboring=credential('contract-ledger',scope('contract',str(uuid.uuid4())))
        request(contract,'POST',base+'/reference-oa-materials',commands[0],token=neighboring,expected=403)
        mark('neighbor_material_command_rejected',status=403)
        wrong={**commands[0],'command_id':str(uuid.uuid4()),'expected_revision':3,'file_id':str(uuid.uuid4())}
        request(contract,'POST',base+'/reference-oa-materials',wrong,token=token,expected=403)
        mark('missing_file_scope_rejected_without_write',status=403)
        def signal(name,sig): pid=int((ROOT/'data'/(name+'.pid')).read_text()); assert pid>1; run(['docker','exec',NAME,'kill','-'+sig,str(pid)])
        signal('evidence','STOP')
        try:
            request(contract,'POST',base+'/reference-oa-materials',commands[0],token=token)
            request(contract,'POST',base+'/reference-oa-materials',{**commands[0],'command_id':str(uuid.uuid4()),'expected_revision':3},token=token,expected=503)
            assert request(contract,'GET',base+'/workspace',token=token)['reference_oa_materials']==saved['reference_oa_materials']
        finally: signal('evidence','CONT')
        assert {kind:facts(databases[kind]) for kind in ('contract','evidence')}==owned
        audits=json.loads(sql(databases['iam'],"SELECT coalesce(json_agg(details),'[]'::json)::text FROM iam_audit_entries WHERE action='iam_service_credential_issued' AND details->>'workload_id'='contract-reference-oa'"))
        assert audits and all(a['audience']=='evidence-service' and a['actions']==['evidence-service.metadata.read'] and a['scope']['kind']=='resources' and len(a['scope']['resources'])==1 and a['scope']['resources'][0]['type']=='file' for a in audits)
        mark('real_iam_single_metadata_workload_and_outage_zero_write',audits=len(audits),anonymous=401,neighbor=403,missing_file_scope=403,outage=503)
        for name in ('contract','evidence'): signal(name,'TERM')
        execute('for name in contract evidence; do while kill -0 "$(cat /probe/data/$name.pid)" 2>/dev/null; do sleep 1; done; done')
        execute('/probe/bin/contract-backup create --database-url "$CONTRACT_DATABASE_URL" --evidence-dir /probe/data/legacy --output /probe/data/contract.zip >/probe/data/contract-backup.log 2>&1')
        execute('/probe/bin/evidence-backup create --database-url "$EVIDENCE_DATABASE_URL" --provider local --storage-root /probe/data/objects --output /probe/data/evidence.zip --confirm-service-stopped >/probe/data/evidence-backup.log 2>&1')
        execute('/probe/bin/contract-backup restore --database-url "$RESTORED_CONTRACT_DATABASE_URL" --evidence-dir /probe/data/restored-legacy --input /probe/data/contract.zip >/probe/data/contract-restore.log 2>&1')
        execute('/probe/bin/evidence-backup restore --database-url "$RESTORED_EVIDENCE_DATABASE_URL" --provider local --storage-root /probe/data/restored-objects --input /probe/data/evidence.zip >/probe/data/evidence-restore.log 2>&1')
        assert facts(databases['restored_contract'])==owned['contract'] and facts(databases['restored_evidence'])==owned['evidence']
        launch('restored'); token=credential('contract-ledger',scope('contract',cid)); file_token=credential('evidence-service',scope('file',fid))
        assert request(contract,'GET',base+'/workspace',token=token)['reference_oa_materials']==saved['reference_oa_materials']; download()
        assert request(contract,'POST',base+'/reference-oa-materials',commands[0],token=token)['result']['replayed']
        remove={'command_id':str(uuid.uuid4()),'expected_revision':3,'operation':'remove','kind':'contract','material_id':saved['reference_oa_materials']['items'][0]['id']}
        assert request(contract,'POST',base+'/reference-oa-materials',remove,token=token)['result']['materials']['revision']==4
        download(); mark('cold_app_backups_empty_targets_restore_ids_history_bytes_continue',contract_schema=22,evidence_bytes_restored=True,material_revision=4,removal_keeps_bytes=True)
        result['passed']=True
    except Exception as error:
        result['failure_type']=type(error).__name__
        if isinstance(error, AssertionError): result['failure_check']=str(error)[:500]
        raise
    finally:
        if started:
            pidfile=ROOT/'data/evidence.pid'
            if pidfile.exists():
                try: run(['docker','exec',NAME,'kill','-CONT',pidfile.read_text().strip()])
                except Exception: pass
            run(['docker','stop','--time','20',NAME]); result['private_runner_stopped']=not inspect(NAME)['State']['Running']
        if inspect(PG)['State']['Running']: run(['docker','stop','--time','15',PG])
        after=snapshot(); result['main_unchanged']=before==after; result['private_pg_stopped']=not inspect(PG)['State']['Running']; result['events']=events
        result['passed']=result['passed'] and result['main_unchanged'] and result['private_pg_stopped']
        save(ROOT/'results.json',json.dumps(result,indent=2).encode())
    assert result['passed']; print(json.dumps(result),flush=True)

if __name__=='__main__':
    try: main()
    except Exception as error:
        trace=error.__traceback__
        while trace.tb_next: trace=trace.tb_next
        raise SystemExit('private contract material combo failed: '+type(error).__name__+' line '+str(trace.tb_lineno)+'; private diagnostics retained')
