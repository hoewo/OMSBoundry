// Test-owned native IAM/Project/People/Contract composition. All databases are
// random names in the one task-owned loopback PG18 container; never Main.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {execFileSync,spawn,spawnSync} from 'node:child_process';
import {mkdirSync,writeFileSync} from 'node:fs';
import {randomUUID,createHash} from 'node:crypto';
import {applicationSourcePath} from './application-source-paths.mjs';

export async function createLiveBonusRuntime({root,output,container,tenant,project,person,assignments}){
  assert(/^\/tmp\/oa-bonus-components\.[A-Za-z0-9]+$/.test(output));
  assert.equal(container,'oms-bonus-overlay-test-20261004');
  const suffix=randomUUID().replaceAll('-','').slice(0,10),processes=new Map(),ports=new Map(),dbs={},proxies=[],exchanges=[];
  const workloadSubject=randomUUID(),issuer='https://bonus-live.private.test',password=randomUUID()+randomUUID();
  const exitCleanup=()=>{for(const {child} of processes.values())if(child.exitCode===null)child.kill('SIGTERM')};process.once('exit',exitCleanup);
  const safeEnv={PATH:process.env.PATH,GOMAXPROCS:'2'},privateKey=output+'/workload-private.json',publicKey=output+'/workload-public.json';
  let subject,csrf,iamURL,gatewayURL,cookies=new Map();
  const freePort=async()=>{const s=createServer();await new Promise(r=>s.listen(0,'127.0.0.1',r));const port=s.address().port;await new Promise(r=>s.close(r));return port};
  const sql=(app,statement)=>execFileSync('docker',['exec',container,'psql','-X','-A','-t','-v','ON_ERROR_STOP=1','-U','postgres','-d',dbs[app],'-c',statement],{encoding:'utf8'}).trim();
  const sha=value=>createHash('sha256').update(value).digest('hex');
  function sourceDigest(){return {project:sha(sql('project-workspace',"SELECT json_build_object('projects',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM projects p),'assignments',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM project_assignments a),'commands',(SELECT count(*) FROM project_commands),'history',(SELECT count(*) FROM project_history_events))")),people:sha(sql('people-groups',"SELECT json_build_object('persons',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM persons p),'revisions',(SELECT count(*) FROM person_revisions))")),contract_count:sql('contract-ledger','SELECT count(*) FROM contracts')};}
  function environment(app){const env={...safeEnv,HTTP_ADDR:'127.0.0.1:'+ports.get(app),DATABASE_URL:`postgres://postgres@127.0.0.1:55477/${dbs[app]}?sslmode=disable`,MIGRATION_MODE:'auto'};
    if(app==='gateway')return {...safeEnv,HTTP_ADDR:env.HTTP_ADDR,SUITE_MANIFEST_PATH:output+'/gateway-suite.json',IAM_PUBLIC_KEYS_PATH:output+'/iam-public.json',IAM_ISSUER:issuer,COOKIE_SECURE:'false',REQUESTS_PER_MINUTE:'10000'};
    if(app==='unified-workbench')return {...safeEnv,HTTP_ADDR:env.HTTP_ADDR,SUITE_MANIFEST_PATH:output+'/gateway-suite.json'};
    if(app==='identity-access')Object.assign(env,{TENANT_ID:tenant,TENANT_NAME:'原 OA 奖金真实来源私有验收',PUBLIC_ISSUER:issuer,SIGNING_KEY_FILE:output+'/iam-private.json',SESSION_COOKIE_SECURE:'false',BOOTSTRAP_ADMIN_USERNAME:'admin',BOOTSTRAP_ADMIN_DISPLAY_NAME:'隔离组合验收管理员',BOOTSTRAP_ADMIN_PASSWORD:password,CAPABILITY_MANIFEST_PATHS:['project-workspace','people-groups','contract-ledger','bonus-distribution'].map(v=>root+'contracts/access/v1/examples/'+v+'-capabilities.json').join(','),WORKLOAD_IDENTITY_POLICY_PATH:output+'/workload-policy.json'});
    else Object.assign(env,{IDENTITY_PROVIDER:'iam',IAM_ISSUER:issuer,IAM_TENANT_ID:tenant,IAM_PUBLIC_KEYS_PATH:output+'/iam-public.json'});
    if(app==='contract-ledger')env.EVIDENCE_STORAGE_DIR=output+'/contract-private-bytes';
    return env;
  }
  async function stop(app){const entry=processes.get(app);if(!entry||entry.child.exitCode!==null)return;await new Promise(r=>{entry.child.once('exit',r);entry.child.kill('SIGTERM')});}
  async function start(app){if(!ports.has(app))ports.set(app,await freePort());const child=spawn(output+'/'+app+'-server',[],{env:environment(app),stdio:['ignore','pipe','pipe']});let log='';child.stdout.on('data',v=>log+=v);child.stderr.on('data',v=>log+=v);processes.set(app,{child,log:()=>log});
    const url='http://127.0.0.1:'+ports.get(app);for(let n=0;n<120;n++){if(child.exitCode!==null)throw new Error(app+' failed: '+log);try{if((await fetch(url+'/health/ready')).ok)return url}catch{}await new Promise(r=>setTimeout(r,50))}throw new Error(app+' readiness timeout');}
  async function iam(path,input,expected=200){const r=await fetch(iamURL+path,{method:input===undefined?'GET':'POST',headers:{...(input===undefined?{}:{'Content-Type':'application/json','X-CSRF-Token':csrf}),Cookie:[...cookies].map(([k,v])=>k+'='+v).join('; ')},...(input===undefined?{}:{body:JSON.stringify(input)})});for(const value of r.headers.getSetCookie()){const [key,v]=value.split(';',1)[0].split('=');cookies.set(key,v)}const text=await r.text();assert.equal(r.status,expected,'IAM '+path+': '+text.slice(0,150));return text?JSON.parse(text):null;}
  const scope=side=>({kind:'resources',resources:[{type:side+'_bonus_project',id:project}]}),actions=['read','amount.read','record','withdraw','history.read'].map(v=>'bonus-manual-allocation.'+v);
  const grant=(side,action,revoke=false)=>iam('/api/v1/admin/grants'+(revoke?'/revoke':''),{subject_id:subject,audience:'bonus-distribution',action,scope:scope(side)},204);
  async function credential(side){const v=await iam('/api/v1/credentials',{audience:'bonus-distribution',scope:scope(side)});const claims=JSON.parse(Buffer.from(v.token.split('.')[1],'base64url'));assert.equal(claims.sub,subject);assert.deepEqual(claims.scope,scope(side));return v.token;}
  async function transparentProxy(target,kind){const proxy=createServer(async(req,res)=>{try{const buffers=[];for await(const b of req)buffers.push(b);const body=buffers.length?Buffer.concat(buffers):undefined;
      if(kind==='iam'&&body){const v=JSON.parse(body);exchanges.push({audience:v.audience,action:v.action,scope:v.scope});}
      const r=await fetch(target+req.url,{method:req.method,headers:{...(req.headers.authorization?{Authorization:req.headers.authorization}:{}),...(body?{'Content-Type':'application/json'}:{})},body});res.statusCode=r.status;res.setHeader('Content-Type','application/json');res.end(Buffer.from(await r.arrayBuffer()));
    }catch{res.statusCode=503;res.end('{"error":"native dependency unavailable"}')}});await new Promise(r=>proxy.listen(0,'127.0.0.1',r));proxies.push(proxy);return 'http://127.0.0.1:'+proxy.address().port;}
  async function cleanup(){for(const app of [...processes.keys()].reverse())await stop(app);for(const p of proxies)await new Promise(r=>p.close(r));for(const [app,entry] of processes)writeFileSync(output+'/'+app+'.log',entry.log(),{mode:0o600});process.removeListener('exit',exitCleanup);}
  try{
    for(const app of ['identity-access','project-workspace','people-groups','contract-ledger']){
      const b=spawnSync('go',['build','-o',output+'/'+app+'-server','./cmd/server'],{cwd:applicationSourcePath(root,app),env:{...process.env,GOWORK:'off'},encoding:'utf8'});assert.equal(b.status,0,b.stderr);dbs[app]='bonus_live_'+suffix+'_'+app.replaceAll('-','_');execFileSync('docker',['exec',container,'createdb','-U','postgres',dbs[app]]);
    }
    const gatewayBuild=spawnSync('go',['build','-o',output+'/gateway-server','./cmd/server'],{cwd:root+'backend/platform/gateway',env:{...process.env,GOWORK:'off'},encoding:'utf8'});assert.equal(gatewayBuild.status,0,gatewayBuild.stderr);
    const workbenchBuild=spawnSync('go',['build','-o',output+'/unified-workbench-server','./cmd/server'],{cwd:root+'frontend/oms-workbench',env:{...process.env,GOWORK:'off'},encoding:'utf8'});assert.equal(workbenchBuild.status,0,workbenchBuild.stderr);
    const b=spawnSync('go',['build','-o',output+'/keyctl','./cmd/workload-keyctl'],{cwd:root+'packages/workload-identity',env:{...process.env,GOWORK:'off'},encoding:'utf8'});assert.equal(b.status,0,b.stderr);
    execFileSync(output+'/keyctl',['init'],{env:{...safeEnv,WORKLOAD_ID:'bonus-distribution',WORKLOAD_SUBJECT_ID:workloadSubject,WORKLOAD_KEY_FILE:privateKey,WORKLOAD_PUBLIC_KEY_FILE:publicKey}});
    writeFileSync(output+'/workload-policy.json',JSON.stringify({schema_version:'1',identities:[{id:'bonus-distribution',subject_id:workloadSubject,display_name:'独立 Bonus',public_keys_path:publicKey,permissions:[{audience:'project-workspace',action:'project-workspace.read',resource_types:['project']},{audience:'people-groups',action:'people-groups.person.reference.read',resource_types:['person']}]}]}),{mode:0o600});
    iamURL=await start('identity-access');const keys=await fetch(iamURL+'/api/v1/.well-known/jwks.json');assert(keys.ok);writeFileSync(output+'/iam-public.json',await keys.text(),{mode:0o600});
    csrf=(await iam('/api/v1/csrf')).csrf_token;await iam('/api/v1/login',{username:'admin',password});subject=(await iam('/api/v1/session')).principal.subject_id;
    const projectURL=await start('project-workspace'),peopleURL=await start('people-groups');mkdirSync(output+'/contract-private-bytes');const contractURL=await start('contract-ledger');
    // Explicit source-owned fixture setup; runtime never reads their tables.
    sql('people-groups',`INSERT INTO persons(id,tenant_id,reference_code,display_name,status,revision,created_at,updated_at) VALUES ('${person}','${tenant}','BONUS-LIVE','同人多分工验收人员','active',2,now(),now())`);
    sql('project-workspace',`INSERT INTO projects(id,tenant_id,project_number,project_number_normalized,name,summary,project_type_keys,status,revision,created_by,created_at,updated_at) VALUES ('${project}','${tenant}','UI-BONUS-TEST','ui-bonus-test','原 OA 奖金双侧闭环验证','真实来源进程隔离夹具',ARRAY['design'],'active',18,'${subject}',now(),now()); INSERT INTO project_assignments(id,tenant_id,project_id,subject_kind,subject_id,source_revision,display_snapshot,role_key,effective_from,created_project_revision,created_at) VALUES `+assignments.map(a=>`('${a.id}','${tenant}','${project}','person','${person}',2,'同人多分工验收人员','${a.role_key}',now(),${a.created_project_revision},now())`).join(','));
    await iam('/api/v1/admin/grants',{subject_id:subject,audience:'bonus-distribution',action:'bonus-pool.read',scope:{kind:'resources',resources:[{type:'project',id:project}]}},204);
    const legacy=(await iam('/api/v1/credentials',{audience:'bonus-distribution',scope:{kind:'resources',resources:[{type:'project',id:project}]}})).token;
    for(const side of ['business','production'])for(const action of actions)await grant(side,action);
    // Workbench directory asks for the existing tenant read scope. This isolated
    // test tenant contains only the fixture Project; Bonus grants remain exact.
    await iam('/api/v1/admin/grants',{subject_id:subject,audience:'project-workspace',action:'project-workspace.read',scope:{kind:'tenant'}},204);
    const before=sourceDigest(),exchangeURL=await transparentProxy(iamURL,'iam');
    const env={IDENTITY_PROVIDER:'iam',IAM_ISSUER:issuer,IAM_TENANT_ID:tenant,IAM_PUBLIC_KEYS_PATH:output+'/iam-public.json',IAM_SERVICE_CREDENTIAL_URL:exchangeURL+'/api/v1/service-credentials',WORKLOAD_ID:'bonus-distribution',WORKLOAD_SUBJECT_ID:workloadSubject,WORKLOAD_KEY_FILE:privateKey,PROJECT_BASE_URL:projectURL,PEOPLE_BASE_URL:peopleURL,CONTRACT_BASE_URL:contractURL,REFERENCE_TIMEOUT:'2s'};
    async function startGateway(owner){
      if(!ports.has('unified-workbench'))ports.set('unified-workbench',await freePort());
      await stop('gateway');writeFileSync(output+'/gateway-suite.json',JSON.stringify({schema_version:'1',suite_id:'bonus-live-test',tenant_id:tenant,applications:[
        {id:'identity-access',version:'0.1.0',audience:'identity-access',enabled:true,required:true,route_prefix:'/auth',readiness_path:'/health/ready',upstream:iamURL},
        {id:'unified-workbench',version:'0.1.0',audience:'unified-workbench',enabled:true,required:true,route_prefix:'/workbench',readiness_path:'/health/ready',upstream:'http://127.0.0.1:'+ports.get('unified-workbench')},
        {id:'project-workspace',version:'0.1.0',audience:'project-workspace',enabled:true,required:false,route_prefix:'/project',readiness_path:'/health/ready',upstream:projectURL,entry_action:'project-workspace.read'},
        {id:'people-groups',version:'0.1.0',audience:'people-groups',enabled:true,required:false,route_prefix:'/people',readiness_path:'/health/ready',upstream:peopleURL,entry_action:'people-groups.view'},
        {id:'bonus-distribution',version:'0.1.0',audience:'bonus-distribution',enabled:true,required:false,route_prefix:'/bonus',readiness_path:'/health/ready',upstream:owner,capability_namespaces:['bonus-pool','bonus-manual-allocation'],entry_actions:['bonus-pool.read','bonus-manual-allocation.read']}
      ]}),{mode:0o600});gatewayURL=await start('gateway');
      if(!processes.has('unified-workbench'))await start('unified-workbench');
    }
    async function gatewayRequest(side,path,options={}){
      const form=new URLSearchParams({csrf_token:csrf,resource_type:side+'_bonus_project',resource_id:project});
      const authorized=await fetch(gatewayURL+'/gateway/authorize/bonus-distribution',{method:'POST',redirect:'manual',headers:{Cookie:[...cookies].map(([k,v])=>k+'='+v).join('; ')},body:form});
      assert.equal(authorized.status,303,'native Gateway admission');assert.equal(authorized.headers.get('location'),'/bonus/');
      const set=authorized.headers.getSetCookie();assert.equal(set.length,1);assert(/Path=\/bonus(?:;|$)/.test(set[0])&&/HttpOnly/.test(set[0]));const cookie=set[0].split(';',1)[0],token=cookie.slice(cookie.indexOf('=')+1),claims=JSON.parse(Buffer.from(token.split('.')[1],'base64url'));assert.deepEqual(claims.scope,scope(side));assert(claims.actions.includes('bonus-manual-allocation.read'));
      return fetch(gatewayURL+'/bonus'+path,{...options,redirect:'manual',headers:{...options.headers,Cookie:cookie,Authorization:'Bearer forged-client-token','X-OMS-Subject-ID':randomUUID(),'X-OMS-Tenant-ID':randomUUID()}});
    }
    async function preflight(owner){
      const path='/api/v1/reference-oa/bonus-allocations',get=async(token,p)=>fetch(owner+p,{headers:{Authorization:'Bearer '+token}}),token=await credential('business');
      assert.equal((await get(legacy,path+'?project_id='+project+'&scope=business')).status,403);
      assert.equal((await get(token,path+'?project_id='+project+'&scope=production')).status,403);
      const oldCookie='oms_cap_bonus_distribution='+legacy;
      const oldPool=await fetch(gatewayURL+'/bonus/api/v1/bonus-pools?project_id='+project,{redirect:'manual',headers:{Cookie:oldCookie}});assert.equal(oldPool.status,200,'legacy pool still admitted');
      const wrongScope=await fetch(gatewayURL+'/bonus'+path+'?project_id='+project+'&scope=business',{redirect:'manual',headers:{Cookie:oldCookie}});assert.equal(wrongScope.status,403,'entry cannot upgrade legacy scope');
      assert.equal((await gatewayRequest('business',path+'?project_id='+project+'&scope=production')).status,403,'Gateway admission cannot widen side');
      const noCookie=await fetch(gatewayURL+'/bonus'+path+'?project_id='+project+'&scope=business',{redirect:'manual',headers:{Authorization:'Bearer '+token,'X-OMS-Subject-ID':subject}});assert.equal(noCookie.status,303,'forged headers cannot replace cookie');
      await grant('business','bonus-manual-allocation.read',true);const recordOnly=await credential('business');
      const noRead=await fetch(gatewayURL+'/bonus'+path+'?project_id='+project+'&scope=business',{redirect:'manual',headers:{Cookie:'oms_cap_bonus_distribution='+recordOnly}});assert.equal(noRead.status,303,'record capability cannot imply admission read');await grant('business','bonus-manual-allocation.read');
      const candidates=await get(token,path+'/candidates?project_id='+project+'&project_revision=18&scope=business');assert.equal(candidates.status,200);const choices=await candidates.json();assert.equal(choices.items.length,2);assert.equal(choices.items[1].assignment.role_key,'commercial_contributor');
      await grant('business','bonus-manual-allocation.amount.read',true);const redacted=await credential('business'),read=await get(redacted,path+'?project_id='+project+'&scope=business');assert.equal(read.status,200);assert(!(await read.json()).allowed_actions.includes('bonus-manual-allocation.record'));
      await grant('business','bonus-manual-allocation.amount.read');
      const payload={command_id:randomUUID(),project_ref:{id:project,revision:18},scope:'business',base_amount_minor:'101',occurred_on:'2026-10-04',description:'故障零写夹具',items:[{person_ref:{id:person,revision:2},assignment_ref:{id:assignments[0].id,revision:10},method:'ratio',ratio_basis_points:5000,fixed_amount_minor:'0'}]};
      await stop('people-groups');const failed=await fetch(owner+path,{method:'POST',headers:{Authorization:'Bearer '+await credential('business'),'Content-Type':'application/json'},body:JSON.stringify(payload)});assert.equal(failed.status,503);await start('people-groups');
      assert.deepEqual(sourceDigest(),before);return {legacy_scope_denied:true,side_scope_denied:true,native_candidate_roles:true,formal_amount_revoke_hides_record:true,real_people_outage_zero_source_write:true,native_gateway_legacy_pool_compatible:true,gateway_cannot_widen_scope_or_accept_forged_headers:true,record_does_not_imply_read_admission:true};
    }
    async function afterflight(owner){
      await grant('business','bonus-manual-allocation.amount.read',true);const token=await credential('business'),base=owner+'/api/v1/reference-oa/bonus-allocations';
      const list=await fetch(base+'?project_id='+project+'&scope=business',{headers:{Authorization:'Bearer '+token}});assert.equal(list.status,200);const v=await list.json();assert.equal(v.items.length,3);
      const fields=['currency','base_amount_minor','ratio_basis_points','fixed_amount_minor','amount_minor'];for(const row of v.items)for(const key of fields)assert.equal(row[key],undefined);
      const history=await fetch(base+'/'+v.items.find(x=>x.state==='withdrawn').id+'/history',{headers:{Authorization:'Bearer '+token}});assert.equal(history.status,200);for(const row of (await history.json()).items)for(const key of fields)assert.equal(row.snapshot[key],undefined);
      const denied=await fetch(base,{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify({command_id:randomUUID(),project_ref:{id:project,revision:18},scope:'business',base_amount_minor:'101',occurred_on:'2026-10-04',description:'金额撤权零写夹具',items:[{person_ref:{id:person,revision:2},assignment_ref:{id:assignments[0].id,revision:10},method:'ratio',ratio_basis_points:5000,fixed_amount_minor:'0'}]})});assert.equal(denied.status,403);
      return {native_current_and_history_five_amount_fields_absent:true,revoked_amount_record_denied:true};
    }
    async function close(){try{assert.deepEqual(sourceDigest(),before);assert(exchanges.some(v=>v.audience==='project-workspace'));assert(exchanges.some(v=>v.audience==='people-groups'));assert(exchanges.every(v=>['project-workspace','people-groups'].includes(v.audience)&&v.scope.kind==='resources'&&v.scope.resources.length===1));await iam('/api/v1/logout',{},204)}finally{await cleanup()}}
    return {subject,env,credential,startGateway,gatewayRequest,preflight,afterflight,close,abort:cleanup,sourceDigest:before,exchanges,
      workbenchSession:()=>({url:gatewayURL+'/workbench/?project_id='+project+'&tab=business&business_tab=bonus',cookies:[...cookies].map(([name,value])=>({name,value,domain:'127.0.0.1',path:'/',httpOnly:name!=='oms_iam_csrf',sameSite:'Lax'}))})};
  }catch(error){await cleanup();throw error;}
}
