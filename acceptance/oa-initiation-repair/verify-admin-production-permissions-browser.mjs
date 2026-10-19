// Public, read-only browser acceptance for the OA administrator permission chain.
// It proves that the OA rule template, IAM grants and native application
// allowed_actions all meet at the UI without granting through the browser.
import assert from 'node:assert/strict'
import {spawnSync} from 'node:child_process'
import {mkdirSync,writeFileSync} from 'node:fs'
import {setTimeout as delay} from 'node:timers/promises'
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs'

const output=process.argv[2]
assert(output&&/^\/tmp\/oa-admin-production-browser\.[A-Za-z0-9]+$/.test(output))
mkdirSync(output,{recursive:true})

const origin='https://oatest.antuinfo.cn'
const project='3b4c44d3-7b33-4a29-add3-3f6211ca893e'
const inspected=spawnSync('ssh',[
  '-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes',
  'ecs-user@47.98.156.166',
  "sudo python3 -c 'import json,subprocess;v=json.loads(subprocess.check_output([\"docker\",\"inspect\",\"omsbizfoundry-identity-access-1\"]))[0];e=dict(x.split(\"=\",1) for x in v[\"Config\"][\"Env\"]);print(json.dumps({k:e[k] for k in (\"BOOTSTRAP_ADMIN_USERNAME\",\"BOOTSTRAP_ADMIN_PASSWORD\")}))'",
],{encoding:'utf8',timeout:25000})
assert.equal(inspected.status,0,'private login credential inspection unavailable')
const secret=JSON.parse(inspected.stdout)

const result={
  passed:false,project,stage:'login',responses:[],forbidden:[],rate_limited:[],
  server_errors:[],business_writes:[],page_errors:[],tabs:{},checks:{},
}
const browser=await chromium.launch({
  executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  headless:true,
})
const context=await browser.newContext({viewport:{width:1900,height:1100}})
const page=await context.newPage()

page.on('pageerror',error=>result.page_errors.push(error.message))
page.on('request',request=>{
  const url=new URL(request.url())
  if(url.origin!==origin)return
  const item={stage:result.stage,method:request.method(),path:url.pathname}
  if(['POST','PUT','PATCH','DELETE'].includes(request.method())&&!url.pathname.startsWith('/gateway/'))result.business_writes.push(item)
})
page.on('response',response=>{
  const url=new URL(response.url())
  if(url.origin!==origin)return
  const item={stage:result.stage,status:response.status(),path:url.pathname}
  result.responses.push(item)
  if(item.status===403)result.forbidden.push(item)
  if(item.status===429)result.rate_limited.push(item)
  if(item.status>=500)result.server_errors.push(item)
})

const waitSettled=async()=>{
  await page.locator('.workspace-read-skeleton,.ant-skeleton').first().waitFor({state:'hidden',timeout:30000}).catch(()=>{})
  await page.locator('.ant-spin-spinning').first().waitFor({state:'hidden',timeout:30000}).catch(()=>{})
  await delay(350)
}
const tab=label=>page.getByRole('tab',{name:new RegExp(label)})
const clickTab=async(label,expectedText)=>{
  result.stage=`production-${label}`
  const node=tab(label)
  await node.waitFor({timeout:30000})
  assert.notEqual(await node.getAttribute('aria-disabled'),'true',`${label} tab is disabled`)
  await node.click()
  await waitSettled()
  const active=await node.getAttribute('aria-selected')
  assert.equal(active,'true',`${label} tab did not become active`)
  const pane=page.locator('[data-real-production-layout] .ant-tabs-tabpane-active')
  await pane.getByText(expectedText,{exact:false}).first().waitFor({timeout:30000})
  const errors=await pane.locator('.ant-alert-error').allInnerTexts()
  assert.equal(errors.length,0,`${label} rendered an error alert: ${errors.join(' | ')}`)
  result.tabs[label]={active:true,error_alerts:errors}
}

