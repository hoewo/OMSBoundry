// Public, read-only acceptance for instant tab return and stale-while-refresh UI.
import assert from 'node:assert/strict'
import {spawnSync} from 'node:child_process'
import {mkdirSync,writeFileSync} from 'node:fs'
import {setTimeout as delay} from 'node:timers/promises'
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs'

const output=process.argv[2]
assert(output&&/^\/tmp\/oa-page-read-browser\.[A-Za-z0-9]+$/.test(output))
mkdirSync(output,{recursive:true})
const origin='https://oatest.antuinfo.cn',project='3b4c44d3-7b33-4a29-add3-3f6211ca893e'
const inspected=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',"sudo python3 -c 'import json,subprocess;v=json.loads(subprocess.check_output([\"docker\",\"inspect\",\"omsbizfoundry-identity-access-1\"]))[0];e=dict(x.split(\"=\",1) for x in v[\"Config\"][\"Env\"]);print(json.dumps({k:e[k] for k in (\"BOOTSTRAP_ADMIN_USERNAME\",\"BOOTSTRAP_ADMIN_PASSWORD\")}))'"],{encoding:'utf8',timeout:25000})
assert.equal(inspected.status,0,'private login credential inspection unavailable')
const secret=JSON.parse(inspected.stdout)
const result={passed:false,project,stage:'login',requests:[],rate_limited:[],business_writes:[],page_errors:[],checks:{}}
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true})
const context=await browser.newContext({viewport:{width:1700,height:1050}}),page=await context.newPage()
page.on('pageerror',error=>result.page_errors.push(error.message))
page.on('request',request=>{const url=new URL(request.url());if(url.origin!==origin)return;const item={stage:result.stage,method:request.method(),path:url.pathname};result.requests.push(item);if(['POST','PUT','PATCH','DELETE'].includes(request.method())&&!url.pathname.startsWith('/gateway/')&&!url.pathname.startsWith('/auth/'))result.business_writes.push(item)})
page.on('response',response=>{const url=new URL(response.url());if(url.origin===origin&&response.status()===429)result.rate_limited.push({stage:result.stage,path:url.pathname})})
const stageRequests=name=>result.requests.filter(item=>item.stage===name)
const waitPayment=async()=>{const pane=page.locator('[data-real-cash-react]');await pane.waitFor({timeout:30000});await pane.getByRole('button',{name:'刷新当前支付',exact:true}).waitFor({timeout:30000});await pane.locator('.workspace-read-skeleton').waitFor({state:'hidden',timeout:30000}).catch(()=>{});assert.equal(await pane.locator('.ant-alert-error').count(),0);return pane}
const waitReader=async area=>{const status=page.locator(`[data-real-${area}-status]`);await status.waitFor({timeout:30000});await page.waitForFunction(selector=>{const node=document.querySelector(selector);return node&&!node.textContent.startsWith('正在读取')},`[data-real-${area}-status]`,{timeout:30000});await page.locator(`[data-real-${area}-content] .workspace-read-skeleton`).waitFor({state:'hidden',timeout:30000}).catch(()=>{})}

try{
  await page.goto(origin+'/workbench/',{waitUntil:'domcontentloaded'})
  await page.locator('[data-login-mode="iam"]').click()
  await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME)
  await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD)
  await page.locator('form[action="/gateway/login"] button[type="submit"]').click()
  await page.locator('[data-real-project-list] [data-real-detail-open]').first().waitFor({timeout:30000})

  result.stage='initial-payments'
  await page.goto(origin+`/workbench/?project_id=${project}&tab=business&business_tab=payments`,{waitUntil:'domcontentloaded'})
  const payment=await waitPayment(),paymentText=(await payment.innerText()).slice(0,2500)

  result.stage='initial-overview'
  await page.locator('[data-real-business-tab="overview"]').click()
  await waitReader('business')

  result.stage='cached-payment-return'
  await page.locator('[data-real-business-tab="payments"]').click()
  await delay(450)
  const paymentAfterReturn=await waitPayment()
  result.checks.cached_payment_return_requests=stageRequests('cached-payment-return')
  result.checks.cached_payment_visible=(await paymentAfterReturn.innerText()).slice(0,2500)
  assert.equal(result.checks.cached_payment_return_requests.length,0,'fresh payment return still issued network requests')
  assert.equal(await paymentAfterReturn.locator('.workspace-read-skeleton').count(),0,'fresh payment return painted a blocking skeleton')

  result.stage='explicit-payment-refresh'
  await paymentAfterReturn.getByRole('button',{name:'刷新当前支付',exact:true}).click()
  assert.equal(await paymentAfterReturn.locator('.workspace-read-skeleton').count(),0,'explicit refresh cleared visible payment content')
  assert((await paymentAfterReturn.innerText()).includes('刷新当前支付'),'payment controls disappeared during refresh')
  await paymentAfterReturn.locator('.workspace-read-status .ant-spin-spinning').waitFor({state:'hidden',timeout:30000}).catch(()=>{})
  await delay(450)
  result.checks.explicit_payment_refresh_requests=stageRequests('explicit-payment-refresh')
  assert(result.checks.explicit_payment_refresh_requests.length>0,'explicit refresh did not reach native owners')

  result.stage='initial-production-overview'
  await page.locator('[data-real-detail-tab="production"]').click()
  await waitReader('production')

  result.stage='cached-business-primary-return'
  await page.locator('[data-real-detail-tab="business"]').click()
  await delay(450)
  await waitReader('business')
  result.checks.cached_business_primary_return_requests=stageRequests('cached-business-primary-return')
  assert.equal(result.checks.cached_business_primary_return_requests.length,0,'primary tab return invalidated a fresh business projection')

  assert.equal(result.rate_limited.length,0,'public experience returned HTTP 429')
  assert.equal(result.business_writes.length,0,'read-only experience verification sent a business write')
  assert.equal(result.page_errors.length,0,'browser emitted page errors')
  result.checks.initial_payment_text=paymentText
  result.passed=true
  result.stage='complete'
  await page.screenshot({path:output+'/cached-business-return.png',animations:'disabled'})
}catch(error){result.failure={stage:result.stage,type:error.name,message:error.message.slice(0,700)};process.exitCode=1;await page.screenshot({path:output+'/failure.png',animations:'disabled'}).catch(()=>{})}
finally{await browser.close();writeFileSync(output+'/browser.json',JSON.stringify(result,null,2)+'\n',{mode:0o600});console.log(JSON.stringify({passed:result.passed,stage:result.failure?.stage,request_count:result.requests.length,cached_payment_return_requests:result.checks.cached_payment_return_requests?.length,explicit_payment_refresh_requests:result.checks.explicit_payment_refresh_requests?.length,cached_business_primary_return_requests:result.checks.cached_business_primary_return_requests?.length,rate_limited:result.rate_limited.length,business_writes:result.business_writes.length,page_errors:result.page_errors.length,failure:result.failure}))}
