// Actual copied React/AntD UI, isolated in-memory versioned port fixture.
// This proves component interactions, never production IAM or PostgreSQL.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const output=process.argv[2];assert(output&&output.startsWith('/tmp/oa-complete-operations.'));
mkdirSync(output,{recursive:true});
const asset=new URL('../../frontend/oms-workbench/internal/server/',import.meta.url);
const server=createServer((r,s)=>{
 const files={'/ui.js':'reference_oa_react_bidding.js','/ui.css':'reference_oa_react_bidding.css'};
 if(files[r.url]){s.setHeader('Content-Type',r.url.endsWith('.js')?'text/javascript':'text/css');return s.end(readFileSync(new URL(files[r.url],asset)))}
 s.setHeader('Content-Type','text/html');s.end('<!doctype html><html lang="zh"><head><meta charset="utf-8"><link rel="stylesheet" href="/ui.css"></head><body><section id="root"><div data-real-personnel-react></div></section><script src="/ui.js"></script></body></html>');
});await new Promise(r=>server.listen(0,'127.0.0.1',r));
const base='http://127.0.0.1:'+server.address().port;
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
const result={passed:false,fixture_ports:true,production_browser:false,original_components:true,scenarios:[],page_errors:[]};
async function mount(page,readonly=false){
 await page.goto(base,{waitUntil:'load'});
 await page.evaluate(async readonly=>{
  const tenant='11111111-1111-4111-8111-111111111111',project='22222222-2222-4222-8222-222222222222',subject='33333333-3333-4333-8333-333333333333',person='44444444-4444-4444-8444-444444444444';
  const details={project:{id:project,tenant_id:tenant,revision:4,status:'active'},allowed_actions:readonly?[]:['project-workspace.assignment.commercial-contributor.manage'],assignments:[]};
  window.fixture={details,writes:[]};
  const request=async(path,options)=>{
   if(!options)return structuredClone(details);
   const c=JSON.parse(options.body);window.fixture.writes.push(c);details.project.revision++;
   let row;
   if(c.operation==='assign'){row={id:'55555555-5555-4555-8555-555555555555',project_id:project,tenant_id:tenant,role_key:'commercial_contributor',subject_kind:'person',subject_id:person,source_revision:2,display_snapshot:'张工'};details.assignments.push(row)}
   else {row=details.assignments.find(a=>a.id===c.assignment_id);row.ended_at='2026-10-03T12:00:00Z'}
   return structuredClone({project:details.project,assignment:row});
  };
  window.workspace=ReferenceOAReactBidding.createPersonnelWorkspace({root:document.querySelector('#root'),details:()=>details,request,authorize:async()=>{},applicationRequest:async()=>[{id:person,tenant_id:tenant,revision:2,display_name:'张工',status:'active'}],tenantID:tenant,subjectID:subject});
  await window.workspace.load();
 },readonly);
 await page.getByText('当前还没有经营参与人',{exact:true}).waitFor();
}
try{
 const page=await browser.newPage({viewport:{width:1280,height:850}});globalThis.testPage=page;page.on('pageerror',e=>result.page_errors.push(e.message));
 await mount(page);
 await page.locator('button').filter({hasText:'添加参与人'}).click();
 const drawer=page.locator('.ant-drawer-content');await drawer.waitFor();
 assert.equal(Math.round((await page.locator('.ant-drawer-content-wrapper').boundingBox()).width),520);
 await drawer.getByRole('button',{name:/取\s*消/}).click();
 await drawer.waitFor({state:'hidden'});assert.equal(await page.evaluate(()=>fixture.writes.length),0);
 result.scenarios.push({name:'source-drawer-520-cancel-zero-write',passed:true});
 await page.locator('button').filter({hasText:'添加参与人'}).click();
 await drawer.getByRole('combobox').click();await page.locator('.ant-select-item-option').filter({hasText:'张工'}).click();
 await drawer.getByRole('button',{name:/添\s*加/}).click();
 await drawer.waitFor({state:'hidden'});await page.locator('.business-personnel-list__item').filter({hasText:'张工'}).waitFor();
 assert.equal(await page.evaluate(()=>fixture.writes.length),1);
 await page.getByRole('button',{name:'刷新当前人员',exact:true}).click();assert.equal(await page.evaluate(()=>fixture.writes.length),1);
 await page.screenshot({path:output+'/added.png'});
 result.scenarios.push({name:'source-select-save-close-reread-refresh-zero-write',passed:true});
 await page.locator('.business-personnel-list__actions button').click();
 await page.locator('.confirm-delete__dialog').getByRole('button',{name:/取\s*消/}).click();assert.equal(await page.evaluate(()=>fixture.writes.length),1);
 await page.locator('.business-personnel-list__actions button').click();
 await page.locator('.confirm-delete__dialog').getByRole('button',{name:'确认移出',exact:true}).click();
 await page.getByText('当前还没有经营参与人',{exact:true}).waitFor();
 assert.equal(await page.evaluate(()=>fixture.writes.length),2);assert.equal(await page.evaluate(()=>fixture.details.assignments.length),1);assert.ok(await page.evaluate(()=>fixture.details.assignments[0].ended_at));
 result.scenarios.push({name:'source-confirmation-cancel-and-historical-end',passed:true});
 await page.reload();await mount(page,true);assert.equal(await page.locator('button').filter({hasText:'添加参与人'}).count(),0);assert.equal(await page.evaluate(()=>fixture.writes.length),0);
 result.scenarios.push({name:'readonly-no-management-button-zero-write',passed:true});
 await page.setViewportSize({width:390,height:844});await mount(page);await page.locator('button').filter({hasText:'添加参与人'}).click();
 await drawer.getByRole('combobox').click();await page.locator('.ant-select-item-option').filter({hasText:'张工'}).click();
 const button=drawer.getByRole('button',{name:/添\s*加/});await button.scrollIntoViewIfNeeded();const box=await button.boundingBox();assert(box.x>=0&&box.x+box.width<=390&&box.y+box.height<=844);
 await page.screenshot({path:output+'/mobile-drawer.png'});result.scenarios.push({name:'mobile-footer-and-selector-reachable',passed:true});
 assert.equal(result.page_errors.length,0);result.passed=true;
}catch(error){result.failure={type:error.name,message:error.message};if(globalThis.testPage){await testPage.screenshot({path:output+'/failure.png'});result.dom=await testPage.locator('body').innerText();result.buttons=await testPage.locator('button').allTextContents();result.adapter=await testPage.evaluate(()=>window.workspace?{can:workspace.current?.canManage(),pending:!!workspace.current?.pending,working:workspace.current?.working,actions:workspace.current?.value?.allowed_actions}:null)}process.exitCode=1}
finally{await browser.close();await new Promise(r=>server.close(r));writeFileSync(output+'/results.json',JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result));}
