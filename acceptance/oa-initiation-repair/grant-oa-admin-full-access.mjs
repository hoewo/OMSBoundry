// Reconcile the installed Suite Capability Catalog to the explicit OA administrator.
// Secrets remain in memory; every grant is written through native IAM and audited.
import assert from 'node:assert/strict'
import {spawnSync} from 'node:child_process'
import {mkdirSync,readdirSync,readFileSync,writeFileSync} from 'node:fs'
import {setTimeout as delay} from 'node:timers/promises'
import {request} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs'

const output=process.argv[2]
assert(output&&/^\/tmp\/oa-admin-full-access\.[A-Za-z0-9]+$/.test(output),'use a private /tmp/oa-admin-full-access.<suffix> directory')
mkdirSync(output,{recursive:true})
const origin='https://oatest.antuinfo.cn'
const expectedSubject='12e800ac-00bf-4516-8748-b45c5623493a'
const manifestDirectory=new URL('../../contracts/access/v1/examples/',import.meta.url)
const expectedManifests=readdirSync(manifestDirectory)
  .filter(name=>name.endsWith('-capabilities.json'))
  .map(name=>JSON.parse(readFileSync(new URL(name,manifestDirectory),'utf8')))
  .map(value=>({application_id:value.application_id,application_version:value.application_version,audience:value.audience,actions:value.capabilities.map(item=>item.id)}))
  .sort((left,right)=>left.application_id.localeCompare(right.application_id))
assert.equal(expectedManifests.length,12,'expected exactly twelve enabled application manifests')

function remote(code){
  const encoded=Buffer.from(code).toString('base64')
  const result=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',`sudo python3 -c 'import base64;exec(base64.b64decode("${encoded}"))'`],{encoding:'utf8',timeout:180000})
  assert.equal(result.status,0,`private ECS inspection failed: ${result.stderr.slice(0,300)}`)
  return JSON.parse(result.stdout)
}

const snapshotCode=`import importlib.util,json,subprocess
spec=importlib.util.spec_from_file_location('costrelease','/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a/cost-release-ecs.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);b=m.b
container=json.loads(subprocess.check_output(['docker','inspect','omsbizfoundry-identity-access-1']))[0]
env=dict(value.split('=',1) for value in container['Config']['Env'])
manifests=[]
for path in env['CAPABILITY_MANIFEST_PATHS'].split(','):
 data=json.loads(subprocess.check_output(['docker','exec','omsbizfoundry-identity-access-1','cat',path]))
 manifests.append({'application_id':data['application_id'],'application_version':data['application_version'],'audience':data['audience'],'actions':[item['id'] for item in data['capabilities']]})
def rows(query):return json.loads(b.sql('identity_access',query))
admin=rows("SELECT row_to_json(t) FROM (SELECT id,username,display_name,status,administrator FROM subjects WHERE username='admin') t")
tenant=rows("SELECT coalesce(json_agg(to_jsonb(t) ORDER BY subject_id,audience,action),'[]'::json) FROM capability_grants t")
resource=rows("SELECT coalesce(json_agg(to_jsonb(t) ORDER BY subject_id,audience,action,resource_type,resource_id),'[]'::json) FROM capability_resource_grants t")
audit=rows("SELECT json_build_object('total',count(*),'grants',count(*) FILTER (WHERE action='iam_capability_granted')) FROM iam_audit_entries")
business={db:b.facts(db) for db in b.DBS if db!='identity_access'}
containers=[]
for raw in subprocess.check_output(['docker','ps','--format','{{json .}}']).decode().splitlines():
 value=json.loads(raw)
 if value['Names'].startswith('omsbizfoundry-'):containers.append({'name':value['Names'],'id':value['ID'],'image':value['Image'],'status':value['Status']})
print(json.dumps({'tenant_id':env['TENANT_ID'],'manifests':manifests,'admin':admin,'tenant_grants':tenant,'resource_grants':resource,'audit':audit,'business':business,'containers':sorted(containers,key=lambda v:v['name'])}))`

const secret=remote(`import json,subprocess
value=json.loads(subprocess.check_output(['docker','inspect','omsbizfoundry-identity-access-1']))[0]
env=dict(item.split('=',1) for item in value['Config']['Env'])
print(json.dumps({key:env[key] for key in ('BOOTSTRAP_ADMIN_USERNAME','BOOTSTRAP_ADMIN_PASSWORD')}))`)
const before=remote(snapshotCode)
assert.equal(before.admin.id,expectedSubject)
assert.equal(before.admin.username,'admin')
assert.equal(before.admin.status,'active')
assert.equal(before.admin.administrator,true)
assert.equal(before.manifests.length,12)
const liveManifests=[...before.manifests].sort((left,right)=>left.application_id.localeCompare(right.application_id))
assert.deepEqual(liveManifests,expectedManifests,'live Capability Catalog does not match the current versioned repository contracts')