try{
  await page.goto(origin+'/workbench/',{waitUntil:'domcontentloaded'})
  await page.locator('[data-login-mode="iam"]').click()
  await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME)
  await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD)
  await page.locator('form[action="/gateway/login"] button[type="submit"]').click()
  await page.locator('[data-real-project-list] [data-real-detail-open]').first().waitFor({timeout:30000})

  result.stage='open-project-production'
  await page.goto(origin+`/workbench/?project_id=${project}&tab=production`,{waitUntil:'domcontentloaded'})
  await page.locator('[data-real-detail-tab="production"]').waitFor({timeout:30000})
  await page.locator('[data-real-production-layout] [role="tab"]').first().waitFor({timeout:30000})
  await waitSettled()

  const visibleTabs=await page.locator('[data-real-production-layout] [role="tab"]').evaluateAll(nodes=>nodes.map(node=>({
    name:(node.textContent||'').trim(),disabled:node.getAttribute('aria-disabled')==='true',
  })))
  result.checks.visible_tabs=visibleTabs
  for(const required of ['人员配置','批复审计','方案阶段','初步设计','施工图设计','变更竣工','生产成本管理','对外委托','奖金分配']){
    const found=visibleTabs.find(item=>item.name.includes(required))
    assert(found,`${required} tab is missing`)
    assert.equal(found.disabled,false,`${required} tab is disabled`)
  }

  await clickTab('人员配置',/生产人员尚未初始化|生产人员配置/)
  if(await page.getByText('生产人员尚未初始化，不是无权限',{exact:true}).count()){
    const enroll=page.getByRole('button',{name:'初始化生产人员',exact:true})
    await enroll.waitFor()
    assert.equal(await enroll.isDisabled(),false,'OA administrator cannot initialize production personnel')
    result.tabs['人员配置'].buttons=[{name:'初始化生产人员',disabled:false}]
  }
  await clickTab('批复审计','批复审计信息')
  const approvalButtons=await page.locator('.ant-tabs-tabpane-active button').evaluateAll(nodes=>nodes.map(node=>({name:(node.textContent||'').trim(),disabled:node.disabled})).filter(item=>item.name))
  result.tabs['批复审计'].buttons=approvalButtons
  assert(approvalButtons.some(item=>item.name.replace(/\s+/g,'')==='刷新'&&!item.disabled),'approval refresh action is unavailable')

  await clickTab('方案阶段','方案阶段')
  await clickTab('初步设计','初步设计')
  result.tabs['初步设计'].connection_state=await page.locator('[data-oa-stage-surface="preliminary"]').getByText('原生端口待接通',{exact:false}).first().innerText()
  await page.screenshot({path:output+'/preliminary-stage.png',fullPage:true,animations:'disabled'})
  await clickTab('施工图设计','施工图设计')
  result.tabs['施工图设计'].connection_state=await page.locator('[data-oa-stage-surface="construction"]').getByText('原生端口待接通',{exact:false}).first().innerText()
  await page.screenshot({path:output+'/construction-stage.png',fullPage:true,animations:'disabled'})
  await clickTab('变更竣工','变更洽商')
  await page.locator('[data-oa-stage-surface="change-completion"]').getByText('竣工验收',{exact:true}).waitFor({timeout:10000})
  result.tabs['变更竣工'].dual_sections=['变更洽商','竣工验收']
  await page.screenshot({path:output+'/change-completion-stage.png',fullPage:true,animations:'disabled'})
  await clickTab('生产成本管理','刷新当前成本')

  await clickTab('对外委托','对外委托')
  const commissionButtons=await page.locator('.ant-tabs-tabpane-active button').evaluateAll(nodes=>nodes.map(node=>({name:(node.textContent||'').trim(),disabled:node.disabled})).filter(item=>item.name))
  result.tabs['对外委托'].buttons=commissionButtons
  const createCommission=commissionButtons.find(item=>item.name.replace(/\s+/g,'').includes('新增对外委托'))
  assert(createCommission,'commission create action is missing')
  assert.equal(createCommission.disabled,false,'commission create action is disabled despite OA administrator grants')
  await page.getByRole('button',{name:/新增对外委托/}).click()
  const commissionDrawer=page.locator('.ant-drawer-content').filter({hasText:'新增对外委托'})
  await commissionDrawer.waitFor({timeout:10000})
  result.tabs['对外委托'].create_form_opened=true
  await commissionDrawer.locator('.ant-drawer-close').click()
  await commissionDrawer.waitFor({state:'hidden',timeout:10000})

  await clickTab('奖金分配','刷新当前奖金')
  const bonusButtons=await page.locator('.ant-tabs-tabpane-active button').evaluateAll(nodes=>nodes.map(node=>({name:(node.textContent||'').trim(),disabled:node.disabled})).filter(item=>item.name))
  result.tabs['奖金分配'].buttons=bonusButtons
  assert(bonusButtons.some(item=>item.name.replace(/\s+/g,'')==='刷新当前奖金'&&!item.disabled),'bonus read action is unavailable')
  const allocate=page.locator('.ant-tabs-tabpane-active button').filter({hasText:/分\s*配\s*奖\s*金/})
  const candidatesResponse=page.waitForResponse(response=>response.url().includes('/bonus/api/v1/reference-oa/bonus-allocations/candidates'))
  await allocate.click()
  const candidates=await candidatesResponse
  result.tabs['奖金分配'].candidates={status:candidates.status(),body:(await candidates.text()).slice(0,1200)}
  assert.equal(result.tabs['奖金分配'].candidates.status,200,`奖金候选读取未成功: ${result.tabs['奖金分配'].candidates.status} ${result.tabs['奖金分配'].candidates.body}`)
  const bonusDrawer=page.locator('.ant-drawer-content').filter({hasText:'分配生产奖金'})
  await bonusDrawer.waitFor({timeout:10000})
  result.tabs['奖金分配'].create_form_opened=true
  await bonusDrawer.locator('.ant-drawer-close').click()
  await bonusDrawer.waitFor({state:'hidden',timeout:10000})

  assert.equal(result.forbidden.length,0,'public administrator flow returned HTTP 403')
  assert.equal(result.rate_limited.length,0,'public administrator flow returned HTTP 429')
  assert.equal(result.server_errors.length,0,'public administrator flow returned HTTP 5xx')
  assert.equal(result.business_writes.length,0,'read-only administrator verification sent a business write')
  assert.equal(result.page_errors.length,0,'browser emitted page errors')
  result.passed=true
  result.stage='complete'
  await page.screenshot({path:output+'/production-admin-complete.png',fullPage:true,animations:'disabled'})
}catch(error){
  result.failure={stage:result.stage,type:error.name,message:error.message.slice(0,900)}
  process.exitCode=1
  await page.screenshot({path:output+'/failure.png',fullPage:true,animations:'disabled'}).catch(()=>{})
}finally{
  await browser.close()
  writeFileSync(output+'/browser.json',JSON.stringify(result,null,2)+'\n',{mode:0o600})
  console.log(JSON.stringify({
    passed:result.passed,stage:result.failure?.stage,tabs:result.checks.visible_tabs,
    forbidden:result.forbidden.length,rate_limited:result.rate_limited.length,
    server_errors:result.server_errors.length,business_writes:result.business_writes.length,
    page_errors:result.page_errors.length,failure:result.failure,
  }))
}
