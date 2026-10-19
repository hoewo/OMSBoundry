// Public, read-only regression for the OA request-amplification repair.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync, writeFileSync} from 'node:fs';
import {setTimeout as delay} from 'node:timers/promises';
import {chromium} from '/Users/crispydog/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';

const output = process.argv[2];
assert(output && /^\/tmp\/oa-rate-limit-browser\.[A-Za-z0-9]+$/.test(output));
mkdirSync(output, {recursive: true});

const origin = 'https://oatest.antuinfo.cn';
const project = '3b4c44d3-7b33-4a29-add3-3f6211ca893e';
const ssh = code => {
  const result = spawnSync(
    'ssh',
    ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', '-o', 'StrictHostKeyChecking=yes', 'ecs-user@47.98.156.166', code],
    {encoding: 'utf8', timeout: 25000},
  );
  assert.equal(result.status, 0, 'private credential inspection unavailable');
  return result.stdout;
};
const secretCode = 'import json,subprocess;v=json.loads(subprocess.check_output(["docker","inspect","omsbizfoundry-identity-access-1"]))[0];e=dict(x.split("=",1) for x in v["Config"]["Env"]);print(json.dumps({k:e[k] for k in ("BOOTSTRAP_ADMIN_USERNAME","BOOTSTRAP_ADMIN_PASSWORD")}))';
const secret = JSON.parse(ssh("sudo python3 -c '" + secretCode + "'"));

const result = {
  passed: false,
  project,
  scenario: 'initial payment read plus five user refreshes without synthetic throttling',
  measuring: false,
  requests: [],
  authorization_posts: [],
  rate_limited: [],
  asset_failures: [],
  business_writes: [],
  page_errors: [],
};

const browser = await chromium.launch({
  executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  headless: true,
});
const context = await browser.newContext({viewport: {width: 1700, height: 1050}});
const page = await context.newPage();

page.on('pageerror', error => result.page_errors.push(error.message));
page.on('request', request => {
  if (!result.measuring) return;
  const url = new URL(request.url());
  if (url.origin !== origin) return;
  const item = {method: request.method(), path: url.pathname};
  result.requests.push(item);
  if (request.method() === 'POST' && url.pathname.startsWith('/gateway/authorize/')) {
    result.authorization_posts.push(item);
  }
  if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(request.method()) &&
      !url.pathname.startsWith('/gateway/') && !url.pathname.startsWith('/auth/')) {
    result.business_writes.push(item);
  }
});
page.on('response', async response => {
  if (!result.measuring) return;
  const url = new URL(response.url());
  if (url.origin !== origin) return;
  if (response.status() === 429) {
    result.rate_limited.push({status: response.status(), path: url.pathname});
  }
  if (url.pathname.startsWith('/workbench/assets/')) {
    const contentType = response.headers()['content-type'] || '';
    if (response.status() !== 200 ||
        (url.pathname.endsWith('.css') && !contentType.startsWith('text/css')) ||
        (url.pathname.endsWith('.js') && !contentType.includes('javascript'))) {
      result.asset_failures.push({status: response.status(), path: url.pathname, content_type: contentType});
    }
  }
});

let stage = 'login';
try {
  await page.goto(origin + '/workbench/', {waitUntil: 'domcontentloaded'});
  await page.locator('[data-login-mode="iam"]').click();
  await page.locator('input[name="username"]').fill(secret.BOOTSTRAP_ADMIN_USERNAME);
  await page.locator('input[name="password"]').fill(secret.BOOTSTRAP_ADMIN_PASSWORD);
  await page.locator('form[action="/gateway/login"] button[type="submit"]').click();
  await page.locator('[data-real-project-list] [data-real-detail-open]').first().waitFor({timeout: 30000});

  stage = 'initial-payment-read';
  result.measuring = true;
  await page.goto(
    origin + `/workbench/?project_id=${project}&tab=business&business_tab=payments`,
    {waitUntil: 'domcontentloaded'},
  );
  const pane = page.locator('[data-real-cash-react]');
  await pane.waitFor({timeout: 30000});
  await pane.locator('.ant-spin-spinning').waitFor({state: 'hidden', timeout: 30000});
  const refresh = pane.getByRole('button', {name: '刷新当前支付', exact: true});
  await refresh.waitFor({timeout: 30000});
  assert.equal(await pane.locator('.ant-alert-error').count(), 0, 'payment panel returned an error alert');

  stage = 'five-explicit-refreshes';
  for (let index = 0; index < 5; index += 1) {
    await refresh.click();
    await pane.locator('.ant-spin-spinning').waitFor({state: 'hidden', timeout: 30000});
    await delay(250);
  }

  result.measuring = false;
  result.total_requests = result.requests.length;
  result.authorization_post_count = result.authorization_posts.length;
  result.request_counts_by_path = Object.fromEntries(
    [...new Set(result.requests.map(item => item.path))]
      .sort()
      .map(path => [path, result.requests.filter(item => item.path === path).length]),
  );
  result.panel_text = (await pane.innerText()).slice(0, 2500);
  assert.equal(result.rate_limited.length, 0, 'public flow returned HTTP 429');
  assert.equal(result.asset_failures.length, 0, 'a Workbench asset failed or had the wrong MIME type');
  assert.equal(result.business_writes.length, 0, 'read-only verification sent a business write');
  assert.equal(result.page_errors.length, 0, 'browser page errors were emitted');
  assert(result.authorization_post_count <= 14, 'authorization amplification remains unexpectedly high');
  result.passed = true;
  await page.screenshot({path: output + '/payments-after-five-refreshes.png', animations: 'disabled'});
} catch (error) {
  result.measuring = false;
  result.failure = {stage, type: error.name, message: error.message.slice(0, 600)};
  process.exitCode = 1;
  await page.screenshot({path: output + '/failure.png', animations: 'disabled'}).catch(() => {});
} finally {
  await browser.close();
  writeFileSync(output + '/browser.json', JSON.stringify(result, null, 2) + '\n', {mode: 0o600});
  console.log(JSON.stringify({
    passed: result.passed,
    stage: result.failure?.stage,
    total_requests: result.total_requests ?? result.requests.length,
    authorization_posts: result.authorization_post_count ?? result.authorization_posts.length,
    rate_limited: result.rate_limited.length,
    asset_failures: result.asset_failures.length,
    business_writes: result.business_writes.length,
    page_errors: result.page_errors.length,
    failure: result.failure,
  }));
}
