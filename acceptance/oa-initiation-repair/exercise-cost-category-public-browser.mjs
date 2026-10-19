// Fill the remaining eight side/category pairs through the original public UI.
// Exact C scope only; no IAM grants, mocks, SQL writes or Evidence uploads.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {setTimeout as delay} from 'node:timers/promises';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const output=process.argv[2];assert(/^\/tmp\/oa-cost-category\.[A-Za-z0-9]+$/.test(output));mkdirSync(output,{recursive:true});
const resume=process.env.OA_CONTINUE_EXISTING==='1';
const recovery=JSON.parse(readFileSync(resume?output+'/recovery-private.json':'/tmp/oa-commercial-journey.35jPS3/recovery-private.json'));
const previous=resume?JSON.parse(readFileSync(output+'/browser.json')):null;
const project='70edead7-9dc9-44a3-9b35-b9d41b83264e',origin='https://oatest.antuinfo.cn',prefix='公网成本分类-20261004-';
const result={passed:false,project_id:project,administrator_only:true,grant_writes:0,commands:[],page_errors:[],scenarios:[],native_records:[],...previous,passed:false};delete result.failure;
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
const context=await browser.newContext({viewport:{width:1600,height:1200},storageState:recovery.storage});
if(resume)await context.addInitScript(session=>{if(location.origin==='https://oatest.antuinfo.cn'&&!sessionStorage.getItem('__oa_cost_category_restore_once')){for(const[k,v]of Object.entries(session))sessionStorage.setItem(k,v);sessionStorage.setItem('__oa_cost_category_restore_once','1')}},recovery.session);
const page=await context.newPage();let next=0,side='business',stage='open';const responses=new Map();
page.on('pageerror',e=>result.page_errors.push(e.message));
page.on('request',r=>{const path=new URL(r.url()).pathname;if(!['POST','PUT','PATCH','DELETE'].includes(r.method())||path.startsWith('/gateway/'))return;let v;try{v=r.postDataJSON()}catch{};result.commands.push({method:r.method(),path,command_id:v?.command_id});});
page.on('response',async r=>{try{if(r.ok()&&r.request().method()==='GET'&&r.url().includes('/api/v1/reference-oa/cost-entries?'))responses.set(side,await r.json());}catch{}});
await page.route('**/*',async route=>{const at=Math.max(Date.now(),next);next=at+800;await delay(Math.max(0,at-Date.now()));await route.continue();});
const pane=()=>page.locator(side==='business'?'[data-real-business-panel="costs"]':'[data-real-production-cost]');
const card=note=>pane().locator('.cost-panel__item').filter({hasText:note});
const pairs={business:[['accommodation','住宿'],['expert_fee','评标专家费'],['bid_signup_fee','投标报名费'],['bid_service_fee','中标服务费']],production:[['vehicle','用车'],['accommodation','住宿'],['transport','交通'],['other','其他']]};
const options={business:['用车','住宿','交通','评标专家费','投标报名费','中标服务费','其他'],production:['用车','住宿','交通','方案评审专家费','其他']};
const projectRead=async()=>{const r=await context.request.get(origin+'/project/api/v1/projects/'+project);assert.equal(r.status(),200);return r.json();};
const close=async d=>{await d.locator('.ant-drawer-close').click();await d.waitFor({state:'hidden'});};
try{
 let before;
 for(const scope of ['business','production']){
  side=scope;stage='open-'+side;const query=side==='business'?'tab=business&business_tab=costs':'tab=production&production_tab=cost';
  await page.goto(origin+'/workbench/?project_id='+project+'&'+query,{waitUntil:'domcontentloaded'});await pane().locator('.cost-panel').waitFor({timeout:30000});
  await pane().locator('.ant-spin-spinning').waitFor({state:'hidden'});
  if(!before){
   const source=await page.locator('script[src$="reference-oa-react-bidding.js"]').getAttribute('src');
   assert(source);const url=new URL(source,page.url());assert.equal(url.origin,origin);
   const asset=await context.request.get(url.href);assert.equal(asset.status(),200);
   result.runtime_asset_path=url.pathname;
   result.runtime_asset_sha256=createHash('sha256').update(await asset.body()).digest('hex');
   const manifest=JSON.parse(readFileSync('/tmp/oa-cost-modal.xxGT4L/artifacts/manifest.json'));
   assert.equal(result.runtime_asset_sha256,manifest.assets['reference_oa_react_bidding.js']);
   before=await projectRead();assert.equal(before.project.project_number,'OA-VERIFY-20261002-C');assert.equal(before.project.revision,18);
  }
  const pending=pane().getByRole('button',{name:'继续原成本操作'});if(await pending.count()){await pending.click();await pending.waitFor({state:'hidden',timeout:30000});}
  // A cancelled original drawer must write nothing; options must exactly
  // match this side, not show the two business-only fees under production.
  const count=result.commands.length;await pane().getByRole('button',{name:'记录成本'}).first().click();let d=page.locator('.ant-drawer:visible');
  await d.getByLabel('成本类型',{exact:true}).click();let popup=page.locator('.ant-select-dropdown:visible');
  await popup.locator('.ant-select-item-option-content').last().waitFor();
  result.dropdown_diagnostic={side,popups:await popup.count(),contents:await popup.locator('.ant-select-item-option-content').count(),options:await popup.locator('.ant-select-item-option').count(),text:await popup.innerText()};
  await page.screenshot({path:output+'/'+side+'-dropdown.png',animations:'disabled'});
  assert.deepEqual(await popup.locator('.ant-select-item-option-content').allTextContents(),options[side]);
  await popup.locator('.ant-select-item-option').first().click();await d.getByRole('button',{name:/^取\s*消$/}).click();await d.waitFor({state:'hidden'});assert.equal(result.commands.length,count);
  for(const[category,label]of pairs[side]){
   stage=side+'-'+category;const note=prefix+side+'-'+category;
   if(!await card(note).count()){
    await pane().getByRole('button',{name:'记录成本'}).first().click();d=page.locator('.ant-drawer:visible');
    await d.getByLabel('成本类型',{exact:true}).click();
    popup=page.locator('.ant-select-dropdown:visible');await popup.locator('.ant-select-item-option').filter({hasText:new RegExp('^'+label+'$')}).click();
    await d.getByLabel('金额',{exact:true}).fill('0.01');await d.getByLabel('描述',{exact:true}).fill(note);
    await d.getByRole('button',{name:/^提\s*交$/}).click();await d.waitFor({state:'hidden',timeout:30000});await card(note).waitFor();
   }
   await card(note).click();d=page.locator('.ant-drawer:visible');assert.equal(await d.getByLabel('金额',{exact:true}).inputValue(),'0.01');
   if(!await d.getByText(/已撤回，只读/).count()){
    await d.getByRole('button',{name:/撤\s*回/}).click();const modal=page.locator('.ant-modal:visible');
    await modal.getByLabel('撤回原因',{exact:true}).fill('分类公网验收结束：非真实费用，保留原金额和历史');
    const confirm=modal.getByRole('button',{name:'确认撤回'});
    assert(await confirm.evaluate(button=>{const r=button.getBoundingClientRect();return document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)?.closest('button')===button}),'normal confirmation is covered by another surface');
    await confirm.click();await modal.waitFor({state:'hidden',timeout:30000});
    // Normal success closes the original edit drawer. Reopen the persisted
    // list item to verify withdrawn read-only state, not the closed form.
    await card(note).getByText('已撤回（原金额保留）',{exact:true}).waitFor();
    await d.waitFor({state:'hidden'});await card(note).click();d=page.locator('.ant-drawer:visible');
   }
   await d.getByText(/已撤回，只读/).waitFor();assert.equal(await d.getByLabel('金额',{exact:true}).isDisabled(),true);
   await d.getByRole('button',{name:'查看成本历史'}).click();await d.locator('[data-cost-history] p').nth(1).waitFor();assert.equal(await d.locator('[data-cost-history] p').count(),2);
   await close(d);if(!result.scenarios.some(v=>v.side===side&&v.category===category))result.scenarios.push({side,category,amount_minor:'1',revisions:2,passed:true});
   console.log(JSON.stringify({stage,passed:true}));
  }
  await page.screenshot({path:output+'/'+side+'-all-categories.png',animations:'disabled'});
  const list=responses.get(side);assert(list?.scope===side);const rows=list.items.filter(v=>v.description?.startsWith(prefix));assert.equal(rows.length,4);
  assert(rows.every(v=>v.scope===side&&v.state==='withdrawn'&&v.revision===2&&v.amount_minor==='1'));
  result.native_records=result.native_records.filter(v=>v.scope!==side);
  result.native_records.push(...rows.map(v=>({id:v.id,scope:v.scope,category:v.category,revision:v.revision,state:v.state,amount_minor:v.amount_minor})));
 }
 const after=await projectRead();assert.deepEqual(after,before);result.project_revision=after.project.revision;
 assert.equal(result.page_errors.length,0);assert.equal(result.commands.length,16);assert.equal(new Set(result.commands.map(v=>v.command_id)).size,16);
 const route=await page.locator('[data-real-application="cost-outsourcing"]').first().getAttribute('data-real-route');
 assert(route?.startsWith('/')&&!route.endsWith('/'));result.configured_cost_route=route;
 assert(result.commands.every(v=>v.method==='POST'&&v.path.startsWith(route+'/api/v1/reference-oa/cost-entries')&&/^(?:\/[a-f0-9-]+\/withdrawal)?$/.test(v.path.slice((route+'/api/v1/reference-oa/cost-entries').length))));
 result.passed=true;result.full_role_acceptance=false;result.full_oa_accepted=false;
}catch(e){result.failure={stage,type:e.name,message:e.message.slice(0,500)};process.exitCode=1;await page.screenshot({path:output+'/failure.png'}).catch(()=>{});}
finally{writeFileSync(output+'/recovery-private.json',JSON.stringify({storage:await context.storageState(),session:await page.evaluate(()=>Object.fromEntries(Object.entries(sessionStorage))).catch(()=>({}))}),{mode:0o600});await browser.close();writeFileSync(output+'/browser.json',JSON.stringify(result,null,2)+'\n',{mode:0o600});console.log(JSON.stringify({passed:result.passed,scenarios:result.scenarios.length,commands:result.commands.length,failure:result.failure}));}
