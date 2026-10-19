#!/usr/bin/env python3
"""Read-only fact guard for the single continued G browser fixture.

Only native BE commands and Evidence uploads from that fixture may differ.
Login/audit/nonces are explicitly dynamic; IAM grants and all other owner facts
must remain equal. Digests/IDs only, never credentials or private file bytes.
"""
import importlib.util, json, os, sys, uuid
from pathlib import Path
ROOT=Path('/opt/omsbizfoundry/acceptance/oa-bidding-material-release-20261003a')
spec=importlib.util.spec_from_file_location('deploy',ROOT/'deploy-bidding-materials-v2-ecs.py')
deploy=importlib.util.module_from_spec(spec);spec.loader.exec_module(deploy)
FIXTURE='3b4c44d3-7b33-4a29-add3-3f6211ca893e'
SOURCE='231f2977-79d4-4b36-a178-f19a0d8d3119'
TENANT='6539cd13-7190-40f1-8bd5-57f9e7c9b968'
OUT=ROOT/'material-browser-ownership'

def preserved(db):
    current={}
    if db=='evidence_service':
        ids=json.loads(deploy.sql('business_engagement',"SELECT coalesce(json_agg(DISTINCT evidence_file_id),'[]') FROM business_engagement_materials WHERE engagement_id='"+SOURCE+"'"))
        for identity in ids:assert str(uuid.UUID(identity))==identity
        if ids:
            values=','.join("'"+identity+"'" for identity in ids)
            sessions=json.loads(deploy.sql(db,"SELECT coalesce(json_agg(id),'[]') FROM upload_sessions WHERE file_id IN ("+values+")"))
            ids+=sessions
    for table in json.loads(deploy.sql(db,"SELECT coalesce(json_agg(tablename ORDER BY tablename),'[]') FROM pg_tables WHERE schemaname='public'")):
        if table=='schema_migrations':continue
        if db=='identity_access' and table in ('iam_audit_entries','suite_sessions','workload_assertion_nonces'):continue
        where=''
        if db=='business_engagement':
            if table=='business_engagements':where=" WHERE id <> '"+SOURCE+"'"
            elif table=='business_engagement_commands':where=" WHERE coalesce(response_json->'engagement'->>'id','') <> '"+SOURCE+"'"
            else:where=" WHERE engagement_id <> '"+SOURCE+"'"
        elif db=='evidence_service' and ids:
            where=' WHERE '+' AND '.join("to_jsonb(t)::text NOT LIKE '%"+identity+"%'" for identity in ids)
        quoted='"'+table.replace('"','""')+'"'
        current[table]=deploy.sql(db,"SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public."+quoted+' t'+where)
    return current

def main():
    assert os.geteuid()==0 and len(sys.argv)==2 and sys.argv[1] in ('before','after')
    phase=sys.argv[1];OUT.mkdir(mode=0o700,exist_ok=True)
    output=OUT/(phase+'.json');assert not output.exists(),'never replace an observation'
    deploy.healthy()
    facts={db:preserved(db) for db in deploy.DBS}
    if phase=='before':
        assert deploy.sql('business_engagement',"SELECT revision::text || ':' || tenant_id::text || ':' || project_id::text FROM business_engagements WHERE id='"+SOURCE+"'")=='7:'+TENANT+':'+FIXTURE
        assert deploy.sql('business_engagement',"SELECT count(*) FROM business_engagement_materials WHERE engagement_id='"+SOURCE+"'")=='0'
    else:
        prior=json.loads((OUT/'before.json').read_text())
        for db in deploy.DBS:assert facts[db]==prior['facts'][db],db+' non-fixture facts changed'
        assert deploy.sql('business_engagement',"SELECT revision::text || ':' || status FROM business_engagements WHERE id='"+SOURCE+"'")=='15:ready'
        assert deploy.sql('business_engagement',"SELECT count(*) FROM business_engagement_materials WHERE engagement_id='"+SOURCE+"'")=='7'
        assert deploy.sql('business_engagement',"SELECT count(*) FROM business_engagement_materials WHERE engagement_id='"+SOURCE+"' AND superseded_at IS NULL")=='6'
        assert deploy.sql('project_workspace',"SELECT revision FROM projects WHERE id='"+FIXTURE+"'")=='4'
    value={'passed':True,'phase':phase,'fixture_id':FIXTURE,'facts':facts,'grant_writes':0,'business_writes_by_checker':0,'project_facts_unchanged':phase=='after','all_non_fixture_owner_facts_unchanged':phase=='after'}
    deploy.record(output,value)
    print(json.dumps({key:value[key] for key in value if key!='facts'}),flush=True)

if __name__=='__main__':
    try:main()
    except Exception as error:
        trace=error.__traceback__
        while trace.tb_next:trace=trace.tb_next
        raise SystemExit('ownership check failed: '+type(error).__name__+' at line '+str(trace.tb_lineno)+'; private details suppressed')
