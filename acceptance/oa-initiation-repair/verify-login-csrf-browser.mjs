// Opt-in public browser checks. Credentials stay in memory, never in receipts.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync, writeFileSync} from 'node:fs';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';

const output=process.argv[2];
assert(output && process.env.OA_LOGIN_PUBLIC_CHECK==='1', 'explicit opt-in and evidence directory required');
mkdirSync(output,{recursive:true});
const secretCode='import json,subprocess; v=json.loads(subprocess.check_output(["docker","inspect","omsbizfoundry-identity-access-1"]))[0]; e=dict(x.split("=",1) for x in v["Config"]["Env"]); print(json.dumps({k:e[k] for k in ("BOOTSTRAP_ADMIN_USERNAME","BOOTSTRAP_ADMIN_PASSWORD")}))';
const result=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',"sudo python3 -c '"+secretCode+"'"],{encoding:'utf8',timeout:20000});
assert(result.status===0,'private credential read failed');
const privateCredentials=JSON.parse(result.stdout);
const origin='https://oatest.antuinfo.cn', entry=origin+'/workbench/';
const fullSmoke=process.env.OA_LOGIN_FULL_WORKBENCH_SMOKE==='1';
const receipt={passed:false,entry,real_iam_login:true,credentials_exported:false,scenarios:[],business_writes:[],page_errors:[],background_reads_checked:fullSmoke,post_login_business_scripts_checked:fullSmoke};
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
async function scenario(name, mutate, beforeSubmit) {
  const context=await browser.newContext();
  try {
    const page=await context.newPage(), traffic=[];
    receipt.current_stage=name+':page-load';
    page.on('pageerror',error=>receipt.page_errors.push({scenario:name,message:error.message}));
    page.on('request',request=> {
      const path=new URL(request.url()).pathname, method=request.method();
      if (path==='/gateway/login' || path.endsWith('/api/v1/csrf')) traffic.push({method,path});
      if (['POST','PUT','PATCH','DELETE'].includes(method) && path!=='/gateway/login' && !path.startsWith('/gateway/authorize/')) receipt.business_writes.push({scenario:name,method,path});
    });
    // Suppress unrelated post-login dashboard authorization reads to keep the
    // public rate limiter undisturbed. This check proves login, not business UI.
    if (!fullSmoke) await page.route('**/gateway/authorize/**',route=>route.abort());
    if (!fullSmoke) await page.route('**/workbench/assets/**',route=> {
      const path=new URL(route.request().url()).pathname;
      const loginScript=path.endsWith('/reference-oa-login.js') || path.endsWith('/reference-oa-initiation-runtime.js');
      return path.endsWith('.js') && !loginScript ? route.abort() : route.continue();
    });
    await page.goto(entry,{waitUntil:'domcontentloaded'});
    const panel=page.locator('[data-reference-login]');
    const iamRoute=await panel.getAttribute('data-iam-route-prefix');
    assert(/^\/[A-Za-z0-9_-]+(?:\/[A-Za-z0-9_-]+)*$/.test(iamRoute));
    await page.locator('[data-login-mode="iam"]').click();
    await page.locator('input[name="username"]').fill(privateCredentials.BOOTSTRAP_ADMIN_USERNAME);
    await page.locator('input[name="password"]').fill(privateCredentials.BOOTSTRAP_ADMIN_PASSWORD);
    receipt.current_stage=name+':inputs-ready';
    if (mutate) await mutate({context,page,iamRoute,traffic});
    if (beforeSubmit) await beforeSubmit({context,page,iamRoute,traffic});
    await page.locator('form[action="/gateway/login"] button[type="submit"]').click();
    receipt.current_stage=name+':await-authenticated-html';
    await page.waitForFunction(()=>!document.querySelector('[data-reference-login]') && !!document.querySelector('.app-sidebar__user-card') && document.body.dataset.demoPreview==='false',null,{timeout:20000});
    assert.equal(new URL(page.url()).origin,origin);
    const session=await context.request.get(origin+iamRoute+'/api/v1/session');
    assert.equal(session.status(),200);
    const identity=await session.json();
    assert(identity.principal?.subject_id && identity.principal?.tenant_id);
    if (fullSmoke) {
      await page.locator('[data-real-project-list] [data-real-detail-open]').first().waitFor({state:'visible',timeout:20000});
      receipt.native_project_directory_loaded=true;
    }
    await page.screenshot({path:output+'/'+name+'.png',fullPage:false});
    receipt.scenarios.push({name,passed:true,authenticated_page:true,login_posts:traffic.filter(r=>r.path==='/gateway/login').length,csrf_gets:traffic.filter(r=>r.path.endsWith('/api/v1/csrf')).length});
    console.log(JSON.stringify(receipt.scenarios.at(-1)));
  } finally { await context.close(); }
}
try {
  await scenario('fresh');
  if (!fullSmoke) {
  await scenario('expired-cookie',async({context,page})=> {
    await context.clearCookies({name:'oms_iam_csrf'});
    assert.equal((await context.cookies()).some(c=>c.name==='oms_iam_csrf'),false);
    assert((await page.locator('input[name="csrf_token"]').inputValue()).length>16);
  });
  await scenario('other-tab-rotation',async({context,page,iamRoute})=> {
    const old=await page.locator('input[name="csrf_token"]').inputValue();
    const other=await context.newPage();
    const response=await other.goto(origin+iamRoute+'/api/v1/csrf');
    assert.equal(response.status(),200);
    const current=(await context.cookies()).find(c=>c.name==='oms_iam_csrf').value;
    assert.notEqual(old,current); await other.close();
    const negative=await context.request.post(origin+'/gateway/login',{form:{csrf_token:old,username:'',password:'',return_to:'/workbench/'},maxRedirects:0});
    assert.equal(negative.status(),403); assert.equal((await negative.json()).error.code,'csrf_failed');
    receipt.server_stale_csrf_still_rejected=true;
  });
  await scenario('renewal-unavailable-retry',null,async({context,page,iamRoute,traffic})=> {
    await context.clearCookies({name:'oms_iam_csrf'});
    const pattern=origin+iamRoute+'/api/v1/csrf';
    await page.route(pattern,route=>route.fulfill({status:503,contentType:'application/json',body:'{"error":{"code":"fixture_unavailable"}}'}));
    await page.locator('form[action="/gateway/login"] button[type="submit"]').click();
    await page.locator('[data-login-error]').waitFor({state:'visible'});
    assert.match(await page.locator('[data-login-error]').textContent(),/安全令牌.*输入已保留/);
    assert.equal(await page.locator('input[name="password"]').inputValue(),privateCredentials.BOOTSTRAP_ADMIN_PASSWORD);
    assert.equal(traffic.filter(r=>r.path==='/gateway/login').length,0);
    assert.equal(page.url(),entry);
    await page.screenshot({path:output+'/renewal-unavailable.png'});
    await page.unroute(pattern);
    receipt.renewal_outage={injected:true,inputs_preserved:true,login_posts_before_manual_retry:0};
  });
  await scenario('race-rejection-manual-retry',null,async({page,traffic})=> {
    const pattern=origin+'/gateway/login';
    await page.route(pattern,route=>route.fulfill({status:403,contentType:'application/json',body:'{"error":{"code":"csrf_failed"}}'}));
    await page.locator('form[action="/gateway/login"] button[type="submit"]').click();
    await page.locator('[data-login-error]').waitFor({state:'visible'});
    assert.match(await page.locator('[data-login-error]').textContent(),/再次点击登录.*输入已保留/);
    assert.equal(await page.locator('input[name="password"]').inputValue(),privateCredentials.BOOTSTRAP_ADMIN_PASSWORD);
    assert.equal(traffic.filter(r=>r.path==='/gateway/login').length,1);
    assert.equal(page.url(),entry);
    await page.screenshot({path:output+'/race-rejected.png'});
    await page.unroute(pattern);
    receipt.race_rejection={injected:true,inputs_preserved:true,automatic_retry:false};
  });
  assert.equal(receipt.scenarios.find(s=>s.name==='fresh').login_posts,1);
  assert.equal(receipt.scenarios.find(s=>s.name==='expired-cookie').csrf_gets,1);
  assert.equal(receipt.scenarios.find(s=>s.name==='other-tab-rotation').login_posts,1);
  }
  assert.equal(receipt.business_writes.length,0); assert.equal(receipt.page_errors.length,0);
  receipt.passed=true;
  delete receipt.current_stage;
} catch(error) {
  // Assertion failures may contain inputs; never export stack or values.
  receipt.failure={type:error.name,completed_scenarios:receipt.scenarios.length};
  process.exitCode=1;
} finally {
  await browser.close();
  writeFileSync(output+'/browser.json',JSON.stringify(receipt,null,2)+'\n');
  console.log(JSON.stringify({passed:receipt.passed,scenarios:receipt.scenarios.length,page_errors:receipt.page_errors.length,business_writes:receipt.business_writes.length}));
}
