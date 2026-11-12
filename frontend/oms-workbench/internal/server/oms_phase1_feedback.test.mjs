import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';

const source = readFileSync(new URL('./oms_phase1_feedback.js', import.meta.url), 'utf8');

assert.match(source, /fetch\('\/gateway\/feedback\/config'/, 'runtime config must come from the protected Gateway endpoint');
assert.match(source, /fetch\(route \+ '\/api\/v1\/csrf'/, 'an expired CSRF cookie must be refreshed through the configured IAM route');
assert.match(source, /if \(csrfRequest\) return csrfRequest;/, 'concurrent feedback actions must share one CSRF refresh');
assert.match(source, /configRequest = csrfToken\(\)\.then/, 'runtime config must recover an expired CSRF cookie before its protected request');
assert.match(source, /fetchSubmissionSnapshot\(\)[\s\S]*?var csrf = await csrfToken\(\);/, 'submission snapshots must recover an expired CSRF cookie before their protected request');
assert.match(source, /event\.origin !== sdkOrigin \|\| event\.source !== frame\.contentWindow/, 'inbound messages must bind exact origin and iframe window');
assert.match(source, /message\.source !== 'feedback-sdk-web'/, 'inbound messages must bind the SDK source');
assert.match(source, /sessionRenewalLeadMs = 2 \* 60 \* 1000/, 'short Feedback sessions must renew before the user reaches expiry');
assert.match(source, /expiresInSeconds = Number\(value\.expiresInSeconds\)/, 'session mode must use the server-calculated relative lifetime');
assert.match(source, /sessionExpiresAt = Date\.now\(\) \+ expiresInSeconds \* 1000/, 'client clock skew must not control the renewal schedule');
assert.match(source, /if \(configRequest\) return configRequest;/, 'concurrent foreground and background renewals must share one server exchange');
assert.match(source, /function refreshFeedbackSession\(required\)/, 'session renewal must have one bounded coordinator');
assert.match(source, /remaining > 5000[\s\S]*runtimeConfig = previous[\s\S]*scheduleSessionRenewal\(previous/, 'a transient proactive-renewal failure must retain a still-valid bounded token');
assert.match(source, /message\.type === 'feedback-sdk:refresh-session'[\s\S]*refreshFeedbackSession\(true\)/, 'an SDK 401 must require a fresh host-issued session');
assert.match(source, /document\.addEventListener\('visibilitychange'[\s\S]*sessionRemaining\(runtimeConfig\) <= sessionRenewalLeadMs/, 'a throttled background tab must recheck expiry when visible');
assert.match(source, /requiredSDKContract = 'v3'/, 'the OMS host must require the Feedback V3 product contract');
assert.match(source, /message\.supported_contracts\.indexOf\(requiredSDKContract\) < 0/, 'the host must reject an alias that does not actually expose V3');
assert.match(source, /microphone; clipboard-read; clipboard-write/, 'the V3 intelligent-support frame must declare its bounded media and clipboard features');
assert.match(source, /frame\.contentWindow\.postMessage\([\s\S]*?, sdkOrigin\);/, 'outbound messages must use an exact target origin');
assert.equal((source.match(/document\.createElement\('iframe'\)/g) || []).length, 1, 'the host must own one reusable iframe');
assert.match(source, /message\.request_id === unreadRequestID/, 'unread replies must match the current request');
assert.match(source, /badge\.hidden = count === 0/, 'zero unread must hide the badge');
assert.match(source, /setEntryVisible\(true\)/, 'capability success or a transient failure must expose a retry entry');
assert.match(source, /error && error\.status === 401/, 'an expired login must receive the dedicated re-login notice');
assert.match(source, /form\.addEventListener\('submit', clearRuntime\)/, 'logout must clear in-memory feedback state');
assert.match(source, /window\.addEventListener\('pagehide', clearRuntime/, 'page disposal must clear in-memory feedback state');
assert.doesNotMatch(source, /localStorage|sessionStorage/, 'feedback credentials must not be persisted in browser storage');
assert.doesNotMatch(source, /navigator\.userAgent\b/, 'full user-agent diagnostics are forbidden');
assert.doesNotMatch(source, /ak_[a-f0-9]{32,}/i, 'a real-looking Feedback API key must not enter source');
assert.match(source, /Feedback is auxiliary\. Preparation failure must never affect OMS\./, 'background preparation failure must stay isolated');
assert.match(source, /dismissedBySDK = true/, 'an explicit SDK close must record a reopenable dismissed state');
assert.match(source, /event\.target\.closest\('\[data-oms-v1-nav="feedback"\]'\)/, 'feedback navigation must reopen an explicitly dismissed SDK layer');
assert.match(source, /fetch\('\/gateway\/feedback\/submission-snapshot'/, 'the host must observe V2 creation through the protected same-origin Gateway');
assert.match(source, /fetchSubmissionSnapshot\(\)\.catch\(function \(\) \{ return null; \}\)/, 'a snapshot outage must not block the V3 SDK from opening');
assert.match(source, /function startSubmissionWatch\(baseline\)/, 'submit mode must watch the scoped V2 collection');
assert.match(source, /completeFeedbackSubmission\(\)/, 'a newly created feedback must complete the embedded submit journey');
const completionSource = source.slice(source.indexOf('function completeFeedbackSubmission()'), source.indexOf('function startSubmissionWatch(baseline)'));
assert.doesNotMatch(completionSource, /layer\.hidden = true/, 'submission completion must keep the SDK layer open');
assert.match(completionSource, /lastOpenedMode = 'status';/, 'submission completion must make status the reopen mode');
assert.match(completionSource, /post\('feedback-sdk:open', \{ mode: 'status' \}\);[\s\S]*反馈已提交，已进入“我的反馈”查看处理进度。/, 'submission completion must keep the user on the feedback status page');
assert.doesNotMatch(source, /\/workshop\/v2\//, 'the browser host must not call the external Feedback API directly');

const listeners = { window: {}, document: {}, page: {} };
const scheduled = new Map();
const posted = [];
let timerID = 0;
let requestID = 0;
let frame = null;
let configReads = 0;
const root = { dataset: { feedbackSdkUrl: 'https://feedback.example.test/sdk-v3', tenantId: 'tenant', subjectId: 'subject', identityRoutePrefix: '/auth' } };
const notice = { textContent: '' };
const page = {
  querySelector: () => notice,
  addEventListener: (type, listener) => { listeners.page[type] = listener; },
};
const body = { appendChild: () => {} };
const document = {
  cookie: 'oms_iam_csrf=csrf_token_0123456789',
  visibilityState: 'visible',
  body,
  querySelector: selector => selector === '[data-oms-v1]' ? root : selector === '[data-feedback-page]' ? page : selector === '[data-oms-v1-page].is-active' ? { dataset: { omsV1Page: 'feedback' } } : null,
  querySelectorAll: () => [],
  addEventListener: (type, listener) => { listeners.document[type] = listener; },
  createElement: tag => {
    if (tag === 'iframe') {
      frame = { contentWindow: { postMessage: message => posted.push(message) }, setAttribute: () => {} };
      return frame;
    }
    return { hidden: false, setAttribute: () => {}, appendChild: () => {}, remove: () => {} };
  },
};
const window = {
  location: { origin: 'https://oms.example.test' },
  crypto: { randomUUID: () => `00000000-0000-4000-8000-${String(++requestID).padStart(12, '0')}` },
  addEventListener: (type, listener) => { listeners.window[type] = listener; },
  setTimeout: (callback, delay) => { const id = ++timerID; scheduled.set(id, { callback, delay }); return id; },
  clearTimeout: id => scheduled.delete(id),
};
const fetch = async url => {
  assert.equal(url, '/gateway/feedback/config');
  configReads += 1;
  return {
    ok: true,
    status: 200,
    redirected: false,
    json: async () => ({ config: {
      feedbackV2Enabled: true,
      feedbackV2NotificationsEnabled: true,
      feedbackV2AuthMode: 'session',
      feedbackSessionToken: `fbs_session_${configReads}_0123456789`,
      expiresAt: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
      expiresInSeconds: 900,
      projectId: 115,
      customUserId: `oms1_v1_${'A'.repeat(43)}`,
      gatewayUrl: 'https://feedback.example.test',
    } }),
  };
};
runInNewContext(source, { window, document, navigator: { language: 'zh-CN' }, fetch, URL, Date, Number, Promise, Set, Uint8Array, console });
listeners.page.click({ target: { closest: () => ({ dataset: { feedbackMode: 'status' } }) } });
await new Promise(resolve => setImmediate(resolve));
assert.equal(configReads, 1, 'opening Feedback should exchange one short session');
listeners.window.message({ origin: 'https://feedback.example.test', source: frame.contentWindow, data: { source: 'feedback-sdk-web', type: 'feedback-sdk:ready', supported_contracts: ['v3'] } });
await new Promise(resolve => setImmediate(resolve));
const initialConfigure = posted.find(message => message.type === 'feedback-sdk:configure');
assert.ok(initialConfigure, 'the mediated session must be configured into the SDK');
assert.equal(initialConfigure.config.feedbackSessionToken, 'fbs_session_1_0123456789');
assert.equal('expiresAt' in initialConfigure.config, false, 'host lease metadata must not expand the SDK credential contract');
listeners.window.message({ origin: 'https://feedback.example.test', source: frame.contentWindow, data: { source: 'feedback-sdk-web', type: 'feedback-sdk:configured', request_id: initialConfigure.request_id } });
await new Promise(resolve => setImmediate(resolve));
const renewalTimers = [...scheduled.values()].filter(timer => timer.delay > 12 * 60 * 1000 && timer.delay <= 13 * 60 * 1000);
assert.equal(renewalTimers.length, 1, 'a 15-minute session must schedule one renewal about two minutes early');
listeners.window.message({ origin: 'https://feedback.example.test', source: frame.contentWindow, data: { source: 'feedback-sdk-web', type: 'feedback-sdk:refresh-session' } });
listeners.window.message({ origin: 'https://feedback.example.test', source: frame.contentWindow, data: { source: 'feedback-sdk-web', type: 'feedback-sdk:refresh-session' } });
await new Promise(resolve => setImmediate(resolve));
assert.equal(configReads, 2, 'concurrent expiry signals must collapse into one server exchange');

console.log('oms_phase1_feedback.test.mjs: 47 assertions passed');
