// Real Nginx/Gateway/IAM and the continued C project. No grants or user101 writes.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,writeFileSync,readFileSync} from 'node:fs';
import {setTimeout as delay} from 'node:timers/promises';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const [output,mode='inspect']=process.argv.slice(2);
assert(/^\/tmp\/oa-commercial-browser\.[A-Za-z0-9]+$/.test(output)&&['before','inspect'].includes(mode));
mkdirSync(output,{recursive:true});
const origin='https://oatest.antuinfo.cn',project='70edead7-9dc9-44a3-9b35-b9d41b83264e';
const ssh=code=>{const r=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',code],{encoding:'utf8',timeout:25000});assert.equal(r.status,0,'private inspection unavailable');return r.stdout};
const secretCode='import json,subprocess;v=json.loads(subprocess.check_output(["docker","inspect","omsbizfoundry-identity-access-1"]))[0];e=dict(x.split("=",1) for x in v["Config"]["Env"]);print(json.dumps({k:e[k] for k in ("BOOTSTRAP_ADMIN_USERNAME","BOOTSTRAP_ADMIN_PASSWORD")}))';
const secret=JSON.parse(ssh("sudo python3 -c '"+secretCode+"'"));
const result={passed:false,mode,fixture_id:project,public_real_owners:true,grant_writes:0,business_commands:[],page_errors:[],panels:[]};
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
const context=await browser.newContext({viewport:{width:1500,height:1000}}),page=await context.newPage();
// Keep this automated driver below the existing 120/minute public limit. No
// response, identity or authorization is replaced; every request reaches D1.
let nextRequest=0;
await page.route('**/*',async route=>{const at=Math.max(Date.now(),nextRequest);nextRequest=at+800;await delay(Math.max(0,at-Date.now()));await route.continue();});
page.on('pageerror',e=>result.page_errors.push(e.message));
page.on('request',r=>{const path=new URL(r.url()).pathname;if(['POST','PUT','DELETE','PATCH'].includes(r.method())&&!path.startsWith('/gateway/')&&!path.startsWith('/auth/'))result.business_commands.push({method:r.method(),path});});
let stage='login';
try{
  await page.goto(origin+'/workbench/',{waitUntil:'domcontentloaded'});
  await page.locator('[data-login-mode="iam"]').click();
  await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME);
  await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD);
  await page.locator('form[action="/gateway/login"] button[type="submit"]').click();
  await page.locator('[data-real-project-list] [data-real-detail-open]').first().waitFor({timeout:30000});
  const revision=async()=>{const r=await context.request.get(origin+'/project/api/v1/projects/'+project);assert.equal(r.status(),200);const v=await r.json();assert.equal(v.project.id,project);return v};
  const original=await revision();result.project_revision_before=original.project.revision;
  result.existing_resource_links=original.resource_links.filter(l=>!l.unlinked_at).map(l=>({target_service:l.target_service,resource_type:l.resource_type,resource_id:l.resource_id,relation_key:l.relation_key,source_revision:l.source_revision}));
  for(const [name,marker] of [['contracts','contract'],['payments','cash'],['invoices','invoice']]){
    stage=name;
    if(name==='contracts')await page.goto(origin+`/workbench/?project_id=${project}&tab=business&business_tab=${name}`,{waitUntil:'domcontentloaded'});
    else await page.locator(`[data-real-business-tab="${name}"]`).click();
    const pane=page.locator(`[data-real-${marker}-react]`);
    if(mode==='before'){
      await page.locator('[data-real-business-title]').waitFor();
      result.panels.push({tab:name,original_component_mounts:await pane.count(),text:(await page.locator('[data-real-business-reader]').innerText()).slice(0,1800)});
    }else{
      await pane.waitFor({timeout:30000});
      await pane.locator('.ant-spin-spinning').waitFor({state:'hidden',timeout:30000});
      await pane.locator('button').first().waitFor();
      const text=await pane.innerText();
      result.panels.push({tab:name,original_component_mounts:await pane.count(),text:text.slice(0,3500),alerts:await pane.locator('.ant-alert-message').allTextContents()});
      assert.equal(await pane.locator('.ant-alert-error').count(),0,'native component has an error alert: '+name);
      assert(!/未完整回读|当前项目组件尚未|读取未完成|读取失败|request failed|未获.*权限|没有明确两链|没有明确.*分类/.test(text),'native component not usable: '+name);
      await pane.getByRole('button',{name:name==='contracts'?'刷新当前合同':name==='payments'?'刷新当前支付':'刷新当前开票',exact:true}).click();
      await pane.locator('.ant-spin-spinning').waitFor({state:'hidden',timeout:30000});
      await pane.locator('button').first().waitFor({timeout:30000});
    }
    await page.screenshot({path:output+'/'+name+'.png',animations:'disabled'});
  }
  result.project_revision_after=(await revision()).project.revision;
  assert.equal(result.project_revision_after,result.project_revision_before);
  assert.equal(result.business_commands.length,0);assert.equal(result.page_errors.length,0);
  result.passed=true;
}catch(e){result.failure={stage,type:e.name,message:e.message.slice(0,300)};process.exitCode=1;await page.screenshot({path:output+'/failure.png'}).catch(()=>{});writeFileSync(output+'/failure-ui-private.txt',await page.locator('body').innerText().catch(()=>''),{mode:0o600});}
finally{await browser.close();writeFileSync(output+'/browser.json',JSON.stringify(result,null,2)+'\n',{mode:0o600});console.log(JSON.stringify({passed:result.passed,mode,panels:result.panels.length,business_commands:result.business_commands.length,page_errors:result.page_errors.length,failure:result.failure}));}
