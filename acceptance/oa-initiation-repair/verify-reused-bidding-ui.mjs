// Real Chrome rendering/interaction of the compiled original React components.
// Deliberate HTTP contract fixtures, NOT a formal IAM/public business journey.
import { createServer } from 'node:http';
import { readFile, mkdir, writeFile } from 'node:fs/promises';
import { randomUUID, createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
const playwrightRoot=process.env.OA_PLAYWRIGHT_MODULE;
assert(playwrightRoot,'OA_PLAYWRIGHT_MODULE must select an installed Playwright module');
const { chromium } = await import(playwrightRoot);
const root = new URL('../../',import.meta.url), serverRoot = new URL('frontend/oms-workbench/internal/server/',root);
const evidenceDir=process.argv[2];assert(evidenceDir,'new evidence directory required');
await mkdir(evidenceDir,{recursive:true});
const tenant='11111111-1111-4111-8111-111111111111', project='22222222-2222-4222-8222-222222222222', engagement='33333333-3333-4333-8333-333333333333';
const details={project:{id:project,tenant_id:tenant,revision:4,status:'active'},allowed_actions:[],resource_links:[{target_service:'business-engagement',resource_type:'business-engagement',resource_id:engagement}]};
const source={engagement:{id:engagement,tenant_id:tenant,project_id:project,revision:1,status:'collecting',method:'tender',applicability:'applicable'},allowed_actions:['business-engagement.evidence.manage','business-engagement.method.select','business-engagement.applicability.manage'],materials:[],requirements:[]};
const facts={writes:0,uploads:0,access:0,deletes:0},files=new Map(), sessions=new Map(), commands=new Map();let base;
const requirements=()=>source.requirements=['tender_document','bid_document','award_document'].map(key=>({key,required:key!=='tender_document'||source.engagement.method==='tender',satisfied:source.materials.some(row=>row.requirement_key===key&&!row.superseded_at)}));requirements();
const boot=`const details=${JSON.stringify(details)}; const request=async(app,path,options)=>{const r=await fetch('/be'+path,options);const data=await r.json();if(!r.ok)throw Object.assign(new Error(data.error.message),{status:r.status});return data;};const evidenceReferenceRequest=async(id,version)=>{const r=await fetch('/v1/references/files/'+id+'/versions/'+version);if(!r.ok)throw new Error('metadata denied');return r.json()};const flow=new ReferenceOABidding.BiddingFlow({tenantID:'${tenant}',applicationRequest:request,projectRequest:async()=>{throw new Error('no Project writes')},projectAuthorize:async()=>{throw new Error('no Project authorization')},evidenceReferenceRequest});const workspace=ReferenceOAReactBidding.createWorkspace({root:document.querySelector('#panel'),flow,details:()=>details,evidenceReferenceRequest,evidenceFetch:(path,options)=>fetch(path,options),tenantID:'${tenant}',subjectID:'fixture-only',record:()=>{}});workspace.load();`;
const body=async request=>{const chunks=[];for await(const chunk of request)chunks.push(chunk);return Buffer.concat(chunks)};
const web=createServer(async(req,res)=>{
  try {
    const path=new URL(req.url,base).pathname;const json=(data,status=200)=>{res.writeHead(status,{'Content-Type':'application/json'});res.end(JSON.stringify(data))};
    if(path==='/'){res.writeHead(200,{'Content-Type':'text/html','Content-Security-Policy':"default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; frame-src 'self' blob:; connect-src 'self'; form-action 'self'; frame-ancestors 'none'"});res.end('<!doctype html><html lang="zh-CN"><head><meta charset="UTF-8"><link rel="stylesheet" href="/assets/reference_oa_react_bidding.css"><link rel="stylesheet" href="/assets/reference_oa_workspace.css"><link rel="stylesheet" href="/assets/reference_oa_detail.css"></head><body style="margin:28px;background:#f6f7f2"><section id="panel" class="project-workspace__card business-info-tab__surface-card"><div data-real-bidding-react></div></section><script src="/assets/reference_oa_bidding_runtime.js"></script><script src="/assets/reference_oa_react_bidding.js"></script><script src="/bootstrap.js"></script></body></html>');return;}
    if(path==='/bootstrap.js'){res.writeHead(200,{'Content-Type':'text/javascript'});res.end(boot);return;}
    if(path.startsWith('/assets/')){const name=path.slice(8);assert(/^[a-z_]+\.(js|css)$/.test(name));res.writeHead(200,{'Content-Type':name.endsWith('.css')?'text/css':'text/javascript'});res.end(await readFile(new URL(name,serverRoot)));return;}
    if(path.startsWith('/be/')) {
      if(req.method==='GET')return json(path.includes('/business-engagements/')?source:{project_id:project,items:[source.engagement],allowed_actions:source.allowed_actions});
      const value=JSON.parse((await body(req)).toString());
      if(!commands.has(value.command_id)){
        if(value.expected_revision!==source.engagement.revision)return json({error:{message:'version conflict'}},409);
        facts.writes++;source.engagement.revision++;
        if(path.endsWith('/method'))source.engagement.method=value.method;
        else if(value.operation==='append')source.materials.push({id:randomUUID(),tenant_id:tenant,engagement_id:engagement,requirement_key:value.requirement_key,evidence_file_id:value.evidence_file_id,evidence_version:value.evidence_version,evidence_source_revision:value.evidence_source_revision,created_at:'2026-10-03T00:00:00Z'});
        else if(value.operation==='remove_one')source.materials.find(row=>row.id===value.material_reference_id).superseded_at='2026-10-03T01:00:00Z';
        else throw new Error('unexpected fixture command');
        requirements();commands.set(value.command_id,structuredClone(source.engagement));
      }
      return json({engagement:commands.get(value.command_id)});
    }
    if(path==='/v1/upload-sessions'){
      const input=JSON.parse((await body(req)).toString()),session=randomUUID(),id=randomUUID();const expires=new Date(Date.now()+600000).toISOString();
      sessions.set(session,{id,input});return json({session_id:session,file_id:id,version:1,status:'pending',expires_at:expires,authorization:{provider:'local-http',upload_url:base+'/upload/'+session,method:'PUT',object_key:id,expires_at:expires,scope:{object_key:id,allowed_actions:['evidence:PutObject']}}});
    }
    if(path.startsWith('/upload/')){const s=sessions.get(path.split('/').at(-1));s.bytes=await body(req);facts.uploads++;res.writeHead(200,{ETag:'fixture-etag'});res.end();return;}
    if(path.endsWith('/complete')){const s=sessions.get(path.split('/')[3]);assert(s.bytes.length===s.input.size_bytes);const version={file_id:s.id,version:1,status:'available',file_name:s.input.file_name,size_bytes:s.bytes.length,media_type:s.input.media_type,sha256:createHash('sha256').update(s.bytes).digest('hex'),created_at:'2026-10-03T00:00:00Z'};files.set(s.id,{...version,bytes:s.bytes});return json(version);}
    if(path.startsWith('/v1/references/')){const file=files.get(path.split('/')[4]);assert(file);const {bytes,...v}=file;return json({...v,schema_version:'1',source_application:'evidence-service',resource_type:'evidence-file',id:file.file_id,tenant_id:tenant,revision:1});}
    if(path.endsWith('/access-grants')){const id=path.split('/')[3],input=JSON.parse((await body(req)).toString());facts.access++;return json({file_id:id,version:1,url:base+'/download/'+id,method:'GET',disposition:input.disposition,delivery_mode:input.delivery_mode,expires_at:new Date(Date.now()+60000).toISOString(),refresh_after:new Date(Date.now()+30000).toISOString()});}
    if(path.startsWith('/download/')){const file=files.get(path.split('/').at(-1));res.writeHead(200,{'Content-Type':file.media_type,'Content-Disposition':'attachment; filename="fixture.png"'});res.end(file.bytes);return;}
    if(req.method==='DELETE')facts.deletes++;res.writeHead(404);res.end();
  }catch(error){res.writeHead(500,{'Content-Type':'application/json'});res.end(JSON.stringify({error:{message:error.message}}));}
});
await new Promise(resolve=>web.listen(0,'127.0.0.1',resolve));base='http://127.0.0.1:'+web.address().port;
let browser;let page;const errors=[];let passed=false;
try {
  browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
  page=await browser.newPage({viewport:{width:1440,height:1000}});page.on('pageerror',error=>errors.push(error.message));
  await page.goto(base);await page.locator('.bidding-file-slot').nth(2).waitFor();
  assert.equal(await page.locator('.bidding-file-slot').count(),3);assert.equal(facts.writes,0);
  await page.screenshot({path:evidenceDir+'/empty.png',fullPage:true});
  const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aX1kAAAAASUVORK5CYII=','base64');
  const slot=page.locator('.bidding-file-slot').first();
  await slot.locator('input[type=file]').setInputFiles(['招标-第一份.png','招标-第二份.png'].map(name=>({name,mimeType:'image/png',buffer:png})));
  for(const name of ['招标-第一份.png','招标-第二份.png'])await slot.getByRole('button',{name,exact:true}).waitFor();
  assert.equal(source.materials.length,2);assert.equal(files.size,2);assert.equal(facts.uploads,2);
  await page.screenshot({path:evidenceDir+'/two-files.png',fullPage:true});
  await slot.getByRole('button',{name:'招标-第一份.png',exact:true}).click();await page.locator('.file-preview-drawer__shell img').waitFor();
  await page.screenshot({path:evidenceDir+'/preview.png',fullPage:true});
  await page.locator('.ant-drawer-close').click();
  await slot.locator('.bidding-file-slot__file-entry').first().getByRole('button',{name:/移\s*除/}).click();
  await page.getByRole('dialog').getByRole('button',{name:'确认移除',exact:true}).click();
  await slot.getByRole('button',{name:'招标-第一份.png',exact:true}).waitFor({state:'detached'});
  assert.equal(source.materials.filter(x=>!x.superseded_at).length,1);assert.equal(files.size,2);assert.equal(facts.deletes,0);
  const download=page.waitForEvent('download');await slot.locator('.bidding-file-slot__file-entry').getByRole('button',{name:/下\s*载/}).click();await download;
  const before=facts.writes;await page.getByRole('button',{name:'刷新当前资料',exact:true}).click();await slot.getByRole('button',{name:'招标-第二份.png',exact:true}).waitFor();assert.equal(facts.writes,before);
  await page.screenshot({path:evidenceDir+'/after-remove.png',fullPage:true});
  assert.deepEqual(errors,[]);passed=true;
  console.log(JSON.stringify({passed,scope:'Compiled original React UI + HTTP contract fixtures; NOT real IAM or public deployment',facts,active_materials:1,files_preserved:2,browser_errors:errors}));
}finally{
  if(!passed&&page)await page.screenshot({path:evidenceDir+'/failed.png',fullPage:true}).catch(()=>{});
  await writeFile(evidenceDir+'/browser-fixture.json',JSON.stringify({passed,scope:'Rendering/interaction only, fixture HTTP owner APIs',facts,browser_errors:errors},null,2));
  await browser?.close();await new Promise(resolve=>web.close(resolve));
}
