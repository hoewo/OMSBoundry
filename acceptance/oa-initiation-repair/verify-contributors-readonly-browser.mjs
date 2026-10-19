// Read-only diagnosis of the same C fixture after public-d; never POST business.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
import {ContributorAdapter} from '../../frontend/oms-workbench/frontend/src/contributor-adapter.mjs';
const output=process.argv[2];assert(output&&process.env.OA_CONTRIBUTORS_PUBLIC_CHECK==='1');mkdirSync(output,{recursive:true});
const prior=JSON.parse(readFileSync('/tmp/oa-complete-operations.TEfzlpv7/public-d/browser.json'));assert.equal(prior.commands.length,2);
const project=prior.fixture_id,origin='https://oatest.antuinfo.cn';assert.equal(project,'70edead7-9dc9-44a3-9b35-b9d41b83264e');
const code='import json,subprocess;v=json.loads(subprocess.check_output(["docker","inspect","omsbizfoundry-identity-access-1"]))[0];e=dict(x.split("=",1) for x in v["Config"]["Env"]);print(json.dumps({k:e[k] for k in ("BOOTSTRAP_ADMIN_USERNAME","BOOTSTRAP_ADMIN_PASSWORD")}))';
const inspected=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',"sudo python3 -c '"+code+"'"],{encoding:'utf8',timeout:25000});assert.equal(inspected.status,0);const secret=JSON.parse(inspected.stdout);
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true}),context=await browser.newContext({viewport:{width:1450,height:930}}),page=await context.newPage();
const result={passed:false,business_posts:0,fixture_id:project,errors:[],responses:[],full_oa_accepted:false};
page.on('request',r=>{if(r.method()==='POST'&&!/^\/(gateway|auth)\//.test(new URL(r.url()).pathname))result.business_posts++});
page.on('response',async r=>{if(r.status()>=400){const path=new URL(r.url()).pathname;let errorCode;try{const v=await r.json();errorCode=v.error?.code||v.error}catch{}result.responses.push({path,status:r.status(),error_code:typeof errorCode==='string'?errorCode:undefined})}});
try{
 await page.goto(origin+'/workbench/',{waitUntil:'domcontentloaded'});await page.locator('[data-login-mode="iam"]').click();
 await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME);await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD);await page.locator('form[action="/gateway/login"] button[type="submit"]').click();
 await page.locator('[data-real-project-list] [data-real-detail-open]').first().waitFor({timeout:30000});
 const session=await context.request.get(origin+'/auth/api/v1/session');assert.equal(session.status(),200);const identity=(await session.json()).principal;
 const r=await context.request.get(origin+`/project/api/v1/projects/${project}?include_inactive=true`);assert.equal(r.status(),200);const value=await r.json();
 result.native_revision=value.project.revision;result.assignments=value.assignments.map(a=>({id:a.id,role_key:a.role_key,ended:!!a.ended_at}));
 const adapter=new ContributorAdapter({details:()=>value,tenantID:identity.tenant_id,subjectID:identity.subject_id});try{adapter.validate(value);result.adapter_validation=true}catch(e){result.adapter_validation=false;result.errors.push(e.message)}
 const active=await context.request.get(origin+`/project/api/v1/projects/${project}`);const current=await active.json();result.active_assignment_count=Array.isArray(current.assignments)?current.assignments.length:null;try{adapter.validate(current);result.active_validation=true}catch(e){result.active_validation=false;result.errors.push(e.message)}
 await page.goto(origin+`/workbench/?project_id=${project}&tab=business&business_tab=personnel`,{waitUntil:'domcontentloaded'});const panel=page.locator('[data-real-personnel-react]');await panel.locator('button').first().waitFor({timeout:30000});
 await panel.getByText('当前还没有经营参与人',{exact:true}).waitFor({timeout:15000});await page.screenshot({path:output+'/read-only.png'});assert.equal(result.business_posts,0);assert(result.active_validation);result.passed=true;
}catch(e){result.failure={type:e.name};await page.screenshot({path:output+'/failure.png'}).catch(()=>{})}
finally{await browser.close();writeFileSync(output+'/browser.json',JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result));if(!result.passed)process.exitCode=1}
