#!/usr/bin/env python3
"""Actual IAM workloading + Project/Results/Cost/Evidence read processes.
Bootstrap adjacent references remain declared fixtures. No Main/restore20 write.
"""
import base64
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

ROOT = Path('/opt/omsbizfoundry/acceptance/material-workload-20261001')
INCLUDE_PRODUCTION = os.environ.get('MATERIAL_INCLUDE_PRODUCTION') == 'yes'
NAME = 'material_workload_auth58_20261001c' if INCLUDE_PRODUCTION else 'material_workload_auth58_20261001b'
WORK = ROOT / NAME
SOURCE = Path('/opt/omsbizfoundry/acceptance/production-material-source-20261001/production_material_source_auth58_20261001a')
PG = 'production_material_source_auth58_20261001a_pg'
NETWORK = 'production_material_source_auth58_20261001a_isolated'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
RUNTIME = BASE+'base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66'
IAM = BASE+'identity-access@sha256:69806b6fbf055dcdbb50dcc93a135149e0a523684213ef4607bcc1c21a006266'
EVIDENCE_DIGEST = '4145253fb806fab382fe03a42dcbda335ed7597095fb4e10421f0a58e858ebdf'
TENANT = '11111111-1111-4111-8111-111111111111'
FILE = 'c5000000-0000-4000-8000-000000000010'
spec = importlib.util.spec_from_file_location('entry_checks','/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/verify-entry-capabilities.py')
entry = importlib.util.module_from_spec(spec);spec.loader.exec_module(entry)
run, inspect, save, snapshot = entry.run, entry.inspect, entry.save, entry.snapshot
events=[]

def mark(check, **values):
    event={'check':check,'result':'pass',**values};events.append(event);print(json.dumps(event),flush=True)

def request(base, method, path, body=None, token=None, opener=None, csrf=None, expected=200):
    headers={'Content-Type':'application/json'}
    if token:headers['Authorization']='Bearer '+token
    if csrf:headers['X-CSRF-Token']=csrf
    raw=json.dumps(body).encode() if body is not None else None
    req=urllib.request.Request(base+path,data=raw,headers=headers,method=method)
    try:response=opener.open(req,timeout=12) if opener else urllib.request.urlopen(req,timeout=12)
    except urllib.error.HTTPError as error:response=error
    assert response.status==expected, method+' '+path+' status='+str(response.status)
    with response:content=response.read(1<<20)
    return json.loads(content) if content else None

def wait_http(base, path='/health/ready'):
    deadline=time.monotonic()+75
    while True:
        try:request(base,'GET',path);return
        except (OSError,AssertionError,ValueError):
            assert time.monotonic()<deadline,'private process readiness timeout'
            time.sleep(1)

def sql(db, statement):
    return run(['docker','exec','-i',PG,'psql','-X','-A','-t','-q','-v','ON_ERROR_STOP=1','-U','probe_admin','-d',db],statement.encode()).strip()

def facts(db):
    tables=sql(db,"SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename").splitlines()
    assert tables and all(name.replace('_','').isalnum() for name in tables)
    values=[]
    for name in tables:values.append([name,sql(db,"SELECT count(*)::text||':'||md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public.\""+name+'" t')])
    return hashlib.sha256(json.dumps(values,ensure_ascii=False,separators=(',',':')).encode()).hexdigest()

