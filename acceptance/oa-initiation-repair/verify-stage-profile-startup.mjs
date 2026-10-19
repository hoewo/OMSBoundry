// Bounded standalone startup checks; no Docker, database, IAM grants or Main.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {createServer} from 'node:net';
import {readFileSync,writeFileSync} from 'node:fs';
import {createHash,randomUUID} from 'node:crypto';
const [oldBinary,fixedBinary,output]=process.argv.slice(2);
for(const path of [oldBinary,fixedBinary])assert(/^\/tmp\/oa-stage-suite\.[A-Za-z0-9]+\/unified-workbench$/.test(path||''));
assert(/^\/tmp\/oa-stage-profile\.[A-Za-z0-9]+$/.test(output||''));
const sha=path=>createHash('sha256').update(readFileSync(path)).digest('hex');
const receipt={started_at:new Date().toISOString(),passed:false,checks:[],
  evidence_layer:'actual-standalone-Workbench-startup-not-business-chain',
  local_docker_started:false,main_changed:false,owner_business_accepted:false,
  binaries:{before:sha(oldBinary),after:sha(fixedBinary)}};
const tenant=randomUUID(),profile='reference-oa-stage-materials-v1';
const apps=['identity-access','unified-workbench','project-workspace','production-operations','evidence-service'];
const manifest=people=>({schema_version:'1',suite_id:'stage-startup-private',tenant_id:tenant,
  applications:[...apps,...(people?['people-groups']:[])].map(id=>({id,version:'0.1.0',audience:id,
    enabled:true,required:apps.slice(0,2).includes(id),route_prefix:'/'+id,
    upstream:'http://127.0.0.1:9',readiness_path:'/health/ready'}))});
async function run(name,binary,selectedProfile,people,expected){
  const sock=createServer();await new Promise(r=>sock.listen(0,'127.0.0.1',r));const port=sock.address().port;await new Promise(r=>sock.close(r));
  const path=output+'/'+name+'.json';writeFileSync(path,JSON.stringify(manifest(people)),{mode:0o600});
  const child=spawn(binary,[],{env:{PATH:process.env.PATH,GOMAXPROCS:'2',HTTP_ADDR:'127.0.0.1:'+port,
    SUITE_MANIFEST_PATH:path,REFERENCE_OA_PRODUCTION_STAGE_PROFILE:selectedProfile},stdio:['ignore','pipe','pipe']});
  let logs='';for(const s of [child.stdout,child.stderr])s.on('data',v=>logs+=v);
  let ready=false,stopped=false;
  try{
    for(let i=0;i<50;i++){
      if(child.exitCode!==null)break;
      try{if((await fetch('http://127.0.0.1:'+port+'/health/ready')).ok){ready=true;break}}catch{}
      await new Promise(r=>setTimeout(r,100));
    }
    if(expected==='ready')assert(ready,name+' did not start');
    else {assert.equal(ready,false,name+' accepted invalid profile');assert.equal(child.exitCode,1,name+' did not exit');assert(logs.includes(expected),name+' wrong rejection');}
    receipt.checks.push({name,passed:true,ready,exit_code:child.exitCode,
      expected_rejection:expected==='ready'?null:expected});
    console.log('PASS '+name);
  } finally {
    if(child.exitCode===null)await new Promise(r=>{
      const timer=setTimeout(()=>child.kill('SIGKILL'),3000);
      child.once('exit',()=>{clearTimeout(timer);stopped=true;r()});child.kill('SIGTERM');
    });else stopped=true;
    assert(stopped,name+' process not stopped');
  }
}
try{
  await run('reproduced-before-patch-accepted-missing-People',oldBinary,profile,false,'ready');
  await run('fixed-profile-rejects-missing-People',fixedBinary,profile,false,'matching Production, Evidence, Project and People');
  await run('fixed-default-off-preserves-minimal-Suite-startup',fixedBinary,'',false,'ready');
  await run('fixed-profile-on-all-four-owners-starts',fixedBinary,profile,true,'ready');
  await run('fixed-unknown-profile-rejected-before-listening',fixedBinary,'unknown',true,'unsupported REFERENCE_OA_PRODUCTION_STAGE_PROFILE');
  receipt.passed=true;receipt.all_processes_stopped=true;
}catch(e){receipt.error=e.message;process.exitCode=1}
receipt.source_files=['frontend/oms-workbench/internal/server/server.go','frontend/oms-workbench/internal/server/server_test.go','frontend/oms-workbench/internal/config/config.go'].map(path=>({path,sha256:sha(path)}));
receipt.finished_at=new Date().toISOString();
writeFileSync(output+'/startup.json',JSON.stringify(receipt,null,2),{mode:0o600});
console.log(JSON.stringify({passed:receipt.passed,checks:receipt.checks.length,output}));
