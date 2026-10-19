// Visible original form checks only: no business mutation is permitted.
import assert from 'node:assert/strict';
import {mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {setTimeout as delay} from 'node:timers/promises';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const output=process.argv[2];assert(/^\/tmp\/oa-cash-cancel\.[A-Za-z0-9]+$/.test(output));mkdirSync(output,{recursive:true});
const origin='https://oatest.antuinfo.cn',project='70edead7-9dc9-44a3-9b35-b9d41b83264e';
const recovery=JSON.parse(readFileSync('/tmp/oa-cost-category.HvqnNw/recovery-private.json'));
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
const context=await browser.newContext({viewport:{width:1600,height:1200},storageState:recovery.storage});
const page=await context.newPage(),result={passed:false,project_id:project,read_only_business:true,blocked_business_mutations:[],page_errors:[],scenarios:[],full_oa_accepted:false};
let next=0,stage='open';
page.on('pageerror',e=>result.page_errors.push(e.message));
await page.route('**/*',async route=>{
 const request=route.request(),path=new URL(request.url()).pathname;
 if(['POST','PUT','PATCH','DELETE'].includes(request.method())&&!path.startsWith('/gateway/')){
  result.blocked_business_mutations.push({method:request.method(),path});await route.abort('blockedbyclient');return;
 }
 const at=Math.max(Date.now(),next);next=at+800;await delay(Math.max(0,at-Date.now()));await route.continue();
});
const readProject=async()=>{const r=await context.request.get(origin+'/project/api/v1/projects/'+project);assert.equal(r.status(),200);return r.json();};
try{
 await page.goto(origin+'/workbench/?project_id='+project+'&tab=business&business_tab=payments',{waitUntil:'domcontentloaded'});
 const pane=page.locator('[data-real-cash-react]');await pane.locator('.transaction-panel__group--client .create-action-button').waitFor({timeout:30000});
 await pane.locator('.ant-spin-spinning').waitFor({state:'hidden'});
 const before=await readProject();assert.equal(before.project.revision,18);
 assert.equal(await pane.locator('.transaction-panel__item').count(),2);
 for(const chain of ['client','income'])for(const fee of ['设计费','勘察费','咨询费']){
  stage=chain+'-'+fee;await pane.locator('.transaction-panel__group--'+chain+' .create-action-button').click();const drawer=page.locator('.ant-drawer:visible');
  for(const label of ['收款账簿主体','付款主体','费用类型','支付金额','实际收款账户','原始资金依据','备注'])await drawer.getByLabel(label,{exact:true}).waitFor();
  await drawer.getByLabel('费用类型',{exact:true}).locator('xpath=ancestor::div[contains(concat(" ",normalize-space(@class)," ")," ant-select ")][1]').locator('.ant-select-selector').click();
  const popup=page.locator('.ant-select-dropdown:visible');await popup.locator('.ant-select-item-option').last().waitFor();await popup.innerText();
  assert.deepEqual(new Set(await popup.locator('.ant-select-item-option-content').allTextContents()),new Set(['设计费','勘察费','咨询费']));
  await popup.locator('.ant-select-item-option').filter({hasText:new RegExp('^'+fee+'$')}).click();
  await drawer.getByLabel('支付金额',{exact:true}).fill('0.01');await drawer.getByLabel('备注',{exact:true}).fill('只读表单取消验收，不提交、不登记资金');
  await page.screenshot({path:output+'/'+chain+'-'+['设计费','勘察费','咨询费'].indexOf(fee)+'-form.png',animations:'disabled'});
  await drawer.getByRole('button',{name:/^取\s*消$/}).click();await drawer.waitFor({state:'hidden'});
  assert.equal(await pane.locator('.transaction-panel__item').count(),2);assert.equal(result.blocked_business_mutations.length,0);
  result.scenarios.push({chain,fee,original_form_visible:true,cancel_zero_write:true});console.log(JSON.stringify({stage,passed:true}));
 }
 const after=await readProject();assert.deepEqual(after,before);result.project_revision=18;
 assert.equal(result.blocked_business_mutations.length,0);assert.equal(result.page_errors.length,0);result.passed=true;
}catch(e){result.failure={stage,type:e.name,message:e.message.slice(0,600)};process.exitCode=1;await page.screenshot({path:output+'/failure.png'}).catch(()=>{});}
finally{await browser.close();writeFileSync(output+'/browser.json',JSON.stringify(result,null,2)+'\n',{mode:0o600});console.log(JSON.stringify(result));}