const catalog=before.manifests.flatMap(manifest=>manifest.actions.map(action=>({audience:manifest.audience,action}))).sort((a,b)=>(a.audience+'\0'+a.action).localeCompare(b.audience+'\0'+b.action))
const key=value=>value.audience+'\0'+value.action
const stableContainers=values=>values.map(({name,id,image})=>({name,id,image}))
assert.equal(new Set(catalog.map(key)).size,catalog.length,'live Capability Catalog contains duplicate actions')
const activeBefore=new Set(before.tenant_grants.filter(value=>value.subject_id===expectedSubject&&value.revoked_at===null).map(key))
const missing=catalog.filter(value=>!activeBefore.has(key(value)))
const api=await request.newContext({baseURL:origin})
const receipt={schema_version:'1',passed:false,observed_at:new Date().toISOString(),origin,tenant_id:before.tenant_id,subject_id:expectedSubject,username:'admin',manifests:before.manifests.map(value=>({application_id:value.application_id,application_version:value.application_version,audience:value.audience,capability_count:value.actions.length})),catalog_capability_count:catalog.length,active_before:activeBefore.size,missing_before:missing.length,created_grants:[],credential_checks:[],rate_limit_retries:0}

function jwtPayload(token){
  const parts=token.split('.')
  assert.equal(parts.length,3,'IAM credential is not a compact JWS')
  return JSON.parse(Buffer.from(parts[1],'base64url').toString('utf8'))
}

try{
  const csrfResponse=await api.get('/auth/api/v1/csrf')
  assert.equal(csrfResponse.status(),200)
  const csrf=(await csrfResponse.json()).csrf_token
  const login=await api.post('/auth/api/v1/login',{headers:{'X-CSRF-Token':csrf},data:{username:secret.BOOTSTRAP_ADMIN_USERNAME,password:secret.BOOTSTRAP_ADMIN_PASSWORD}})
  assert.equal(login.status(),200,'native IAM login failed')
  const session=await api.get('/auth/api/v1/session')
  assert.equal(session.status(),200)
  const principal=(await session.json()).principal
  assert.equal(principal.subject_id,expectedSubject)
  assert.equal(principal.administrator,true)

  for(const capability of missing){
    let response=await api.post('/auth/api/v1/admin/grants',{headers:{'X-CSRF-Token':csrf},data:{subject_id:expectedSubject,audience:capability.audience,action:capability.action,scope:{kind:'tenant',resources:[]}}})
    if(response.status()===429){
      receipt.rate_limit_retries++
      await delay(61000)
      response=await api.post('/auth/api/v1/admin/grants',{headers:{'X-CSRF-Token':csrf},data:{subject_id:expectedSubject,audience:capability.audience,action:capability.action,scope:{kind:'tenant',resources:[]}}})
    }
    assert.equal(response.status(),204,`native IAM rejected ${capability.audience}/${capability.action}: HTTP ${response.status()}`)
    receipt.created_grants.push(capability)
  }

  const after=remote(snapshotCode)
  const activeAfter=after.tenant_grants.filter(value=>value.subject_id===expectedSubject&&value.revoked_at===null).map(key).sort()
  assert.deepEqual(activeAfter,catalog.map(key).sort(),'OA administrator does not exactly cover the installed Capability Catalog')
  assert.deepEqual(after.business,before.business,'a business database changed while granting IAM capabilities')
  assert.deepEqual(after.resource_grants,before.resource_grants,'resource-scoped grants changed')
  assert.deepEqual(after.tenant_grants.filter(value=>value.subject_id!==expectedSubject),before.tenant_grants.filter(value=>value.subject_id!==expectedSubject),'another subject tenant grant changed')
  assert.deepEqual(stableContainers(after.containers),stableContainers(before.containers),'a running Suite container identity or image changed')
  assert(before.containers.every(value=>/healthy/.test(value.status))&&after.containers.every(value=>/healthy/.test(value.status)),'a Suite container is not healthy')
  assert.equal(after.audit.grants-before.audit.grants,missing.length,'IAM grant audit count does not match created grants')

  for(const manifest of before.manifests){
    const response=await api.post('/auth/api/v1/credentials',{headers:{'X-CSRF-Token':csrf},data:{audience:manifest.audience,scope:{kind:'tenant',resources:[]}}})
    assert.equal(response.status(),200,`credential issuance failed for ${manifest.audience}`)
    const claims=jwtPayload((await response.json()).token)
    assert.equal(claims.sub,expectedSubject)
    assert.equal(claims.tid,before.tenant_id)
    assert.equal(claims.aud,manifest.audience)
    assert.equal(claims.scope.kind,'tenant')
    assert.deepEqual([...claims.actions].sort(),[...manifest.actions].sort(),`credential does not contain every ${manifest.audience} action`)
    receipt.credential_checks.push({audience:manifest.audience,action_count:claims.actions.length,scope:'tenant'})
  }

  receipt.active_after=activeAfter.length
  receipt.all_catalog_capabilities_issued=true
  receipt.business_databases_unchanged=Object.keys(before.business).length
  receipt.other_subject_tenant_grants_unchanged=true
  receipt.resource_grants_unchanged=true
  receipt.running_containers_unchanged=before.containers.length
  receipt.iam_grant_audits_added=missing.length
  receipt.passed=true
}catch(error){
  receipt.failure={type:error.name,message:error.message.slice(0,700)}
  process.exitCode=1
}finally{
  await api.dispose()
  const serialized=JSON.stringify(receipt,null,2)+'\n'
  assert(!/password|private_key|access_token|refresh_token|cookie|session/i.test(serialized),'secret-bearing field entered public receipt')
  writeFileSync(output+'/result.json',serialized,{mode:0o600})
  console.log(JSON.stringify({passed:receipt.passed,catalog_capabilities:receipt.catalog_capability_count,active_before:receipt.active_before,created:receipt.created_grants.length,active_after:receipt.active_after,credential_checks:receipt.credential_checks.length,rate_limit_retries:receipt.rate_limit_retries,failure:receipt.failure}))
}
