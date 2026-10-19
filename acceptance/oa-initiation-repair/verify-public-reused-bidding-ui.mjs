// Actual HTTPS / native IAM / original OA React components. No mocked routes,
// injected Actor headers, IAM grants, Project writes or physical file deletes.
import { readFile, mkdir, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import assert from 'node:assert/strict';
import { setTimeout as delay } from 'node:timers/promises';
import { resolve, dirname, join } from 'node:path';

const [sessionPath, evidenceDir, manifestPath, mode = 'inspect'] = process.argv.slice(2);
assert(sessionPath && evidenceDir && manifestPath && ['inspect','exercise','continue'].includes(mode));
assert(process.env.OA_PLAYWRIGHT_MODULE, 'Select the installed Playwright module');
const { chromium } = await import(process.env.OA_PLAYWRIGHT_MODULE);
const privateState = JSON.parse(await readFile(sessionPath, 'utf8'));
const manifest = JSON.parse(await readFile(manifestPath, 'utf8'));
const base = 'https://oatest.antuinfo.cn';
const project = '3b4c44d3-7b33-4a29-add3-3f6211ca893e';
const engagement = '231f2977-79d4-4b36-a178-f19a0d8d3119';
const humanProject = 'f75d4570-5d6a-42ba-ade6-d1cda913ddda';
const tenant = '6539cd13-7190-40f1-8bd5-57f9e7c9b968';
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const stableProject = detail => ({project:detail.project, resource_links:detail.resource_links, assignments:detail.assignments});
await mkdir(evidenceDir, {recursive:true});
const result = {passed:false, mode, public_origin:base, fixture_id:project, engagement_id:engagement,
  native_IAM:true, grant_writes:0, project_writes:0, physical_file_deletes:0, full_oa_accepted:false,
  evidence_provider:'local-http', browser_errors:[], network_failures:[], assets:{}, phases:[]};
let browser, page;
try {
  browser = await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
  const context = await browser.newContext({viewport:{width:1600,height:1100},
    storageState:{cookies:privateState.cookies,origins:privateState.origins},acceptDownloads:true});
  const authorize = async application => {
    const csrfResponse=await context.request.get(base+'/auth/api/v1/csrf');assert.equal(csrfResponse.status(),200);
    const csrf=await csrfResponse.json();
    const entry=await context.request.post(base+'/gateway/authorize/'+application,{form:{csrf_token:csrf.csrf_token},headers:{Origin:base}});
    assert.equal(entry.status(),200,'Native entry exchange failed');
    result.entry_exchanges=(result.entry_exchanges||0)+1;
  };
  const read = async path => {
    let response = await context.request.get(base+path);
    for(let attempt=0;response.status()===429&&attempt<3;attempt++) {
      result.readonly_rate_limit_waits=(result.readonly_rate_limit_waits||0)+1;
      await delay(Math.min(55000,Math.max(1000,Number(response.headers()['retry-after']||30)*1000)));
      response=await context.request.get(base+path);
    }
    if(!response.headers()['content-type']?.includes('application/json')||[401,403].includes(response.status())) {
      await authorize(path.startsWith('/project/')?'project-workspace':path.startsWith('/business/')?'business-engagement':'evidence-service');
      response=await context.request.get(base+path);
    }
    assert.equal(response.status(),200,'Native GET failed: '+path+' status '+response.status());
    assert.equal(new URL(response.url()).origin,base);
    return response.json();
  };
  const native = () => read('/business/api/v1/business-engagements/'+engagement);
  const projectDetails = () => read('/project/api/v1/projects/'+project);
  const beforeProject = await projectDetails();
  const beforeHuman = stableProject(await read('/project/api/v1/projects/'+humanProject));
  assert.equal(beforeProject.project.id,project);
  assert.equal(beforeProject.project.tenant_id,tenant);
  assert.equal(beforeProject.project.revision,4);
  assert.equal(beforeProject.project.status,'active');
  assert.equal(beforeProject.resource_links.filter(link=>link.target_service==='business-engagement'&&!link.unlinked_at).length,1);
  const beforeSource = await native();
  assert.equal(beforeSource.engagement.project_id,project);
  assert.equal(beforeSource.engagement.tenant_id,tenant);
  result.before = {project_revision:4,engagement_revision:beforeSource.engagement.revision,
    materials:beforeSource.materials,requirements:beforeSource.requirements};
  if(mode==='inspect') {
    assert.equal(beforeSource.engagement.revision,7);
    assert.deepEqual(beforeSource.materials,[]);
    result.passed=true;
  } else {
    for(const [asset,expected] of Object.entries(manifest.assets)) {
      const response = await context.request.get(base+'/workbench/assets/'+asset.replaceAll('_','-'));
      assert.equal(response.status(),200,'Public asset '+asset);
      const digest = sha(await response.body()); assert.equal(digest,expected,asset);
      result.assets[asset]=digest;
    }
    page = await context.newPage();
    page.on('pageerror',error=>result.browser_errors.push(error.message));
    page.on('requestfailed',request=>result.network_failures.push({path:new URL(request.url()).pathname,method:request.method(),error:request.failure()?.errorText}));
    page.on('request',request=>{
      const path = new URL(request.url()).pathname;
      if(request.method()==='DELETE'&&path.startsWith('/evidence/'))result.physical_file_deletes++;
      if(!['GET','HEAD'].includes(request.method())&&path.startsWith('/project/api/'))result.project_writes++;
    });
    await page.goto(base+'/workbench/?project_id='+project+'&tab=business&business_tab=bidding');
    const mounted=page.locator('[data-real-bidding-react]');
    await mounted.locator('.bidding-file-slot').nth(2).waitFor({timeout:60000});
    assert.equal(await mounted.locator('.bidding-file-slot').count(),3);
    await page.getByRole('button',{name:'刷新当前资料',exact:true}).waitFor();
    await page.screenshot({path:evidenceDir+'/empty-public.png',fullPage:true});
    assert.equal((await native()).engagement.revision,beforeSource.engagement.revision,'Opening panel must not write');
    const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aX1kAAAAASUVORK5CYII=','base64');
    const names=['OA-VERIFY-20261003-A-招标一.png','OA-VERIFY-20261003-A-招标二.png','OA-VERIFY-20261003-A-投标.png','OA-VERIFY-20261003-A-中标材料.png'];
    const upload=async(slot,names)=>{
      await mounted.locator('.bidding-file-slot').nth(slot).locator('input[type=file]').setInputFiles(names.map(name=>({name,mimeType:'image/png',buffer:png})));
      for(const name of names)await mounted.locator('.bidding-file-slot').nth(slot).getByRole('button',{name,exact:true}).waitFor({timeout:60000});
    };
    if(mode==='continue') {
      assert.equal(beforeSource.engagement.revision,11);
      assert.equal(beforeSource.materials.filter(x=>!x.superseded_at).length,4);
      const failed=JSON.parse(await readFile(join(dirname(resolve(evidenceDir)),'public-d','public-react-exercise.json'),'utf8'));
      assert.equal(failed.passed,false);assert.equal(failed.fixture_id,project);assert.equal(failed.before.engagement_revision,7);
      assert(failed.error.includes('429'));
      result.continued_from='public-d/public-react-exercise.json';
      result.phases.push('Continued the same real four-upload fixture after read-only verification hit the Gateway rate limit; no replacement or repeated upload');
    } else {
      assert.equal(beforeSource.engagement.revision,7);
      assert.deepEqual(beforeSource.materials,[],'Use only the known empty, owned acceptance fixture');
      await upload(0,names.slice(0,2));
      const two=await native();assert.equal(two.materials.filter(x=>!x.superseded_at).length,2);
      assert.equal(two.engagement.revision,9);assert.notEqual(two.engagement.status,'ready');
      result.phases.push('Two Evidence uploads + two native BE append commands');
      await page.screenshot({path:evidenceDir+'/two-tender-public.png',fullPage:true});
      await upload(1,[names[2]]);await delay(3000);await upload(2,[names[3]]);
    }
    const ready=await native(); const active=ready.materials.filter(x=>!x.superseded_at);
    assert.equal(ready.engagement.revision,11);assert.equal(ready.engagement.status,'ready');
    assert.equal(active.length,4);assert(ready.requirements.every(item=>!item.required||item.satisfied));
    for(const material of active) {
      const meta=await read('/evidence/v1/references/files/'+material.evidence_file_id+'/versions/'+material.evidence_version);
      assert.equal(meta.tenant_id,tenant);assert.equal(meta.status,'available');assert.equal(meta.sha256,sha(png));
      assert.equal(meta.size_bytes,png.length);assert(names.includes(meta.file_name));
    }
    result.phases.push('Four native available versions; size + SHA-256 verified; BE ready is material readiness only');
    await page.screenshot({path:evidenceDir+'/ready-public.png',fullPage:true});
    const first=mounted.locator('.bidding-file-slot').first();
    await first.getByRole('button',{name:names[0],exact:true}).click();
    await page.locator('.file-preview-drawer__shell img').waitFor({timeout:60000});
    await page.screenshot({path:evidenceDir+'/preview-public.png',fullPage:true});
    await page.locator('.ant-drawer-close').click();
    const download=page.waitForEvent('download');
    await first.locator('.bidding-file-slot__file-entry').filter({has:page.getByRole('button',{name:names[1],exact:true})}).getByRole('button',{name:/下\s*载/}).click();
    const downloaded=await download;
    assert.equal(await downloaded.failure(),null);
    const stream=await downloaded.createReadStream(); const chunks=[];for await(const chunk of stream)chunks.push(chunk);
    assert.equal(sha(Buffer.concat(chunks)),sha(png));result.phases.push('Actual preview + signed download bytes verified');
    await first.locator('.bidding-file-slot__file-entry').filter({has:page.getByRole('button',{name:names[0],exact:true})}).getByRole('button',{name:/移\s*除/}).click();
    await page.getByRole('dialog').getByRole('button',{name:'确认移除',exact:true}).click();
    await first.getByRole('button',{name:names[0],exact:true}).waitFor({state:'detached',timeout:60000});
    const removed=await native();const remaining=removed.materials.filter(x=>!x.superseded_at);
    assert.equal(removed.engagement.revision,12);assert.equal(removed.engagement.status,'ready');
    assert.equal(remaining.length,3);assert.equal(removed.materials.filter(x=>x.superseded_at).length,1);
    for(const material of active)assert.equal((await read('/evidence/v1/references/files/'+material.evidence_file_id+'/versions/'+material.evidence_version)).status,'available');
    const collection=await read('/project/api/v1/projects/'+project+'/materials');
    const segments=collection.segments.filter(segment=>segment.source_application==='business-engagement'&&segment.evidence_file_id);
    assert.equal(segments.length,3);
    const link=beforeProject.resource_links.find(x=>x.target_service==='business-engagement'&&!x.unlinked_at);
    const expectedState=link.source_revision&&link.source_revision!==12?'revision_changed':'available';
    assert(segments.every(segment=>segment.state===expectedState&&segment.source_revision===12&&segment.evidence_digest===sha(png)));
    assert.deepEqual(segments.map(x=>x.evidence_file_id+':'+x.evidence_version).sort(),remaining.map(x=>x.evidence_file_id+':'+x.evidence_version).sort());
    assert.deepEqual(stableProject(await projectDetails()),stableProject(beforeProject));
    assert.deepEqual(stableProject(await read('/project/api/v1/projects/'+humanProject)),beforeHuman);
    const historyBefore=await read('/business/api/v1/business-engagements/'+engagement+'/history?limit=100');
    await page.getByRole('button',{name:'刷新当前资料',exact:true}).click();
    await first.getByRole('button',{name:names[1],exact:true}).waitFor({timeout:60000});
    assert.equal((await native()).engagement.revision,12);
    assert.deepEqual(await read('/business/api/v1/business-engagements/'+engagement+'/history?limit=100'),historyBefore);
    await page.reload();await mounted.locator('.bidding-file-slot').nth(2).waitFor({timeout:60000});
    await mounted.locator('.bidding-file-slot').first().getByRole('button',{name:names[1],exact:true}).waitFor({timeout:60000});
    assert.equal((await native()).engagement.revision,12);
    await page.screenshot({path:evidenceDir+'/after-remove-public.png',fullPage:true});
    result.phases.push('Exact reference removal preserves four files/history; Project reads three current references; refresh/reload writes zero facts');
    const retryName='OA-VERIFY-20261003-A-原命令继续.png';
    const materialsPattern='**/business/api/v1/business-engagements/'+engagement+'/materials';
    const retryPayloads=[];let dropped=false;
    await page.route(materialsPattern,async route=>{
      if(route.request().method()!=='POST'){await route.continue();return;}
      const payload=route.request().postDataJSON();retryPayloads.push(payload);
      if(!dropped){const response=await route.fetch();assert.equal(response.status(),200);dropped=true;await route.abort('failed');}
      else await route.continue();
    });
    await mounted.locator('.bidding-file-slot').nth(1).locator('input[type=file]').setInputFiles({name:retryName,mimeType:'image/png',buffer:png});
    await page.getByRole('button',{name:'继续原操作',exact:true}).waitFor({timeout:60000});
    assert.equal((await native()).engagement.revision,13,'Dropped reply must be an already-committed native command');
    await page.screenshot({path:evidenceDir+'/lost-reply-public.png',fullPage:true});
    await page.reload();await page.getByRole('button',{name:'继续原操作',exact:true}).waitFor({timeout:60000});
    await page.getByRole('button',{name:'继续原操作',exact:true}).click();
    await page.getByRole('button',{name:'继续原操作',exact:true}).waitFor({state:'detached',timeout:60000});
    await mounted.locator('.bidding-file-slot').nth(1).getByRole('button',{name:retryName,exact:true}).waitFor({timeout:60000});
    assert.equal(retryPayloads.length,2);assert.deepEqual(retryPayloads[0],retryPayloads[1],'Retry must keep command ID, expected revision and full payload');
    const continued=await native();assert.equal(continued.engagement.revision,13);assert.equal(continued.materials.filter(x=>!x.superseded_at).length,4);
    const finalCollection=await read('/project/api/v1/projects/'+project+'/materials');
    assert.equal(finalCollection.segments.filter(x=>x.source_application==='business-engagement'&&x.evidence_file_id).length,4);
    assert.deepEqual(stableProject(await projectDetails()),stableProject(beforeProject));
    await page.unroute(materialsPattern);await page.screenshot({path:evidenceDir+'/continued-public.png',fullPage:true});
    result.phases.push('Deliberately lost one committed native HTTP reply; browser reload + original command replay adds no duplicate revision/upload');
    result.fault_injection={scope:'Browser response loss only, real owner commit preserved',same_full_command_replayed:true,persisted_across_reload:true,final_revision:13};
    result.after={project_revision:4,engagement_revision:13,status:'ready',active_materials:4,preserved_files:5,project_segments:finalCollection.segments};
    assert.equal(result.project_writes,0);assert.equal(result.physical_file_deletes,0);assert.deepEqual(result.browser_errors,[]);
    assert.equal(new URL(page.url()).origin,base);result.passed=true;
  }
  console.log(JSON.stringify({passed:result.passed,mode,phases:result.phases,after:result.after,scope:'Only B03 original component + real native owner chain, not full OA or OSS acceptance'}));
} catch(error) {
  result.error=error.message;
  if(page)await page.screenshot({path:evidenceDir+'/failed-public.png',fullPage:true}).catch(()=>{});
  throw error;
} finally {
  await writeFile(evidenceDir+'/public-react-'+mode+'.json',JSON.stringify(result,null,2));
  await browser?.close();
}
