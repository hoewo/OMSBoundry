#!/usr/bin/env node

import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { readFileSync } from 'node:fs';

const playwrightModule = process.env.PLAYWRIGHT_MODULE_PATH || 'playwright';
let chromium;
try {
  ({ chromium } = await import(playwrightModule));
} catch (error) {
  console.error('Playwright is required. Set PLAYWRIGHT_MODULE_PATH to its index.mjs or install playwright.');
  process.exit(2);
}

const feedbackSource = readFileSync(
  new URL('../../frontend/oms-workbench/internal/server/oms_phase1_feedback.js', import.meta.url),
  'utf8',
);

function listen(server) {
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      assert(address && typeof address === 'object');
      resolve(`http://127.0.0.1:${address.port}`);
    });
  });
}

function close(server) {
  return new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
}

function send(response, status, type, body) {
  response.writeHead(status, {
    'Content-Type': type,
    'Cache-Control': 'no-store',
    'X-Content-Type-Options': 'nosniff',
  });
  response.end(body);
}

function sdkDocument() {
  return `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>Feedback fixture</title></head>
<body data-config-count="0" data-open-mode="" data-last-project="" data-last-user="" data-parent-origin="" data-host-module="" data-host-route="">
<button id="sdk-control" type="button">SDK fixture alive</button>
<script>
(() => {
  const params = new URLSearchParams(location.search);
  const parentOrigin = params.get('parentOrigin');
  const state = document.body.dataset;
  function send(type, extra) {
    parent.postMessage(Object.assign({source: 'feedback-sdk-web', type}, extra || {}), parentOrigin);
  }
  addEventListener('message', event => {
    if (event.origin !== parentOrigin || event.source !== parent) return;
    const message = event.data;
    if (!message || message.source !== 'feedback-sdk-host') return;
    if (message.type === 'feedback-sdk:configure') {
      state.configCount = String(Number(state.configCount || 0) + 1);
      state.lastProject = String(message.config && message.config.projectId || '');
      state.lastUser = String(message.config && message.config.customUserId || '');
      state.parentOrigin = String(message.config && message.config.parentOrigins && message.config.parentOrigins[0] || '');
      state.hostModule = String(message.config && message.config.hostContext && message.config.hostContext.module || '');
      state.hostRoute = String(message.config && message.config.hostContext && message.config.hostContext.route || '');
      return;
    }
    if (message.type === 'feedback-sdk:get-unread-count') {
      send('feedback-sdk:unread-count', {request_id: message.request_id, unread_count: 3});
      return;
    }
    if (message.type === 'feedback-sdk:open') state.openMode = String(message.mode || '');
  });
  window.fixtureSend = send;
  send('feedback-sdk:ready');
})();
</script></body></html>`;
}

function hostDocument(sdkURL) {
  return `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>OMS Feedback host fixture</title>
<style>[hidden]{display:none!important}.oms-v1__feedback-layer{position:fixed;inset:0}.oms-v1__feedback-frame{width:100%;height:100%;border:0}.oms-v1__feedback-toast{position:fixed;right:8px;bottom:8px}</style></head>
<body>
<main data-oms-v1 data-feedback-sdk-url="${sdkURL}" data-tenant-id="tenant-fixture" data-subject-id="subject-fixture">
  <section class="is-active" data-oms-v1-page="internal"><button id="business-button" type="button">业务页面</button><output id="business-state">0</output></section>
</main>
<div data-feedback-entry hidden>
  <button type="button" data-feedback-toggle aria-expanded="false">产品反馈 <b data-feedback-unread hidden></b></button>
  <div data-feedback-menu hidden>
    <button type="button" data-feedback-mode="submit">提交反馈</button>
    <button type="button" data-feedback-mode="status">我的反馈</button>
  </div>
</div>
<button type="button" data-feedback-mobile-toggle hidden aria-expanded="false">移动端反馈 <b data-feedback-unread hidden></b></button>
<div data-feedback-mobile-menu hidden>
  <button type="button" data-feedback-mode="submit">提交反馈</button>
  <button type="button" data-feedback-mode="status">我的反馈</button>
</div>
<form action="/gateway/logout" method="post"><button type="submit">退出</button></form>
<script>document.querySelector('#business-button').addEventListener('click',()=>{const state=document.querySelector('#business-state');state.value=String(Number(state.value||0)+1)})</script>
<script src="/assets/oms-phase1-feedback.js"></script>
</body></html>`;
}

