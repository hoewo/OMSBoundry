// Task-owned native composition; optional Bonus and Nginx/Gateway/Workbench.
// No Main access or source SQL fixture writes.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {execFileSync,spawn,spawnSync} from 'node:child_process';
import {writeFileSync,mkdirSync,readFileSync} from 'node:fs';
import {randomUUID,createHash} from 'node:crypto';
import {applicationSourcePath} from './application-source-paths.mjs';
export async function createPersonnelRuntime({root,output,bonus=false}){
 assert(/^\/tmp\/oa-production-personnel-native\.[A-Za-z0-9]+$/.test(output));
 const container='oms-bonus-overlay-test-20261004';
 assert.equal(execFileSync('docker',['inspect','--format','{{index .Config.Labels "omsbizfoundry.task"}}',container],{encoding:'utf8'}).trim(),'oa-bonus-20261004');
 const tenant=randomUUID(),workloadSubject=randomUUID(),suffix=randomUUID().replaceAll('-','').slice(0,10),password=randomUUID()+randomUUID(),issuer='https://personnel-native.private.test';
 const apps=['identity-access','people-groups','project-workspace',...(bonus?['contract-ledger','bonus-distribution']:[])],dbs={},ports={},urls={},processes=new Map(),exchanges=[];
 const safeEnv={PATH:process.env.PATH,GOMAXPROCS:'2'},cookies=new Map();let csrf,subject,selectedProfile='reference-oa-production-personnel-v1';
 const privateKey=output+'/workload-private.json',publicKey=output+'/workload-public.json',bonusSubject=randomUUID();let edgeID;
 const freePort=async()=>{const s=createServer();await new Promise(r=>s.listen(0,'127.0.0.1',r));const port=s.address().port;await new Promise(r=>s.close(r));return port};
 const sql=(app,statement)=>execFileSync('docker',['exec',container,'psql','-X','-A','-t','-v','ON_ERROR_STOP=1','-U','postgres','-d',dbs[app],'-c',statement],{encoding:'utf8'}).trim();
 const sha=value=>createHash('sha256').update(value).digest('hex');
 const digest=()=>sha(sql('project-workspace',"SELECT jsonb_build_object('projects',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM projects p),'facts',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM reference_oa_production_assignments p),'events',(SELECT jsonb_agg(to_jsonb(p) ORDER BY command_id) FROM reference_oa_production_events p),'commands',(SELECT count(*) FROM project_commands),'versions',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id,revision) FROM reference_oa_production_discipline_versions p))"));
 function env(app){const e={...safeEnv,HTTP_ADDR:'127.0.0.1:'+ports[app],DATABASE_URL:`postgres://postgres@127.0.0.1:55477/${dbs[app]}?sslmode=disable`,MIGRATION_MODE:'auto'};
  if(app==='gateway')return {...safeEnv,HTTP_ADDR:e.HTTP_ADDR,SUITE_MANIFEST_PATH:output+'/suite.json',IAM_PUBLIC_KEYS_PATH:output+'/iam-public.json',IAM_ISSUER:issuer,COOKIE_SECURE:'false',REQUESTS_PER_MINUTE:'10000'};
  if(app==='unified-workbench')return {...safeEnv,HTTP_ADDR:e.HTTP_ADDR,SUITE_MANIFEST_PATH:output+'/suite.json',REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE:selectedProfile,REFERENCE_OA_BONUS_PROFILE:'reference-oa-bonus-v3'};
  if(app==='identity-access')return {...e,TENANT_ID:tenant,TENANT_NAME:'原 OA D01 隔离原生验收',PUBLIC_ISSUER:issuer,SIGNING_KEY_FILE:output+'/iam-private.json',SESSION_COOKIE_SECURE:'false',BOOTSTRAP_ADMIN_USERNAME:'admin',BOOTSTRAP_ADMIN_DISPLAY_NAME:'隔离验收管理员',BOOTSTRAP_ADMIN_PASSWORD:password,CAPABILITY_MANIFEST_PATHS:apps.filter(v=>v!=='identity-access').map(v=>root+'contracts/access/v1/examples/'+v+'-capabilities.json').join(','),WORKLOAD_IDENTITY_POLICY_PATH:output+'/workload-policy.json'};
  Object.assign(e,{IDENTITY_PROVIDER:'iam',IAM_ISSUER:issuer,IAM_TENANT_ID:tenant,IAM_PUBLIC_KEYS_PATH:output+'/iam-public.json'});
  if(app==='project-workspace')Object.assign(e,{REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE:selectedProfile,PEOPLE_BASE_URL:urls['people-groups'],IAM_SERVICE_CREDENTIAL_URL:urls['exchange']+'/api/v1/service-credentials',WORKLOAD_ID:'project-workspace',WORKLOAD_SUBJECT_ID:workloadSubject,WORKLOAD_KEY_FILE:privateKey,REFERENCE_TIMEOUT:'1s'});
  if(app==='contract-ledger')Object.assign(e,{EVIDENCE_STORAGE_DIR:output+'/contract-private-bytes'});
  if(app==='bonus-distribution')Object.assign(e,{REFERENCE_OA_BONUS_PROFILE:'reference-oa-bonus-v3',PROJECT_BASE_URL:urls['project-workspace'],PEOPLE_BASE_URL:urls['people-groups'],CONTRACT_BASE_URL:urls['contract-ledger'],IAM_SERVICE_CREDENTIAL_URL:urls['exchange']+'/api/v1/service-credentials',WORKLOAD_ID:'bonus-distribution',WORKLOAD_SUBJECT_ID:bonusSubject,WORKLOAD_KEY_FILE:output+'/bonus-private.json',REFERENCE_TIMEOUT:'1s'});
  return e;
 }
 async function start(app){const child=spawn(output+'/'+app+'-server',[],{env:env(app),stdio:['ignore','pipe','pipe']});let log='';child.stdout.on('data',v=>log+=v);child.stderr.on('data',v=>log+=v);processes.set(app,{child,log:()=>log});const url='http://127.0.0.1:'+ports[app];
  for(let n=0;n<150;n++){if(child.exitCode!==null)throw Error(app+' exited: '+log.slice(-500));try{if((await fetch(url+'/health/ready')).ok)return urls[app]=url}catch{}await new Promise(r=>setTimeout(r,50))}throw Error(app+' readiness timeout')}
 async function stop(app){const e=processes.get(app);if(!e||e.child.exitCode!==null)return;await new Promise(r=>{e.child.once('exit',r);e.child.kill('SIGTERM')});writeFileSync(output+'/'+app+'.log',e.log(),{mode:0o600})}
 async function iam(path,input,expected=200){const r=await fetch(urls['identity-access']+path,{method:input===undefined?'GET':'POST',headers:{Cookie:[...cookies].map(([k,v])=>k+'='+v).join('; '),...(input===undefined?{}:{'Content-Type':'application/json','X-CSRF-Token':csrf})},...(input===undefined?{}:{body:JSON.stringify(input)})});for(const c of r.headers.getSetCookie()){const [k,v]=c.split(';',1)[0].split('=');cookies.set(k,v)}const body=await r.text();assert.equal(r.status,expected,'native IAM '+path+': '+body.slice(0,120));return body?JSON.parse(body):null}
 const tenantScope={kind:'tenant'},scope=id=>({kind:'resources',resources:[{type:'project',id}]});
 const grant=(audience,action,s=tenantScope,revoke=false)=>iam('/api/v1/admin/grants'+(revoke?'/revoke':''),{subject_id:subject,audience,action,scope:s},204);
 async function credential(audience,s){const v=await iam('/api/v1/credentials',{audience,scope:s});const claims=JSON.parse(Buffer.from(v.token.split('.')[1],'base64url'));assert.equal(claims.sub,subject);assert.deepEqual(claims.scope,{...s,resources:s.resources||[]});return v.token}
 async function request(app,path,input,s=tenantScope,expected=200,extra={}){const r=await fetch(urls[app]+path,{method:input===undefined?'GET':'POST',headers:{Authorization:'Bearer '+await credential(app,s),...(input===undefined?{}:{'Content-Type':'application/json'}),...extra},...(input===undefined?{}:{body:JSON.stringify(input)})});const text=await r.text();if(expected!==null)assert.equal(r.status,expected,app+' '+path+': '+text.slice(0,150));if(text&&!r.headers.get('Content-Type')?.includes('json')){assert.equal(r.status,404,'only an unregistered optional route may return a non-JSON reply');return {status:r.status,body:null}}return {status:r.status,body:text?JSON.parse(text):null}}
 function stopEdge(){
  if(!edgeID)return;
  assert.equal(execFileSync('docker',['inspect','--format','{{.Id}}',edgeID],{encoding:'utf8'}).trim(),edgeID);
  assert.equal(execFileSync('docker',['inspect','--format','{{index .Config.Labels "omsbizfoundry.task"}}',edgeID],{encoding:'utf8'}).trim(),'oa-bonus-20261004');
  try{writeFileSync(output+'/nginx.log',execFileSync('docker',['logs',edgeID],{encoding:'utf8',stdio:['ignore','pipe','pipe']}),{mode:0o600})}
  finally{execFileSync('docker',['stop',edgeID]);edgeID=undefined}
 }
 const exitCleanup=()=>{for(const {child} of processes.values())if(child.exitCode===null)child.kill('SIGTERM');if(edgeID)try{stopEdge()}catch{}};process.once('exit',exitCleanup);let exchange;
 async function close(){
  try{if(subject)await iam('/api/v1/logout',{},204)}
  finally{try{stopEdge()}finally{for(const a of [...apps].reverse())await stop(a);if(exchange)await new Promise(r=>exchange.close(r));process.removeListener('exit',exitCleanup)}}
 }
 try{
  for(const app of apps){const b=spawnSync('go',['build','-o',output+'/'+app+'-server','./cmd/server'],{cwd:applicationSourcePath(root,app),env:{...process.env,GOWORK:'off',GOMAXPROCS:'2'},encoding:'utf8'});assert.equal(b.status,0,b.stderr);ports[app]=await freePort();dbs[app]='personnel_native_'+suffix+'_'+app.replaceAll('-','_');execFileSync('docker',['exec',container,'createdb','-U','postgres',dbs[app]])}
  const k=spawnSync('go',['build','-o',output+'/keyctl','./cmd/workload-keyctl'],{cwd:root+'packages/workload-identity',env:{...process.env,GOWORK:'off',GOMAXPROCS:'2'},encoding:'utf8'});assert.equal(k.status,0,k.stderr);
  execFileSync(output+'/keyctl',['init'],{env:{...safeEnv,WORKLOAD_ID:'project-workspace',WORKLOAD_SUBJECT_ID:workloadSubject,WORKLOAD_KEY_FILE:privateKey,WORKLOAD_PUBLIC_KEY_FILE:publicKey}});
  const identities=[{id:'project-workspace',subject_id:workloadSubject,display_name:'独立 Project',public_keys_path:publicKey,permissions:[{audience:'people-groups',action:'people-groups.view',resource_types:['person']}]}];
  if(bonus){mkdirSync(output+'/contract-private-bytes');execFileSync(output+'/keyctl',['init'],{env:{...safeEnv,WORKLOAD_ID:'bonus-distribution',WORKLOAD_SUBJECT_ID:bonusSubject,WORKLOAD_KEY_FILE:output+'/bonus-private.json',WORKLOAD_PUBLIC_KEY_FILE:output+'/bonus-public.json'}});identities.push({id:'bonus-distribution',subject_id:bonusSubject,display_name:'独立 Bonus',public_keys_path:output+'/bonus-public.json',permissions:[{audience:'project-workspace',action:'project-workspace.read',resource_types:['project']},{audience:'project-workspace',action:'project-workspace.reference-oa-personnel.read',resource_types:['project']},{audience:'people-groups',action:'people-groups.person.reference.read',resource_types:['person']}]})}
  writeFileSync(output+'/workload-policy.json',JSON.stringify({schema_version:'1',identities}),{mode:0o600});
  await start('identity-access');const keys=await fetch(urls['identity-access']+'/api/v1/.well-known/jwks.json');assert(keys.ok);writeFileSync(output+'/iam-public.json',await keys.text(),{mode:0o600});csrf=(await iam('/api/v1/csrf')).csrf_token;await iam('/api/v1/login',{username:'admin',password});subject=(await iam('/api/v1/session')).principal.subject_id;
  exchange=createServer(async(req,res)=>{try{const chunks=[];for await(const c of req)chunks.push(c);const body=Buffer.concat(chunks),v=JSON.parse(body);exchanges.push({audience:v.audience,action:v.action,scope:v.scope});const r=await fetch(urls['identity-access']+req.url,{method:req.method,headers:{'Content-Type':'application/json'},body});res.statusCode=r.status;res.setHeader('Content-Type','application/json');res.end(Buffer.from(await r.arrayBuffer()))}catch{res.statusCode=503;res.end('{}')}});await new Promise(r=>exchange.listen(0,'127.0.0.1',r));urls.exchange='http://127.0.0.1:'+exchange.address().port;
  await start('people-groups');await start('project-workspace');for(const action of ['people-groups.view','people-groups.person.manage'])await grant('people-groups',action);await grant('project-workspace','project-workspace.create');
  for(const action of ['project-workspace.reference-oa-discipline.read','project-workspace.reference-oa-discipline.manage'])await grant('project-workspace',action);
  const people=[];for(const [i,display_name] of ['林设计','陈复核','周审定'].entries())people.push((await request('people-groups','/api/v1/persons',{command_id:randomUUID(),reference_code:'D01-'+i,display_name},tenantScope,201)).body);
  const project=(await request('project-workspace','/api/v1/projects',{command_id:randomUUID(),project_number:'OA-NATIVE-D01',name:'原 OA 生产人员原生点击验收',project_type_keys:['design']},tenantScope,201)).body;
  for(const action of ['project-workspace.reference-oa-personnel.read','project-workspace.reference-oa-personnel.manage'])await grant('project-workspace',action,scope(project.id));
  const base='/api/v1/projects/'+project.id+'/reference-oa/';await request('project-workspace',base+'production-profile',{command_id:randomUUID(),expected_revision:project.revision},scope(project.id));
  const disciplines=[];for(const name of ['桥梁','道路'])disciplines.push((await request('project-workspace','/api/v1/reference-oa/production-disciplines',{command_id:randomUUID(),operation:'create',expected_revision:0,name,description:'隔离原生验收'},tenantScope)).body);
  if(bonus){await start('contract-ledger');await start('bonus-distribution')}
  const view=async(includeEnded=false)=>(await request('project-workspace',base+'production-personnel'+(includeEnded?'?include_ended=true':''),undefined,scope(project.id))).body;
  const stats=()=>JSON.parse(sql('project-workspace',"SELECT json_build_object('facts',(SELECT count(*) FROM reference_oa_production_assignments),'active',(SELECT count(*) FROM reference_oa_production_assignments WHERE ended_at IS NULL),'ended',(SELECT count(*) FROM reference_oa_production_assignments WHERE ended_at IS NOT NULL),'events',(SELECT count(*) FROM reference_oa_production_events),'commands',(SELECT count(*) FROM project_commands),'base_assignments',(SELECT count(*) FROM project_assignments),'profiles',(SELECT count(*) FROM reference_oa_production_profiles),'discipline_versions',(SELECT count(*) FROM reference_oa_production_discipline_versions))"));
  const peopleDigest=()=>sha(sql('people-groups',"SELECT jsonb_build_object('persons',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM persons p),'revisions',(SELECT jsonb_agg(to_jsonb(p) ORDER BY to_jsonb(p)::text) FROM person_revisions p),'commands',(SELECT jsonb_agg(to_jsonb(p) ORDER BY to_jsonb(p)::text) FROM business_commands p),'audit',(SELECT jsonb_agg(to_jsonb(p) ORDER BY to_jsonb(p)::text) FROM people_groups_audit_entries p))"));
  const trustedAudit=()=>sql('project-workspace',`SELECT NOT EXISTS(SELECT 1 FROM reference_oa_production_events WHERE actor_subject<>'${subject}') AND NOT EXISTS(SELECT 1 FROM reference_oa_production_discipline_versions WHERE actor_subject<>'${subject}')`) === 't';
  async function startAssembly(){
   assert(bonus);assert(!edgeID);
   for(const app of ['unified-workbench','gateway']){const b=spawnSync('go',['build','-o',output+'/'+app+'-server','./cmd/server'],{cwd:applicationSourcePath(root,app),env:{...process.env,GOWORK:'off',GOMAXPROCS:'2'},encoding:'utf8'});assert.equal(b.status,0,b.stderr);ports[app]=await freePort();urls[app]='http://127.0.0.1:'+ports[app];apps.push(app)}
   const routes={'identity-access':'/auth','unified-workbench':'/workbench','project-workspace':'/project','people-groups':'/people','contract-ledger':'/contract','bonus-distribution':'/bonus'};
   const manifest={schema_version:'1',suite_id:'d01-b12-native-private',tenant_id:tenant,applications:apps.filter(a=>a!=='gateway').map(a=>({id:a,version:'0.1.0',audience:a,enabled:true,required:['identity-access','unified-workbench'].includes(a),route_prefix:routes[a],upstream:urls[a],readiness_path:'/health/ready',...(['identity-access','unified-workbench'].includes(a)?{}:a==='bonus-distribution'?{capability_namespaces:['bonus-pool','bonus-manual-allocation'],entry_actions:['bonus-pool.read','bonus-manual-allocation.read']}:{entry_action:a==='people-groups'?'people-groups.view':a==='contract-ledger'?'contract-ledger.view':'project-workspace.read'})}))};
   writeFileSync(output+'/suite.json',JSON.stringify(manifest),{mode:0o600});await start('unified-workbench');await start('gateway');
   const config=readFileSync(root+'deployments/d1/nginx/nginx.conf','utf8').replace('server gateway:8088;','server host.docker.internal:'+ports.gateway+';');
   writeFileSync(output+'/nginx.conf',config,{mode:0o600});
   edgeID=execFileSync('docker',['run','-d','--rm','--name','oms-d01-edge-'+suffix,'--label','omsbizfoundry.task=oa-bonus-20261004','-p','127.0.0.1::8080','-v',output+'/nginx.conf:/etc/nginx/nginx.conf:ro','nginx:1.28-alpine'],{encoding:'utf8'}).trim();assert(/^[a-f0-9]{64}$/.test(edgeID));
   const published=execFileSync('docker',['port',edgeID,'8080/tcp'],{encoding:'utf8'}).trim();assert(/^127\.0\.0\.1:\d+$/.test(published));const url='http://'+published;
   for(let n=0;n<100;n++){try{if((await fetch(url+'/workbench/assets/reference-oa-react-bidding.js',{redirect:'manual'})).status===303||(await fetch(url+'/gateway/login')).ok)break}catch{}await new Promise(r=>setTimeout(r,50))}
   return {url:url+'/workbench/?project_id='+project.id+'&tab=production&production_tab=overview',cookies:[...cookies].map(([name,value])=>({name,value,domain:'127.0.0.1',path:'/',httpOnly:name!=='oms_iam_csrf',sameSite:'Lax'})),normal_asset_sha256:sha(readFileSync(root+'frontend/oms-workbench/internal/server/reference_oa_react_bidding.js')),nginx_config_sha256:sha(config)};
  }
  return {tenant,subject,project,people,disciplines,base,urls,scope:scope(project.id),tenantScope,view,stats,digest,peopleDigest,trustedAudit,request,credential,grant,stop,start,startAssembly,sql,dbs,selectProfile(value){assert(['','reference-oa-production-personnel-v1'].includes(value));selectedProfile=value},exchanges,close};
 }catch(e){await close();throw e}
}
