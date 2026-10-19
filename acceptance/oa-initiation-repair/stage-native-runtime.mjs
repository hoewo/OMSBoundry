// Low-memory private native assembly: local Go processes, remote bounded PG18
// and Nginx through loopback-only SSH forwards. No local Docker or Main writes.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {spawn,spawnSync,execFileSync} from 'node:child_process';
import {readFileSync,writeFileSync,mkdirSync,cpSync,readdirSync} from 'node:fs';
import {randomUUID,randomBytes,createHash} from 'node:crypto';
import {applicationSourcePath} from './application-source-paths.mjs';

export async function createStageRuntime({root,output}){
 assert(/^\/tmp\/oa-(?:stage|preliminary|construction|change-completion|test-report)-suite\.[A-Za-z0-9]+$/.test(output));
 const suffix=randomBytes(5).toString('hex'),pg='oa-stage-pg-'+suffix,edge='oa-stage-edge-'+suffix,remote='/tmp/oa-stage-suite-'+suffix,label='oms.task=oa-stage-native-browser';
 const host='ecs-user@47.98.156.166',sshArgs=['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes'],password=randomUUID()+randomUUID(),dbPassword=randomUUID(),tenant=randomUUID(),issuer='https://oa-stage.private.test';
 const apps=['identity-access','people-groups','project-workspace','evidence-service','production-operations','unified-workbench','gateway'],modules={'production-operations':'design-operations'},urls={},ports={},dbs={},children=[],byApp={},logs={},cookies=new Map(),exchanges=[],localSigningKey=randomBytes(32).toString('base64url');
 const pgImage='crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres:18.6-alpine@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66',nginxImage='crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-nginx:1.28-alpine';
 let tunnel,exchange,csrf,subject,pgStarted=false,edgeStarted=false,closed=false,bytesRoot=output+'/bytes',restored=false;
 const run=(cmd,args)=>execFileSync(cmd,args,{encoding:'utf8',timeout:60000,stdio:['ignore','pipe','pipe']}).trim(),ssh=cmd=>run('ssh',[...sshArgs,host,cmd]);
 const freePort=async()=>{const server=createServer();await new Promise(r=>server.listen(0,'127.0.0.1',r));const port=server.address().port;await new Promise(r=>server.close(r));return port};
 const mainBefore=ssh('docker ps --filter name=omsbizfoundry --no-trunc --format "{{.Names}} {{.ID}} {{.Image}}"');
 const stopChild=async child=>{if(!child||child.exitCode!==null)return;await new Promise(resolve=>{const timer=setTimeout(()=>{child.kill('SIGKILL')},5000);child.once('exit',()=>{clearTimeout(timer);resolve()});child.kill('SIGTERM')})};
 const stopContainer=name=>{const v=JSON.parse(ssh(`docker inspect '${name}' --format '{{json .}}'`));assert.equal(v.Name,'/'+name);assert.equal(v.Config.Labels['oms.task'],'oa-stage-native-browser');ssh(`docker stop --time 10 '${name}'`)};
 const exitCleanup=()=>{for(const c of children)if(c.exitCode===null)c.kill('SIGTERM');if(tunnel?.exitCode===null)tunnel.kill('SIGTERM')};process.once('exit',exitCleanup);
 async function close(){if(closed)return;closed=true;try{if(edgeStarted)stopContainer(edge)}finally{try{for(const child of children.reverse())await stopChild(child);if(exchange)await new Promise(r=>exchange.close(r));await stopChild(tunnel)}finally{if(pgStarted)stopContainer(pg);process.removeListener('exit',exitCleanup);for(const [a,log]of Object.entries(logs))writeFileSync(output+'/'+a+'.log',log.text,{mode:0o600});assert.equal(ssh('docker ps --filter name=omsbizfoundry --no-trunc --format "{{.Names}} {{.ID}} {{.Image}}"'),mainBefore,'Main container IDs and images unchanged')}}}
 const sql=(app,statement)=>ssh(`docker exec '${pg}' psql -X -A -t -v ON_ERROR_STOP=1 -U postgres -d '${dbs[app]}' -c '${statement.replaceAll("'","'\\''")}'`);
 async function iam(path,body,status=200){const r=await fetch(urls['identity-access']+path,{method:body===undefined?'GET':'POST',headers:{Cookie:[...cookies].map(([k,v])=>k+'='+v).join('; '),...(body===undefined?{}:{'Content-Type':'application/json','X-CSRF-Token':csrf})},...(body===undefined?{}:{body:JSON.stringify(body)})});for(const c of r.headers.getSetCookie()){const [k,v]=c.split(';',1)[0].split('=');cookies.set(k,v)}assert.equal(r.status,status,'private IAM '+path);const text=await r.text();return text?JSON.parse(text):null}
 const tenantScope={kind:'tenant'},scope=id=>({kind:'resources',resources:[{type:'project',id}]}),grant=(a,action,s=tenantScope,revoke=false)=>iam('/api/v1/admin/grants'+(revoke?'/revoke':''),{subject_id:subject,audience:a,action,scope:s},204);
 async function request(app,path,body,s=tenantScope,expected=200){const credential=await iam('/api/v1/credentials',{audience:app,scope:s});const r=await fetch(urls[app]+path,{method:body===undefined?'GET':'POST',headers:{Authorization:'Bearer '+credential.token,...(body===undefined?{}:{'Content-Type':'application/json'})},...(body===undefined?{}:{body:JSON.stringify(body)})});const text=await r.text();assert.equal(r.status,expected,app+' '+path+' '+text.slice(0,200));return text?JSON.parse(text):null}
 try{
  assert.equal(ssh("ss -lnt | awk '/2810[1-4]/'").trim(),'','private ports must be unused');
  for(const app of apps){ports[app]=await freePort();urls[app]='http://127.0.0.1:'+ports[app];const built=spawnSync('go',['build','-p','1','-o',output+'/'+app,'./cmd/server'],{cwd:applicationSourcePath(root,app,modules),env:{...process.env,GOMAXPROCS:'2',GOWORK:'off'},encoding:'utf8',timeout:60000});assert.equal(built.status,0,built.stderr)}
  const dbPort=await freePort(),edgePort=await freePort(),browserURL='http://127.0.0.1:'+edgePort;
  ssh(`test ! -d '${remote}' && mkdir -m 700 '${remote}'; docker run -d --name '${pg}' --label '${label}' --memory 256m --cpus 0.5 --pids-limit 128 --tmpfs /var/lib/postgresql:rw,size=268435456 -p 127.0.0.1:28101:5432 -e POSTGRES_PASSWORD='${dbPassword}' '${pgImage}' -c shared_buffers=16MB -c work_mem=1MB -c max_connections=60`);pgStarted=true;
  ssh(`for n in 1 2 3 4 5 6 7 8 9 10; do docker exec '${pg}' pg_isready -U postgres >/dev/null 2>&1 && exit 0; sleep 1; done; exit 1`);
  tunnel=spawn('ssh',[...sshArgs,'-o','ExitOnForwardFailure=yes','-N','-L',`127.0.0.1:${dbPort}:127.0.0.1:28101`,'-L',`127.0.0.1:${edgePort}:127.0.0.1:28102`,'-R',`127.0.0.1:28103:127.0.0.1:${ports.gateway}`,'-R',`127.0.0.1:28104:127.0.0.1:${ports['evidence-service']}`,host],{stdio:['ignore','pipe','pipe']});logs.tunnel={text:''};tunnel.stderr.on('data',v=>logs.tunnel.text+=v);await new Promise(r=>setTimeout(r,500));assert.equal(tunnel.exitCode,null,'private SSH forwards');
  for(const app of apps.slice(0,5)){dbs[app]='stage_'+suffix+'_'+app.replaceAll('-','_');ssh(`docker exec '${pg}' createdb -U postgres '${dbs[app]}'`)}
  const identities=[];for(const [app,permissions]of [['project-workspace',[{audience:'people-groups',action:'people-groups.view',resource_types:['person']}]],['production-operations',[{audience:'project-workspace',action:'project-workspace.read',resource_types:['project']},{audience:'people-groups',action:'people-groups.policy-fact.read',resource_types:['person']},{audience:'evidence-service',action:'evidence-service.metadata.read',resource_types:['file']}]]]){
   const subjectID=randomUUID(),ctl=spawnSync('go',['run','./cmd/workload-keyctl','init'],{cwd:root+'packages/workload-identity',env:{...process.env,GOWORK:'off',GOMAXPROCS:'2',WORKLOAD_ID:app,WORKLOAD_SUBJECT_ID:subjectID,WORKLOAD_KEY_FILE:output+'/'+app+'-private.json',WORKLOAD_PUBLIC_KEY_FILE:output+'/'+app+'-public.json'},encoding:'utf8',timeout:60000});assert.equal(ctl.status,0,ctl.stderr);identities.push({id:app,subject_id:subjectID,display_name:'独立 '+app,public_keys_path:output+'/'+app+'-public.json',permissions});
  }
  writeFileSync(output+'/workload-policy.json',JSON.stringify({schema_version:'1',identities}),{mode:0o600});mkdirSync(output+'/bytes');
  const routes={'identity-access':'/auth','unified-workbench':'/workbench','people-groups':'/people','project-workspace':'/project','evidence-service':'/evidence','production-operations':'/production'};
  const manifest={schema_version:'1',suite_id:'oa-stage-native-private',tenant_id:tenant,applications:apps.filter(a=>a!=='gateway').map(a=>({id:a,version:'0.1.0',audience:a,enabled:true,required:['identity-access','unified-workbench'].includes(a),route_prefix:routes[a],upstream:urls[a],readiness_path:'/health/ready',...(['identity-access','unified-workbench'].includes(a)?{}:a==='production-operations'?{capability_namespace:'production-work-item',entry_action:'production-work-item.read'}:{entry_action:a==='people-groups'?'people-groups.view':a==='evidence-service'?'evidence-service.view':'project-workspace.read'})}))};
  writeFileSync(output+'/suite.json',JSON.stringify(manifest),{mode:0o600});
  async function start(app){
   const env={PATH:process.env.PATH,GOMAXPROCS:'2',HTTP_ADDR:'127.0.0.1:'+ports[app],MIGRATION_MODE:'auto',DATABASE_URL:`postgres://postgres:${dbPassword}@127.0.0.1:${dbPort}/${dbs[app]}?sslmode=disable`};
   if(app==='identity-access')Object.assign(env,{TENANT_ID:tenant,TENANT_NAME:'方案私有验收',PUBLIC_ISSUER:issuer,SIGNING_KEY_FILE:output+'/iam-private.json',SESSION_COOKIE_SECURE:'false',BOOTSTRAP_ADMIN_USERNAME:'admin',BOOTSTRAP_ADMIN_PASSWORD:password,BOOTSTRAP_ADMIN_DISPLAY_NAME:'隔离验收',CAPABILITY_MANIFEST_PATHS:apps.slice(1,5).map(a=>root+'contracts/access/v1/examples/'+a+'-capabilities.json').join(','),WORKLOAD_IDENTITY_POLICY_PATH:output+'/workload-policy.json'});
   else if(app==='unified-workbench')Object.assign(env,{SUITE_MANIFEST_PATH:output+'/suite.json',REFERENCE_OA_PRODUCTION_STAGE_PROFILE:'reference-oa-stage-materials-v1'});
   else if(app==='gateway')Object.assign(env,{SUITE_MANIFEST_PATH:output+'/suite.json',IAM_PUBLIC_KEYS_PATH:output+'/iam-public.json',IAM_ISSUER:issuer,COOKIE_SECURE:'false',REQUESTS_PER_MINUTE:'10000'});
   else{Object.assign(env,{IDENTITY_PROVIDER:'iam',IAM_ISSUER:issuer,IAM_TENANT_ID:tenant,IAM_PUBLIC_KEYS_PATH:output+'/iam-public.json'});
    // This fixture reaches PostgreSQL through an SSH loopback and runs it under
    // deliberately small remote CPU/memory limits. Keep the product's bounded
    // reference timeout, but do not make transport jitter look like a domain
    // rejection in the browser acceptance.
    if(['project-workspace','production-operations'].includes(app)){const identity=identities.find(i=>i.id===app);Object.assign(env,{PEOPLE_BASE_URL:urls['people-groups'],IAM_SERVICE_CREDENTIAL_URL:urls.exchange+'/api/v1/service-credentials',WORKLOAD_ID:app,WORKLOAD_SUBJECT_ID:identity.subject_id,WORKLOAD_KEY_FILE:output+'/'+app+'-private.json',REFERENCE_TIMEOUT:'15s'});}
    if(app==='production-operations')Object.assign(env,{PROJECT_BASE_URL:urls['project-workspace'],EVIDENCE_BASE_URL:urls['evidence-service'],REFERENCE_STAGE_MATERIAL_PROFILE:'reference-oa-stage-materials-v1'});
    if(app==='evidence-service')Object.assign(env,{STORAGE_PROVIDER:'local',LOCAL_STORAGE_ROOT:bytesRoot,LOCAL_SIGNING_KEY:localSigningKey,PUBLIC_BASE_URL:browserURL+'/evidence',MAX_UPLOAD_BYTES:'209715200'});
   }
   const child=spawn(output+'/'+app,[],{env,stdio:['ignore','pipe','pipe']});children.push(child);byApp[app]=child;logs[app]??={text:''};for(const stream of [child.stdout,child.stderr])stream.on('data',v=>logs[app].text+=v);
   for(let n=0;n<100;n++){if(child.exitCode!==null)throw Error(app+' stopped: '+logs[app].text.slice(-500));try{if((await fetch(urls[app]+'/health/ready')).ok)return}catch{}await new Promise(r=>setTimeout(r,100))}throw Error(app+' readiness timeout');
  }
  await start('identity-access');const jwks=await fetch(urls['identity-access']+'/api/v1/.well-known/jwks.json');assert(jwks.ok);writeFileSync(output+'/iam-public.json',await jwks.text(),{mode:0o600});csrf=(await iam('/api/v1/csrf')).csrf_token;await iam('/api/v1/login',{username:'admin',password});subject=(await iam('/api/v1/session')).principal.subject_id;
  exchange=createServer(async(req,res)=>{let trace;const started=Date.now();try{const chunks=[];for await(const c of req)chunks.push(c);const data=Buffer.concat(chunks),v=JSON.parse(data);trace={audience:v.audience,action:v.action,scope:v.scope};exchanges.push(trace);const r=await fetch(urls['identity-access']+req.url,{method:req.method,headers:{'Content-Type':'application/json'},body:data}),payload=Buffer.from(await r.arrayBuffer());trace.status=r.status;trace.duration_ms=Date.now()-started;if(!r.ok)trace.error_body=payload.toString('utf8').slice(0,500);res.statusCode=r.status;res.setHeader('Content-Type','application/json');res.end(payload)}catch(error){if(trace){trace.status=503;trace.duration_ms=Date.now()-started;trace.transport_error=String(error?.message||error).slice(0,500)}res.statusCode=503;res.end('{}')}});await new Promise(r=>exchange.listen(0,'127.0.0.1',r));urls.exchange='http://127.0.0.1:'+exchange.address().port;
  for(const a of ['people-groups','project-workspace','evidence-service','production-operations'])await start(a);
  for(const action of ['people-groups.view','people-groups.person.manage'])await grant('people-groups',action);
  const person=await request('people-groups','/api/v1/persons',{command_id:randomUUID(),reference_code:'STAGE-'+suffix,display_name:'阶段验收负责人'},tenantScope,201);
  // The directory uses Tenant read in this one-project disposable Tenant.
  // All new stage mutation grants remain exact Project resources.
  await grant('project-workspace','project-workspace.read');await grant('project-workspace','project-workspace.create');const created=await request('project-workspace','/api/v1/projects',{command_id:randomUUID(),project_number:'OA-STAGE-'+suffix,name:'原 OA 方案多文件真实验收',project_type_keys:['design','consultation','survey','mapping']},tenantScope,201);const projectScope=scope(created.id);
  for(const action of ['project-workspace.read','project-workspace.assignment.business-owner.manage','project-workspace.initiation.complete'])await grant('project-workspace',action,projectScope);
  const owner=await request('project-workspace','/api/v1/projects/'+created.id+'/assignments',{command_id:randomUUID(),expected_revision:created.revision,operation:'assign',role_key:'business_owner',subject_kind:'person',subject_id:person.id,source_revision:1},projectScope);
  const initiated=await request('project-workspace','/api/v1/projects/'+created.id+'/complete-initiation',{command_id:randomUUID(),expected_revision:owner.project.revision,initiation_profile:'business-owner-v1'},projectScope);
  for(const action of ['production-work-item.read','production-work-item.create',...['read','register','withdraw','history.read'].map(n=>'production-work-item.stage-material.'+n)])await grant('production-operations',action,projectScope);
  for(const action of ['evidence-service.view','evidence-service.metadata.read','evidence-service.upload','evidence-service.download'])await grant('evidence-service',action);
  await start('unified-workbench');await start('gateway');
  const nginx=readFileSync(root+'deployments/d1/nginx/nginx.conf','utf8').replace('worker_processes auto;','worker_processes 1;').replace('server gateway:8088;','server 127.0.0.1:28103;').replace('listen 8080;','listen 127.0.0.1:28102;').replace('set $evidence_data_upstream http://evidence-service:8085;','set $evidence_data_upstream http://127.0.0.1:28104;');
  writeFileSync(output+'/nginx.conf',nginx,{mode:0o600});run('scp',[...sshArgs,output+'/nginx.conf',host+':'+remote+'/nginx.conf']);ssh(`docker run -d --name '${edge}' --label '${label}' --network host --memory 40m --cpus 0.25 --pids-limit 64 -v '${remote}/nginx.conf:/etc/nginx/nginx.conf:ro' '${nginxImage}'`);edgeStarted=true;
  let edgeReady=false;for(let n=0;n<50;n++){try{if((await fetch(browserURL+'/workbench/')).ok){edgeReady=true;break}}catch{}await new Promise(r=>setTimeout(r,100))}assert(edgeReady,'private edge readiness');
  const tableNames={};
  const digest=app=>{
   tableNames[app]??=sql(app,"SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' ORDER BY table_name").split('\n');
   assert(tableNames[app].every(t=>/^[a-z_][a-z0-9_]*$/.test(t)));
   const query='SELECT jsonb_build_object('+tableNames[app].map(t=>`'${t}',(SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),'[]'::jsonb) FROM "${t}" r)`).join(',')+')';
   return createHash('sha256').update(sql(app,query)).digest('hex');
  };
  const stopReference=async app=>{assert.equal(app,'evidence-service');await stopChild(byApp[app])},restartReference=async app=>{assert.equal(app,'evidence-service');assert.notEqual(byApp[app].exitCode,null);await start(app)};
  async function restoreEmptyOwners(){
   assert.equal(restored,false,'one explicitly bounded empty-target restore');restored=true;
   for(const app of [...apps].reverse())await stopChild(byApp[app]);
   const owners=apps.slice(0,5),before={},after={},backups={};
   const tree=(dir,prefix='')=>readdirSync(dir,{withFileTypes:true}).sort((a,b)=>a.name.localeCompare(b.name)).flatMap(entry=>{assert(!entry.isSymbolicLink(),'fixture byte symlinks forbidden');const relative=prefix+entry.name;return entry.isDirectory()?tree(dir+'/'+entry.name,relative+'/'):[{path:relative,sha256:createHash('sha256').update(readFileSync(dir+'/'+entry.name)).digest('hex')}]});
   const bytesBefore=tree(bytesRoot);assert(bytesBefore.length>0);const sourceDatabases={...dbs};
   for(const app of owners){before[app]=digest(app);const archive='/tmp/'+app+'.dump';ssh(`docker exec '${pg}' sh -c 'umask 077; pg_dump -Fc -U postgres -d ${dbs[app]} -f ${archive}'`);backups[app]=ssh(`docker exec '${pg}' sha256sum '${archive}'`).split(' ')[0];ssh(`docker cp '${pg}:${archive}' '${remote}/${app}.dump'; chmod 600 '${remote}/${app}.dump'`);const target=dbs[app]+'_restored';ssh(`docker exec '${pg}' createdb -U postgres '${target}'`);dbs[app]=target;assert.equal(sql(app,"SELECT count(*) FROM information_schema.tables WHERE table_schema='public'"),'0');ssh(`docker exec '${pg}' pg_restore --exit-on-error -U postgres -d '${target}' '${archive}'`);after[app]=digest(app);assert.equal(after[app],before[app],app+' all tables restored unchanged')}
   const nextBytes=output+'/restored-bytes';cpSync(bytesRoot,nextBytes,{recursive:true,force:false,errorOnExist:true});bytesRoot=nextBytes;const bytesAfter=tree(bytesRoot);assert.deepEqual(bytesAfter,bytesBefore);
   for(const app of apps)await start(app);
   return {mode:'private-native-five-owner-logical-empty-databases-and-copied-local-bytes',d1_script_executed:false,main_changed:false,source_databases:sourceDatabases,target_databases:{...dbs},before,after,archive_sha256:backups,bytes_before:bytesBefore,bytes_after:bytesAfter,signing_key_identity_preserved:true};
  }
  const identitiesRunning=ssh(`docker inspect '${pg}' '${edge}' --format '{{.Id}} {{.Image}}'`).split('\n').map(line=>{const [id,image]=line.split(' ');return {id,image}});
  return {tenant,subject,projectID:created.id,projectRevision:initiated.revision,projectScope,tenantScope,urls,browserURL,grant,request,sql,digest,exchanges,close,stopReference,restartReference,restoreEmptyOwners,cookies:[...cookies].map(([name,value])=>({name,value,domain:'127.0.0.1',path:'/',httpOnly:name!=='oms_iam_csrf',sameSite:'Lax'})),infra:{pg,edge,remote,local_docker:false,main_changed:false,actual_container_identities:identitiesRunning,nginx_sha256:createHash('sha256').update(nginx).digest('hex')}};
 }catch(e){await close();throw e}
}
