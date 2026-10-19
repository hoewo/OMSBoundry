// Read-only continuation of the exact saved C fixture; never reset/recreate it.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const output=process.argv[2];assert(output&&process.env.OA_CONTRIBUTORS_PUBLIC_CHECK==='1');mkdirSync(output,{recursive:true});
const prior=JSON.parse(readFileSync('/tmp/oa-complete-operations.TEfzlpv7/public-j-continuation/browser.json'));assert.equal(prior.commands.length,3);assert.deepEqual(prior.commands[1],prior.commands[2]);assert(prior.scenarios.some(s=>s.name.includes('manual-same-command')&&s.passed));
const project=prior.fixture_id,origin='https://oatest.antuinfo.cn';assert.equal(project,'70edead7-9dc9-44a3-9b35-b9d41b83264e');
const code='import json,subprocess;v=json.loads(subprocess.check_output(["docker","inspect","omsbizfoundry-identity-access-1"]))[0];e=dict(x.split("=",1) for x in v["Config"]["Env"]);print(json.dumps({k:e[k] for k in ("BOOTSTRAP_ADMIN_USERNAME","BOOTSTRAP_ADMIN_PASSWORD")}))';
const inspected=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',"sudo python3 -c '"+code+"'"],{encoding:'utf8',timeout:25000});assert.equal(inspected.status,0);const secret=JSON.parse(inspected.stdout);
const receipt={passed:false,fixture_id:project,native_revision:13,continued_from:'public-j-continuation',business_posts:0,grant_writes:0,scenarios:[],responses:[],page_errors:[],projection_requests:[],request_paths:[],full_oa_accepted:false};
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true}),context=await browser.newContext({viewport:{width:1450,height:930}}),page=await context.newPage(),panel=page.locator('[data-real-personnel-react]');let phase='login';
page.on('request',r=>{const path=new URL(r.url()).pathname;if(receipt.request_paths.length<300)receipt.request_paths.push({phase,method:r.method(),path});if(r.method()==='POST'&&!/^\/(gateway|auth)\//.test(path))receipt.business_posts++;if(/\/(?:work-items|metrics\/operating-summary)$/.test(path))receipt.projection_requests.push({phase,path})});
page.on('response',r=>{if(r.status()>=400)receipt.responses.push({path:new URL(r.url()).pathname,status:r.status(),phase})});page.on('pageerror',e=>receipt.page_errors.push(e.name));
async function native(){const r=await context.request.get(origin+`/project/api/v1/projects/${project}?include_inactive=true`);assert.equal(r.status(),200);const v=await r.json();assert.equal(v.project.id,project);assert.equal(v.project.revision,13);const commercial=v.assignments.filter(a=>a.role_key==='commercial_contributor');assert.equal(commercial.length,3);assert.equal(commercial.filter(a=>!a.ended_at).length,1);return v}
// Use visible application completion, not global Chrome networkidle: k/l/m
// loaded the native directory successfully but never emitted that lifecycle.
async function ready(){await panel.locator('.business-personnel-list__item').first().waitFor({timeout:30000});await panel.locator('button:enabled').filter({hasText:'刷新当前人员'}).waitFor({timeout:30000})}
try{
 await page.goto(origin+'/workbench/',{waitUntil:'domcontentloaded'});await page.locator('[data-login-mode="iam"]').click();await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME);await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD);await page.locator('form[action="/gateway/login"] button[type="submit"]').click();phase='login-directory';await page.locator('[data-view="projects"].is-active [data-real-project-list] .project-list-page__name').first().waitFor({timeout:30000});
 phase='detail';await page.goto(origin+`/workbench/?project_id=${project}&tab=business&business_tab=personnel`,{waitUntil:'domcontentloaded'});await ready();
 const url=await page.locator('script[src*="workbench.js"]').getAttribute('src');const asset=await context.request.get(new URL(url,page.url()).href);assert.equal(asset.status(),200);const bridge=await asset.text();assert(bridge.includes('loadVisibleSources(workRoot,sources,loadWork)')&&bridge.includes('loadVisibleSources(metricRoot,sources,loadMetrics)'));receipt.workbench_bridge_sha256=createHash('sha256').update(bridge).digest('hex');
 const initial=await native();assert.equal(receipt.projection_requests.filter(r=>r.phase==='detail').length,0);receipt.scenarios.push({name:'deep-link-native-history-no-hidden-projection-requests',passed:true});
 phase='refresh';await panel.getByRole('button',{name:'刷新当前人员',exact:true}).click();await ready();await native();
 phase='adjacent';for(const tab of ['bidding','contracts','payments','invoices','costs','bonus','personnel']){await page.locator(`[data-real-business-tab="${tab}"]`).click();assert.equal(new URL(page.url()).searchParams.get('business_tab'),tab)}await ready();await native();
 phase='reload';await page.reload({waitUntil:'domcontentloaded'});await ready();const final=await native();assert.deepEqual(final.assignments,initial.assignments);assert.equal(receipt.projection_requests.length,0);assert.equal(receipt.business_posts,0);assert.equal(receipt.page_errors.length,0);assert.equal(receipt.responses.filter(r=>r.status===429).length,0);await page.screenshot({path:output+'/personnel-final.png'});receipt.scenarios.push({name:'refresh-fast-adjacent-tabs-reload-zero-write-no-429',passed:true});
 // The projections are deferred, not disabled: entering the real statistics
 // view must initiate its guarded read and visibly settle the native source.
 phase='statistics';await page.locator('.app-sidebar [data-view-target="statistics-reference"]').click();await page.locator('#companyMetrics .source-status').first().waitFor();await page.waitForFunction(()=>{const states=[...document.querySelectorAll('#companyMetrics .source-status')];return states.length>0&&states.every(x=>x.textContent!=='正在连接')},null,{timeout:30000});assert(receipt.projection_requests.some(r=>r.phase==='statistics'&&r.path.endsWith('/metrics/operating-summary')));assert.equal(receipt.business_posts,0);assert.equal(receipt.responses.filter(r=>r.status===429).length,0);await page.screenshot({path:output+'/statistics-guarded-read.png'});receipt.scenarios.push({name:'visible-statistics-still-uses-native-authorized-read',passed:true});receipt.passed=true;
}catch(error){receipt.failure={type:error.name,phase};receipt.dom_diagnostic=await page.evaluate(()=>({active_views:[...document.querySelectorAll('[data-view].is-active')].map(x=>x.getAttribute('data-view')),directory_lists:document.querySelectorAll('[data-real-project-list]').length,directory_names:document.querySelectorAll('[data-real-project-list] .project-list-page__name').length,directory_button_boxes:[...document.querySelectorAll('[data-real-project-list] [data-real-detail-open]')].slice(0,5).map(x=>({display:getComputedStyle(x).display,width:x.getBoundingClientRect().width,height:x.getBoundingClientRect().height}))})).catch(()=>null);await page.screenshot({path:output+'/failure.png'}).catch(()=>{})}
finally{
 try{
  const cookies=await context.cookies(),csrf=cookies.find(c=>c.name==='oms_iam_csrf')?.value;
  if(cookies.some(c=>c.name==='oms_suite_session')){
   assert(csrf);const loggedOut=await context.request.post(origin+'/auth/api/v1/logout',{headers:{Origin:origin,'X-CSRF-Token':csrf}});
   assert.equal(loggedOut.status(),204);receipt.test_session_logged_out=true;
  }
 }catch(error){receipt.cleanup_failure={type:error.name};receipt.passed=false}
 await browser.close();writeFileSync(output+'/browser.json',JSON.stringify(receipt,null,2)+'\n');console.log(JSON.stringify({passed:receipt.passed,scenarios:receipt.scenarios.length,business_posts:receipt.business_posts,failure:receipt.failure,cleanup_failure:receipt.cleanup_failure}));if(!receipt.passed)process.exitCode=1
}
