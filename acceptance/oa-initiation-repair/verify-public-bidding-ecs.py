#!/usr/bin/env python3
"""Read back separately observed public Chrome B03 actions. No business writes.

Only fixture G may change. Preserve user records, native grants and all other
owners. Baseline/final checks are deliberately serial on the two-core ECS.
"""
import hashlib
import http.cookiejar
import importlib.util
import json
import os
from pathlib import Path
import sys
import urllib.parse
import urllib.request
import uuid

RUN = os.environ.get('OA_PUBLIC_RUN', 'h')
assert RUN in ('h', 'i', 'j', 'k')
RELEASE = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002' + RUN)
# Checker v2 keeps the failed j/v1 baseline immutable; it does not change the
# application release or create a replacement business fixture.
CHECKER_VERSION = 2
ROOT = RELEASE / ('bidding-browser-v2' if RUN == 'j' else 'bidding-browser')
PUBLIC = 'https://oatest.antuinfo.cn'
NUMBER = 'OA-VERIFY-20261002-G'
NAME = 'OA 招投标复测 20261002G（验收夹具）'
USER = 'f75d4570-5d6a-42ba-ade6-d1cda913ddda'
CONTINUED_FIXTURE = '3b4c44d3-7b33-4a29-add3-3f6211ca893e'
CONTINUED_SOURCE = '231f2977-79d4-4b36-a178-f19a0d8d3119'
EXPECTED = {'completed': None, 'comparison': ('comparison','applicable',3), 'negotiation': ('competitive_negotiation','applicable',4),
            'tender': ('tender','applicable',5), 'not-applicable': ('tender','not_applicable',6), 'restored': ('tender','applicable',7), 'final': ('tender','applicable',7)}
os.environ['OA_PUBLIC_RUN'] = RUN
spec = importlib.util.spec_from_file_location('bidding_deploy', RELEASE / 'deploy-public-slice-ecs.py')
deploy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(deploy)


def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,ensure_ascii=False).encode()).hexdigest()


def validate_continuation(result, baseline, completed):
    """Release i resumes the failed h UI journey; never create a new fixture."""
    assert baseline['passed'] and completed['passed']
    assert baseline['fixture_id'] is None and completed['fixture_id']==CONTINUED_FIXTURE
    assert completed['project_revision']==3 and completed['no_get_creation']
    assert result['fixture_id']==CONTINUED_FIXTURE
    for key in ('existing_rows','grants_digest','user_facts_sha256'):
        assert result[key]==baseline[key], key+' changed across the repair'
    for key in ('reference-oa-initiation-runtime.js','reference-oa-business-runtime.js'):
        assert result['assets'][key]==baseline['assets'][key], key+' unexpectedly changed'
    if result['other_business_databases'] is not None:
        assert result['other_business_databases']==baseline['other_business_databases']


def validate_receipt_repair(result, baseline, failed):
    assert baseline['passed'] and failed['passed'] is False
    assert failed['project']['id']==CONTINUED_FIXTURE and failed['project']['revision']==3
    assert len(failed['engagement'])==1
    source=failed['engagement'][0]
    assert source['id']==CONTINUED_SOURCE and source['project_id']==CONTINUED_FIXTURE
    assert source['revision']==1 and source['applicability']=='undetermined' and source['engagement_method'] is None
    assert result['fixture_id']==CONTINUED_FIXTURE and result['engagement_id']==CONTINUED_SOURCE
    for key in ('existing_rows','grants_digest','user_facts_sha256'):
        assert result[key]==baseline[key]


