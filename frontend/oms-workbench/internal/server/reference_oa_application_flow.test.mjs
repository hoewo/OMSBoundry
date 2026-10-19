import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const template=readFileSync(new URL('./server.go',import.meta.url),'utf8');
const start=template.indexOf('async function businessApplicationFetch(');
const end=template.indexOf('async function businessApplicationJSON(',start);
assert.ok(start>0&&end>start);
const projectID='11111111-1111-4111-8111-111111111111',tenantID='22222222-2222-4222-8222-222222222222';
function fixture({application='billing-invoice',authStatus=200,sourceStatus=200,project={id:projectID,tenant_id:tenantID}}={}){
  const calls=[],trace=[];
  const context=vm.createContext({Headers,URL,URLSearchParams,location:{origin:'https://oa.example.test'},
    document:{querySelectorAll:()=>[{dataset:{realApplication:application,realRoute:'/configured'}}]},
    architectureTrace:{epoch:()=>7},referenceOACSRF:{token:async()=> 'csrf-fixture'},
    initiationWorkspace:{dataset:{tenantId:tenantID}},
    referenceOAController:{projectID,flow:{details:new Map([[projectID,{project}]])}},
    recordArchitectureStep:(...v)=>trace.push(v),projectAPIError:(status)=>Object.assign(new Error('status '+status),{status}),
    fetch:async(url,options)=>{calls.push({url,options});return url.startsWith('/gateway/authorize/')?{ok:authStatus===200,status:authStatus,url:'https://oa.example.test/configured/'}:{ok:sourceStatus===200,status:sourceStatus};}
  });
  vm.runInContext(template.slice(start,end),context);
  return {calls,trace,request:context.businessApplicationFetch};
}
test('native owner routing preserves configured routes and names Contract, Money, Billing and Party accurately',async()=>{
  for(const [application,label] of [['contract-ledger','Contract Ledger'],['money-flow-ledger','Money Flow Ledger'],['billing-invoice','Billing'],['party-directory','Party Directory']]){
    const f=fixture({application});await f.request(application,'/api/v1/resources');
    assert.equal(f.calls[0].url,'/gateway/authorize/'+application);assert.equal(f.calls[1].url,'/configured/api/v1/resources');
    assert.ok(f.trace.some(v=>v[1]==='Gateway → '+label));assert.ok(f.trace.some(v=>v[0]===application&&v[1]===label+' 原生接口已回复'));
  }
});
test('PUT, PATCH, DELETE and POST receipts are never labelled as zero-write reads',async()=>{
  for(const method of ['POST','PUT','PATCH','DELETE','put']){
    const f=fixture();await f.request('billing-invoice','/api/v1/invoice-registrations',{method});
    const receipt=f.trace.find(v=>v[0]==='billing-invoice');assert.match(receipt[2],/写回执/);assert.doesNotMatch(receipt[2],/读取不自动/);
    assert.equal(f.calls[1].options.method,method);assert.equal(f.calls[1].options.redirect,'error');
  }
});
test('GET remains a zero-write read and failed source commands never report success',async()=>{
  const read=fixture();await read.request('billing-invoice','/api/v1/invoice-registrations');assert.match(read.trace.find(v=>v[0]==='billing-invoice')[2],/读取不自动/);
  const failed=fixture({sourceStatus:503});await failed.request('billing-invoice','/api/v1/invoice-registrations',{method:'PUT'});
  assert.equal(failed.calls.length,2);assert.ok(failed.trace.some(v=>v[0]==='billing-invoice'&&v[3]==='error'));assert.ok(!failed.trace.some(v=>v[0]==='billing-invoice'&&v[3]==='complete'));
});
test('issuer rejection or unconfigured route submits no business request and uses no alternate identity',async()=>{
  const denied=fixture({authStatus:403});await assert.rejects(denied.request('billing-invoice','/api/v1/invoice-registrations',{method:'POST'}),/403/);assert.equal(denied.calls.length,1);
  const missing=fixture();await assert.rejects(missing.request('contract-ledger','/api/v1/contracts'),/未配置/);assert.equal(missing.calls.length,0);
});
test('Billing and both Cost sides authorize the exact current project and never forward a UI-only scope option',async()=>{
 for(const scope of [null,'business','production']){
  const application=scope?'cost-outsourcing':'billing-invoice',f=fixture({application});
  await f.request(application,'/api/v1/resources',scope?{costScope:scope}:{});
  assert.equal(f.calls[0].options.body.get('resource_id'),projectID);
  assert.equal(f.calls[0].options.body.get('resource_type'),scope?scope+'_cost_project':'project');
  assert.equal(f.calls[1].options.costScope,undefined);
 }
});
test('missing project, foreign Tenant, or unknown Cost side fails before any issuer or business request',async()=>{
 for(const project of [null,{id:projectID,tenant_id:'foreign'}]){
  for(const application of ['billing-invoice','cost-outsourcing']){
   const f=fixture({application,project});await assert.rejects(f.request(application,'/api/v1/resources',{costScope:'business'}),/上下文/);assert.equal(f.calls.length,0);
  }
 }
 const f=fixture({application:'cost-outsourcing'});await assert.rejects(f.request('cost-outsourcing','/api/v1/resources',{costScope:'unknown'}),/上下文/);assert.equal(f.calls.length,0);
});
test('D01 transport distinguishes exact Project personnel from Tenant catalogue without aliasing authority',async()=>{
 const path='/api/v1/projects/'+projectID+'/reference-oa/production-personnel',f=fixture({application:'project-workspace'});
 await f.request('project-workspace',path,{projectPersonnelScope:'project'});
 assert.equal(f.calls[0].options.body.get('resource_id'),projectID);assert.equal(f.calls[0].options.body.get('resource_type'),'project');assert.equal(f.calls[1].options.projectPersonnelScope,undefined);
 const catalogue=fixture({application:'project-workspace'});await catalogue.request('project-workspace','/api/v1/reference-oa/production-disciplines',{projectPersonnelScope:'catalogue'});assert.equal(catalogue.calls[0].options.body.get('resource_id'),null);
 for(const [scope,p] of [['catalogue',path],['project','/api/v1/projects/33333333-3333-4333-8333-333333333333/reference-oa/production-personnel'],['wrong',path],['project','/api/v1/projects/'+projectID+'/assignments']]){const bad=fixture({application:'project-workspace'});await assert.rejects(bad.request('project-workspace',p,{projectPersonnelScope:scope}),/上下文/);assert.equal(bad.calls.length,0)}
});
test('shared scheme uses exact Project native authorization, never forwards UI option',async()=>{
 const f=fixture({application:'production-operations'});await f.request('production-operations','/api/v1/production-stage-work-items?project_id='+projectID+'&type_key=reference-oa-stage-set-v1',{productionStageScope:projectID});
 assert.equal(f.calls[0].options.body.get('resource_id'),projectID);assert.equal(f.calls[0].options.body.get('resource_type'),'project');assert.equal(f.calls[1].options.productionStageScope,undefined);
 const progress=fixture({application:'production-operations'});await progress.request('production-operations','/api/v1/production-stage-progress?project_id='+projectID+'&type_key=reference-oa-stage-set-v1',{productionStageScope:projectID});assert.equal(progress.calls[0].options.body.get('resource_id'),projectID);assert.equal(progress.calls[1].options.productionStageScope,undefined);
 const bad=fixture({application:'production-operations'});await assert.rejects(bad.request('production-operations','/api/v1/production-work-items',{productionStageScope:'foreign'}),/上下文/);assert.equal(bad.calls.length,0);
});
test('one audience cookie lease serializes authorization and actual request',async()=>{
 const f=fixture({application:'production-operations'});await Promise.all([f.request('production-operations','/api/v1/resources',{productionStageScope:projectID}),f.request('production-operations','/api/v1/resources')]);
 assert.deepEqual(f.calls.map(c=>c.url),['/gateway/authorize/production-operations','/configured/api/v1/resources','/gateway/authorize/production-operations','/configured/api/v1/resources']);
});
test('one selected panel renews an unchanged audience and scope only once for all linked reads',async()=>{
 const f=fixture({application:'money-flow-ledger'});
 await f.request('money-flow-ledger','/api/v1/cash-workspace');
 for(let index=0;index<25;index++)await f.request('money-flow-ledger','/api/v1/cash-transactions/'+String(index));
 assert.equal(f.calls.filter(call=>call.url==='/gateway/authorize/money-flow-ledger').length,1);
 assert.equal(f.calls.filter(call=>call.url.startsWith('/configured/')).length,26);
});
test('a changed exact scope replaces the audience lease instead of reusing broader authority',async()=>{
 const f=fixture({application:'cost-outsourcing'});
 await f.request('cost-outsourcing','/api/v1/cost-entries',{costScope:'business'});
 await f.request('cost-outsourcing','/api/v1/cost-entries',{costScope:'business'});
 await f.request('cost-outsourcing','/api/v1/cost-entries',{costScope:'production'});
 assert.equal(f.calls.filter(call=>call.url==='/gateway/authorize/cost-outsourcing').length,2);
 assert.equal(f.calls.length,5);
});
