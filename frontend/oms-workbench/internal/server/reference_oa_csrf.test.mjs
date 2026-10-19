import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const context = vm.createContext({});
vm.runInContext(readFileSync(new URL('./reference_oa_initiation_runtime.js', import.meta.url), 'utf8'), context);
const CSRF = context.ReferenceOAGatewayCSRF;
const old = 'old-cookie-token-0123456789', fresh = 'new-cookie-token-0123456789';

test('current cookie wins over stale page data and other-tab rotation needs no renewal', async () => {
  let cookie = 'other=unrelated; oms_iam_csrf=' + old, calls = 0;
  const helper = new CSRF({cookies: () => cookie, request: () => calls++});
  assert.equal(await helper.token(), old);
  cookie = 'oms_iam_csrf=' + fresh;
  assert.equal(await helper.token(), fresh);
  assert.equal(calls, 0);
});

test('expired cookie is renewed by one configured same-origin GET, then used without rotating again', async () => {
  let cookie = '', calls = 0;
  const helper = new CSRF({route: '/configured-iam', cookies: () => cookie, request: async (url, options) => {
    calls++;
    assert.equal(url, '/configured-iam/api/v1/csrf');
    assert.equal(options.method, 'GET'); assert.equal(options.credentials, 'same-origin');
    assert.equal(options.cache, 'no-store'); assert.equal(options.redirect, 'error');
    assert.equal('body' in options, false);
    cookie = 'oms_iam_csrf=' + fresh;
    return {ok: true, json: async () => ({csrf_token: fresh})};
  }});
  assert.equal(await helper.token(), fresh); assert.equal(await helper.token(), fresh); assert.equal(calls, 1);
});

test('concurrent Project and People renewal share one pending GET', async () => {
  let cookie = '', finish, calls = 0;
  const helper = new CSRF({route: '/iam', cookies: () => cookie, request: () => {
    calls++; return new Promise(resolve => { finish = () => { cookie = 'oms_iam_csrf=' + fresh; resolve({ok: true, json: async () => ({csrf_token: fresh})}); }; });
  }});
  const a = helper.token(), b = helper.token(); finish();
  assert.equal(await a, fresh); assert.equal(await b, fresh); assert.equal(calls, 1);
});

test('missing or external IAM routes fail closed before any request', async () => {
  for (const route of [undefined, '', '//evil.example', 'https://evil.example', '/iam?redirect=x', '/iam/../other']) {
    let calls = 0;
    const helper = new CSRF({route, cookies: () => '', request: () => calls++});
    await assert.rejects(helper.token(), /入口未配置/); assert.equal(calls, 0);
  }
});

test('invalid response, missing cookie, mismatched cookie and redirect never fall back to old HTML token', async () => {
  for (const response of [{ok:false}, {ok:true, redirected:true}, {ok:true, json:async()=>({csrf_token:old})}, {ok:true, json:async()=>({csrf_token:fresh})}]) {
    const helper = new CSRF({route:'/iam', cookies:()=>'', request:async()=>response});
    await assert.rejects(helper.token(), /输入已保留/);
  }
  let cookie = '';
  const helper = new CSRF({route:'/iam', cookies:()=>cookie, request:async()=> { cookie='oms_iam_csrf='+fresh; return {ok:true, json:async()=>({csrf_token:old})}; }});
  await assert.rejects(helper.token(), /不会自动提交业务/);
});

test('temporary renewal failure is explicitly retryable, not a cached failure or automatic business retry', async () => {
  let cookie = '', calls = 0;
  const helper = new CSRF({route:'/iam', cookies:()=>cookie, request:async()=> {
    if (++calls === 1) throw new Error('network');
    cookie='oms_iam_csrf='+fresh; return {ok:true, json:async()=>({csrf_token:fresh})};
  }});
  await assert.rejects(helper.token(), /请重试/); assert.equal(calls,1);
  assert.equal(await helper.token(),fresh); assert.equal(calls,2);
});

test('authorization renewal failure prevents an entire business command and retains command-free state', async () => {
  const helper = new CSRF({route:'/iam', cookies:()=>'', request:async()=>({ok:false})});
  let writes=0;
  const flow = new context.ReferenceOAInitiationFlow({authorize:()=>helper.token(), request:async()=>writes++, uuid:()=>{throw new Error('must not create command');}});
  await assert.rejects(flow.dispatch({name:'验收项目',node_type:'project',business_owner:{person_id:'55555555-5555-4555-8555-555555555555',source_revision:1}}), /输入已保留/);
  assert.equal(writes,0); assert.equal(flow.commands.size,0); assert.equal(flow.busy,false);
});

test('malformed cookie values are not sent to authorization', () => {
  for (const cookie of ['oms_iam_csrf=', 'oms_iam_csrf=short', 'oms_iam_csrf=<script>', 'other=oms_iam_csrf='+fresh]) {
    const helper=new CSRF({cookies:()=>cookie}); assert.equal(helper.current(),'');
  }
});
