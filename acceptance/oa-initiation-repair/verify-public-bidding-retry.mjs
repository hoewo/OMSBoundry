// Continue the same G facts after public-g's committed command / early observation.
// This adds one explicit acceptance file; no hidden reset or replacement fixture.
import {readFile,mkdir,writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {setTimeout as delay} from 'node:timers/promises';
import assert from 'node:assert/strict';
const [sessionPath,out]=process.argv.slice(2);assert(sessionPath&&out&&process.env.OA_PLAYWRIGHT_MODULE);
const {chromium}=await import(process.env.OA_PLAYWRIGHT_MODULE);
const state=JSON.parse(await readFile(sessionPath,'utf8'));
const base='https://oatest.antuinfo.cn',project='3b4c44d3-7b33-4a29-add3-3f6211ca893e',source='231f2977-79d4-4b36-a178-f19a0d8d3119';
const result={passed:false,scope:'Real native owner commit + browser response loss + reload/replay; same G fixture',fixture_id:project,engagement_id:source,project_writes:0,grant_writes:0,physical_file_deletes:0,full_oa_accepted:false,browser_errors:[],stages:[]};
await mkdir(out,{recursive:true});let browser,page;
const hash=value=>createHash('sha256').update(value).digest('hex');
try {
  browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
  const context=await browser.newContext({viewport:{width:1600,height:1100},storageState:{cookies:state.cookies,origins:[]}});
  const read=async(path,application)=>{
    let response=await context.request.get(base+path);
    if(!response.headers()['content-type']?.includes('application/json')||[401,403].includes(response.status())){
      const csrf=await(await context.request.get(base+'/auth/api/v1/csrf')).json();
      assert((await context.request.post(base+'/gateway/authorize/'+application,{form:{csrf_token:csrf.csrf_token},headers:{Origin:base}})).ok());
      response=await context.request.get(base+path);
    }
    for(let attempt=0;response.status()===429&&attempt<3;attempt++){await delay(30000);response=await context.request.get(base+path);}
    assert.equal(response.status(),200,path);return response.json();
  };
  const native=()=>read('/business/api/v1/business-engagements/'+source,'business-engagement');
  const before=await native();assert.equal(before.engagement.revision,14);assert.equal(before.materials.length,6);
  result.stages.push('Retained public-g native revision 14 and six file references; no failed-run reset');
  page=await context.newPage();page.on('pageerror',error=>result.browser_errors.push(error.message));
  page.on('request',request=>{const path=new URL(request.url()).pathname;if(!['GET','HEAD'].includes(request.method())&&path.startsWith('/project/api/'))result.project_writes++;if(request.method()==='DELETE'&&path.startsWith('/evidence/'))result.physical_file_deletes++;});
  const url=base+'/workbench/?project_id='+project+'&tab=business&business_tab=bidding';
  await page.goto(url);const mounted=page.locator('[data-real-bidding-react]');await mounted.locator('.bidding-file-slot').nth(2).waitFor({timeout:60000});
  await mounted.locator('.bidding-file-slot').nth(1).getByRole('button',{name:'OA-VERIFY-20261003-A-原命令继续二.png',exact:true}).waitFor();
  assert.equal((await native()).engagement.revision,14,'Opening current facts must be zero-write');
  // Avoid amplifying the Gateway quota with verification reads. Its configured
  // rate/credential lifetimes are not changed for the acceptance runner.
  await delay(55000);
  const pattern='**/business/api/v1/business-engagements/'+source+'/materials';let lost=false;const payloads=[];
  await page.route(pattern,async route=>{
    if(route.request().method()!=='POST'){await route.continue();return;}
    payloads.push(route.request().postDataJSON());
    if(!lost){const response=await route.fetch();assert.equal(response.status(),200);lost=true;await route.abort('failed');}
    else await route.continue();
  });
  const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aX1kAAAAASUVORK5CYII=','base64');
  const name='OA-VERIFY-20261003-A-原命令继续三.png';
  await mounted.locator('.bidding-file-slot').nth(1).locator('input[type=file]').setInputFiles({name,mimeType:'image/png',buffer:png});
  await page.getByRole('button',{name:'继续原操作',exact:true}).waitFor({timeout:60000});
  // Visibility is not completion: the panel can render intent while working.
  // Wait for the product's finally/persist, rather than closing an in-flight tab.
  await page.waitForFunction(()=>{
    const button=[...document.querySelectorAll('button')].find(x=>x.textContent==='继续原操作');
    const entry=Object.entries(sessionStorage).find(([key])=>key.startsWith('oms.oa.b03.pending.v1:'));
    return button&&!button.disabled&&entry&&JSON.parse(entry[1]).command;
  },null,{timeout:60000});
  const stored=await page.evaluate(()=>Object.fromEntries(Object.entries(sessionStorage).filter(([key])=>key.startsWith('oms.oa.b03.pending.v1:'))));
  assert.equal(Object.keys(stored).length,1);const pending=JSON.parse(Object.values(stored)[0]);assert.deepEqual(pending.command,payloads[0]);
  assert.equal((await native()).engagement.revision,15);result.stages.push('Native append committed to revision 15; settled failed response leaves the exact bounded pending command');
  result.original_command_sha256=hash(JSON.stringify(payloads[0]));
  await page.screenshot({path:out+'/lost-reply.png',fullPage:true});
  // Preserve the product's own non-secret pending command as failure evidence;
  // the verifier never injects it back or manufactures session/Actor headers.
  await writeFile(out+'/pending-observation.json',JSON.stringify({scope:'Product-generated bounded temporary browser intent; not business fact or credentials',pending},null,2));
  await delay(55000);
  await page.reload();await page.getByRole('button',{name:'继续原操作',exact:true}).waitFor({timeout:60000});
  await page.getByRole('button',{name:'继续原操作',exact:true}).click();
  await page.getByRole('button',{name:'继续原操作',exact:true}).waitFor({state:'detached',timeout:60000});
  await mounted.locator('.bidding-file-slot').nth(1).getByRole('button',{name,exact:true}).waitFor({timeout:60000});
  assert.equal(payloads.length,2);assert.deepEqual(payloads[0],payloads[1]);
  const after=await native();assert.equal(after.engagement.revision,15);assert.equal(after.materials.length,7);assert.equal(after.materials.filter(x=>!x.superseded_at).length,6);
  assert.equal(after.engagement.status,'ready');
  const ref=after.materials.find(x=>x.created_engagement_revision===15);assert(ref);
  const metadata=await read('/evidence/v1/references/files/'+ref.evidence_file_id+'/versions/'+ref.evidence_version,'evidence-service');
  assert.equal(metadata.sha256,hash(png));assert.equal(metadata.status,'available');assert.equal(metadata.file_name,name);
  const materialCollection=await read('/project/api/v1/projects/'+project+'/materials','project-workspace');
  const segments=materialCollection.segments.filter(x=>x.source_application==='business-engagement'&&x.evidence_file_id);
  assert.equal(segments.length,6);assert.equal(materialCollection.project_revision,4);
  assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith('oms.oa.b03.pending.v1:')).length),0);
  result.stages.push('Actual tab reload restores original intent; identical command replay leaves revision 15, seven available files and six active references');
  result.after={project_revision:4,engagement_revision:15,active_materials:6,preserved_files:7,project_segments:segments};
  assert.equal(result.project_writes,0);assert.equal(result.physical_file_deletes,0);assert.deepEqual(result.browser_errors,[]);
  await page.screenshot({path:out+'/continued.png',fullPage:true});result.passed=true;
  console.log(JSON.stringify({passed:true,stages:result.stages,after:result.after}));
}catch(error){result.error=error.message;if(page)await page.screenshot({path:out+'/failed.png',fullPage:true}).catch(()=>{});throw error;}
finally{await writeFile(out+'/public-retry.json',JSON.stringify(result,null,2));await browser?.close();}
