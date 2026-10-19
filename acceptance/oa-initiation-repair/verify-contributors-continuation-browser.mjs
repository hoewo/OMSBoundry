// Continue public-d's saved/end history, not a new project or reset database.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import {mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const output=process.argv[2];assert(output&&process.env.OA_CONTRIBUTORS_PUBLIC_CHECK==='1');mkdirSync(output,{recursive:true});
const d=JSON.parse(readFileSync('/tmp/oa-complete-operations.TEfzlpv7/public-d/browser.json')),e=JSON.parse(readFileSync('/tmp/oa-complete-operations.TEfzlpv7/public-e-readonly/browser.json'));
assert.equal(d.commands.length,2);assert(e.passed&&e.business_posts===0&&e.native_revision===10);
const project=d.fixture_id,origin='https://oatest.antuinfo.cn';assert.equal(project,'70edead7-9dc9-44a3-9b35-b9d41b83264e');
const code='import json,subprocess;v=json.loads(subprocess.check_output(["docker","inspect","omsbizfoundry-identity-access-1"]))[0];e=dict(x.split("=",1) for x in v["Config"]["Env"]);print(json.dumps({k:e[k] for k in ("BOOTSTRAP_ADMIN_USERNAME","BOOTSTRAP_ADMIN_PASSWORD")}))';
const inspected=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',"sudo python3 -c '"+code+"'"],{encoding:'utf8',timeout:25000});assert.equal(inspected.status,0);const secret=JSON.parse(inspected.stdout);
const receipt={passed:false,fixture_id:project,continued_from:'public-d/public-e-readonly/public-i-interrupted-readback',real_iam_gateway_postgresql:true,full_oa_accepted:false,commands:[],errors:[],responses:[],scenarios:[],grant_writes:0};
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true}),context=await browser.newContext({viewport:{width:1450,height:930}}),page=await context.newPage(),panel=page.locator('[data-real-personnel-react]');
let stage='login';
page.on('pageerror',e=>receipt.errors.push(e.message));
page.on('response',r=>{if(r.status()>=400)receipt.responses.push({path:new URL(r.url()).pathname,status:r.status()})});
page.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname===`/project/api/v1/projects/${project}/assignments`)receipt.commands.push(r.postDataJSON())});
async function read(history=false){const r=await context.request.get(origin+`/project/api/v1/projects/${project}`+(history?'?include_inactive=true':''));assert.equal(r.status(),200);const v=await r.json();assert.equal(v.project.id,project);return v}
try{
 await page.goto(origin+'/workbench/',{waitUntil:'domcontentloaded'});await page.locator('[data-login-mode="iam"]').click();
 await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME);await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD);await page.locator('form[action="/gateway/login"] button[type="submit"]').click();
 await page.locator('[data-real-project-list] [data-real-detail-open]').first().waitFor({timeout:30000});
 stage='continued-history';await page.goto(origin+`/workbench/?project_id=${project}&tab=business&business_tab=personnel`,{waitUntil:'domcontentloaded'});await panel.locator('.business-personnel-list__item').first().waitFor({timeout:30000});
 receipt.served_assets={};for(const extension of ['js','css']){const name='reference_oa_react_bidding.'+extension,url=await page.locator(extension==='js'?'script[src*="reference-oa-react-bidding.js"]':'link[href*="reference-oa-react-bidding.css"]').getAttribute(extension==='js'?'src':'href');assert(url);const response=await context.request.get(new URL(url,page.url()).href);assert.equal(response.status(),200);const hash=createHash('sha256').update(await response.body()).digest('hex');assert.equal(hash,createHash('sha256').update(readFileSync(new URL('../../frontend/oms-workbench/internal/server/'+name,import.meta.url))).digest('hex'));receipt.served_assets[extension]=hash;}
 const initial=await read(true);assert.equal(initial.project.revision,11);const ended=initial.assignments.find(a=>a.id===d.commands[1].assignment_id);assert(ended?.ended_at&&ended.role_key==='commercial_contributor');const resumed=initial.assignments.filter(a=>a.role_key==='commercial_contributor'&&!a.ended_at);assert.equal(resumed.length,1);assert.equal(resumed[0].subject_id,ended.subject_id);assert.notEqual(resumed[0].id,ended.id);
 await panel.getByRole('button',{name:'刷新当前人员',exact:true}).click();await panel.locator('.business-personnel-list__item').first().waitFor();assert.equal(receipt.commands.length,0);receipt.scenarios.push({name:'interrupted-native-commit-history-refresh-zero-write',passed:true});
 stage='cancel-end-then-historical-end';await panel.locator('.business-personnel-list__actions button').first().click();await page.locator('.confirm-delete__dialog').getByRole('button',{name:/取\s*消/}).click();assert.equal(receipt.commands.length,0);
 await panel.locator('.business-personnel-list__actions button').first().click();await page.locator('.confirm-delete__dialog').getByRole('button',{name:'确认移出',exact:true}).click();await panel.getByText('当前还没有经营参与人',{exact:true}).waitFor();assert.equal(receipt.commands.length,1);assert.equal(receipt.commands[0].assignment_id,resumed[0].id);assert.equal((await read()).project.revision,12);receipt.scenarios.push({name:'cancel-zero-write-native-end-readback-no-rate-limit',passed:true});
 stage='committed-response-loss';const pattern=origin+`/project/api/v1/projects/${project}/assignments`;let injected=false;
 await page.route(pattern,async route=>{if(route.request().method()!=='POST'||injected)return route.continue();injected=true;try{const native=await route.fetch();assert.equal(native.status(),200);await route.fulfill({status:503,contentType:'application/json',body:'{"error":{"code":"injected_response_loss"}}'})}catch(error){receipt.route_failure={type:error.name};await route.abort().catch(()=>{})}});
 const directoryResponse=context.request.get(origin+'/people/api/v1/persons');
 await panel.locator('button').filter({hasText:'添加参与人'}).click();const drawer=page.locator('.ant-drawer-content');await drawer.waitFor();
 const directory=await directoryResponse;assert.equal(directory.status(),200);const people=await directory.json();assert(Array.isArray(people));
 // The fixture directory has several different Person IDs sharing one name.
 // Derive the visible option position from the exact native ID, never guess a
 // same-name person or resolve a strict locator by blindly taking first().
 const names=people.filter(p=>p.status==='active'&&p.display_name===ended.display_snapshot);const position=names.findIndex(p=>p.id===ended.subject_id);assert(position>=0);
 await drawer.getByRole('combobox').fill(ended.display_snapshot);const options=page.locator('.ant-select-item-option').filter({hasText:ended.display_snapshot});await options.first().waitFor();assert.equal(await options.count(),names.length);await options.nth(position).click();
 const lostResponse=page.waitForResponse(r=>r.url()===pattern&&r.request().method()==='POST'&&r.status()===503);await drawer.getByRole('button',{name:/添\s*加/}).click();await lostResponse;
 await page.getByRole('button',{name:'继续原人员操作',exact:true}).waitFor({state:'attached'});assert.equal(receipt.commands.length,2);assert.equal((await read()).project.revision,13);
 await drawer.getByRole('button',{name:/取\s*消/}).click();await drawer.waitFor({state:'hidden'});await page.unroute(pattern);
 // A reload must recover the same scoped pending command, not silently discard
 // it or create a new assignment on a fresh command ID.
 stage='reload-pending';await page.reload({waitUntil:'domcontentloaded'});await panel.getByRole('button',{name:'继续原人员操作',exact:true}).waitFor({timeout:30000});await page.screenshot({path:output+'/pending-after-reload.png'});
 stage='manual-exact-replay';await panel.getByRole('button',{name:'继续原人员操作',exact:true}).click();await panel.getByRole('button',{name:'继续原人员操作',exact:true}).waitFor({state:'hidden'});await panel.locator('.business-personnel-list__item').first().waitFor();
 assert.equal(receipt.commands.length,3);assert.deepEqual(receipt.commands[1],receipt.commands[2]);assert.equal(receipt.commands[1].expected_revision,12);assert.equal(receipt.commands[1].subject_id,ended.subject_id);
 const final=await read(true),active=final.assignments.filter(a=>a.role_key==='commercial_contributor'&&!a.ended_at);assert.equal(final.project.revision,13);assert.equal(active.length,1);assert.notEqual(active[0].id,ended.id);assert.notEqual(active[0].id,resumed[0].id);assert(final.assignments.some(a=>a.id===ended.id&&a.ended_at));assert(final.assignments.some(a=>a.id===resumed[0].id&&a.ended_at));
 receipt.scenarios.push({name:'new-assignment-response-loss-reload-manual-same-command-history-preserved',passed:true});
 stage='refresh-and-adjacent-tabs';await panel.getByRole('button',{name:'刷新当前人员',exact:true}).click();for(const tab of ['bidding','contracts','payments','invoices','costs','bonus','personnel']){await page.locator(`[data-real-business-tab="${tab}"]`).click();assert.equal(new URL(page.url()).searchParams.get('business_tab'),tab)}
 await panel.locator('.business-personnel-list__item').first().waitFor({timeout:30000});assert.equal((await read()).project.revision,13);assert.equal(receipt.commands.length,3);assert.equal(receipt.errors.length,0);assert.equal(receipt.responses.filter(r=>r.status===429).length,0);
 await page.screenshot({path:output+'/personnel-final.png'});receipt.final_project_revision=13;receipt.assignment_ids=[ended.id,resumed[0].id,active[0].id];receipt.scenarios.push({name:'fast-refresh-and-adjacent-navigation-zero-new-write-no-rate-limit',passed:true});receipt.passed=true;
}catch(error){receipt.failure={type:error.name,stage};await page.screenshot({path:output+'/failure.png'}).catch(()=>{})}
finally{await page.unrouteAll({behavior:'ignoreErrors'});await browser.close();writeFileSync(output+'/browser.json',JSON.stringify(receipt,null,2)+'\n');console.log(JSON.stringify({passed:receipt.passed,scenarios:receipt.scenarios.length,commands:receipt.commands.length,failure:receipt.failure}));if(!receipt.passed)process.exitCode=1}
