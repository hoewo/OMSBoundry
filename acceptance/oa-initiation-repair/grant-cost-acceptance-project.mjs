// Human-approved seven Cost capabilities, two exact sides, one acceptance Project.
// Business DBs and grants are inspected read-only; grants go through native IAM.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,writeFileSync} from 'node:fs';
import {request} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const output=process.argv[2];assert(/^\/tmp\/oa-cost-grants\.[A-Za-z0-9]+$/.test(output));mkdirSync(output,{recursive:true});
const project='70edead7-9dc9-44a3-9b35-b9d41b83264e',subject='12e800ac-00bf-4516-8748-b45c5623493a';
const actions=['create','read','amount.read','correct','withdraw','evidence.manage','history.read'].map(v=>'cost-entry.'+v);
const resources=['business_cost_project','production_cost_project'].map(type=>({type,id:project}));
function remote(code){const encoded=Buffer.from(code).toString('base64');const r=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',"sudo python3 -c 'import base64;exec(base64.b64decode(\""+encoded+"\"))'"],{encoding:'utf8',timeout:60000});assert.equal(r.status,0,'private read-only verification unavailable');return JSON.parse(r.stdout);}
const inspect=`import importlib.util,json
s=importlib.util.spec_from_file_location('costrelease','/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a/cost-release-ecs.py');m=importlib.util.module_from_spec(s);s.loader.exec_module(m);b=m.b
assert b.sql('project_workspace',"SELECT project_number FROM projects WHERE id='${project}'")=='OA-VERIFY-20261002-C'
assert b.sql('cost_outsourcing','SELECT max(version) FROM schema_migrations')=='0003_reference_oa_cost_entries.sql'
q="SELECT coalesce(json_agg(to_jsonb(t) ORDER BY subject_id,audience,action,resource_type,resource_id),'[]'::json)::text FROM capability_resource_grants t"
rows=json.loads(b.sql('identity_access',q))
selected=[r for r in rows if r['subject_id']=='${subject}' and r['audience']=='cost-outsourcing' and r['resource_id']=='${project}']
others=[r for r in rows if r not in selected]
print(json.dumps({'selected':selected,'others':others,'tenant_grants':b.sql('identity_access',"SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t"),'business':{db:b.facts(db) for db in b.DBS if db!='identity_access'}}))`;
const secret=remote(`import json,subprocess
v=json.loads(subprocess.check_output(['docker','inspect','omsbizfoundry-identity-access-1']))[0];e=dict(x.split('=',1) for x in v['Config']['Env']);print(json.dumps({k:e[k] for k in ('BOOTSTRAP_ADMIN_USERNAME','BOOTSTRAP_ADMIN_PASSWORD')}))`);
const api=await request.newContext({baseURL:'https://oatest.antuinfo.cn'}),receipt={passed:false,subject_id:subject,project_id:project,project_number:'OA-VERIFY-20261002-C',authorization:'human explicitly approved this and later individually identified acceptance projects; no tenant or customer-project expansion',audience:'cost-outsourcing',actions,scope:{kind:'resources',resources},native_iam_requests:[]};
try{
 const csrf=await api.get('/auth/api/v1/csrf');assert.equal(csrf.status(),200);const token=(await csrf.json()).csrf_token;
 const login=await api.post('/auth/api/v1/login',{headers:{'X-CSRF-Token':token},data:{username:secret.BOOTSTRAP_ADMIN_USERNAME,password:secret.BOOTSTRAP_ADMIN_PASSWORD}});assert.equal(login.status(),200,'native IAM login failed');
 const session=await api.get('/auth/api/v1/session');assert.equal(session.status(),200);const principal=(await session.json()).principal;assert.equal(principal.subject_id,subject);assert.equal(principal.administrator,true);
 const before=remote(inspect);
 for(const action of actions){
  const status=await api.post('/auth/api/v1/admin/grants',{headers:{'X-CSRF-Token':token},data:{subject_id:subject,audience:'cost-outsourcing',action,scope:receipt.scope}});
  receipt.native_iam_requests.push({action,status:status.status()});assert.equal(status.status(),204,'native scoped grant rejected: '+action);
 }
 const after=remote(inspect);assert.deepEqual(after.others,before.others);assert.equal(after.tenant_grants,before.tenant_grants);assert.deepEqual(after.business,before.business);
 assert.equal(after.selected.length,14);for(const action of actions)for(const resource of resources)assert(after.selected.some(v=>v.action===action&&v.resource_type===resource.type&&v.resource_id===resource.id&&v.revoked_at===null));
 receipt.passed=true;receipt.resource_rows=14;receipt.other_resource_grants_unchanged=true;receipt.tenant_grants_unchanged=true;receipt.business_facts_sequences_unchanged=true;
}catch(e){receipt.failure={type:e.name,message:e.message.slice(0,300)};process.exitCode=1}
finally{await api.dispose();writeFileSync(output+'/grants.json',JSON.stringify(receipt,null,2)+'\n',{mode:0o600});console.log(JSON.stringify(receipt));}
