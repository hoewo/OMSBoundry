import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const context = vm.createContext({URL, URLSearchParams, document: {querySelector: () => null}});
for (const name of ['reference_oa_initiation_runtime.js', 'reference_oa_login.js']) {
  vm.runInContext(readFileSync(new URL('./' + name, import.meta.url), 'utf8'), context);
}
const CSRF = context.ReferenceOAGatewayCSRF, Login = context.ReferenceOALoginFlow;
const old = 'old-cookie-token-0123456789', fresh = 'new-cookie-token-0123456789';
const values = {username:'fixture', password:'fixture-not-a-secret', csrf_token:old, return_to:'/workbench/'};
function setup({cookie = fresh, refresh, response, post} = {}) {
  let current = cookie;
  const calls = [], navigations = [];
  const csrf = new CSRF({route:'/configured-auth', cookies:()=>current ? 'oms_iam_csrf='+current : '', request:async(url, options)=> {
    calls.push({url, method:options.method});
    if (refresh) return refresh();
    current = fresh;
    return {ok:true, json:async()=>({csrf_token:fresh})};
  }});
  const login = new Login({csrf, location:{origin:'https://suite.example', assign:url=>navigations.push(url)}, request:async(url, options)=> {
    calls.push({url, method:options.method, token:options.body.get('csrf_token')});
    assert.equal(options.credentials,'same-origin'); assert.equal(options.cache,'no-store');
    assert.equal(options.body.get('username'),values.username);
    assert.equal(options.body.get('password'),values.password);
    return post ? post() : response || {ok:true, redirected:true, url:'https://suite.example/workbench/'};
  }});
  return {login, calls, navigations};
}
test('rotated cookie replaces the obsolete HTML token without another rotation', async()=> {
  const {login, calls, navigations} = setup();
  await login.submit(values);
  assert.deepEqual(calls,[{url:'/gateway/login',method:'POST',token:fresh}]);
  assert.deepEqual(navigations,['https://suite.example/workbench/']);
  assert.equal(values.csrf_token,old); assert.equal(login.busy,false);
});
test('expired cookie is renewed through configured IAM before one login POST', async()=> {
  const {login,calls}=setup({cookie:''}); await login.submit(values);
  assert.deepEqual(calls,[{url:'/configured-auth/api/v1/csrf',method:'GET'},{url:'/gateway/login',method:'POST',token:fresh}]);
});
test('renewal failure preserves input and sends no credentials or automatic retry', async()=> {
  const {login,calls,navigations}=setup({cookie:'',refresh:async()=>({ok:false})});
  await assert.rejects(login.submit(values),/输入已保留/);
  assert.equal(calls.length,1); assert.equal(calls[0].method,'GET');
  assert.equal(values.password,'fixture-not-a-secret'); assert.equal(navigations.length,0); assert.equal(login.busy,false);
});
test('racing CSRF rejection stays on the login page, with no automatic POST replay', async()=> {
  const {login,calls,navigations}=setup({response:{ok:false,status:403}});
  await assert.rejects(login.submit(values),/再次点击登录.*输入已保留/);
  assert.equal(calls.length,1); assert.equal(navigations.length,0); assert.equal(login.busy,false);
});
test('bad credentials and service errors never navigate or retry', async()=> {
  for (const [status,message] of [[401,/用户名或密码/],[429,/频繁/],[503,/身份服务/]]) {
    const {login,calls,navigations}=setup({response:{ok:false,status}});
    await assert.rejects(login.submit(values),message);
    assert.equal(calls.length,1); assert.equal(navigations.length,0);
  }
});
test('uncertain network result is not replayed automatically', async()=> {
  const {login,calls,navigations}=setup({post:async()=>{throw new Error('network unavailable')}});
  await assert.rejects(login.submit(values),/network/);
  assert.equal(calls.length,1); assert.equal(navigations.length,0); assert.equal(login.busy,false);
});
test('double-click submits credentials once while awaiting acknowledgement', async()=> {
  let finish;
  const {login,calls}=setup({post:()=>new Promise(resolve=>{finish=resolve})});
  const a=login.submit(values); await Promise.resolve();
  assert.equal(await login.submit(values),null);
  finish({ok:true,redirected:true,url:'https://suite.example/workbench/'});
  await a; assert.equal(calls.length,1); assert.equal(login.busy,false);
});
test('external, non-redirect, or login-loop success responses fail closed', async()=> {
  for (const response of [{ok:true,redirected:true,url:'https://other.example/workbench/'},{ok:true,redirected:false,url:'https://suite.example/workbench/'},{ok:true,redirected:true,url:'https://suite.example/gateway/login'}]) {
    const {login,navigations,calls}=setup({response});
    await assert.rejects(login.submit(values),/未确认/);
    assert.equal(navigations.length,0); assert.equal(calls.length,1);
  }
});
test('missing helper or invalid IAM route cannot cause a login POST fallback', async()=> {
  let calls=0;
  const login=new Login({csrf:new CSRF({route:'//other.example',cookies:()=>'',request:async()=>calls++}),request:async()=>calls++});
  await assert.rejects(login.submit(values),/入口未配置/); assert.equal(calls,0);
});
