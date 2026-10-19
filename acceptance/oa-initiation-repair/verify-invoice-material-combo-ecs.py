#!/usr/bin/env python3
"""Actual IAM/Billing/Evidence + own PG18/bytes cold restore. Project/Party
references are explicit HTTP fixtures, NOT actual applications or public OA.
"""
import base64
import calendar
import fcntl
import hashlib
import http.cookiejar
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import ipaddress
import json
import os
import re
from pathlib import Path
import secrets
import shlex
import threading
import time
import urllib.error
import urllib.request
import uuid

ROOT=Path('/opt/omsbizfoundry/acceptance/oa-invoice-material-combo-20261004a')
NAME='oa_invoice_material_combo_20261004a'
PG='production_material_source_auth58_20261001a_pg'
NETWORK='production_material_source_auth58_20261001a_isolated'
RUNTIME='crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66'
SOURCE=Path('/opt/omsbizfoundry/acceptance/production-material-source-20261001/production_material_source_auth58_20261001a')
PREVIOUS=Path('/opt/omsbizfoundry/acceptance/material-workload-20261001')
COMBO=Path('/opt/omsbizfoundry/acceptance/oa-contract-material-combo-20261004m')
TENANT='11111111-1111-4111-8111-111111111111'
PROJECT='22222222-2222-4222-8222-222222222222'
ISSUER='55555555-5555-4555-8555-555555555555'
RECIPIENT='66666666-6666-4666-8666-666666666666'
spec=importlib.util.spec_from_file_location('reads',PREVIOUS/'material_workload_auth58_20261001c/verify-ecs.py')
reads=importlib.util.module_from_spec(spec);spec.loader.exec_module(reads)
run,inspect,save,snapshot,sql,facts=reads.run,reads.inspect,reads.save,reads.snapshot,reads.sql,reads.facts
events=[]

def request(base,method,path,body=None,token=None,opener=None,csrf=None,expected=200,extra=None):
    headers={'Content-Type':'application/json',**(extra or {})}
    if token:headers['Authorization']='Bearer '+token
    if csrf:headers['X-CSRF-Token']=csrf
    req=urllib.request.Request(base+path,data=json.dumps(body).encode() if body is not None else None,headers=headers,method=method)
    try:response=opener.open(req,timeout=15) if opener else urllib.request.urlopen(req,timeout=15)
    except urllib.error.HTTPError as error:response=error
    assert response.status==expected,method+' '+path+' status='+str(response.status)+' expected='+str(expected)
    with response:content=response.read(1<<20)
    return json.loads(content) if content else None

def ready(base):
    deadline=time.monotonic()+90
    while True:
        try:request(base,'GET','/health/ready');return
        except (OSError,AssertionError,ValueError):
            assert time.monotonic()<deadline;time.sleep(1)

def mark(check,**values):
    events.append({'check':check,'passed':True,**values});print(json.dumps(events[-1]),flush=True)