def preserved(db, fixture):
    tables = json.loads(deploy.sql(db,"SELECT coalesce(json_agg(tablename ORDER BY tablename),'[]') FROM pg_tables WHERE schemaname='public'"))
    result = {}
    for table in tables:
        columns = json.loads(deploy.sql(db,"SELECT json_agg(column_name) FROM information_schema.columns WHERE table_schema='public' AND table_name='"+table+"'"))
        where = ''
        if fixture:
            assert str(uuid.UUID(fixture)) == fixture
            if db == 'project_workspace':
                if table == 'projects': where=" WHERE id <> '"+fixture+"'"
                elif table == 'project_commands': where=" WHERE coalesce(response_json->>'id',response_json->'project'->>'id','') <> '"+fixture+"' AND command_id NOT IN (SELECT command_id FROM project_history_events WHERE project_id='"+fixture+"')"
                elif 'project_id' in columns: where=" WHERE project_id <> '"+fixture+"'"
            else:
                ids="SELECT id FROM business_engagements WHERE project_id='"+fixture+"'"
                if table == 'business_engagements': where=" WHERE project_id <> '"+fixture+"'"
                elif table == 'business_engagement_commands': where=" WHERE coalesce(response_json->'engagement'->>'project_id','') <> '"+fixture+"'"
                elif 'engagement_id' in columns: where=' WHERE engagement_id NOT IN ('+ids+')'
        quoted='"'+table.replace('"','""')+'"'
        result[table]=deploy.sql(db,"SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public."+quoted+' t'+where)
    return result


