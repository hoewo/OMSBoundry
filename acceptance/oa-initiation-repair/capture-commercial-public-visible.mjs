// Read-only visible receipt checks against the existing public C facts.
import assert from 'node:assert/strict';
import {mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {setTimeout as delay} from 'node:timers/promises';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const output=process.argv[2];assert(/^\/tmp\/oa-commercial-visible\.[A-Za-z0-9]+$/.test(output));mkdirSync(output,{recursive:true});
const project='70edead7-9dc9-44a3-9b35-b9d41b83264e',origin='https://oatest.antuinfo.cn';
const recovery=JSON.parse(readFileSync('/tmp/oa-commercial-journey.35jPS3/recovery-private.json'));
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
const context=await browser.newContext({viewport:{width:1600,height:1100},storageState:recovery.storage});
const page=await context.newPage(),result={passed:false,fixture_id:project,read_only_business:true,commands:[],page_errors:[],full_oa_accepted:false};
let next=0,stage='contract';
page.on('pageerror',e=>result.page_errors.push(e.message));
page.on('request',r=>{const path=new URL(r.url()).pathname;if(['POST','PUT','PATCH','DELETE'].includes(r.method()))result.commands.push({method:r.method(),path});});
await page.route('**/*',async route=>{const at=Math.max(Date.now(),next);next=at+800;await delay(Math.max(0,at-Date.now()));await route.continue();});
async function projectRead(){const r=await context.request.get(origin+'/project/api/v1/projects/'+project);assert.equal(r.status(),200);return r.json();}
try{
 await page.goto(origin+`/workbench/?project_id=${project}&tab=business&business_tab=contracts`,{waitUntil:'domcontentloaded'});
 const panel=page.locator('[data-real-contract-react]');await panel.locator('.ant-spin-spinning').waitFor({state:'hidden'});
 const item=panel.locator('.contract-panel__item').filter({hasText:'OA-PUBLIC-35jPS3-client'});await item.waitFor({timeout:30000});
 // Restore the Suite session through the actual Workbench/Gateway bootstrap
 // before reading Project: a stale capability Cookie can redirect to HTML.
 const before=await projectRead();assert.equal(before.project.project_number,'OA-VERIFY-20261002-C');await item.click();
 const drawer=page.locator('.ant-drawer:visible');await drawer.locator('[data-native-contract-preparation]').getByText(/状态：已生效/).waitFor();
 stage='settlement-history';const settlement=drawer.locator('[data-native-contract-settlement]');
 assert.equal(await settlement.locator('[data-settlement-id]').count(),2);
 await settlement.getByText('OA-PUBLIC-35jPS3-progress',{exact:false}).waitFor();
 await settlement.getByText('OA-PUBLIC-35jPS3-final',{exact:false}).waitFor();
 await settlement.scrollIntoViewIfNeeded();await settlement.screenshot({path:output+'/contract-settlement-history.png',animations:'disabled'});
 result.frozen_settlements=2;
 stage='amendment-history';const amendments=drawer.locator('.contract-amendment-list');
 assert.equal(await amendments.locator('.contract-amendment-list__item').count(),3);
 await amendments.scrollIntoViewIfNeeded();await amendments.screenshot({path:output+'/contract-amendment-history.png',animations:'disabled'});
 result.confirmed_amendments=3;
 stage='contract-preview';const row=drawer.locator('[data-contract-material-kind=contract] .attachment-uploader__row').filter({hasText:'公网验收合同-35jPS3.png'});
 await row.getByRole('button',{name:/^预\s*览$/}).click();
 const image=page.locator('.file-preview-drawer__shell img');await image.waitFor({timeout:30000});
 await page.waitForFunction(el=>el.complete&&el.naturalWidth===640&&el.naturalHeight===400,await image.elementHandle(),{timeout:30000});
 await page.screenshot({path:output+'/contract-visible-preview.png',animations:'disabled'});result.actual_image_preview={width:640,height:400};
 // :visible.last() would retarget the still-open Contract drawer after the
 // preview closes. Observe the preview's own content, not another drawer.
 const preview=page.locator('.ant-drawer:visible').filter({has:page.locator('.file-preview-drawer__shell')});
 await preview.locator('.ant-drawer-close').click();await image.waitFor({state:'hidden'});
 stage='billing-authority';await page.goto(origin+`/workbench/?project_id=${project}&tab=business&business_tab=invoices`,{waitUntil:'domcontentloaded'});
 const invoice=page.locator('[data-real-invoice-react]');await invoice.getByRole('button',{name:'刷新当前开票'}).waitFor({timeout:30000});
 await invoice.locator('.ant-spin-spinning').waitFor({state:'hidden'});
 result.billing_registration_buttons=await invoice.locator('.create-action-button').count();
 assert.equal(result.billing_registration_buttons,0,'Do not assume newly granted Billing authority');
 await invoice.getByText(/没有项目级开票列表／登记权限/).waitFor();await page.screenshot({path:output+'/billing-scope-required.png',animations:'disabled'});
 const after=await projectRead();assert.deepEqual(after,before);result.project_revision=after.project.revision;
 assert.equal(result.commands.filter(v=>!v.path.startsWith('/gateway/')&&!/^\/evidence\/v1\/files\/[a-f0-9-]+\/versions\/1\/access-grants$/.test(v.path)).length,0);
 assert.equal(result.page_errors.length,0);result.passed=true;
}catch(e){result.failure={stage,type:e.name,message:e.message.slice(0,500)};process.exitCode=1;await page.screenshot({path:output+'/failure.png'}).catch(()=>{});}
finally{await browser.close();writeFileSync(output+'/visible.json',JSON.stringify(result,null,2)+'\n',{mode:0o600});console.log(JSON.stringify(result));}