let configMode = 'ok';
let configHits = 0;
let hostOrigin = '';
let sdkOrigin = '';
const fakeKey = 'ak_fixture_0123456789abcdef0123456789abcdef';
const fakeUserID = `oms1_v1_${'A'.repeat(43)}`;

const sdkServer = createServer((request, response) => {
  const url = new URL(request.url || '/', sdkOrigin || 'http://127.0.0.1');
  if (url.pathname !== '/embed') return send(response, 404, 'text/plain; charset=utf-8', 'not found');
  if (url.searchParams.get('parentOrigin') !== hostOrigin) return send(response, 403, 'text/plain; charset=utf-8', 'parent denied');
  send(response, 200, 'text/html; charset=utf-8', sdkDocument());
});

const hostServer = createServer((request, response) => {
  const url = new URL(request.url || '/', hostOrigin || 'http://127.0.0.1');
  if (url.pathname === '/') {
    const sdkURL = `${sdkOrigin}/embed?parentOrigin=${encodeURIComponent(hostOrigin)}`;
    return send(response, 200, 'text/html; charset=utf-8', hostDocument(sdkURL));
  }
  if (url.pathname === '/assets/oms-phase1-feedback.js') {
    return send(response, 200, 'text/javascript; charset=utf-8', feedbackSource);
  }
  if (url.pathname === '/gateway/feedback/config') {
    configHits += 1;
    if (request.method !== 'POST') return send(response, 405, 'application/json', '{"error":"method"}');
    if (!/(?:^|;\s*)oms_iam_csrf=fixture_csrf_token_123456(?:;|$)/.test(request.headers.cookie || '') || request.headers['x-csrf-token'] !== 'fixture_csrf_token_123456') {
      return send(response, 403, 'application/json', '{"error":"csrf"}');
    }
    if (configMode === 'forbidden') return send(response, 403, 'application/json', '{"error":"forbidden"}');
    if (configMode === 'unavailable') return send(response, 503, 'application/json', '{"error":"unavailable"}');
    return send(response, 200, 'application/json', JSON.stringify({
      config: {
        feedbackV2Enabled: true,
        feedbackV2NotificationsEnabled: true,
        feedbackV2AuthMode: 'apiKey',
        apiKey: fakeKey,
        projectId: 115,
        customUserId: fakeUserID,
        gatewayUrl: sdkOrigin,
        theme: 'system',
        locale: 'zh-CN',
      },
    }));
  }
  send(response, 404, 'text/plain; charset=utf-8', 'not found');
});