def main():
    assert os.geteuid()==0 and not WORK.exists()
    pg=inspect(PG)
    assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings'] and pg['Config']['Image']==RUNTIME
    assert pg['Config']['Labels']['oms.purpose']=='production-material-source' and pg['HostConfig']['NanoCpus']==200000000
    assert set(pg['NetworkSettings']['Networks'])=={NETWORK} and json.loads(run(['docker','network','inspect',NETWORK]))[0]['Internal']
    assert not inspect('production_material_source_auth58_20261001a')['State']['Running']
    assert not any('restore20' in name for name in run(['docker','ps','--format','{{.Names}}']).splitlines())
    original=json.loads((SOURCE/'binaries.json').read_text())
    assert original['iam_image']==IAM and hashlib.sha256((SOURCE/'bin/iam').read_bytes()).hexdigest()==original['sha256']['iam']
    assert inspect('omsbizfoundry-evidence-service-1')['Config']['Image'].endswith('@sha256:'+EVIDENCE_DIGEST)
    before=snapshot();WORK.mkdir(mode=0o700)
    for subdir in ('bin','config','data'):(WORK/subdir).mkdir(mode=0o700)
    (WORK/'data/objects').mkdir(mode=0o700)
    save(WORK/'main-before.json',json.dumps(before,indent=2).encode())
    save(WORK/'verify-ecs.py',Path(__file__).read_bytes())
    save(WORK/'probe.sh',(ROOT/'probe.sh').read_bytes(),0o500)
    binary_names=[('project-material-process','project'),('material-workload-keyctl','keyctl'),('cost-material-process.test','cost.test'),('results-material-process.test','results.test')]
    if INCLUDE_PRODUCTION:binary_names.append(('production-material-process.test','production.test'))
    for source,target in binary_names:save(WORK/'bin'/target,(ROOT/source).read_bytes(),0o500)
    save(WORK/'bin/iam',(SOURCE/'bin/iam').read_bytes(),0o500)
    run(['docker','cp','omsbizfoundry-evidence-service-1:/app/server',str(WORK/'bin/evidence')]);os.chmod(WORK/'bin/evidence',0o500)
    manifests=[('project-workspace','project'),('evidence-service','evidence'),('approval-audit-results','results'),('cost-outsourcing','cost')]
    if INCLUDE_PRODUCTION:manifests.append(('production-operations','production'))
    for source,target in manifests:save(WORK/'config'/(target+'-capabilities.json'),(ROOT/(source+'-capabilities.json')).read_bytes())
    identity=json.loads((ROOT/'workload-identity-policy.json').read_text())['identities'][0]
    assert identity['id']=='project-workspace'
    identity['public_keys_path']='/probe/data/project-workload-public.json'
    audiences={'approval-audit-results','cost-outsourcing','evidence-service'}
    if INCLUDE_PRODUCTION:audiences.add('production-operations')
    identity['permissions']=[p for p in identity['permissions'] if p['audience'] in audiences]
    assert len(identity['permissions'])==(5 if INCLUDE_PRODUCTION else 4)
    save(WORK/'config/workload-policy.json',json.dumps({'schema_version':'1','identities':[identity]}).encode())
    fixture=dict(line.split('=',1) for line in (SOURCE/'config/fixture.env').read_text().splitlines())
    admin_url=shlex.split(fixture['TEST_DATABASE_URL'])[0];assert '@'+PG+':5432/postgres?' in admin_url
    config={'TEST_DATABASE_URL':admin_url,'MATERIAL_PROBE_ISSUER':'https://material-workload-fixture.example.test/auth','IAM_ADMIN_PASSWORD':secrets.token_hex(24),'LOCAL_SIGNING_KEY':base64.urlsafe_b64encode(secrets.token_bytes(32)).decode().rstrip('='),'MATERIAL_PROBE_PRODUCTION':'yes' if INCLUDE_PRODUCTION else 'no'}
    suffix='c' if INCLUDE_PRODUCTION else 'b'
    databases={name:'material_workload_'+name+'_'+suffix for name in ('iam','project','evidence')}
    passwords={name:secrets.token_hex(24) for name in databases}
    for name,db in databases.items():config[('IAM' if name=='iam' else name.upper())+'_DATABASE_URL']='postgres://'+db+'_owner:'+passwords[name]+'@'+PG+':5432/'+db+'?sslmode=disable'
    save(WORK/'config/fixture.env',''.join(key+'='+shlex.quote(value)+'\n' for key,value in config.items()).encode())
    save(WORK/'binaries.json',json.dumps({'sha256':{path.name:hashlib.sha256(path.read_bytes()).hexdigest() for path in (WORK/'bin').iterdir()},'iam_image':IAM,'evidence_image_digest':EVIDENCE_DIGEST,'runtime_image':RUNTIME},indent=2).encode())
    run(['chown','-R','70:70',str(WORK)])
    pg_started=runner_started=succeeded=False
    try:
        run(['docker','start',PG]);pg_started=True
        deadline=time.monotonic()+40
        while True:
            try:sql('postgres','SELECT 1');break
            except Exception:assert time.monotonic()<deadline;time.sleep(1)
        for name,db in databases.items():sql('postgres',"CREATE ROLE "+db+"_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '"+passwords[name]+"'; CREATE DATABASE "+db+' OWNER '+db+'_owner;')
        run(['docker','run','--detach','--name',NAME,'--network',NETWORK,'--memory','320m','--cpus','0.35','--pids-limit','112','--user','70:70','--label','oms.purpose=material-workload','--mount','type=bind,src='+str(WORK/'bin')+',dst=/probe/bin,readonly','--mount','type=bind,src='+str(WORK/'config')+',dst=/probe/config,readonly','--mount','type=bind,src='+str(WORK/'data')+',dst=/probe/data','--mount','type=bind,src='+str(WORK/'probe.sh')+',dst=/probe/start.sh,readonly','--entrypoint','sh',RUNTIME,'/probe/start.sh']);runner_started=True
        info=inspect(NAME);assert not info['HostConfig']['PortBindings'] and info['HostConfig']['NanoCpus']==350000000
        ip=info['NetworkSettings']['Networks'][NETWORK]['IPAddress'];assert ipaddress.ip_address(ip).is_private
        iam,project='http://'+ip+':8080','http://'+ip+':8086'
        wait_http(iam);wait_http(project);wait_http('http://'+ip+':8085')
        mark('private_iam_project_evidence_ready',max_total_cpu=0.55,memory_limits_mb=512,published_ports=0)
        opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        csrf=request(iam,'GET','/api/v1/csrf',opener=opener)['csrf_token']
        request(iam,'POST','/api/v1/login',{'username':'fixture-admin','password':config['IAM_ADMIN_PASSWORD']},opener=opener,csrf=csrf)
        subject=request(iam,'GET','/api/v1/session',opener=opener)['principal']['subject_id']
        scope=lambda id:{'kind':'resources','resources':[{'type':'project','id':id}]}
        def grant(action,value):request(iam,'POST','/api/v1/admin/grants',{'subject_id':subject,'audience':'project-workspace','action':action,'scope':value},opener=opener,csrf=csrf,expected=204)
        def credential(value):return request(iam,'POST','/api/v1/credentials',{'audience':'project-workspace','scope':value},opener=opener,csrf=csrf)['token']
        grant('project-workspace.create',{'kind':'tenant','resources':[]})
        create=credential({'kind':'tenant','resources':[]})
        def create_project(number):return request(project,'POST','/api/v1/projects',{'command_id':str(uuid.uuid4()),'project_number':number,'name':'synthetic workload fixture','project_type_keys':['design','consultation'] if INCLUDE_PRODUCTION else ['design']},token=create,expected=201)['id']
        project_id=create_project('WORKLOAD-PRIMARY');adjacent_id=create_project('WORKLOAD-ADJACENT')
        for id in (project_id,adjacent_id):
            for action in ('project-workspace.resource-link.manage','project-workspace.materials.read'):grant(action,scope(id))
        token,adjacent=credential(scope(project_id)),credential(scope(adjacent_id))
        save(WORK/'data/project-id',project_id.encode());os.chown(WORK/'data/project-id',70,70)
        wait_http('http://'+ip+':8095');wait_http('http://'+ip+':8096')
        sources=json.loads((WORK/'data/results-source.json').read_text())+json.loads((WORK/'data/cost-source.json').read_text())
        expected_types={'approval_audit_result','cost_entry','outsourcing_engagement'}
        if INCLUDE_PRODUCTION:
            wait_http('http://'+ip+':8097')
            sources+=json.loads((WORK/'data/production-source.json').read_text());expected_types.add('production-work-item')
        assert len(sources)==len(expected_types) and {s['resource_type'] for s in sources}==expected_types
        mark('selected_real_source_http_processes_ready',application_processes=6 if INCLUDE_PRODUCTION else 5)
        blob=b'synthetic metadata workload fixture\n';digest=hashlib.sha256(blob).hexdigest()
        save(WORK/'data/objects/workload-v2',blob);os.chown(WORK/'data/objects/workload-v2',70,70)
        sql(databases['evidence'],f"INSERT INTO files(tenant_id,id,created_at,created_by_subject) VALUES('{TENANT}','{FILE}',now(),'{subject}'); INSERT INTO file_versions(tenant_id,file_id,version,status,file_name,size_bytes,media_type,sha256,provider,bucket,object_key,created_at,created_by_subject,created_by_name) VALUES('{TENANT}','{FILE}',2,'available','synthetic-material.txt',{len(blob)},'text/plain','{digest}','local','','workload-v2',now(),'{subject}','fixture-admin');")
        commands=[]
        for i,source in enumerate(sources):
            command={'command_id':str(uuid.uuid4()),'expected_revision':i+1,'operation':'link','target_service':source['source_application'],'resource_type':source['resource_type'],'resource_id':source['id'],'source_revision':source['revision'],'relation_key':'source-materials'}
            linked=request(project,'POST','/api/v1/projects/'+project_id+'/resource-links',command,token=token,expected=200)
            assert linked['project']['revision']==i+2;commands.append(command)
        final_revision=len(sources)+1
        mark('canonical_links_real_iam_workload_target_verifiers',links=len(sources),project_revision=final_revision,source_bootstrap_references='explicit synthetic providers')
        owned_before={name:facts(databases[name]) for name in ('project','evidence')}
        def materials(label,states):
            value=request(project,'GET','/api/v1/projects/'+project_id+'/materials',token=token)
            assert value['project_revision']==final_revision and len(value['segments'])==len(sources)
            for segment in value['segments']:
                assert segment['state']==states.get(segment['source_application'],'available')
                if segment['state']=='available':assert segment['evidence_file_id']==FILE and segment['evidence_version']==2 and segment['evidence_digest']==digest and segment['source_revision']==next(s['revision'] for s in sources if s['id']==segment['resource_id'])
            assert not any(word in json.dumps(value) for word in ('object_key','access_token','bucket','amount_minor'))
            assert {name:facts(databases[name]) for name in ('project','evidence')}==owned_before
            save(WORK/'data'/('materials-'+label+'.json'),json.dumps(value,indent=2).encode())
        materials('available',{})
        request(project,'GET','/api/v1/projects/'+project_id+'/materials',token=adjacent,expected=403)
        request(project,'GET','/api/v1/projects/'+project_id+'/materials',expected=401)
        wrong={**commands[0],'command_id':str(uuid.uuid4()),'expected_revision':1}
        request(project,'POST','/api/v1/projects/'+adjacent_id+'/resource-links',wrong,token=adjacent,expected=422)
        assert facts(databases['project'])==owned_before['project']
        def signal(kind,sig):
            pid=int((WORK/'data'/(kind+'.pid')).read_text());assert pid>1
            run(['docker','exec',NAME,'kill','-'+sig,str(pid)])
        signal('results','STOP');materials('results-outage',{'approval-audit-results':'unavailable'})
        replay=request(project,'POST','/api/v1/projects/'+project_id+'/resource-links',commands[0],token=token,expected=200);assert replay['project']['revision']==2
        assert facts(databases['project'])==owned_before['project']
        signal('results','CONT');materials('results-recovered',{})
        if INCLUDE_PRODUCTION:
            signal('production','STOP');materials('production-outage',{'production-operations':'unavailable'})
            replay=request(project,'POST','/api/v1/projects/'+project_id+'/resource-links',commands[-1],token=token,expected=200);assert replay['project']['revision']==final_revision
            assert facts(databases['project'])==owned_before['project']
            signal('production','CONT');materials('production-recovered',{})
        signal('evidence','STOP');materials('evidence-outage',{s['source_application']:'unavailable' for s in sources})
        signal('evidence','CONT');materials('evidence-recovered',{})
        audits=json.loads(sql(databases['iam'],"SELECT coalesce(json_agg(details ORDER BY occurred_at),'[]'::json)::text FROM iam_audit_entries WHERE action='iam_service_credential_issued' AND details->>'workload_id'='project-workspace'"))
        expected={('evidence-service','evidence-service.metadata.read','file',FILE)}
        for source in sources:expected.add((source['source_application'],{'approval_audit_result':'approval-audit-result.read','cost_entry':'cost-entry.read','outsourcing_engagement':'outsourcing-engagement.read','production-work-item':'production-work-item.reference.read'}[source['resource_type']],source['resource_type'],source['id']))
        observed=set();safe=[]
        for audit in audits:
            assert len(audit['actions'])==1 and audit['scope']['kind']=='resources' and len(audit['scope']['resources'])==1
            resource=audit['scope']['resources'][0];item=(audit['audience'],audit['actions'][0],resource['type'],resource['id']);assert item in expected;observed.add(item)
            safe.append({'audience':item[0],'action':item[1],'resource_type':item[2],'resource_id':item[3]})
        assert observed==expected
        save(WORK/'data/workload-audit.json',json.dumps(safe,indent=2).encode())
        mark('native_single_action_exact_scope_iam_audit_no_download_or_amount',credential_audits=len(audits),unique_native_scopes=len(expected))
        mark('local_source_and_evidence_outages_recover_read_only',project_and_evidence_canonical=owned_before,anonymous=401,neighbor_project=403,wrong_project_link=422)
        assert snapshot()==before;succeeded=True
    finally:
        if runner_started:
            for kind in ('results','cost','evidence','production'):
                pidfile=WORK/'data'/(kind+'.pid')
                if pidfile.exists():
                    try:run(['docker','exec',NAME,'kill','-CONT',pidfile.read_text().strip()])
                    except Exception:pass
            run(['docker','stop','--time','20',NAME]);assert not inspect(NAME)['State']['Running']
            source_tests=[('results','TestServeResultsMaterialsForIsolatedWorkloadProbe'),('cost','TestServeCostMaterialsForIsolatedWorkloadProbe')]
            if INCLUDE_PRODUCTION:source_tests.append(('production','TestServeProductionMaterialsForIsolatedWorkloadProbe'))
            for kind,test in source_tests:
                log_path=WORK/'data'/(kind+'.log')
                if succeeded:
                    assert log_path.exists(), 'source process log missing'
                    log=log_path.read_text()
                    assert '--- PASS: '+test in log and '--- SKIP' not in log
            assert not inspect(NAME)['State']['OOMKilled']
        if pg_started:run(['docker','stop','--time','15',PG]);assert not inspect(PG)['State']['Running']
        assert snapshot()==before
        mark('probe_stopped_main_thirteen_databases_and_seventeen_ids_unchanged',restore20_running=0,removed_containers=0,removed_volumes=0)
        save(WORK/'results.json',json.dumps({'passed':succeeded,'events':events},indent=2).encode())

if __name__=='__main__':
    try:main()
    except Exception as error:
        trace=error.__traceback__
        while trace.tb_next:trace=trace.tb_next
        raise SystemExit('material workload check failed: '+type(error).__name__+' at line '+str(trace.tb_lineno)+'; private diagnostics retained')
