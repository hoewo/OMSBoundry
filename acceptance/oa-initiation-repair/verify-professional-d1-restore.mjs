// Task-owned real D1 selected composition. No Main, registry push or source SQL writes.
// Local HTTP/build-mode evidence is not public TLS, clean-host or full-suite acceptance.
import assert from 'node:assert/strict';
import {cpSync,readFileSync,writeFileSync,mkdirSync,readdirSync,statSync} from 'node:fs';
import {spawn,execFileSync} from 'node:child_process';
import {randomUUID,randomBytes,createHash} from 'node:crypto';
import {createServer} from 'node:net';
import {fileURLToPath} from 'node:url';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
import {applicationRelativePath} from './application-source-paths.mjs';

const output=process.argv[2];
assert(/^\/tmp\/oa-professional-d1\.[A-Za-z0-9]+$/.test(output));
const root=fileURLToPath(new URL('../../',import.meta.url));
const tag='oa-professional-'+randomUUID().slice(0,8),label='oa-professional-d1-20261004';
const projects={source:tag+'-source',target:tag+'-restore'};
const apps=['identity-access','contract-ledger','people-groups','project-workspace','evidence-service','bonus-distribution','unified-workbench','gateway'];
const businessDBs=['contract_ledger','people_groups','project_workspace','evidence_service','bonus_distribution'];
const bundle=output+'/bundle',d1=bundle+'/deployments/d1',context=output+'/build-context',ops=d1+'/ops/d1';
const tenant=randomUUID(),password=randomBytes(32).toString('hex');
const receipt={passed:false,main_changed:false,public_tls_accepted:false,clean_host_release_accepted:false,full_suite_accepted:false,full_oa_accepted:false,selected_apps:apps,scenarios:[]};
let stage='fixture',browser,page,current='source',origin,csrf,subject,command,lastRecord;
let cookies=new Map();const errors=[],writes=[];
const mark=name=>{receipt.scenarios.push({name,passed:true});console.log('PASS '+name)};
const hash=v=>createHash('sha256').update(v).digest('hex');
const save=(file,v)=>writeFileSync(file,v,{mode:0o600});
const filter=source=>!source.split('/').some(p=>['node_modules','frontend','.git','.runtime','.data'].includes(p)||p.startsWith('.env')||p.endsWith('.log')||p.endsWith('.dump'));
const docker=(...args)=>execFileSync('docker',args,{encoding:'utf8',maxBuffer:20*1024*1024}).trim();
function containers(which){return docker('ps','-a','--filter','label=com.docker.compose.project='+projects[which],'--format','{{.ID}}').split('\n').filter(Boolean)}
function verifyOwnership(which){assert(/^oa-professional-[0-9a-f]{8}-(source|restore)$/.test(projects[which]));for(const id of containers(which))assert.equal(docker('inspect','--format','{{index .Config.Labels "omsbizfoundry.task"}}',id),label)}
function container(which,service){const ids=docker('ps','--filter','label=com.docker.compose.project='+projects[which],'--filter','label=com.docker.compose.service='+service,'--format','{{.ID}}').split('\n').filter(Boolean);assert.equal(ids.length,1,service+' runtime');return ids[0]}
function sql(which,db,query){return docker('exec',container(which,'postgres'),'psql','-X','-A','-t','-v','ON_ERROR_STOP=1','-U','oms_admin','-d',db,'-c',query)}
function snapshot(which,db,exclude=[]){const tables=sql(which,db,"SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename").split('\n').filter(t=>t&&!exclude.includes(t));const values={};for(const table of tables){assert(/^[a-z0-9_]+$/.test(table));const rows=sql(which,db,'SELECT coalesce(jsonb_agg(row ORDER BY row::text),\'[]\'::jsonb) FROM (SELECT to_jsonb(t) AS row FROM "'+table+'" t) x');values[table]={sha256:hash(rows),rows:JSON.parse(rows).length}}return values}
function businessSnapshot(which){return Object.fromEntries(businessDBs.map(db=>[db,snapshot(which,db)]))}
const freePort=async()=>{const s=createServer();await new Promise(r=>s.listen(0,'127.0.0.1',r));const port=s.address().port;await new Promise(r=>s.close(r));return port};
let runNumber=0;
async function run(which,args,{envFile=output+'/d1.env',expect=0,confirm=false}={}){
 verifyOwnership(which);
 const child=spawn('sh',[ops,...args],{env:{...process.env,D1_ENV_FILE:envFile,D1_PROJECT_NAME:projects[which],D1_COMPOSE_OVERRIDE_FILE:output+'/compose.override.json',COMPOSE_PARALLEL_LIMIT:'1',...(confirm?{D1_CONFIRM_EMPTY_RESTORE:'yes'}:{})},stdio:['ignore','pipe','pipe']});
 let text='';child.stdout.on('data',v=>text+=v);child.stderr.on('data',v=>text+=v);
 const code=await new Promise((resolve,reject)=>{child.once('error',reject);child.once('exit',resolve)});
 save(output+'/'+String(++runNumber).padStart(2,'0')+'-'+which+'-'+args[0]+'.log',text);
 if(expect===null)assert.notEqual(code,0,args[0]+' must reject');else assert.equal(code,expect,args[0]+' failed; see private log '+runNumber);
 return text;
}
const tenantScope={kind:'tenant'},projectScope=id=>({kind:'resources',resources:[{type:'project',id}]}),bonusScope=id=>({kind:'resources',resources:[{type:'production_bonus_project',id}]});
async function iam(path,input,expected=200){const r=await fetch(origin+'/auth'+path,{method:input===undefined?'GET':'POST',headers:{Cookie:[...cookies].map(([k,v])=>k+'='+v).join('; '),...(input===undefined?{}:{'Content-Type':'application/json','X-CSRF-Token':csrf})},...(input===undefined?{}:{body:JSON.stringify(input)})});for(const c of r.headers.getSetCookie()){const [k,v]=c.split(';',1)[0].split('=');cookies.set(k,v)}const body=await r.text();assert.equal(r.status,expected,'IAM '+path+' status '+r.status);return body?JSON.parse(body):null}
async function login(){cookies=new Map();csrf=(await iam('/api/v1/csrf')).csrf_token;await iam('/api/v1/login',{username:'admin',password});subject=(await iam('/api/v1/session')).principal.subject_id}
const grant=(audience,action,scope)=>iam('/api/v1/admin/grants',{subject_id:subject,audience,action,scope},204);
async function request(app,prefix,path,input,scope=tenantScope,expected=200,headers={}){
 // The browser edge deliberately strips client Authorization. Obtain the
 // scoped app cookie through the real Gateway, never bypass that boundary.
 const form=new URLSearchParams({csrf_token:csrf,return_to:prefix+'/'});
 if(scope.kind==='resources'){assert.equal(scope.resources.length,1);form.set('resource_type',scope.resources[0].type);form.set('resource_id',scope.resources[0].id)}
 const authorized=await fetch(origin+'/gateway/authorize/'+app,{method:'POST',redirect:'manual',headers:{Cookie:[...cookies].map(([k,v])=>k+'='+v).join('; '),'Content-Type':'application/x-www-form-urlencoded'},body:form});
 assert.equal(authorized.status,303,'Gateway scoped entry '+app+' status '+authorized.status);
 for(const c of authorized.headers.getSetCookie()){const [k,v]=c.split(';',1)[0].split('=');cookies.set(k,v)}
 const r=await fetch(origin+prefix+path,{method:input===undefined?'GET':'POST',headers:{Cookie:[...cookies].map(([k,v])=>k+'='+v).join('; '),...headers,...(input===undefined?{}:{'Content-Type':'application/json'})},...(input===undefined?{}:{body:JSON.stringify(input)})});
 const body=await r.text();assert.equal(r.status,expected,app+' '+path+' status '+r.status+(r.status===expected?'':': '+body.slice(0,180)));return body?JSON.parse(body):null;
}
const browserCookies=()=>[...cookies].map(([name,value])=>({name,value,domain:'127.0.0.1',path:'/',httpOnly:name!=='oms_iam_csrf',sameSite:'Lax'}));
try{
 mkdirSync(output,{recursive:true,mode:0o700});assert.equal(containers('source').length,0);assert.equal(containers('target').length,0);
 // Explicit public sources only: never include live .env or node_modules in Docker context.
 for(const dir of ['ops','docker','nginx','postgres']){mkdirSync(d1+'/'+dir,{recursive:true});cpSync(root+'deployments/d1/'+dir,d1+'/'+dir,{recursive:true,filter})}
 for(const file of ['compose.yaml','compose.pull.yaml','compose.https.yaml','compose.evidence-oss.yaml'])if(statSync(root+'deployments/d1/'+file,{throwIfNoEntry:false}))cpSync(root+'deployments/d1/'+file,d1+'/'+file);
 mkdirSync(bundle+'/contracts/access/v1/examples',{recursive:true});cpSync(root+'contracts/access/v1/examples',bundle+'/contracts/access/v1/examples',{recursive:true});
 for(const app of apps){const path=applicationRelativePath(app);mkdirSync(context+'/'+path,{recursive:true});cpSync(root+path,context+'/'+path,{recursive:true,filter})}
 mkdirSync(context+'/packages',{recursive:true});cpSync(root+'packages',context+'/packages',{recursive:true,filter});mkdirSync(context+'/deployments/d1/docker',{recursive:true});cpSync(d1+'/docker',context+'/deployments/d1/docker',{recursive:true});
 const port=await freePort();origin='http://127.0.0.1:'+port;
 const selections={CONTRACT:true,PEOPLE:true,PROJECT:true,EVIDENCE:true,BONUS_DISTRIBUTION:true};
 const values={SUITE_VERSION:tag,D1_IMAGE_MODE:'build',D1_INGRESS_MODE:'http',PUBLIC_BIND:'127.0.0.1',PUBLIC_PORT:port,PUBLIC_ISSUER:'https://professional-d1.private.test/auth',COOKIE_SECURE:'false',TENANT_ID:tenant,TENANT_NAME:'原 OA 专业奖金 D1 隔离恢复验收',BOOTSTRAP_ADMIN_PASSWORD:password,PROJECT_PARTY_BASE_URL:'',REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE:'reference-oa-production-personnel-v1',REFERENCE_OA_BONUS_PROFILE:'reference-oa-bonus-v3',PARTY_CURSOR_HMAC_KEY:randomBytes(32).toString('hex'),EVIDENCE_LOCAL_SIGNING_KEY:randomBytes(32).toString('base64url'),EVIDENCE_PUBLIC_BASE_URL:origin+'/evidence',REQUESTS_PER_MINUTE:10000,D1_BACKUP_DIR:output+'/backups'};
 let env=readFileSync(root+'deployments/d1/.env.example','utf8').split('\n').map(line=>{const m=line.match(/^([A-Z0-9_]+)=(.*)$/);if(!m)return line;const [_,key]=m;if(key==='POSTGRES_SUPERUSER_PASSWORD'||key.endsWith('_DB_PASSWORD'))values[key]=randomBytes(32).toString('hex');if(key.startsWith('ENABLE_'))values[key]=!!selections[key.slice(7)];return key+'='+(Object.hasOwn(values,key)?values[key]:m[2])}).join('\n');
 save(output+'/d1.env',env);save(output+'/wrong-profile.env',env.replace('REFERENCE_OA_BONUS_PROFILE=reference-oa-bonus-v3','REFERENCE_OA_BONUS_PROFILE=reference-oa-bonus-v2'));
 const builders=['identity-migrate','contract-migrate','people-migrate','project-migrate','evidence-migrate','bonus-distribution-migrate','unified-workbench','gateway'];
 const services=Object.fromEntries([...builders,...apps,'postgres','postgres-provision','nginx','public-keys','public-keys-permissions','project-workload-keys','project-workload-key-permissions','bonus-distribution-workload-keys','bonus-distribution-workload-key-permissions','contract-workload-keys','contract-workload-key-permissions'].map(name=>[name,{labels:{'omsbizfoundry.task':label},cpus:0.75,mem_limit:name==='postgres'?'512m':'192m',...(builders.includes(name)?{build:{context}}:{})}]));
 // Existing unrelated Compose networks can exhaust Docker's automatic pools.
 // Pick four unoccupied private /24s, including one-shot key jobs' default.
 // Never prune other projects' networks.
 const networkIDs=docker('network','ls','--format','{{.ID}}').split('\n').filter(Boolean);
 const networksLive=JSON.parse(execFileSync('docker',['network','inspect',...networkIDs],{encoding:'utf8'}));
 const toInt=s=>s.split('.').reduce((n,v)=>(n*256+Number(v))>>>0,0);
 const overlaps=(a,b)=>{if(a.includes(':')||b.includes(':'))return false;const [ipA,bitsA]=a.split('/'),[ipB,bitsB]=b.split('/'),bits=Math.min(Number(bitsA),Number(bitsB)),mask=bits?((0xffffffff<<(32-bits))>>>0):0;return (toInt(ipA)&mask)===(toInt(ipB)&mask)};
 const used=networksLive.flatMap(n=>(n.IPAM.Config||[]).map(v=>v.Subnet).filter(Boolean));let subnets;
 for(let n=100;n<240;n+=4){const candidate=[n,n+1,n+2,n+3].map(v=>'10.233.'+v+'.0/24');if(candidate.every(s=>!used.some(v=>overlaps(s,v)))){subnets=candidate;break}}
 assert(subnets,'no free task-owned CIDR group');
 const networks=Object.fromEntries(['edge','app','data','default'].map((name,i)=>[name,{ipam:{config:[{subnet:subnets[i]}]}}]));
 save(output+'/compose.override.json',JSON.stringify({services,networks}));
 await run('source',['config','--quiet']);await run('source',['preflight-upgrade']);
 receipt.images=Object.fromEntries(apps.map(app=>[app,docker('image','inspect','--format','{{.Id}}','omsbizfoundry/'+app+':'+tag)]));
 mark('current-source-selected-images-build-and-real-IAM-contract-preflight');
 stage='source-start';await run('source',['up']);await run('source',['check']);assert.equal(docker('ps','--filter','label=com.docker.compose.project='+projects.source,'--format','{{.ID}}').split('\n').length,10);mark('fresh-selected-D1-own-databases-real-nginx-gateway-workbench-ready');
 stage='seed-native';await login();const admin=subject;
 for(const action of ['people-groups.view','people-groups.person.manage'])await grant('people-groups',action,tenantScope);
 await grant('project-workspace','project-workspace.create',tenantScope);await grant('project-workspace','project-workspace.read',tenantScope);
 for(const action of ['project-workspace.reference-oa-discipline.read','project-workspace.reference-oa-discipline.manage'])await grant('project-workspace',action,tenantScope);
 const people=[];for(const [i,name] of ['林设计','陈复核','周审定'].entries())people.push(await request('people-groups','/people','/api/v1/persons',{command_id:randomUUID(),reference_code:'D1-'+i,display_name:name},tenantScope,201));
 const project=await request('project-workspace','/project','/api/v1/projects',{command_id:randomUUID(),project_number:'OA-D1-PROFESSIONAL',name:'原 OA 专业奖金 D1 冷恢复验收',project_type_keys:['design']},tenantScope,201),scope=projectScope(project.id);
 for(const action of ['read','assignment.business-owner.manage','assignment.production-owner.manage','initiation.complete','reference-oa-personnel.read','reference-oa-personnel.manage'])await grant('project-workspace','project-workspace.'+action,scope);
 for(const action of ['read','amount.read','record','withdraw','history.read'])await grant('bonus-distribution','bonus-manual-allocation.'+action,bonusScope(project.id));
 const base='/api/v1/projects/'+project.id,view=()=>request('project-workspace','/project',base+'/reference-oa/production-personnel',undefined,scope);
 await request('project-workspace','/project',base+'/reference-oa/production-profile',{command_id:randomUUID(),expected_revision:project.revision},scope);
 const disciplines=[];for(const name of ['桥梁','道路'])disciplines.push(await request('project-workspace','/project','/api/v1/reference-oa/production-disciplines',{command_id:randomUUID(),operation:'create',expected_revision:0,name,description:'隔离 D1 验收专业'},tenantScope));
 let v=await view();const owner=await request('project-workspace','/project',base+'/assignments',{command_id:randomUUID(),expected_revision:v.project_revision,operation:'assign',role_key:'business_owner',subject_kind:'person',subject_id:people[0].id,source_revision:1},scope);
 const active=await request('project-workspace','/project',base+'/complete-initiation',{command_id:randomUUID(),expected_revision:owner.project.revision,initiation_profile:'business-owner-v1'},scope);
 await request('project-workspace','/project',base+'/assignments',{command_id:randomUUID(),expected_revision:active.revision,operation:'assign',role_key:'production_owner',subject_kind:'person',subject_id:people[2].id,source_revision:1},scope);
 for(const discipline of disciplines){v=await view();await request('project-workspace','/project',base+'/reference-oa/production-personnel',{profile:v.profile,command_id:randomUUID(),expected_project_revision:v.project_revision,operation:'replace_discipline',target_discipline:{id:discipline.id,revision:discipline.revision},changes:[{role:'designer',operation:'replace',people:[{id:people[0].id,revision:people[0].revision}]},{role:'participant',operation:'replace',people:[]},{role:'reviewer',operation:'replace',people:[{id:people[1].id,revision:people[1].revision}]},{role:'auditor',operation:'replace',people:[]}]},scope)}
 mark('formal-native-API-only-fixtures-exact-project-grants-no-SQL-seed');
 stage='file';await grant('evidence-service','evidence-service.upload',tenantScope);const bytes=Buffer.from('原 OA 专业奖金 D1 冷恢复文件证据\n'),sha=hash(bytes),upload=await request('evidence-service','/evidence','/v1/upload-sessions',{file_name:'专业奖金恢复证据.txt',size_bytes:bytes.length,media_type:'text/plain',sha256:sha},tenantScope,201,{'Idempotency-Key':randomUUID()});
 assert.equal((await fetch(upload.authorization.upload_url,{method:upload.authorization.method,headers:upload.authorization.headers,body:bytes})).status,204);
 const file=await request('evidence-service','/evidence','/v1/upload-sessions/'+upload.session_id+'/complete',{observed_size_bytes:bytes.length,observed_sha256:sha},tenantScope,200,{'Idempotency-Key':randomUUID()});
 const fileScope={kind:'resources',resources:[{type:'file',id:file.file_id}]};for(const action of ['metadata.read','download'])await grant('evidence-service','evidence-service.'+action,fileScope);receipt.file={id:file.file_id,version:file.version,sha256:sha,size:bytes.length};mark('native-Evidence-file-id-version-and-real-local-bytes');
 stage='source-original-UI';browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});page=await browser.newPage({viewport:{width:1600,height:1200}});page.setDefaultTimeout(15000);page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname.startsWith('/bonus/api/'))writes.push({path:new URL(r.url()).pathname,body:JSON.parse(r.postData())})});await page.context().addCookies(browserCookies());
 const detail=page.locator('[data-view="project-real-detail"]'),layout=detail.locator('[data-real-production-layout]'),pane=detail.locator('[data-real-production-bonus]'),personnel=detail.locator('[data-real-production-personnel]'),drawer=()=>page.locator('.ant-drawer:visible');
 await page.goto(origin+'/workbench/?project_id='+project.id+'&tab=production&production_tab=bonus');await pane.getByRole('button',{name:/分配奖金/}).waitFor();
 const recordUI=async(description)=>{await pane.getByRole('button',{name:/分配奖金/}).click();const d=drawer();await d.getByLabel('比例池金额',{exact:true}).fill('1.01');await d.getByLabel('备注',{exact:true}).fill(description);for(const [i,profession] of ['桥梁','道路'].entries()){await d.getByRole('button',{name:/添加人员/}).click();const row=d.locator('tbody tr').nth(i);await row.locator('.ant-select').first().click();await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:'设计人（'+profession+'）'}).click();await page.locator('.ant-select-dropdown:visible').waitFor({state:'hidden'});await row.locator('.bonus-allocation-form__value-input input').fill('50')}await d.getByRole('button',{name:'登记奖金分配',exact:true}).click();await d.waitFor({state:'hidden'});await pane.locator('.bonus-allocation__item').nth(1).waitFor()};
 await recordUI('D1 恢复前双专业登记');assert.equal(writes.length,1);command=writes[0].body;assert(command.items.every(i=>i.assignment_source==='reference-oa-production-personnel-v1'));await pane.getByText('¥0.51',{exact:true}).first().waitFor();await page.screenshot({path:output+'/source-original-bonus.png',animations:'disabled'});mark('source-original-bonus-drawer-same-person-two-professions-atomic-native-write');
 const bonusBase='/api/v1/reference-oa/bonus-allocations';const allocations=await request('bonus-distribution','/bonus',bonusBase+'?project_id='+project.id+'&scope=production',undefined,bonusScope(project.id));assert.equal(allocations.items.length,2);lastRecord=allocations.items[0];
 stage='backup';await run('source',['backup']);const backupIDs=readdirSync(output+'/backups').filter(n=>/^\d{8}T\d{6}Z$/.test(n));assert.equal(backupIDs.length,1);const backupID=backupIDs[0],backup=output+'/backups/'+backupID;
 const selection=readFileSync(backup+'/suite-selection.env','utf8');assert(selection.includes('REFERENCE_OA_BONUS_PROFILE=reference-oa-bonus-v3'));assert(selection.includes('REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE=reference-oa-production-personnel-v1'));assert.equal(selection.split('\n').filter(Boolean).length,14);
 receipt.backup={id:backupID,artifacts:Object.fromEntries(readdirSync(backup).filter(n=>statSync(backup+'/'+n).isFile()).map(n=>[n,{sha256:hash(readFileSync(backup+'/'+n)),mode:statSync(backup+'/'+n).mode&0o777}]))};assert(Object.values(receipt.backup.artifacts).every(v=>v.mode===0o600));mark('real-D1-selected-six-owner-backups-keys-bytes-checksums-live-profiles');
 const before=businessSnapshot('source'),iamBefore=snapshot('source','identity_access',['suite_sessions','workload_assertion_nonces','iam_audit_entries']),idsBefore=containers('source');
 stage='mismatch-guard';const rejected=await run('source',['restore',backupID],{envFile:output+'/wrong-profile.env',expect:null,confirm:true});assert(rejected.includes('REFERENCE_OA_BONUS_PROFILE'));assert.deepEqual(containers('source'),idsBefore);assert.deepEqual(businessSnapshot('source'),before);mark('profile-mismatch-real-restore-rejected-before-source-volume-or-fact-mutation');
 const oldCookies=[...cookies].map(([k,v])=>k+'='+v).join('; ');await page.context().close();page=null;
 stage='fresh-target';await run('source',['down']);assert.equal(containers('target').length,0);assert.equal(docker('volume','ls','--filter','label=com.docker.compose.project='+projects.target,'--format','{{.Name}}'),'');
 current='target';await run('target',['restore',backupID],{confirm:true});await run('target',['check']);receipt.before=before;receipt.restored=businessSnapshot('target');assert.deepEqual(receipt.restored,before);assert.deepEqual(snapshot('target','identity_access',['suite_sessions','workload_assertion_nonces','iam_audit_entries']),iamBefore);assert.equal(sql('target','identity_access','SELECT count(*) FROM suite_sessions'),'0');mark('fresh-target-D1-six-databases-immutable-facts-and-exact-grants-restore');
 const old=await fetch(origin+'/auth/api/v1/session',{headers:{Cookie:oldCookies}});assert.equal(old.status,401);await login();assert.equal(subject,admin);mark('old-session-rejected-new-login-same-native-subject-and-grants');
 stage='replay-file';const replayBefore=snapshot('target','bonus_distribution');const replay=await request('bonus-distribution','/bonus',bonusBase,command,bonusScope(project.id),201);assert.equal(replay.items.length,2);assert.deepEqual(snapshot('target','bonus_distribution'),replayBefore);mark('restored-original-command-identical-replay-zero-new-bonus-fact');
 const grantFile=await request('evidence-service','/evidence','/v1/files/'+file.file_id+'/versions/'+file.version+'/access-grants',{disposition:'attachment',delivery_mode:'direct'},fileScope);const restoredBytes=Buffer.from(await (await fetch(grantFile.url)).arrayBuffer());assert.equal(hash(restoredBytes),sha);assert.deepEqual(restoredBytes,bytes);mark('restored-native-Evidence-short-access-original-file-version-and-bytes');
 stage='restored-original-UI';page=await browser.newPage({viewport:{width:1600,height:1200}});page.setDefaultTimeout(15000);page.on('pageerror',e=>errors.push(e.message));await page.context().addCookies(browserCookies());
 const originalURL=origin+'/workbench/?project_id='+project.id+'&tab=production&production_tab=bonus';await page.goto(originalURL);await page.locator('[data-real-production-bonus]').getByText('设计人（桥梁）',{exact:true}).waitFor();await page.locator('[data-real-production-bonus]').getByText('设计人（道路）',{exact:true}).waitFor();assert.deepEqual(snapshot('target','bonus_distribution'),replayBefore);await page.screenshot({path:output+'/restored-original-bonus.png',animations:'disabled'});mark('restored-normal-workbench-original-profession-label-amount-reload-zero-write');
 const restoredPane=page.locator('[data-real-production-bonus]'),card=restoredPane.locator('.bonus-allocation__item').filter({hasText:'设计人（桥梁）'});await card.getByRole('button',{name:/撤回/}).click();const confirm=page.locator('.confirm-delete:visible');await confirm.getByLabel('撤回原因（必填）',{exact:true}).fill('D1 冷恢复后原页面撤回留史');await confirm.getByRole('button',{name:'确认撤回',exact:true}).click();await confirm.waitFor({state:'hidden'});await card.getByText('已撤回 · 历史保留',{exact:true}).waitFor();await card.getByRole('button',{name:'历史',exact:true}).click();let d=page.locator('.ant-drawer:visible');await d.getByText('D1 冷恢复后原页面撤回留史',{exact:true}).waitFor();assert.equal(await d.locator('.bonus-allocation__item').count(),2);await page.screenshot({path:output+'/restored-original-history.png',animations:'disabled'});await d.getByRole('button',{name:/Close|关闭/}).click();await d.waitFor({state:'hidden'});mark('restored-original-withdraw-confirmation-two-immutable-revisions-visible');
 stage='restored-personnel';await page.locator('[data-real-production-layout]').getByRole('tab',{name:/人员配置$/}).click();const p=page.locator('[data-real-production-personnel]');await p.getByText('桥梁',{exact:true}).waitFor();const projectBefore=snapshot('target','project_workspace');await p.getByRole('button',{name:'设置审定人',exact:true}).click();d=page.locator('.ant-drawer:visible');await d.locator('.ant-select').filter({has:page.getByLabel('审定人',{exact:true})}).locator('.ant-select-selector').click();await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:'周审定'}).click();await d.getByRole('button',{name:/^保\s*存$/}).click();await d.waitFor({state:'hidden'});await p.getByText('周审定',{exact:true}).waitFor();assert.notDeepEqual(snapshot('target','project_workspace'),projectBefore);assert.deepEqual(snapshot('target','people_groups'),before.people_groups);await page.screenshot({path:output+'/restored-original-personnel.png',animations:'disabled'});mark('restored-original-personnel-drawer-continues-native-append-no-People-write');
 assert.equal(errors.length,0);receipt.page_errors=errors;receipt.normal_asset_sha256=hash(readFileSync(context+'/frontend/oms-workbench/internal/server/reference_oa_react_bidding.js'));receipt.tenant_id=tenant;receipt.project_id=project.id;receipt.subject_id=subject;receipt.command_id=command.command_id;receipt.final_bonus=snapshot('target','bonus_distribution');receipt.final_project=snapshot('target','project_workspace');receipt.passed=true;
}catch(e){receipt.stage=stage;receipt.error=e.message;receipt.page_errors=errors;process.exitCode=1;console.error(stage+': '+e.message);if(page)await page.screenshot({path:output+'/failed.png',animations:'disabled'}).catch(()=>{})}
finally{
 if(browser)await browser.close();
 for(const which of ['source','target'])try{if(containers(which).length)await run(which,['down'])}catch(e){receipt.cleanup_error=String(e.message);process.exitCode=1}
 const remaining=Object.fromEntries(['source','target'].map(which=>[which,containers(which).length]));
 receipt.cleanup={runtime_stopped:Object.values(remaining).every(n=>n===0),remaining,private_volumes_and_backups_retained:true,projects};
 if(!receipt.cleanup.runtime_stopped||receipt.cleanup_error){receipt.passed=false;process.exitCode=1}
 save(output+'/d1-browser.json',JSON.stringify(receipt,null,2));
}
console.log(JSON.stringify({passed:receipt.passed,scenarios:receipt.scenarios.length,output}));