const result = {
  passed: false,
  scenarios: [],
  real_browser: true,
  separate_origins: false,
  external_project_115_used: false,
  secrets_recorded: false,
};
let browser;
try {
  sdkOrigin = await listen(sdkServer);
  hostOrigin = await listen(hostServer);
  result.separate_origins = sdkOrigin !== hostOrigin;
  assert.equal(result.separate_origins, true);

  const launchOptions = { headless: true };
  if (process.env.BROWSER_EXECUTABLE) launchOptions.executablePath = process.env.BROWSER_EXECUTABLE;
  browser = await chromium.launch(launchOptions);

  async function newPage(mode) {
    configMode = mode;
    configHits = 0;
    const context = await browser.newContext();
    await context.addCookies([{ name: 'oms_iam_csrf', value: 'fixture_csrf_token_123456', url: hostOrigin, sameSite: 'Lax' }]);
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.name));
    await page.goto(hostOrigin, { waitUntil: 'domcontentloaded' });
    return { context, page, errors };
  }

  {
    const { context, page, errors } = await newPage('ok');
    try {
      await page.locator('[data-feedback-entry]').waitFor({ state: 'visible', timeout: 5000 });
      await page.waitForFunction(() => {
        const badge = document.querySelector('[data-feedback-unread]');
        return badge && !badge.hidden && badge.textContent === '3';
      });
      assert.equal(configHits, 1, 'idle preparation should coalesce runtime configuration');
      assert.equal(await page.locator('[data-feedback-unread]').first().textContent(), '3');

      await page.locator('[data-feedback-toggle]').click();
      await page.locator('[data-feedback-mode="submit"]').first().click();
      const layer = page.locator('[data-feedback-layer]');
      await layer.waitFor({ state: 'visible' });
      const frame = page.frames().find(candidate => candidate.url().startsWith(`${sdkOrigin}/embed`));
      assert(frame, 'the cross-origin SDK frame must exist');
      assert.notEqual(new URL(frame.url()).origin, new URL(page.url()).origin);
      await frame.waitForFunction(() => document.body.dataset.openMode === 'submit');
      assert.equal(await frame.locator('body').getAttribute('data-last-project'), '115');
      assert.equal(await frame.locator('body').getAttribute('data-last-user'), fakeUserID);
      assert.equal(await frame.locator('body').getAttribute('data-parent-origin'), hostOrigin);
      assert.equal(await frame.locator('body').getAttribute('data-host-module'), 'workbench');
      assert.equal(await frame.locator('body').getAttribute('data-host-route'), 'internal');
      assert.deepEqual(await page.evaluate(() => [localStorage.length, sessionStorage.length]), [0, 0]);

      await page.evaluate(() => window.postMessage({ source: 'feedback-sdk-web', type: 'feedback-sdk:unread-changed', unread_count: 99 }, location.origin));
      await page.waitForTimeout(80);
      assert.equal(await page.locator('[data-feedback-unread]').first().textContent(), '3', 'same-page spoof must be ignored');

      await frame.evaluate(() => window.fixtureSend('feedback-sdk:unread-changed', { unread_count: 7 }));
      await page.waitForFunction(() => document.querySelector('[data-feedback-unread]').textContent === '7');
      await frame.evaluate(() => window.fixtureSend('feedback-sdk:unread-count', { request_id: 'stale-fixture-request', unread_count: 88 }));
      await page.waitForTimeout(80);
      assert.equal(await page.locator('[data-feedback-unread]').first().textContent(), '7', 'stale unread reply must be ignored');
      await frame.evaluate(() => window.fixtureSend('feedback-sdk:close'));
      await page.waitForFunction(() => document.querySelector('[data-feedback-layer]').hidden === true);

      const beforeRefresh = configHits;
      await frame.evaluate(() => window.fixtureSend('feedback-sdk:refresh-session'));
      for (let attempt = 0; attempt < 50 && configHits <= beforeRefresh; attempt += 1) await page.waitForTimeout(20);
      assert.equal(configHits, beforeRefresh + 1, 'refresh must fetch one new protected configuration');

      await page.locator('[data-feedback-toggle]').click();
      await page.locator('[data-feedback-mode="status"]').first().click();
      await frame.waitForFunction(() => document.body.dataset.openMode === 'status');

      await page.evaluate(() => {
        const form = document.querySelector('form[action="/gateway/logout"]');
        form.addEventListener('submit', event => event.preventDefault(), { once: true });
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
      });
      await page.waitForFunction(() => !document.querySelector('[data-feedback-layer]') && document.querySelector('[data-feedback-entry]').hidden === true);
      assert.equal(errors.length, 0);
      result.scenarios.push({ name: 'cross-origin-bridge', passed: true, assertions: 19 });
    } finally {
      await context.close();
    }
  }

  {
    const { context, page, errors } = await newPage('forbidden');
    try {
      for (let attempt = 0; attempt < 50 && configHits === 0; attempt += 1) await page.waitForTimeout(20);
      assert.equal(configHits, 1);
      assert.equal(await page.locator('[data-feedback-entry]').isHidden(), true, '403 must remain undiscoverable');
      await page.locator('#business-button').click();
      assert.equal(await page.locator('#business-state').textContent(), '1');
      assert.equal(page.frames().length, 1, 'authorization denial must not load the SDK');
      assert.equal(errors.length, 0);
      result.scenarios.push({ name: 'capability-denied', passed: true, assertions: 5 });
    } finally {
      await context.close();
    }
  }

  {
    const { context, page, errors } = await newPage('unavailable');
    try {
      await page.locator('[data-feedback-entry]').waitFor({ state: 'visible', timeout: 5000 });
      await page.locator('[data-feedback-toggle]').click();
      await page.locator('[data-feedback-mode="submit"]').first().click();
      await page.locator('[data-feedback-toast]').waitFor({ state: 'visible' });
      assert.match(await page.locator('[data-feedback-toast]').textContent(), /暂时不可用/);
      await page.locator('#business-button').click();
      assert.equal(await page.locator('#business-state').textContent(), '1');
      assert.equal(page.frames().length, 1, 'config outage must not load the SDK');
      assert.equal(errors.length, 0);
      result.scenarios.push({ name: 'feedback-unavailable', passed: true, assertions: 5 });
    } finally {
      await context.close();
    }
  }

  result.passed = true;
  console.log(JSON.stringify(result));
} finally {
  if (browser) await browser.close();
  await Promise.allSettled([close(hostServer), close(sdkServer)]);
}

