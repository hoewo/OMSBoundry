#!/usr/bin/env python3
"""Read-only guard for the continued C fixture and one explicit admin grant."""
import hashlib, importlib.util, json, os, sys, uuid
from pathlib import Path
ROOT=Path('/opt/omsbizfoundry/acceptance/oa-contributors-release-20261003a')
spec=importlib.util.spec_from_file_location('release',ROOT/'contributors-release-ecs.py')
release=importlib.util.module_from_spec(spec);spec.loader.exec_module(release)
b=release.b
ACTION='project-workspace.assignment.commercial-contributor.manage'
OUT=ROOT/'browser-ownership'

def fixture():
    rows=json.loads(b.sql('project_workspace',"SELECT coalesce(json_agg(id),'[]') FROM projects WHERE project_number='OA-VERIFY-20261002-C' AND name='OA 连续操作复测 20261002C（验收夹具）' AND status='active'"))
    assert len(rows)==1 and str(uuid.UUID(rows[0]))==rows[0]
    return rows[0]

def preserved(db, project):
    result={}
    for table in json.loads(b.sql(db,"SELECT coalesce(json_agg(tablename ORDER BY tablename),'[]') FROM pg_tables WHERE schemaname='public'")):
        if db=='identity_access' and table in ('iam_audit_entries','suite_sessions','workload_assertion_nonces'):continue
        where=''
        if db=='project_workspace':
            if table=='projects':where=" WHERE id <> '"+project+"'"
            elif table in ('project_assignments','project_history_events'):where=" WHERE project_id <> '"+project+"'"
            elif table=='project_commands':where=" WHERE coalesce(response_json->'project'->>'id','') <> '"+project+"'"
        if db=='identity_access' and table=='capability_grants':where=" WHERE action <> '"+ACTION+"'"
        quoted='"'+table.replace('"','""')+'"'
        result[table]=b.sql(db,"SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public."+quoted+' t'+where)
    return result

def detail(project):
    value=json.loads(b.sql('project_workspace',"SELECT to_jsonb(t)::text FROM projects t WHERE id='"+project+"'"))
    revision=value.pop('revision');value.pop('updated_at')
    result={'revision':revision,'unchanged_fields_sha256':hashlib.sha256(json.dumps(value,sort_keys=True).encode()).hexdigest()}
    for table in ('project_assignments','project_history_events'):
        rows=json.loads(b.sql('project_workspace',"SELECT coalesce(json_agg(json_build_object('id',id,'sha256',md5(to_jsonb(t)::text)) ORDER BY id),'[]') FROM "+table+" t WHERE project_id='"+project+"'"))
        result[table]={row['id']:row['sha256'] for row in rows}
    return result

def main():
    assert os.geteuid()==0 and sys.argv[1] in ('before','after')
    phase=sys.argv[1];project=fixture();OUT.mkdir(mode=0o700,exist_ok=True);target=OUT/(phase+'.json');assert not target.exists()
    b.healthy();facts={db:preserved(db,project) for db in b.DBS};current=detail(project)
    newgrants=json.loads(b.sql('identity_access',"SELECT coalesce(json_agg(json_build_object('subject_id',g.subject_id,'tenant_id',s.tenant_id,'audience',g.audience,'scope_kind',g.scope_kind)),'[]') FROM capability_grants g JOIN subjects s ON s.id=g.subject_id WHERE g.action='"+ACTION+"'"))
    if phase=='before':assert not newgrants
    else:
        prior=json.loads((OUT/'before.json').read_text())
        # Successful IAM login intentionally advances subjects.updated_at. Do not
        # exclude the account table: compare every account field to the retained
        # restored snapshot whose FULL digest equals the frozen before receipt.
        proof=json.loads((OUT/'login-metadata.json').read_text())
        assert proof['passed'] and proof['restored_baseline_full_digest_equals_frozen_before']
        assert proof['all_account_fields_except_updated_at_equal'] and proof['other_preserved_tables_equal']
        assert proof['baseline_subjects_digest']==prior['facts']['identity_access']['subjects']
        assert proof['current_subjects_digest']==facts['identity_access']['subjects'],'stale login-metadata proof'
        assert all(row['columns']==['updated_at'] and row['administrator'] is True for row in proof['changed_columns'])
        comparable=json.loads(json.dumps(facts))
        comparable['identity_access']['subjects']=proof['baseline_subjects_digest']
        assert comparable==prior['facts'],'non-fixture facts changed'
        assert current['unchanged_fields_sha256']==prior['fixture']['unchanged_fields_sha256']
        # public-d add/end, public-i's interrupted committed add, then public-j
        # end/rejoin with exact replay: five unique native commands, not a reset.
        assert current['revision']==prior['fixture']['revision']+5
        for identity,digest in prior['fixture']['project_history_events'].items():assert current['project_history_events'][identity]==digest
        for identity,digest in prior['fixture']['project_assignments'].items():assert current['project_assignments'][identity]==digest
        assert len(current['project_assignments'])==len(prior['fixture']['project_assignments'])+3
        assert len(current['project_history_events'])==len(prior['fixture']['project_history_events'])+5
        assert len(newgrants)==1 and newgrants[0]['audience']=='project-workspace' and newgrants[0]['scope_kind']=='tenant'
        assert newgrants[0]['tenant_id']==b.sql('project_workspace',"SELECT tenant_id::text FROM projects WHERE id='"+project+"'")
        subject=newgrants[0]['subject_id'];assert str(uuid.UUID(subject))==subject
        assert b.sql('identity_access',"SELECT count(*) FROM subjects WHERE id='"+subject+"' AND administrator=true")=='1'
    receipt={'passed':True,'phase':phase,'fixture_id':project,'fixture':current,'facts':facts,'explicit_admin_grants':len(newgrants),'business_writes_by_checker':0,'all_other_business_owner_facts_unchanged':phase=='after','iam_login_updated_at_only_proved':phase=='after','full_oa_accepted':False}
    b.record(target,receipt);print(json.dumps({k:v for k,v in receipt.items() if k not in ('facts','fixture')}),flush=True)

if __name__=='__main__':
    try:main()
    except Exception as error:
        trace=error.__traceback__
        while trace.tb_next:trace=trace.tb_next
        raise SystemExit('contributors ownership failed: '+type(error).__name__+' line '+str(trace.tb_lineno)+'; private details suppressed')
