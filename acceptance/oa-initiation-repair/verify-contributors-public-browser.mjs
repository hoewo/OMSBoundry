// Explicit public acceptance on the continued C fixture. Secrets stay in memory.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,writeFileSync} from 'node:fs';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const output=process.argv[2];assert(output&&process.env.OA_CONTRIBUTORS_PUBLIC_CHECK==='1');mkdirSync(output,{recursive:true});
const root='/opt/omsbizfoundry/acceptance/oa-contributors-release-20261003a',origin='https://oatest.antuinfo.cn';
const ssh=code=>{const r=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',code],{encoding:'utf8',timeout:25000});assert.equal(r.status,0,'private inspection unavailable');return r.stdout};
const before=JSON.parse(ssh('sudo cat '+root+'/browser-ownership/before.json'));
const project=before.fixture_id;assert(project!=='f75d4570-5d6a-42ba-ade6-d1cda913ddda');
const secretCode='import json,subprocess;v=json.loads(subprocess.check_output(["docker","inspect","omsbizfoundry-identity-access-1"]))[0];e=dict(x.split("=",1) for x in v["Config"]["Env"]);print(json.dumps({k:e[k] for k in ("BOOTSTRAP_ADMIN_USERNAME","BOOTSTRAP_ADMIN_PASSWORD")}))';
const secret=JSON.parse(ssh("sudo python3 -c '"+secretCode+"'"));
const receipt={passed:false,fixture_id:project,real_iam_gateway_postgresql:true,full_oa_accepted:false,original_components:true,scenarios:[],commands:[],page_errors:[],explicit_admin_grant:false};
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
const context=await browser.newContext({viewport:{width:1450,height:930}}),page=await context.newPage();
const personnel=page.locator('[data-real-personnel-react]');
let stage='login';
page.on('pageerror',e=>receipt.page_errors.push(e.message));
page.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname===`/project/api/v1/projects/${project}/assignments`){const c=r.postDataJSON();receipt.commands.push({command_id:c.command_id,operation:c.operation,expected_revision:c.expected_revision,assignment_id:c.assignment_id,subject_id:c.subject_id,source_revision:c.source_revision})}});
async function detail(){const r=await context.request.get(origin+`/project/api/v1/projects/${project}`);receipt.detail_status=r.status();assert.equal(r.status(),200);const v=await r.json();assert.equal(v.project.id,project);return v}
const members=v=>v.assignments.filter(a=>a.role_key==='commercial_contributor'&&!a.ended_at);
async function openAdd(){await personnel.locator('button').filter({hasText:'添加参与人'}).click();const d=page.locator('.ant-drawer-content');await d.waitFor();return d}
async function selectPerson(d,personName){await d.getByRole('combobox').click();const options=page.locator('.ant-select-item-option');await options.first().waitFor();const option=personName?options.filter({hasText:personName}):options.first();const label=await option.getAttribute('title')||await option.innerText();await option.click();return label}
try{
  await page.goto(origin+'/workbench/',{waitUntil:'domcontentloaded'});
  await page.locator('[data-login-mode="iam"]').click();
  await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME);
  await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD);
  await page.locator('form[action="/gateway/login"] button[type="submit"]').click();
  await page.locator('[data-real-project-list] [data-real-detail-open]').first().waitFor({timeout:30000});
  const session=await context.request.get(origin+'/auth/api/v1/session');assert.equal(session.status(),200);const identity=(await session.json()).principal;assert(identity?.subject_id&&identity?.tenant_id);
  stage='explicit-native-admin-grant';
  const csrf=(await context.cookies()).find(c=>c.name==='oms_iam_csrf')?.value;assert(csrf);
  if(process.env.OA_CONTRIBUTORS_GRANT_ALREADY_CREATED==='1'){
    receipt.explicit_admin_grant=true;receipt.reused_prior_explicit_grant=true;
  }else{
    const grant=await context.request.post(origin+'/auth/api/v1/admin/grants',{headers:{'Content-Type':'application/json','X-CSRF-Token':csrf,Origin:origin},data:{subject_id:identity.subject_id,audience:'project-workspace',action:'project-workspace.assignment.commercial-contributor.manage',scope:{kind:'tenant',resources:[]}}});
    assert.equal(grant.status(),204);receipt.explicit_admin_grant=true;
  }
  const url=origin+`/workbench/?project_id=${project}&tab=business&business_tab=personnel`;
  stage='continued-c-personnel';await page.goto(url,{waitUntil:'domcontentloaded'});
  await personnel.locator('button').filter({hasText:'添加参与人'}).waitFor({timeout:30000});
  stage='continued-c-native-detail';const original=await detail();assert.equal(original.project.revision,before.fixture.revision);assert.equal(members(original).length,0);
  assert(original.allowed_actions.includes('project-workspace.assignment.commercial-contributor.manage'));
  stage='cancel-drawer';let drawer=await openAdd();assert.equal(Math.round((await page.locator('.ant-drawer-content-wrapper').boundingBox()).width),520);
  await drawer.getByRole('button',{name:/取\s*消/}).click();await drawer.waitFor({state:'hidden'});assert.equal(receipt.commands.length,0);assert.equal((await detail()).project.revision,original.project.revision);
  receipt.scenarios.push({name:'original-520-drawer-cancel-zero-write',passed:true});
  stage='native-add';drawer=await openAdd();const personName=await selectPerson(drawer);await drawer.getByRole('button',{name:/添\s*加/}).click();await drawer.waitFor({state:'hidden'});
  await personnel.locator('.business-personnel-list__item').first().waitFor();const first=await detail();assert.equal(members(first).length,1);assert.equal(first.project.revision,original.project.revision+1);const assigned=members(first)[0];
  assert.equal(assigned.role_key,'commercial_contributor');assert.equal(assigned.subject_kind,'person');assert.equal(receipt.commands.length,1);
  await page.screenshot({path:output+'/personnel-added.png'});
  stage='same-project-reload';await page.reload({waitUntil:'domcontentloaded'});await personnel.locator('.business-personnel-list__item').first().waitFor({timeout:30000});await page.getByRole('button',{name:'刷新当前人员',exact:true}).click();assert.equal(receipt.commands.length,1);assert.equal((await detail()).project.revision,first.project.revision);
  receipt.scenarios.push({name:'native-add-close-reload-refresh-zero-write',passed:true});
  stage='cancel-removal';await personnel.locator('.business-personnel-list__actions button').first().click();await page.locator('.confirm-delete__dialog').getByRole('button',{name:/取\s*消/}).click();assert.equal(receipt.commands.length,1);
  stage='native-historical-end';await personnel.locator('.business-personnel-list__actions button').first().click();await page.locator('.confirm-delete__dialog').getByRole('button',{name:'确认移出',exact:true}).click();await personnel.getByText('当前还没有经营参与人',{exact:true}).waitFor();assert.equal(members(await detail()).length,0);assert.equal(receipt.commands.length,2);
  receipt.scenarios.push({name:'confirmation-cancel-zero-write-then-end-preserves-history',passed:true});
  stage='committed-response-loss';let injected=false;
  const pattern=origin+`/project/api/v1/projects/${project}/assignments`;
  await page.route(pattern,async route=>{if(route.request().method()!=='POST'||injected)return route.continue();injected=true;try{const committed=await route.fetch();assert.equal(committed.status(),200);await route.fulfill({status:503,contentType:'application/json',body:'{"error":{"code":"injected_response_loss"}}'})}catch(error){receipt.route_failure={type:error.name};await route.abort().catch(()=>{})}});
  drawer=await openAdd();await selectPerson(drawer,personName);await drawer.getByRole('button',{name:/添\s*加/}).click();
  await page.getByRole('button',{name:'继续原人员操作',exact:true}).waitFor({state:'attached'});assert.equal(receipt.commands.length,3);assert.equal(members(await detail()).length,1);
  await drawer.getByRole('button',{name:/取\s*消/}).click();await drawer.waitFor({state:'hidden'});await page.unroute(pattern);
  stage='manual-same-command-retry';await page.getByRole('button',{name:'继续原人员操作',exact:true}).click();await page.getByRole('button',{name:'继续原人员操作',exact:true}).waitFor({state:'hidden'});
  const last=await detail();assert.equal(receipt.commands.length,4);assert.deepEqual(receipt.commands[2],receipt.commands[3]);assert.equal(last.project.revision,original.project.revision+3);assert.equal(members(last).length,1);assert.notEqual(members(last)[0].id,assigned.id);
  receipt.scenarios.push({name:'committed-response-loss-manual-idempotent-retry-new-historical-assignment',passed:true,injected_browser_response_only:true});
  stage='adjacent-tabs';for(const tab of ['bidding','contracts','payments','invoices','costs','bonus','personnel']){await page.locator(`[data-real-business-tab="${tab}"]`).click();assert.equal(new URL(page.url()).searchParams.get('business_tab'),tab)}
  await page.getByRole('button',{name:'刷新当前人员',exact:true}).waitFor();await page.screenshot({path:output+'/personnel-final.png'});
  assert.equal(receipt.page_errors.length,0);receipt.final_project_revision=last.project.revision;receipt.assignment_ids=[assigned.id,members(last)[0].id];receipt.passed=true;delete receipt.stage;
}catch(error){receipt.failure={type:error.name,stage};process.exitCode=1;await page.screenshot({path:output+'/failure.png'}).catch(()=>{})}
finally{await page.unrouteAll({behavior:'ignoreErrors'});await browser.close();writeFileSync(output+'/browser.json',JSON.stringify(receipt,null,2)+'\n');console.log(JSON.stringify({passed:receipt.passed,scenarios:receipt.scenarios.length,commands:receipt.commands.length,failure:receipt.failure,page_errors:receipt.page_errors.length}))}