def main():
    assert os.geteuid()==0 and len(sys.argv)==2
    phase=sys.argv[1]
    assert phase in set(EXPECTED)|{'baseline'}
    ROOT.mkdir(mode=0o700,exist_ok=True)
    output=ROOT/(phase+'.json')
    assert not output.exists(), 'do not replace an observation'
    current=deploy.healthy()
    release=json.loads((deploy.BACKUP/'rollout.json').read_text())
    changed=['project-workspace','unified-workbench'] if RUN=='k' else ['unified-workbench'] if RUN in ('i','j') else ['business-engagement','unified-workbench']
    assert release['passed'] and release['changed_containers']==changed
    manifest=json.loads((RELEASE/'artifacts/manifest.json').read_text())
    publication=json.loads((RELEASE/'publication/results.json').read_text())
    prior=Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002h')
    business_publication=json.loads((prior/'publication/results.json').read_text()) if RUN in ('i','j','k') else publication
    assert current['BUSINESS_ENGAGEMENT']['Image']==business_publication['images']['business-engagement']['image_id']
    assert current['WORKBENCH']['Image']==publication['images']['unified-workbench']['image_id']
    previous=json.loads((deploy.BACKUP/'containers-before.json').read_text())
    changed_keys=('PROJECT','WORKBENCH') if RUN=='k' else ('WORKBENCH',) if RUN in ('i','j') else ('BUSINESS_ENGAGEMENT','WORKBENCH')
    assert all(current[key]['Id']==value['id'] for key,value in previous.items() if key not in changed_keys)
    if RUN=='k':
        assert current['PROJECT']['Image']==previous['PROJECT']['image'] and release['project_reference_config_only']
        project_env=dict(item.split('=',1) for item in current['PROJECT']['Config']['Env'])
        assert project_env['BUSINESS_ENGAGEMENT_BASE_URL']==deploy.BIDDING_REFERENCE_ENDPOINT
        assert project_env['IDENTITY_PROVIDER']=='iam' and not project_env.get('BUSINESS_ENGAGEMENT_ACCESS_TOKEN')
    matches=json.loads(deploy.sql('project_workspace',"SELECT coalesce(json_agg(id),'[]') FROM projects WHERE project_number='"+NUMBER+"' OR name='"+NAME+"'"))
    assert len(matches)<=1
    fixture=matches[0] if matches else None
    if RUN in ('i','j','k'): assert fixture==CONTINUED_FIXTURE
    else: assert (fixture is None)==(phase=='baseline')
    before={db:preserved(db,fixture) for db in ('project_workspace','business_engagement')}
    stable={db:deploy.facts(db) for db in deploy.DBS if db not in ('identity_access','project_workspace','business_engagement')} if phase in ('baseline','final') else None
    grants=deploy.facts('identity_access')['capability_grants']
    client=urllib.request.build_opener(urllib.request.ProxyHandler({}),urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    def request(path,method='GET',body=None,headers=None,expected=200):
        with client.open(urllib.request.Request(PUBLIC+path,data=body,headers=headers or {},method=method),timeout=30) as response:
            assert response.status==expected and urllib.parse.urlsplit(response.url).netloc=='oatest.antuinfo.cn'
            return response.read()
    def read(path): return json.loads(request(path))
    def authorize(app):
        token=read('/auth/api/v1/csrf')['csrf_token']
        request('/gateway/authorize/'+app,'POST',urllib.parse.urlencode({'csrf_token':token}).encode(),{'Content-Type':'application/x-www-form-urlencoded','Origin':PUBLIC})
    env=dict(item.split('=',1) for item in current['IDENTITY']['Config']['Env'])
    csrf=read('/auth/api/v1/csrf')['csrf_token']
    request('/auth/api/v1/login','POST',json.dumps({'username':env['BOOTSTRAP_ADMIN_USERNAME'],'password':env['BOOTSTRAP_ADMIN_PASSWORD']}).encode(),{'Content-Type':'application/json','X-CSRF-Token':csrf,'Origin':PUBLIC})
    result={'passed':False,'phase':phase,'checker_version':CHECKER_VERSION,'fixture_id':fixture,'business_api_writes':0,'grant_writes':0,'full_oa_accepted':False,'browser_click_observation_recorded_separately':True}
    try:
        authorize('project-workspace')
        user=read('/project/api/v1/projects/'+USER)
        user_history=read('/project/api/v1/projects/'+USER+'/history?limit=100')
        authorize('business-engagement')
        user_source=read('/business/api/v1/business-engagements?project_id='+USER+'&limit=2')
        assert user_source['project_id']==USER
        assert 'business-engagement.create' in user_source['allowed_actions']
        assert 'business-engagement.method.select' in user_source['allowed_actions']
        result.update(existing_rows=before,other_business_databases=stable,grants_digest=grants,user_facts_sha256=digest({'detail':user,'history':user_history,'engagement':user_source}),version=manifest['version'])
        result['assets']={}
        for asset,sha in manifest['assets'].items():
            actual=hashlib.sha256(request('/workbench/assets/'+asset)).hexdigest()
            assert actual==sha
            result['assets'][asset]=actual
        if fixture:
            detail=read('/project/api/v1/projects/'+fixture)
            assert detail['project']['status']=='active'
            result['project_revision']=detail['project']['revision']
            sources=read('/business/api/v1/business-engagements?project_id='+fixture+'&limit=2')
            assert sources['project_id']==fixture
            links=[v for v in detail['resource_links'] if v['target_service']=='business-engagement' and not v.get('unlinked_at')]
            if RUN in ('j','k') and phase in ('baseline','completed'):
                assert detail['project']['revision']==3 and len(sources['items'])==1 and links==[]
                native=read('/business/api/v1/business-engagements/'+sources['items'][0]['id'])
                item=native['engagement']
                assert item['id']==CONTINUED_SOURCE and item['project_id']==fixture
                assert item['revision']==1 and item['applicability']=='undetermined' and item.get('method') is None
                assert native['materials']==[]
                history=read('/business/api/v1/business-engagements/'+item['id']+'/history?limit=100')['items']
                assert len(history)==1
                result.update(engagement_id=item['id'],engagement_revision=1,no_get_creation=True,existing_source_reused=True)
                result['project_link_capability_returned']='project-workspace.resource-link.manage' in detail['allowed_actions']
                assert result['project_link_capability_returned']
            elif phase=='completed' or (RUN=='i' and phase=='baseline'):
                assert detail['project']['revision']==3 and sources['items']==[] and links==[]
                result['no_get_creation']=True
                result['project_link_capability_returned']='project-workspace.resource-link.manage' in detail['allowed_actions']
                assert result['project_link_capability_returned']
            else:
                assert len(sources['items'])==len(links)==1
                native=read('/business/api/v1/business-engagements/'+sources['items'][0]['id'])
                item=native['engagement']
                assert item['project_id']==fixture and item['tenant_id']==detail['project']['tenant_id']
                assert (item.get('method'),item['applicability'],item['revision'])==EXPECTED[phase]
                assert links[0]['resource_id']==item['id'] and detail['project']['revision']==4
                history=read('/business/api/v1/business-engagements/'+item['id']+'/history?limit=100')['items']
                assert len(history)==item['revision']
                assert native['materials']==[] and item['status']!='ready'
                result.update(engagement_id=item['id'],engagement_revision=item['revision'],method=item['method'],applicability=item['applicability'],native_history=history,requirements=native['requirements'],single_source_single_link=True,no_fake_material=True)
        assert {db:preserved(db,fixture) for db in before}==before
        assert deploy.facts('identity_access')['capability_grants']==grants
        if RUN in ('i','j','k'):
            validate_continuation(result,json.loads((prior/'bidding-browser/baseline.json').read_text()),json.loads((prior/'bidding-browser/completed.json').read_text()))
            result['same_failed_fixture_continued']=True
        if RUN in ('j','k'):
            second=Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002i/bidding-browser')
            validate_receipt_repair(result,json.loads((second/'baseline.json').read_text()),json.loads((second/'command-response-mismatch.json').read_text()))
        if RUN=='k':
            previous_checks=Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002j/bidding-browser-v2')
            previous_baseline=json.loads((previous_checks/'baseline.json').read_text())
            previous_failed=json.loads((previous_checks/'comparison.json').read_text())
            assert previous_baseline['passed'] and previous_failed['passed'] is False
            assert previous_failed['fixture_id']==CONTINUED_FIXTURE and previous_failed['project_revision']==3
            for key in ('existing_rows','grants_digest','user_facts_sha256'):
                assert result[key]==previous_baseline[key]==previous_failed[key]
        if phase!='baseline':
            baseline=json.loads((ROOT/'baseline.json').read_text())
            for key in ('existing_rows','grants_digest','user_facts_sha256','assets','version'): assert result[key]==baseline[key], key+' changed'
            if stable is not None: assert stable==baseline['other_business_databases']
            result['all_existing_rows_preserved']=True
        if phase=='final':
            checkpoints={key:json.loads((ROOT/(key+'.json')).read_text()) for key in EXPECTED if key!='final'}
            assert all(v['passed'] and v['fixture_id']==fixture for v in checkpoints.values())
            assert len({v['engagement_id'] for key,v in checkpoints.items() if key!='completed'})==1
            assert result['engagement_id']==checkpoints['restored']['engagement_id']
            assert result['native_history']==checkpoints['restored']['native_history']
            result['refresh_zero_additional_revision']=True
        result.update(passed=True,healthy=17,other_container_ids_unchanged=17-len(changed))
    finally:
        token=read('/auth/api/v1/csrf')['csrf_token']
        request('/auth/api/v1/logout','POST',b'',{'Content-Type':'application/json','X-CSRF-Token':token,'Origin':PUBLIC},204)
        deploy.record(output,result)
    print(json.dumps({key:value for key,value in result.items() if key in ('passed','phase','fixture_id','project_revision','engagement_revision','method','applicability','all_existing_rows_preserved','healthy','version','refresh_zero_additional_revision')}),flush=True)


if __name__=='__main__':
    try:
        main()
    except Exception as error:
        trace=error.__traceback__
        while trace.tb_next: trace=trace.tb_next
        raise SystemExit('bidding readback failed: '+type(error).__name__+' at line '+str(trace.tb_lineno)+'; private diagnostics retained')