class ReferenceFixture(BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_GET(self):
        # This fixture is not a verifier or business owner. Actual IAM-issued
        # request bindings are independently asserted from IAM audit records.
        assert self.headers.get('Authorization','').startswith('Bearer ')
        prefix='/api/v1/references/'
        if self.path==prefix+'projects/'+PROJECT:
            value={'schema_version':'1','source_application':'project-workspace','id':PROJECT,'tenant_id':TENANT,'resource_type':'project','status':'active','revision':1,'display_summary':'explicit Project HTTP fixture'}
        elif self.path in [prefix+'parties/'+ISSUER,prefix+'parties/'+RECIPIENT]:
            pid=self.path.split('/')[-1]
            value={'schema_version':'1','source_application':'party-directory','id':pid,'tenant_id':TENANT,'kind':'organization','status':'active','revision':1 if pid==ISSUER else 2,'legal_name':'explicit issuer fixture' if pid==ISSUER else 'explicit recipient fixture'}
        else:self.send_error(404);return
        self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(json.dumps(value).encode())

def main():
    assert os.geteuid()==0 and ROOT.is_dir() and not (ROOT/'results.json').exists()
    lock=(ROOT/'run.lock').open('a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
    pg=inspect(PG);network=json.loads(run(['docker','network','inspect',NETWORK]))[0]
    assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings'] and pg['Config']['Image']==RUNTIME and network['Internal']
    assert set(pg['NetworkSettings']['Networks'])=={NETWORK}
    assert not any('restore20' in n for n in run(['docker','ps','--format','{{.Names}}']).splitlines())
    before=snapshot();save(ROOT/'main-before.json',json.dumps(before).encode())
    result={'passed':False,'candidate_only':True,'real_iam':False,'real_billing_process':False,'real_evidence_process':False,'postgresql18':True,'project_party_reference_provider':'explicit_http_test_fixture','real_project_party':False,'external_evidence_provider':'local','public_entry':False,'oss_verified':False,'full_oa':False}
    for directory in ['bin','config','data']:(ROOT/directory).mkdir(mode=0o700,exist_ok=True)
    (ROOT/'data/objects').mkdir(mode=0o700)
    original=json.loads((SOURCE/'binaries.json').read_text());assert hashlib.sha256((SOURCE/'bin/iam').read_bytes()).hexdigest()==original['sha256']['iam']
    for name,path in [('iam',SOURCE/'bin/iam'),('keyctl',COMBO/'bin/keyctl'),('evidence',COMBO/'bin/evidence'),('evidence-ops',COMBO/'bin/evidence-ops'),('evidence-backup',COMBO/'bin/evidence-backup')]:save(ROOT/'bin'/name,path.read_bytes(),0o500)
    for name in ['billing','billing-ops']:assert (ROOT/'bin'/name).is_file();os.chmod(ROOT/'bin'/name,0o500)
    model=json.loads((PREVIOUS/'workload-identity-policy.json').read_text())['identities'][0]
    identity={**model,'id':'billing-reference-oa','subject_id':'44444444-4444-4444-8444-444444444444','display_name':'private Billing workload','public_keys_path':'/probe/data/workload-public.json','permissions':[{'audience':'party-directory','action':'party-directory.reference.read','resource_types':['party']},{'audience':'project-workspace','action':'project-workspace.read','resource_types':['project']},{'audience':'evidence-service','action':'evidence-service.metadata.read','resource_types':['file']}]}
    save(ROOT/'config/workload-policy.json',json.dumps({'schema_version':'1','identities':[identity]}).encode())
    cfg={'COMBO_ISSUER':'https://invoice-material-private.example.test/auth','IAM_ADMIN_PASSWORD':secrets.token_hex(24),'LOCAL_SIGNING_KEY':base64.urlsafe_b64encode(secrets.token_bytes(32)).decode().rstrip('=')}
    databases={name:'oa_invoice_combo_'+name+'_20261004a' for name in ['iam','billing','evidence','restored_billing','restored_evidence']};passwords={name:secrets.token_hex(24) for name in databases}
    for name,db in databases.items():cfg[name.upper()+'_DATABASE_URL']='postgres://'+db+'_owner:'+passwords[name]+'@'+PG+':5432/'+db+'?sslmode=disable'
    save(ROOT/'config/fixture.env',''.join(k+'='+shlex.quote(v)+'\n' for k,v in cfg.items()).encode())
    result['binary_sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in (ROOT/'bin').iterdir()}
    run(['chown','-R','70:70',str(ROOT)])
    gateway=network['IPAM']['Config'][0]['Gateway'];assert ipaddress.ip_address(gateway).is_private
    reference_server=ThreadingHTTPServer((gateway,0),ReferenceFixture);thread=threading.Thread(target=reference_server.serve_forever,daemon=True);thread.start()
    reference_url='http://'+gateway+':'+str(reference_server.server_port);started=False
    try:
        run(['docker','start',PG]);deadline=time.monotonic()+40
        while True:
            try:sql('postgres','SELECT 1');break
            except Exception:assert time.monotonic()<deadline;time.sleep(1)
        for name,db in databases.items():sql('postgres',"CREATE ROLE "+db+"_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '"+passwords[name]+"'; CREATE DATABASE "+db+' OWNER '+db+'_owner;')
        run(['docker','run','--detach','--name',NAME,'--network',NETWORK,'--memory=320m','--memory-swap=320m','--cpus=.35','--pids-limit=112','--read-only','--user=70:70','--tmpfs','/tmp:rw,noexec,nosuid,size=16777216,uid=70,gid=70','--mount','type=bind,src='+str(ROOT/'bin')+',dst=/probe/bin,readonly','--mount','type=bind,src='+str(ROOT/'config')+',dst=/probe/config,readonly','--mount','type=bind,src='+str(ROOT/'data')+',dst=/probe/data','--mount','type=bind,src='+str(ROOT/'start.sh')+',dst=/probe/start.sh,readonly','--entrypoint','sh',RUNTIME,'-c','while :; do sleep 2; done']);started=True
        state=inspect(NAME);assert not state['HostConfig']['PortBindings'];ip=state['NetworkSettings']['Networks'][NETWORK]['IPAddress'];assert ipaddress.ip_address(ip).is_private
        iam,billing,evidence='http://'+ip+':8080','http://'+ip+':8081','http://'+ip+':8085'
        def execute(script):return run(['docker','exec',NAME,'sh','-c','. /probe/config/fixture.env; '+script])
        run(['docker','exec','-d',NAME,'sh','-c','exec sh /probe/start.sh iam >/probe/data/iam.log 2>&1']);ready(iam);result['real_iam']=True
        def launch(mode):
            run(['docker','exec','-d','--env','COMBO_REFERENCE_URL='+reference_url,'--env','COMBO_EVIDENCE_PUBLIC_URL='+evidence,NAME,'sh','-c','exec sh /probe/start.sh '+mode+' >/probe/data/pair-'+mode+'.log 2>&1']);ready(billing);ready(evidence)
        launch('source');result['real_billing_process']=True;result['real_evidence_process']=True;mark('actual_iam_billing_evidence_ready',published_ports=0,project_party_fixture=True)
        opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()));csrf=request(iam,'GET','/api/v1/csrf',opener=opener)['csrf_token']
        request(iam,'POST','/api/v1/login',{'username':'fixture-admin','password':cfg['IAM_ADMIN_PASSWORD']},opener=opener,csrf=csrf);subject=request(iam,'GET','/api/v1/session',opener=opener)['principal']['subject_id']
        tenant_scope={'kind':'tenant','resources':[]};scope=lambda kind,id:{'kind':'resources','resources':[{'type':kind,'id':id}]}
        def grant(audience,action,value):request(iam,'POST','/api/v1/admin/grants',{'subject_id':subject,'audience':audience,'action':action,'scope':value},opener=opener,csrf=csrf,expected=204)
        def credential(audience,value):return request(iam,'POST','/api/v1/credentials',{'audience':audience,'scope':value},opener=opener,csrf=csrf)['token']
        grant('billing-invoice','billing-invoice.record',scope('project',PROJECT));create_token=credential('billing-invoice',scope('project',PROJECT))
        body={'command_id':str(uuid.uuid4()),'issued_on':'2026-10-04','amount_minor':900719925474099301,'currency':'CNY','issuer_party_ref':{'id':ISSUER,'revision':1},'recipient_party_ref':{'id':RECIPIENT,'revision':2},'source_refs':[{'kind':'project','resource_id':PROJECT,'source_revision':1}],'note':'actual Billing process, explicit reference fixtures'}
        request(billing,'POST','/api/v1/invoice-registrations',body,token=create_token,expected=403);assert sql(databases['billing'],'SELECT count(*) FROM invoice_registrations')=='0'
        for action in ['billing-invoice.read','billing-invoice.amount.read']:grant('billing-invoice',action,scope('project',PROJECT))
        create_token=credential('billing-invoice',scope('project',PROJECT));created=request(billing,'POST','/api/v1/invoice-registrations',body,token=create_token,expected=201);cid=created['id'];assert created['amount_minor_decimal']=='900719925474099301'
        for action in ['billing-invoice.read','billing-invoice.amount.read','billing-invoice.correct','billing-invoice.evidence.manage','billing-invoice.history.read','billing-invoice.registration.invalidate']:grant('billing-invoice',action,scope('invoice_registration',cid))
        token=credential('billing-invoice',scope('invoice_registration',cid));path='/api/v1/invoice-registrations/'+cid
        grant('evidence-service','evidence-service.upload',tenant_scope);upload_token=credential('evidence-service',tenant_scope);blob=b'%PDF-1.7\nactual private Billing Evidence file\n';digest=hashlib.sha256(blob).hexdigest()
        session=request(evidence,'POST','/v1/upload-sessions',{'file_name':'invoice.pdf','size_bytes':len(blob),'media_type':'application/pdf','sha256':digest},token=upload_token,expected=201,extra={'Idempotency-Key':str(uuid.uuid4())})
        with urllib.request.urlopen(urllib.request.Request(session['authorization']['upload_url'],data=blob,method='PUT',headers={'Content-Type':'application/pdf'}),timeout=15) as response:assert response.status==204
        version=request(evidence,'POST','/v1/upload-sessions/'+session['session_id']+'/complete',{'observed_size_bytes':len(blob),'observed_sha256':digest},token=upload_token,extra={'Idempotency-Key':str(uuid.uuid4())});assert version['status']=='available';fid=version['file_id']
        evidence_body={'command_id':str(uuid.uuid4()),'expected_revision':1,'reason':'登记已完成文件版本','evidence_refs':[{'file_id':fid,'version':1,'purpose':'invoice_document','display_name':'invoice.pdf'}]}
        value=request(billing,'PUT',path+'/evidence',evidence_body,token=token);assert value['revision']==2 and len(value['evidence_refs'])==1
        assert request(billing,'PUT',path+'/evidence',evidence_body,token=token)==value
        receipt=value
        value=request(billing,'GET',path,token=token)
        changed=[key for key in receipt if receipt[key]!=value[key]]
        assert set(changed)<=set(['created_at','updated_at']), 'persisted view differs: '+','.join(changed)
        def timestamp_ns(text):
            matched=re.fullmatch(r'(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?Z',text)
            assert matched, 'invalid UTC timestamp'
            seconds=calendar.timegm(time.strptime(matched.group(1),'%Y-%m-%dT%H:%M:%S'))
            return seconds*1000000000+int((matched.group(2) or '').ljust(9,'0'))
        for key in changed:
            assert abs(timestamp_ns(receipt[key])-timestamp_ns(value[key]))<=1000, 'timestamp exceeds PG precision'
        mark('exact_int64_create_blind_write_denied_actual_upload_owner_reference_replay',revision=2,sha256=digest)
        for action in ['evidence-service.metadata.read','evidence-service.download']:grant('evidence-service',action,scope('file',fid))
        file_token=credential('evidence-service',scope('file',fid))
        def download():
            access=request(evidence,'POST','/v1/files/'+fid+'/versions/1/access-grants',{'disposition':'attachment','delivery_mode':'direct'},token=file_token)
            with urllib.request.urlopen(access['url'],timeout=15) as response:assert response.read()==blob
        download();owned={kind:facts(databases[kind]) for kind in ['billing','evidence']}
        request(billing,'PUT',path+'/evidence',evidence_body,expected=401)
        neighbor_id=str(uuid.uuid4())
        for action in ['billing-invoice.read','billing-invoice.evidence.manage']:
            grant('billing-invoice',action,scope('invoice_registration',neighbor_id))
        neighboring=credential('billing-invoice',scope('invoice_registration',neighbor_id))
        request(billing,'PUT',path+'/evidence',evidence_body,token=neighboring,expected=403)
        def signal(name,sig):pid=int((ROOT/'data'/(name+'.pid')).read_text());assert pid>1;run(['docker','exec',NAME,'kill','-'+sig,str(pid)])
        signal('evidence','STOP')
        try:
            request(billing,'PUT',path+'/evidence',evidence_body,token=token)
            request(billing,'PUT',path+'/evidence',{**evidence_body,'command_id':str(uuid.uuid4()),'expected_revision':2,'evidence_refs':[{'file_id':fid,'version':1,'purpose':'invoice_document','display_name':'outage-probe.pdf'}]},token=token,expected=503)
            assert request(billing,'GET',path,token=token)==value
        finally:signal('evidence','CONT')
        assert {kind:facts(databases[kind]) for kind in ['billing','evidence']}==owned
        audits=json.loads(sql(databases['iam'],"SELECT coalesce(json_agg(details),'[]'::json)::text FROM iam_audit_entries WHERE action='iam_service_credential_issued' AND details->>'workload_id'='billing-reference-oa'"))
        expected={'party-directory':('party-directory.reference.read','party'),'project-workspace':('project-workspace.read','project'),'evidence-service':('evidence-service.metadata.read','file')}
        assert audits and any(a['audience']=='evidence-service' for a in audits)
        for audit in audits:
            action,kind=expected[audit['audience']];assert audit['actions']==[action] and audit['scope']['kind']=='resources' and len(audit['scope']['resources'])==1 and audit['scope']['resources'][0]['type']==kind
        mark('actual_issuer_precise_workload_scopes_and_outage_zero_write',audits=len(audits),anonymous=401,neighbor=403,outage=503)
        for name in ['billing','evidence']:signal(name,'TERM')
        execute('for name in billing evidence; do while kill -0 "$(cat /probe/data/$name.pid)" 2>/dev/null; do sleep 1; done; done')
        execute('pg_dump --no-owner --no-acl --dbname "$BILLING_DATABASE_URL" > /probe/data/billing.sql')
        execute('/probe/bin/evidence-backup create --database-url "$EVIDENCE_DATABASE_URL" --provider local --storage-root /probe/data/objects --output /probe/data/evidence.zip --confirm-service-stopped >/probe/data/evidence-backup.log 2>&1')
        execute('psql --set ON_ERROR_STOP=on --dbname "$RESTORED_BILLING_DATABASE_URL" < /probe/data/billing.sql >/probe/data/billing-restore.log 2>&1')
        execute('/probe/bin/evidence-backup restore --database-url "$RESTORED_EVIDENCE_DATABASE_URL" --provider local --storage-root /probe/data/restored-objects --input /probe/data/evidence.zip >/probe/data/evidence-restore.log 2>&1')
        assert facts(databases['restored_billing'])==owned['billing'] and facts(databases['restored_evidence'])==owned['evidence']
        launch('restored');token=credential('billing-invoice',scope('invoice_registration',cid));file_token=credential('evidence-service',scope('file',fid))
        assert request(billing,'GET',path,token=token)==value;download();assert request(billing,'PUT',path+'/evidence',evidence_body,token=token)==receipt
        correction={'command_id':str(uuid.uuid4()),'expected_revision':2,'reason':'恢复后纠正登记','patch':{'amount_minor':101,'note':'corrected after restore'}}
        corrected=request(billing,'POST',path+'/corrections',correction,token=token);assert corrected['revision']==3 and corrected['amount_minor_decimal']=='101'
        removed=request(billing,'PUT',path+'/evidence',{'command_id':str(uuid.uuid4()),'expected_revision':3,'reason':'移出引用，保留文件','evidence_refs':[]},token=token);assert removed['revision']==4 and not removed['evidence_refs'];download()
        rows=request(billing,'GET',path+'/history',token=token)['items'];assert len(rows)==4 and rows[-1]['snapshot']['amount_minor_decimal']=='900719925474099301'
        mark('cold_owned_db_and_bytes_empty_target_restore_continue',billing_schema=1,evidence_bytes_restored=True,revision=4,removal_keeps_bytes=True,project_party_fixture=True);result['passed']=True
    except Exception as error:
        result['failure_type']=type(error).__name__
        if isinstance(error,AssertionError):result['failure_check']=str(error)[:500]
        raise
    finally:
        reference_server.shutdown();reference_server.server_close()
        if started:
            pidfile=ROOT/'data/evidence.pid'
            if pidfile.exists():
                try:run(['docker','exec',NAME,'kill','-CONT',pidfile.read_text().strip()])
                except Exception:pass
            if inspect(NAME)['State']['Running']:run(['docker','stop','--time','15',NAME])
            assert not inspect(NAME)['State']['Running'] and not inspect(NAME)['State']['OOMKilled']
        if inspect(PG)['State']['Running']:run(['docker','stop','--time','15',PG])
        after=snapshot();save(ROOT/'main-after.json',json.dumps(after).encode());result['main_unchanged']=before==after;result['private_pg_and_runner_stopped']=True;result['checks']=events;result['passed']=result['passed'] and result['main_unchanged'];save(ROOT/'results.json',json.dumps(result,indent=2).encode())
    assert result['passed'];print(json.dumps(result),flush=True)

if __name__=='__main__':
    try:main()
    except Exception as error:
        trace=error.__traceback__
        while trace.tb_next:trace=trace.tb_next
        raise SystemExit('invoice actual-IAM combo failed: '+type(error).__name__+' line '+str(trace.tb_lineno)+'; private diagnostics retained')
