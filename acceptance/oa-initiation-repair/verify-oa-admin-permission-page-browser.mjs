// Public read-only proof that the OA administrator rule template is complete.
import assert from 'node:assert/strict'
import {spawnSync} from 'node:child_process'
import {mkdirSync,writeFileSync} from 'node:fs'
import {setTimeout as delay} from 'node:timers/promises'
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs'

const output=process.argv[2]
assert(output&&/^\/tmp\/oa-admin-permission-page\.[A-Za-z0-9]+$/.test(output))
mkdirSync(output,{recursive:true})
const origin='https://oatest.antuinfo.cn'
const inspected=spawnSync('ssh',[
  '-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes',
  'ecs-user@47.98.156.166',
  "sudo python3 -c 'import json,subprocess;v=json.loads(subprocess.check_output([\"docker\",\"inspect\",\"omsbizfoundry-identity-access-1\"]))[0];e=dict(x.split(\"=\",1) for x in v[\"Config\"][\"Env\"]);print(json.dumps({k:e[k] for k in (\"BOOTSTRAP_ADMIN_USERNAME\",\"BOOTSTRAP_ADMIN_PASSWORD\")}))'",
],{encoding:'utf8',timeout:25000})
assert.equal(inspected.status,0)
const secret=JSON.parse(inspected.stdout)
const result={passed:false,forbidden:[],rate_limited:[],server_errors:[],page_errors:[]}
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true})
const context=await browser.newContext({viewport:{width:1900,height:1100}})
const page=await context.newPage()
page.on('pageerror',error=>result.page_errors.push(error.message))
page.on('response',response=>{
  const url=new URL(response.url())
  if(url.origin!==origin)return
  const item={status:response.status(),path:url.pathname}
  if(item.status===403)result.forbidden.push(item)
  if(item.status===429)result.rate_limited.push(item)
  if(item.status>=500)result.server_errors.push(item)
})
const openView=async name=>{
  for(let attempt=0;attempt<3;attempt++){
    await page.locator(`[data-view-target="${name}"]:visible`).click({force:true})
    await delay(800)
    if(await page.locator(`[data-view="${name}"].is-active`).count())return
  }
  throw new Error(`${name} view did not become active`)
}
try{
  await page.goto(origin+'/workbench/',{waitUntil:'domcontentloaded'})
  await page.locator('[data-login-mode="iam"]').click()
  await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME)
  await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD)
  await page.locator('form[action="/gateway/login"] button[type="submit"]').click()
  await page.locator('[data-view-target="settings"]:visible').waitFor({timeout:30000})
  await delay(8000)
  await openView('settings')
  await openView('permissions')
  const workspace=page.locator('[data-permission-workspace]')
  await workspace.waitFor({timeout:30000})
  const count=Number(await workspace.locator('[data-permission-enabled-count]').textContent())
  const items=workspace.locator('[data-permission-item]')
  const groups=workspace.locator('[data-permission-group]')
  result.enabled_count=count
  result.item_count=await items.count()
  result.checked_count=await items.evaluateAll(nodes=>nodes.filter(node=>node.checked).length)
  result.disabled_count=await items.evaluateAll(nodes=>nodes.filter(node=>node.disabled).length)
  result.group_count=await groups.count()
  result.group_counts=await groups.evaluateAll(nodes=>nodes.map(node=>({
    key:node.dataset.permissionGroup,
    count:node.querySelector('[data-permission-group-count]')?.textContent,
  })))
  result.layer_copy=await workspace.getByText('四层分离',{exact:true}).count()
  result.admin_copy=await workspace.getByText('OA 管理员拥有 OA 规则目录全部权限',{exact:false}).count()
  assert.equal(result.enabled_count,49)
  assert.equal(result.item_count,49)
  assert.equal(result.checked_count,49)
  assert.equal(result.disabled_count,49)
  assert.equal(result.group_count,7)
  assert.equal(result.layer_copy,1)
  assert.equal(result.admin_copy,1)
  assert.equal(result.forbidden.length,0)
  assert.equal(result.rate_limited.length,0)
  assert.equal(result.server_errors.length,0)
  assert.equal(result.page_errors.length,0)
  result.passed=true
  await page.screenshot({path:output+'/oa-admin-all-49.png',fullPage:true,animations:'disabled'})
}catch(error){
  result.failure={type:error.name,message:error.message.slice(0,900)}
  process.exitCode=1
  await page.screenshot({path:output+'/failure.png',fullPage:true,animations:'disabled'}).catch(()=>{})
}finally{
  await browser.close()
  writeFileSync(output+'/browser.json',JSON.stringify(result,null,2)+'\n',{mode:0o600})
  console.log(JSON.stringify(result))
}
