import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./oms_phase1_feedback.js', import.meta.url), 'utf8');

assert.match(source, /fetch\('\/gateway\/feedback\/config'/, 'runtime config must come from the protected Gateway endpoint');
assert.match(source, /event\.origin !== sdkOrigin \|\| event\.source !== frame\.contentWindow/, 'inbound messages must bind exact origin and iframe window');
assert.match(source, /message\.source !== 'feedback-sdk-web'/, 'inbound messages must bind the SDK source');
assert.match(source, /frame\.contentWindow\.postMessage\([\s\S]*?, sdkOrigin\);/, 'outbound messages must use an exact target origin');
assert.equal((source.match(/document\.createElement\('iframe'\)/g) || []).length, 1, 'the host must own one reusable iframe');
assert.match(source, /message\.request_id === unreadRequestID/, 'unread replies must match the current request');
assert.match(source, /badge\.hidden = count === 0/, 'zero unread must hide the badge');
assert.match(source, /setEntryVisible\(true\)/, 'capability success or a transient failure must expose a retry entry');
assert.match(source, /error\.status !== 401 && error\.status !== 403/, 'authorization denial must remain undiscoverable');
assert.match(source, /mobileMenu\.hidden = !mobileMenu\.hidden/, 'mobile entry must expose both feedback actions');
assert.match(source, /form\.addEventListener\('submit', clearRuntime\)/, 'logout must clear in-memory feedback state');
assert.match(source, /window\.addEventListener\('pagehide', clearRuntime/, 'page disposal must clear in-memory feedback state');
assert.doesNotMatch(source, /localStorage|sessionStorage/, 'feedback credentials must not be persisted in browser storage');
assert.doesNotMatch(source, /navigator\.userAgent\b/, 'full user-agent diagnostics are forbidden');
assert.doesNotMatch(source, /ak_[a-f0-9]{32,}/i, 'a real-looking Feedback API key must not enter source');
assert.match(source, /Feedback is auxiliary\. Preparation failure must never affect OMS\./, 'background preparation failure must stay isolated');

console.log('oms_phase1_feedback.test.mjs: 16 assertions passed');
