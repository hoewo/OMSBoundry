// Same continued C fixture, real public Nginx/Gateway/IAM/owners. Never user101.
// Lost replies are injected only AFTER route.fetch commits to the real owner.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,writeFileSync,readFileSync} from 'node:fs';
import {setTimeout as delay} from 'node:timers/promises';
import {cents} from '../../frontend/oms-workbench/frontend/src/contract-money.mjs';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const output=process.argv[2];assert(/^\/tmp\/oa-commercial-journey\.[A-Za-z0-9]+$/.test(output));mkdirSync(output,{recursive:true});
const resume=process.env.OA_CONTINUE_EXISTING==='1';
const previous=resume?JSON.parse(readFileSync(output+'/browser.json')):null;
const recovery=resume?JSON.parse(readFileSync(output+'/recovery-private.json')):null;
if(resume)assert(previous?.commands?.length>0,'resume only when facts already exist');
const origin='https://oatest.antuinfo.cn',project='70edead7-9dc9-44a3-9b35-b9d41b83264e',runID=output.split('.').at(-1);
const ssh=code=>{const r=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',code],{encoding:'utf8',timeout:25000});assert.equal(r.status,0,'private inspection unavailable');return r.stdout};
const secretCode='import json,subprocess;v=json.loads(subprocess.check_output(["docker","inspect","omsbizfoundry-identity-access-1"]))[0];e=dict(x.split("=",1) for x in v["Config"]["Env"]);print(json.dumps({k:e[k] for k in ("BOOTSTRAP_ADMIN_USERNAME","BOOTSTRAP_ADMIN_PASSWORD")}))';
const secret=JSON.parse(ssh("sudo python3 -c '"+secretCode+"'"));
const result={passed:false,fixture_id:project,run_id:runID,public_real_owners:true,administrator_only:true,full_role_acceptance:false,full_oa_accepted:false,grant_writes:0,commands:[],scenarios:[],page_errors:[],native_records:[],...previous,passed:false};delete result.failure;
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
const context=await browser.newContext({viewport:{width:1600,height:1100},acceptDownloads:true,...(resume?{storageState:recovery.storage}:{})});
if(resume)await context.addInitScript(session=>{if(location.origin==='https://oatest.antuinfo.cn'&&!sessionStorage.getItem('__oa_commercial_restore_once')){for(const [key,value]of Object.entries(session))sessionStorage.setItem(key,value);sessionStorage.setItem('__oa_commercial_restore_once','1')}},recovery.session);
const page=await context.newPage();
let stage='login',drop=null,nextRequest=0;const responses=new Map();
page.on('pageerror',e=>result.page_errors.push(e.message));
page.on('request',r=>{const path=new URL(r.url()).pathname;if(!['POST','PUT','DELETE','PATCH'].includes(r.method())||path.startsWith('/gateway/'))return;let body;try{body=r.postDataJSON()}catch{};result.commands.push({method:r.method(),path,command_id:body?.command_id,expected_revision:body?.expected_revision});});
page.on('response',async r=>{try{const path=new URL(r.url()).pathname;if(/^\/(contract|money|billing|project)\/api\//.test(path)){if(!r.ok()){const v=await r.json();(result.http_failures||=[]).push({status:r.status(),path,code:v.error?.code,message:v.error?.message||v.detail})}else if(r.request().method()==='GET')responses.set(path,await r.json())}}catch{}});
// Driver pacing, not a server limit/configuration change or mocked success.
await page.route('**/*',async route=>{const at=Math.max(Date.now(),nextRequest);nextRequest=at+800;await delay(Math.max(0,at-Date.now()));const req=route.request(),path=new URL(req.url()).pathname;if(!drop||req.method()!==drop.method||path!==drop.path)return route.continue();const fault=drop;drop=null;const response=await route.fetch();assert(response.ok(),'fault precondition: owner did not commit successfully');fault.committed=true;return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{message:'验收：真实业务已回复，但浏览器未收到回执'}})});});
const done=name=>result.scenarios.some(v=>v.name===name&&v.passed);
const knownInvoiceIDs=result.invoice_ids||[];result.invoice_ids=knownInvoiceIDs;
const mark=name=>{if(!done(name))result.scenarios.push({name,passed:true});console.log(JSON.stringify({phase:'public-browser',stage:name,passed:true}));};
const readProject=async()=>{const r=await context.request.get(origin+'/project/api/v1/projects/'+project);assert.equal(r.status(),200);const v=await r.json();assert.equal(v.project.id,project);assert.match(v.project.project_number,/^OA-VERIFY-20261002-C$/);return v;};
const paneFor={contracts:'contract',payments:'cash',invoices:'invoice'};
const openTab=async name=>{stage=name;await page.goto(origin+`/workbench/?project_id=${project}&tab=business&business_tab=${name}`,{waitUntil:'domcontentloaded'});const p=page.locator(`[data-real-${paneFor[name]}-react]`);await p.locator('button').first().waitFor({timeout:30000});await p.locator('.ant-spin-spinning').waitFor({state:'hidden',timeout:30000});assert.equal(await p.locator('.ant-alert-error').count(),0);return p;};
const openSelect=async(d,label)=>{const input=d.getByLabel(label,{exact:true});await input.locator('xpath=ancestor::div[contains(concat(" ",normalize-space(@class)," ")," ant-select ")][1]').locator('.ant-select-selector').click();};
const selectIndex=async(d,label,index)=>{await openSelect(d,label);const popup=page.locator('.ant-select-dropdown:visible');const options=popup.locator('.ant-select-item-option');await options.nth(index).waitFor({timeout:30000});const text=await options.nth(index).innerText();await options.nth(index).click();await popup.waitFor({state:'hidden'});return text;};
const selectText=async(d,label,text)=>{await openSelect(d,label);const popup=page.locator('.ant-select-dropdown:visible');await popup.locator('.ant-select-item-option').filter({hasText:text}).click();await popup.waitFor({state:'hidden'});};
const date=async(d,label)=>{const input=d.getByLabel(label,{exact:true});await input.fill('2026-10-04');await input.press('Enter');await page.locator('.ant-picker-dropdown:visible').waitFor({state:'hidden'});};
const close=async d=>{
 // Some original forms close automatically after the refreshed list arrives.
 // A visible check can race that transition; accept it only if the real
 // drawer has actually become hidden, never force a click or hide its DOM.
 try{await d.locator('.ant-drawer-close').click({timeout:3000});}
 catch(e){if(await d.isVisible())throw e;}
 await d.waitFor({state:'hidden'});
};
const lost=async fault=>{const deadline=Date.now()+30000;while(!fault.committed&&Date.now()<deadline)await delay(100);assert(fault.committed,'native commit must precede the lost-reply assertion');await page.locator('.ant-alert-error').filter({hasText:'验收：真实业务已回复'}).waitFor({timeout:30000});};
const png=Buffer.from(await page.evaluate(()=>{const c=document.createElement('canvas');c.width=640;c.height=400;const g=c.getContext('2d');g.fillStyle='#f5f8ff';g.fillRect(0,0,640,400);g.fillStyle='#19335d';g.font='bold 32px sans-serif';g.fillText('OA 公网附件验收样张',44,88);g.font='22px sans-serif';g.fillText('非真实合同 / 票据 · 仅验证文件链',44,145);g.fillText('上传、预览、下载与业务引用分别验证',44,232);g.fillText('移出业务引用不删除 Evidence 字节',44,296);return c.toDataURL('image/png').split(',')[1]}),'base64');
try{
 await page.goto(origin+'/workbench/',{waitUntil:'domcontentloaded'});if(!resume){await page.locator('[data-login-mode="iam"]').click();await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME);await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD);await page.locator('form[action="/gateway/login"] button[type="submit"]').click();}await page.locator('[data-real-project-list] [data-real-detail-open]').first().waitFor({timeout:30000});
 const original=await readProject();if(!resume){result.project_revision_before=original.project.revision;result.existing_resource_ids=original.resource_links.filter(v=>!v.unlinked_at).map(v=>v.resource_id);}
 // Create through the reused original form, then reopen the same actual record.
 await openTab('contracts');
 const continuation=page.getByRole('button',{name:/^(继续原合同操作|修复无效关联并续办)$/});
 if(resume&&await continuation.count()){const repair=(await continuation.innerText()).includes('修复');await continuation.click();await continuation.waitFor({state:'hidden',timeout:30000});mark(repair?'same-contract-rejected-link-explicit-repair-no-recreation':'resume-existing-contract-same-link-command');}
 const numbers=[];
 for(const [chain,amount] of [['client','1.01'],['income','2.02']]){
  stage='contract-create-'+chain;const number='OA-PUBLIC-'+runID+'-'+chain;numbers.push(number);if(resume&&await page.locator('.contract-panel__item-number').filter({hasText:number}).count())continue;await page.locator('.contract-panel__group--'+chain+' .create-action-button').click();let d=page.locator('.ant-drawer:visible');await d.getByLabel('合同编号',{exact:true}).fill(number);await date(d,'签订日期');await d.getByLabel('设计费',{exact:true}).fill(amount);await d.getByLabel('勘察费',{exact:true}).fill('0.09');await d.getByLabel('咨询费',{exact:true}).fill('0.90');await d.getByLabel('合同费率(%)',{exact:true}).fill('0');await d.locator('.contract-form__block').filter({hasText:'原始合同区域'}).getByRole('button',{name:/^保\s*存$/}).click();await page.locator('.contract-panel__item-number').filter({hasText:number}).waitFor({timeout:30000});if(await d.isVisible())await close(d);
 }
 const countBefore=result.commands.length;await openTab('contracts');assert.equal(result.commands.length,countBefore);for(const number of numbers)await page.locator('.contract-panel__item-number').filter({hasText:number}).waitFor();mark('real-two-chain-contract-save-reload-zero-write');
 await page.locator('.contract-panel__item').filter({hasText:numbers[0]}).click();let d=page.locator('.ant-drawer:visible');assert.equal(await d.getByLabel('设计费',{exact:true}).inputValue(),'1.01');const prep=d.locator('[data-native-contract-preparation]');
 if(!await prep.getByText(/状态：已生效/).count()){
  await date(prep,'原始合同生效日期');await prep.getByRole('button',{name:'保存生效日期'}).click();await prep.getByRole('button',{name:'读取合同主体目录'}).click();
  // Reading the native directory is asynchronous. Wait for its control to
  // re-enable before opening the Select; its selector div alone is clickable
  // even while the underlying input is disabled and no options are loaded.
  const directory=prep.getByRole('button',{name:'读取合同主体目录'});const readyUntil=Date.now()+30000;while(await directory.isDisabled()&&Date.now()<readyUntil)await delay(100);assert.equal(await directory.isDisabled(),false,'native party directory read must finish');
  for(const [index,role] of [[0,'client'],[1,'provider']]){await prep.locator('.ant-select[aria-label="选择合同签约主体"] .ant-select-selector').click();const pop=page.locator('.ant-select-dropdown:visible');const options=pop.locator('.ant-select-item-option');await options.nth(index).waitFor({timeout:30000});const name=(await options.nth(index).innerText()).trim();await options.nth(index).click();if(await prep.getByText(name+' · '+role,{exact:true}).count())continue;await prep.getByLabel('合同内主体角色').fill(role);await prep.getByRole('button',{name:'添加签约主体'}).click();await prep.getByText(name+' · '+role,{exact:true}).waitFor();}
  await prep.getByRole('button',{name:'确认原始合同生效'}).click();await page.locator('.ant-modal-confirm').getByRole('button',{name:'确认生效'}).click();await prep.getByText(/状态：已生效/).waitFor({timeout:30000});await page.locator('.ant-modal-confirm').waitFor({state:'hidden'});
 }
 assert.equal(await d.getByLabel('设计费',{exact:true}).isDisabled(),true);mark('real-contract-native-activation-freezes-draft');
 if(process.env.OA_PUBLIC_FULL_CONTRACT==='1'){
  for(const [suffix,type,fee]of [['01','增加/扣减合同金额','0.01'],['02','修订为新的合同金额','2.02'],['03','仅修订条款，不改金额',null]]){stage='contract-amendment-'+suffix;const number='OA-PUBLIC-'+runID+'-A'+suffix;const amendment=d.locator('.contract-amendment-list__item').filter({hasText:number});if(!await amendment.count()){await d.getByRole('button',{name:'新建补充协议'}).click();const modal=page.locator('.ant-modal:visible');await modal.getByLabel('补充协议编号',{exact:true}).fill(number);await date(modal,'签订日期');await selectText(modal,'协议影响方式',type);if(fee!==null)await modal.getByLabel(/^设计费/).fill(fee);await modal.getByLabel('备注',{exact:true}).fill('公网验收：'+type+'，仅测试合同，不产生付款');await modal.getByRole('button',{name:/^保\s*存$/}).click();await modal.waitFor({state:'hidden',timeout:30000});await amendment.waitFor();}if(!await amendment.getByText(/^已确认：正式版本/).count()){await date(amendment,'协议生效日期 '+number);await amendment.getByRole('button',{name:'确认协议生效',exact:true}).click();await page.locator('.ant-modal-confirm').getByRole('button',{name:'确认协议生效',exact:true}).click();await amendment.getByText(/^已确认：正式版本/).waitFor({timeout:30000});await page.locator('.ant-modal-confirm').waitFor({state:'hidden'});}}
  await page.screenshot({path:output+'/contract-three-confirmed-amendments.png',animations:'disabled'});mark('real-contract-three-amendment-types-explicit-confirmation');
  const settlement=d.locator('[data-native-contract-settlement]');
  for(const [kind,label,amount]of [['progress','阶段结算','1.01'],['final','最终结算','2.02']]){
   stage='contract-settlement-'+kind;
   const number='OA-PUBLIC-'+runID+'-'+kind;
   const receipt=settlement.locator('[data-settlement-id]').filter({hasText:number});
   if(await receipt.count())continue;
   await selectText(settlement,'正式结算类型',label);
   await settlement.getByLabel('结算文书编号',{exact:true}).fill(number);
   await settlement.getByLabel('正式结算标题',{exact:true}).fill(label+'公网验收');
   await settlement.getByLabel('正式累计结算金额',{exact:true}).fill(amount);
   await date(settlement,'正式结算确认日期');
   await settlement.getByLabel('正式结算依据',{exact:true}).fill('OA 公网验收专用，确认合同累计结算，不代表实际付款');
   await settlement.getByRole('heading',{name:'正式结算记录',exact:true}).click();
   const parties=settlement.locator('.ant-checkbox-input');assert.equal(await parties.count(),2);
   // Ant controlled checkboxes settle after the change callback. Observe the
   // actual checked state after a normal click instead of check()'s immediate
   // post-click assertion; never set DOM state or dispatch a synthetic change.
   for(const checkbox of await parties.all())if(!await checkbox.isChecked()){
    await checkbox.click();
    await page.waitForFunction(el=>el.checked,await checkbox.elementHandle(),{timeout:5000});
    assert.equal(await checkbox.isChecked(),true);
   }
   assert.equal(JSON.parse(await settlement.getAttribute('data-selected-party-positions')).length,2);
   await settlement.getByRole('button',{name:'确认正式结算'}).click();
   if(kind==='progress'){
    await page.locator('.ant-modal-confirm').getByRole('button',{name:/取\s*消/}).click();
    assert.equal(await receipt.count(),0);
    await settlement.getByRole('button',{name:'确认正式结算'}).click();
   }
   await page.locator('.ant-modal-confirm').getByRole('button',{name:'确认并冻结结算'}).click();
   await receipt.waitFor({timeout:30000});
   await page.locator('.ant-modal-confirm').waitFor({state:'hidden'});
  }
  await settlement.getByText('最终结算已冻结。后续更正不能覆盖历史，当前更正用例尚未实现。',{exact:true}).waitFor();assert.equal(await settlement.locator('[data-settlement-id]').count(),2);await page.screenshot({path:output+'/contract-final-settlement.png',animations:'disabled'});mark('real-contract-progress-final-settlement-cancel-freeze-history');
 }
 stage='contract-material';const slot=d.locator('[data-contract-material-kind=contract]');const contractFile='公网验收合同-'+runID+'.png';const row=slot.locator('.attachment-uploader__row').filter({hasText:contractFile});if(!await row.count()){await slot.locator('input[type=file]').setInputFiles({name:contractFile,mimeType:'image/png',buffer:png});await slot.getByRole('button',{name:'上传并登记合同文件'}).click();}await row.getByRole('button',{name:/^下\s*载$/}).waitFor({timeout:30000});await page.screenshot({path:output+'/contract-active-file.png',animations:'disabled'});const downloadEvent=page.waitForEvent('download');await row.getByRole('button',{name:/^下\s*载$/}).click();await (await downloadEvent).saveAs(output+'/contract.png');assert.deepEqual(readFileSync(output+'/contract.png'),png);mark('real-evidence-contract-reference-byte-exact-download');await close(d);
 // Same project, both original cash groups, actual Money commands and Link.
 await openTab('payments');const pendingCash=page.getByRole('button',{name:'继续原支付操作'});if(resume&&await pendingCash.count()){await pendingCash.click();await pendingCash.waitFor({state:'hidden',timeout:30000});}const cashNotes=[];
 for(const [chain,fee,amount] of [['client','设计费','1.01'],['income','咨询费','2.02']]){
  stage='cash-create-'+chain;const note='公网连续验收-'+runID+'-'+chain;cashNotes.push(note);if(resume&&await page.locator('.transaction-panel__item').filter({hasText:note}).count())continue;await page.locator('.transaction-panel__group--'+chain+' .create-action-button').click();d=page.locator('.ant-drawer:visible');await selectIndex(d,'收款账簿主体',0);await selectIndex(d,'付款主体',1);await date(d,'支付时间');await d.getByLabel('实际收款账户',{exact:true}).fill('public-acceptance-'+runID);await selectText(d,'费用类型',fee);await d.getByLabel('支付金额',{exact:true}).fill(amount);await d.getByLabel('原始资金依据',{exact:true}).fill('OA-VERIFY-20261002-C-'+runID);await d.getByLabel('备注',{exact:true}).fill(note);
  if(chain==='client'){drop={method:'POST',path:'/money/api/v1/cash-transactions'};result.lost_reply_money=drop;}
  await d.getByRole('button',{name:/^提\s*交$/}).click();
  if(chain==='client'){await lost(result.lost_reply_money);await page.getByText(/支付操作尚未全部确认/).waitFor({timeout:30000});await page.reload();await page.getByRole('button',{name:'继续原支付操作'}).click();}
  await page.locator('.transaction-panel__item').filter({hasText:note}).waitFor({timeout:30000});if(await d.isVisible())await close(d);
 }
 const cashCommands=result.commands.filter(v=>v.path==='/money/api/v1/cash-transactions');assert.equal(cashCommands.length,3);assert.equal(cashCommands[0].command_id,cashCommands[1].command_id);assert.notEqual(cashCommands[1].command_id,cashCommands[2].command_id);mark('real-cash-committed-lost-reply-reload-same-command');
 const cashBefore=result.commands.length;await openTab('payments');assert.equal(result.commands.length,cashBefore);const firstCash=page.locator('.transaction-panel__item').filter({hasText:cashNotes[0]});if(!await firstCash.getByText('已冲销（原金额保留）').count()){await firstCash.click();d=page.locator('.ant-drawer:visible');assert.equal(await d.getByLabel('支付金额',{exact:true}).inputValue(),'1.01');assert.equal(await d.getByLabel('支付金额',{exact:true}).isDisabled(),true);await d.getByRole('button',{name:/冲\s*销/}).click();const reverse=page.locator('.ant-modal:visible');await reverse.getByLabel('冲销原因',{exact:true}).fill('公网连续验收：保留原记录与冲销证据');await reverse.getByRole('button',{name:'确认冲销'}).click();await reverse.waitFor({state:'hidden'});}await firstCash.getByText('已冲销（原金额保留）').waitFor();await page.screenshot({path:output+'/cash-both-chains.png',animations:'disabled'});mark('real-cash-immutable-detail-explicit-reversal');
 const partial=await readProject();result.project_revision_after=partial.project.revision;
 result.native_records=partial.resource_links.filter(v=>!v.unlinked_at&&!result.existing_resource_ids.includes(v.resource_id)).map(v=>({application:v.target_service,resource_type:v.resource_type,id:v.resource_id,relation_key:v.relation_key}));
 for(const record of result.native_records.filter(v=>v.application==='money-flow-ledger')){
  const r=await context.request.get(origin+'/money/api/v1/cash-transactions/'+record.id);assert.equal(r.status(),200);
  const native=(await r.json()).transaction;assert.equal(native.id,record.id);assert.equal(native.revision,1);
  record.native_status=native.status;record.native_revision=native.revision;record.amount=native.amount;record.effective_amount=native.effective_amount;
  assert(['1.0100','2.0200','1.01','2.02'].includes(native.amount));
  if(native.status==='reversed')assert.equal(cents(native.effective_amount.replace(/(\.\d*?)0+$/,'$1').replace(/\.$/,'')),0n);
 }
 await page.screenshot({path:output+'/payments-final.png',animations:'disabled'});mark('same-project-cash-native-readback-original-and-effective-amount');
 // Billing registration, append correction and actual Evidence reference.
 const invoicePane=await openTab('invoices');
 if(!await invoicePane.locator('.create-action-button').count()){
  result.blocked_authority={application:'billing-invoice',fixture_id:project,reason:'No native project-scoped Billing registration authority; Cost approval is not Billing approval.'};
  throw new Error('Billing project-level authorization is missing; no grant or business write was attempted.');
 }
 const pendingInvoice=page.getByRole('button',{name:'继续原开票操作'});if(resume&&await pendingInvoice.count()){await pendingInvoice.click();await pendingInvoice.waitFor({state:'hidden',timeout:30000});await openTab('invoices');}const invoiceIDs=[];
 for(const [chain,fee,amount] of [['client','设计费','1.01'],['income','咨询费','2.02']]){
  stage='invoice-create-'+chain;const expectedNote='公网开票验收-'+runID+'-'+chain;const existing=[...responses.entries()].filter(([path,v])=>/^\/billing\/api\/v1\/invoice-registrations\/[a-f0-9-]+$/.test(path)&&v.note===expectedNote).map(([,v])=>v);assert(existing.length<=1,'multiple invoices for one continued intent');if(existing.length){invoiceIDs.push(existing[0].id);if(!knownInvoiceIDs.includes(existing[0].id))knownInvoiceIDs.push(existing[0].id);continue;}await page.locator('.transaction-panel__group--'+chain+' .create-action-button').click();d=page.locator('.ant-drawer:visible');await selectIndex(d,'开票方',0);await selectIndex(d,'受票方',1);await date(d,'开票时间');await selectText(d,'费用类型',fee);await d.getByLabel('开票金额',{exact:true}).fill(amount);await d.getByLabel('备注',{exact:true}).fill(expectedNote);const before=await readProject();await d.getByRole('button',{name:/^提\s*交$/}).click();await d.waitFor({state:'hidden',timeout:30000});const after=await readProject();const added=after.resource_links.filter(v=>v.target_service==='billing-invoice'&&!v.unlinked_at&&!before.resource_links.some(old=>old.id===v.id));assert.equal(added.length,1);invoiceIDs.push(added[0].resource_id);knownInvoiceIDs.push(added[0].resource_id);
 }
 const invoiceBefore=result.commands.length;await openTab('invoices');assert.equal(result.commands.length,invoiceBefore);mark('real-billing-two-chains-save-reload-zero-write');
 const current=await readProject(),clientLinks=current.resource_links.filter(v=>v.target_service==='billing-invoice'&&!v.unlinked_at&&v.relation_key.startsWith('reference-oa-invoice-v1-client-'));const index=clientLinks.findIndex(v=>v.resource_id===invoiceIDs[0]);assert(index>=0);await page.locator('.transaction-panel__group--client .transaction-panel__item').nth(index).click();d=page.locator('.ant-drawer:visible');
 if((await d.getByLabel('开票金额',{exact:true}).inputValue())==='1.01'){await d.getByLabel('开票金额',{exact:true}).fill('1.02');await d.getByLabel('纠正原因',{exact:true}).fill('公网连续验收：检验追加纠正历史');drop={method:'POST',path:'/billing/api/v1/invoice-registrations/'+invoiceIDs[0]+'/corrections'};result.lost_reply_billing=drop;await d.getByRole('button',{name:/追加纠正/}).click();await lost(result.lost_reply_billing);await page.getByText(/开票操作尚未全部确认/).waitFor({timeout:30000});await page.reload();await page.getByRole('button',{name:'继续原开票操作'}).click();await page.getByRole('button',{name:'继续原开票操作'}).waitFor({state:'hidden',timeout:30000});await page.locator('.transaction-panel__group--client .transaction-panel__item').nth(index).click();d=page.locator('.ant-drawer:visible');}
 assert.equal(await d.getByLabel('开票金额',{exact:true}).inputValue(),'1.02');assert(result.lost_reply_billing?.committed);await d.getByRole('button',{name:'查看登记历史'}).click();await d.getByText(/1.01/).waitFor();mark('real-billing-correction-lost-reply-same-command-history');
 if(!done('real-billing-file-preview-download-reference-removal-no-byte-delete')){
  const invoiceFile='公网验收发票-'+runID+'.png';
  if(!await d.getByText(invoiceFile,{exact:true}).count()){
   await d.locator('input[type=file]').setInputFiles({name:invoiceFile,mimeType:'image/png',buffer:png});
   await d.getByRole('button',{name:'上传并登记发票文件'}).click();
  }
  await d.getByText(invoiceFile,{exact:true}).waitFor({timeout:30000});
  await d.getByRole('button',{name:/^预\s*览$/}).click();
  await page.locator('.file-preview-drawer__shell img').waitFor({timeout:30000});
  await page.screenshot({path:output+'/invoice-preview.png',animations:'disabled'});
  await page.locator('.ant-drawer:visible').last().locator('.ant-drawer-close').click();
  const invoiceDownload=page.waitForEvent('download');await d.getByRole('button',{name:/^下\s*载$/}).click();
  await (await invoiceDownload).saveAs(output+'/invoice.png');assert.deepEqual(readFileSync(output+'/invoice.png'),png);
  await d.getByRole('button',{name:/^移\s*除$/}).click();
  await page.locator('.ant-modal-confirm').getByRole('button',{name:/确\s*定/}).click();
  await d.getByText(invoiceFile,{exact:true}).waitFor({state:'hidden'});
  assert.equal(result.commands.filter(v=>v.method==='DELETE'&&v.path.startsWith('/evidence/')).length,0);
  await page.screenshot({path:output+'/invoice-history.png',animations:'disabled'});
  mark('real-billing-file-preview-download-reference-removal-no-byte-delete');
 }
 await close(d);
 const final=await readProject();result.project_revision_after=final.project.revision;assert(result.project_revision_after>result.project_revision_before);assert.equal(final.resource_links.filter(v=>!v.unlinked_at&&!result.existing_resource_ids.includes(v.resource_id)).length,6);for(const id of result.existing_resource_ids)assert(final.resource_links.some(v=>v.resource_id===id&&!v.unlinked_at));assert.equal(result.page_errors.length,0);assert(result.commands.every(v=>!v.path.includes('/projects/')||v.path.includes('/projects/'+project+'/')));assert.equal(result.commands.filter(v=>/\/grants|\/assignment/.test(v.path)).length,0);
 result.native_records=final.resource_links.filter(v=>!v.unlinked_at&&!result.existing_resource_ids.includes(v.resource_id)).map(v=>({application:v.target_service,resource_type:v.resource_type,id:v.resource_id,relation_key:v.relation_key}));
 for(const [application,tab,collection] of [['contract-ledger','contracts','contracts'],['money-flow-ledger','payments','cash-transactions'],['billing-invoice','invoices','invoice-registrations']]){
  await openTab(tab);const route=await page.locator(`[data-real-application="${application}"]`).first().getAttribute('data-real-route');assert(route?.startsWith('/'));
  for(const record of result.native_records.filter(v=>v.application===application)){
   const r=await context.request.get(origin+route+'/api/v1/'+collection+'/'+record.id);assert.equal(r.status(),200);const value=await r.json(),native=value.contract||value.transaction||value;assert.equal(native.id,record.id);
   record.native_status=native.status;record.native_revision=native.current_revision||native.revision||native.draft_version;
   if(application==='contract-ledger')record.total_amount=native.total_amount;
   if(application==='money-flow-ledger'){record.amount=native.amount;record.effective_amount=native.effective_amount;assert.equal(native.revision,1);const minor=s=>cents(s.replace(/(\.\d*?)0+$/,'$1').replace(/\.$/,''));assert([101n,202n].includes(minor(native.amount)));if(native.status==='reversed')assert.equal(minor(native.effective_amount),0n);}
   if(application==='billing-invoice'){record.amount_minor_decimal=native.amount_minor_decimal;assert(['102','202'].includes(native.amount_minor_decimal));assert(native.source_refs.some(v=>v.kind==='project'&&v.resource_id===project));const history=await context.request.get(origin+route+'/api/v1/'+collection+'/'+record.id+'/history');assert.equal(history.status(),200);const rows=(await history.json()).items;record.history_revisions=rows.map(v=>v.revision);if(record.id===invoiceIDs[0])assert(rows.some(v=>v.snapshot.amount_minor_decimal==='101'));}
  }
  await page.screenshot({path:output+'/'+tab+'-final.png',animations:'disabled'});
 }
 assert.equal(result.page_errors.length,0);mark('same-project-real-owner-reads-and-historical-invoice-amount');result.passed=true;
}catch(e){result.failure={stage,type:e.name,message:e.message.slice(0,500)};process.exitCode=1;await page.screenshot({path:output+'/failure.png'}).catch(()=>{});writeFileSync(output+'/failure-ui-private.txt',await page.locator('body').innerText().catch(()=>''),{mode:0o600});
 // Preserve pending command IDs and login context privately. Unexpected test
 // failure must be continued against the same facts, never blindly recreated.
 writeFileSync(output+'/recovery-private.json',JSON.stringify({stage,storage:await context.storageState(),session:await page.evaluate(()=>Object.fromEntries(Object.entries(sessionStorage))).catch(()=>({}))}),{mode:0o600});
}
finally{await browser.close();writeFileSync(output+'/browser.json',JSON.stringify(result,null,2)+'\n',{mode:0o600});console.log(JSON.stringify({passed:result.passed,stage,scenarios:result.scenarios.length,commands:result.commands.length,page_errors:result.page_errors.length,failure:result.failure}));}
