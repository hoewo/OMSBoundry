import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

// Execute the embedded production bridge, not a second implementation. The
// private ports below observe actual request order/body; no Main grants exist.
const server=readFileSync(new URL('../../internal/server/server.go',import.meta.url),'utf8');
const start=server.indexOf('async function businessApplicationFetch(');
const bridge=server.slice(start,server.indexOf('async function businessApplicationJSON(',start));
assert(start>0&&bridge.includes('bonusScope'));
const tenant='22222222-2222-4222-8222-222222222222',project='33333333-3333-4333-8333-333333333333';
function fixture({status=200,redirect='/configured-bonus/',projectValue={id:project,tenant_id:tenant}}={}){
  const calls=[];
  const context=vm.createContext({URL,URLSearchParams,Headers,Object,Error,
    document:{querySelectorAll:()=>[{dataset:{realApplication:'bonus-distribution',realRoute:'/configured-bonus'}}]},
    location:{origin:'https://private.example'},architectureTrace:null,recordArchitectureStep:()=>{},
    referenceOACSRF:{token:async()=>'test-csrf'},
    initiationWorkspace:{dataset:{tenantId:tenant}},
    referenceOAController:{projectID:project,flow:{details:new Map([[project,{project:projectValue}]])}},
    projectAPIError:(code,data)=>Object.assign(new Error(data.error.message),{status:code}),
    fetch:async(path,options)=>{
      calls.push({path,options,authorizationBody:options.body instanceof URLSearchParams?Object.fromEntries(options.body):null});
      return path.startsWith('/gateway/')?{ok:status===200,status,url:'https://private.example'+redirect}:{ok:true,status:200};
    },
  });
  vm.runInContext(bridge,context);
  return {calls,request:context.businessApplicationFetch};
}
for(const scope of ['business','production'])for(const method of ['GET','POST'])test(`Workbench ${scope} ${method} sends exact scope before IAM and strips adapter options`,async()=>{
  const f=fixture(),path='/api/v1/reference-oa/bonus-allocations?project_id='+project+'&scope='+scope;
  await f.request('bonus-distribution',path,{bonusScope:scope,method,...(method==='POST'?{body:'{"command_id":"private-test"}'}:{})});
  assert.equal(f.calls.length,2);
  assert.deepEqual(f.calls[0].authorizationBody,{csrf_token:'test-csrf',resource_type:scope+'_bonus_project',resource_id:project});
  assert.equal(f.calls[0].path,'/gateway/authorize/bonus-distribution');
  const owner=f.calls[1];assert.equal(owner.path,'/configured-bonus'+path);
  assert.equal(owner.options.method,method);assert.equal(owner.options.bonusScope,undefined);assert.equal(owner.options.costScope,undefined);
  assert.equal(owner.options.credentials,'same-origin');assert.equal(owner.options.cache,'no-store');assert.equal(owner.options.redirect,'error');
  assert.equal(owner.options.headers.get('Accept'),'application/json');
});
test('manual Bonus malformed side, missing Project or other Tenant never requests IAM or owner',async()=>{
  for(const [projectValue,bonusScope] of [[null,'business'],[{id:project,tenant_id:'another'},'production'],[{id:project,tenant_id:tenant},'unknown']]){
    const f=fixture({projectValue});await assert.rejects(f.request('bonus-distribution','/api/v1/reference-oa/bonus-allocations',{bonusScope}),/授权上下文/);assert.equal(f.calls.length,0);
  }
});
test('Bonus IAM denial or wrong redirect cannot send the native write',async()=>{
  for(const options of [{status:403},{redirect:'/wrong-app/'}]){
    const f=fixture(options);await assert.rejects(f.request('bonus-distribution','/api/v1/reference-oa/bonus-allocations',{bonusScope:'business',method:'POST',body:'private-intent'}));assert.equal(f.calls.length,1);
  }
});
test('legacy source Pool uses its unchanged authorization path, never inherits manual Scope',async()=>{
  const f=fixture();await f.request('bonus-distribution','/api/v1/bonus-pools?project_id='+project);
  assert.deepEqual(f.calls[0].authorizationBody,{csrf_token:'test-csrf'});
});

function workspaceFixture(){
  const field=()=>({hidden:false,textContent:'',replaceChildren(){},classList:{toggle(){}},setAttribute(){}});
  const fields=new Map(),buttons=['overview','client','personnel','bidding','contracts','payments','invoices','costs','bonus'].map(key=>({...field(),dataset:{realBusinessTab:key}}));
  const root={addEventListener(){},querySelector(selector){if(!fields.has(selector))fields.set(selector,field());return fields.get(selector)},querySelectorAll:()=>buttons};
  root.querySelector('[data-real-client-form]').hidden=true;
  const calls=[],dependency=label=>({load:async()=>{calls.push(label)},invalidate:()=>{calls.push('invalidate-'+label)}});
  const context=vm.createContext({Object,Error});vm.runInContext(readFileSync(new URL('../../internal/server/reference_oa_business_runtime.js',import.meta.url),'utf8'),context);
  const workspace=new context.ReferenceOABusiness.BusinessWorkspace({root,details:()=>({project:{id:project}}),selected:()=>{},loadClient:()=>{},flow:{load:async(_,tab)=>{calls.push('reader-'+tab);return {message:'native',records:[]}}},bonus:dependency('business-bonus'),productionBonus:dependency('production-bonus'),costs:dependency('business-cost'),productionCosts:dependency('production-cost')});
  return {calls,fields,workspace};
}
test('hidden Bonus workspaces read nothing; only selected business or production panel loads',async()=>{
  const f=workspaceFixture();assert.deepEqual(f.calls,[]);
  await f.workspace.select('overview');assert.deepEqual(f.calls,['reader-overview']);
  assert.equal(f.fields.get('[data-real-business-panel="bonus"]').hidden,true);
  await f.workspace.select('bonus');assert.deepEqual(f.calls,['reader-overview','business-bonus']);
  assert.equal(f.fields.get('[data-real-business-panel="bonus"]').hidden,false);
  await f.workspace.select('costs');assert.deepEqual(f.calls,['reader-overview','business-bonus','business-cost']);
  assert.equal(f.fields.get('[data-real-business-panel="bonus"]').hidden,true);
  await f.workspace.production();assert.deepEqual(f.calls,['reader-overview','business-bonus','business-cost','reader-production']);
  await f.workspace.production('bonus');assert.deepEqual(f.calls,['reader-overview','business-bonus','business-cost','reader-production','production-bonus']);
  await f.workspace.production('cost');assert.deepEqual(f.calls,['reader-overview','business-bonus','business-cost','reader-production','production-bonus','production-cost']);
});
test('project switch invalidates both Bonus sides and adjacent Cost workspaces',()=>{
  const f=workspaceFixture();f.workspace.invalidate();assert.deepEqual(f.calls,['invalidate-business-cost','invalidate-production-cost','invalidate-business-bonus','invalidate-production-bonus']);
});
