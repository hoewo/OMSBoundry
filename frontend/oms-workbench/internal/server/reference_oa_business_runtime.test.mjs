import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const context=vm.createContext({});
vm.runInContext(readFileSync(new URL('./reference_oa_business_runtime.js',import.meta.url),'utf8'),context);
const {BusinessFlow,BusinessWorkspace,minorAmount,decimalAmount,tabs}=context.ReferenceOABusiness;
const id='33333333-3333-4333-8333-333333333333', other='44444444-4444-4444-8444-444444444444', tenant='11111111-1111-4111-8111-111111111111', resource='55555555-5555-4555-8555-555555555555';
const details=()=>({project:{id,tenant_id:tenant},allowed_actions:['project-workspace.commercial-summary.read'],resource_links:[]});
function setup(response, applicationResponse) {
  const calls=[];
  const flow=new BusinessFlow({tenantID:tenant, projectAuthorize:async()=>calls.push(['authorize-project']), projectRequest:async path=>{calls.push(['project',path]);return response;},applicationRequest:async(app,path,options)=>{calls.push(options?[app,path,options]:[app,path]);if(applicationResponse instanceof Error)throw applicationResponse;return typeof applicationResponse==='function'?applicationResponse(app,path,options):applicationResponse;}});
  return {flow,calls};
}
test('all nine original business navigation keys have a readable label',()=>assert.equal(Object.keys(tabs).length,9));
test('financial values are exact strings, null and unsafe int64 never become zero or rounded amounts',()=>{
  assert.equal(minorAmount(10001,'CNY'),'CNY 100.01');assert.equal(minorAmount(-10001,'CNY'),'CNY -100.01');
  assert.equal(minorAmount(0,'CNY'),'CNY 0.00');assert.match(minorAmount(null,'CNY'),/未知/);assert.match(minorAmount(9007199254740992,'CNY'),/精确/);
  assert.equal(decimalAmount('9007199254740993.01','CNY'),'CNY 9,007,199,254,740,993.01');assert.equal(decimalAmount(null,'CNY'),'金额未知');
});
test('native commercial permission is mandatory and denial does not read a source',async()=>{
  const {flow,calls}=setup();const d=details();d.allowed_actions=[];
  const result=await flow.load(d,'contracts');assert.equal(result.state,'denied');assert.equal(calls.length,0);
});
test('wrong tenant fails before authorization or data read',async()=>{
  const {flow,calls}=setup();const d=details();d.project.tenant_id=other;
  await assert.rejects(flow.load(d,'overview'),/租户/);assert.equal(calls.length,0);
});
test('commercial sections filter exact resource type and never sum mixed money',async()=>{
  const segments=[{resource_type:'contract',source_application:'contract-ledger',state:'available',contract:{currency:'CNY',current_effective_amount:'100.01'}},{resource_type:'transaction',source_application:'money-flow-ledger',state:'available',transaction:{currency:'CNY',effective_amount:'20.01'}},{resource_type:'invoice_registration',source_application:'billing-invoice',state:'available',invoice_registration:{currency:'CNY',amount_minor:1001}}];
  const {flow}=setup({project_id:id,segments});
  for(const tab of ['contracts','payments','invoices']) assert.equal((await flow.load(details(),tab)).records.length,1);
  const overview=await flow.load(details(),'overview');assert.equal(overview.records.length,3);assert.match(overview.message,/不混合加总/);
});
test('wrong-project commercial response is rejected, not shown in the current project',async()=>{
  const {flow}=setup({project_id:other,segments:[]});await assert.rejects(flow.load(details(),'payments'),/不一致/);
});
test('unavailable financial source retains its explicit failure instead of displaying zero',async()=>{
  const {flow}=setup({project_id:id,segments:[{resource_type:'contract',state:'unavailable',source_application:'contract-ledger',error_code:'DEPENDENCY_UNAVAILABLE'}]});
  const result=await flow.load(details(),'contracts');assert.equal(result.records[0].state,'unavailable');assert.equal(result.records[0].fields.at(-1)[1],'DEPENDENCY_UNAVAILABLE');
});
test('no active exact link is not a false empty source list and performs no tenant-wide discovery',async()=>{
  const {flow,calls}=setup();const d=details();d.resource_links=[{target_service:'business-engagement',resource_type:'business-engagement',resource_id:resource,unlinked_at:'2026-10-02'}];
  const result=await flow.load(d,'bidding');assert.equal(result.state,'unconnected');assert.match(result.message,/不能据此认定/);assert.equal(calls.length,0);
});
test('linked source failure is a per-record error, successful sibling remains readable',async()=>{
  const {flow,calls}=setup(null,(app,path)=>{if(path.endsWith(other))throw new Error('source stopped');return {engagement:{id:resource,project_id:id,tenant_id:tenant,status:'ready'},materials:[]};});
  const d=details();d.resource_links=[resource,other].map(resource_id=>({target_service:'business-engagement',resource_type:'business-engagement',resource_id}));
  const result=await flow.load(d,'bidding');assert.equal(result.records.length,2);assert.equal(result.records[0].state,'available');assert.equal(result.records[1].state,'unavailable');assert.equal(calls.length,2);
});
test('wrong-project linked source is not rendered as a valid business fact',async()=>{
  const {flow}=setup(null,{engagement:{id:resource,project_id:other,tenant_id:tenant,status:'ready'}});const d=details();d.resource_links=[{target_service:'business-engagement',resource_type:'business-engagement',resource_id:resource}];
  const result=await flow.load(d,'bidding');assert.equal(result.records[0].state,'unavailable');assert.match(result.records[0].fields[0][1],/不一致/);
});
test('production overview is one exact Project-scoped Production projection and never rereads Project',async()=>{
  const response={schema_version:'1.0',profile:'reference-oa-production-progress-v1',tenant_id:tenant,project_id:id,applicable_stage_count:3,completed_stage_count:2,required_stage_count:3,required_complete_count:2,production_complete:false,items:[{work_item_id:resource,type_definition_version:6,project_source_revision:11,revision:8,status:'in-progress',project_type_keys:['survey'],required_stage_count:3,required_complete_count:2,type_progress:[{project_type_key:'survey',required_stage_count:3,required_complete_count:2,complete:false}],stages:[{stage_key:'scheme',status:'completed',main_file_count:2,review_file_count:1},{stage_key:'test',status:'completed',main_file_count:2,review_file_count:0},{stage_key:'report',status:'ready',main_file_count:0,review_file_count:0}]}]};
  const {flow,calls}=setup(null,response);const result=await flow.load(details(),'production');
  assert.equal(calls.length,1);assert.equal(calls[0][0],'production-operations');assert.match(calls[0][1],/^\/api\/v1\/production-stage-progress\?/);assert.equal(calls[0][2].productionStageScope,id);
  assert.deepEqual({...result.progress,stages:undefined,types:undefined},{complete:false,requiredComplete:2,requiredTotal:3,stageComplete:2,stageTotal:3,typeComplete:0,typeTotal:1,stages:undefined,types:undefined});assert.equal(result.progress.stages.length,3);assert.equal(result.records.length,1);assert.match(result.records[0].fields.at(-1)[1],/主文件 0 份/);assert.match(result.message,/不回查 Project/);
});
test('production overview rejects a cross-project or unversioned projection',async()=>{
  const {flow}=setup(null,{schema_version:'1.0',profile:'reference-oa-production-progress-v1',tenant_id:tenant,project_id:other,items:[]});await assert.rejects(flow.load(details(),'production'),/不一致/);
});
test('cost reader uses the native exact project query and does not invent business classification',async()=>{
  const {flow,calls}=setup(null,{project_id:id,items:[{id:resource,tenant_id:tenant,project:{id},category:'other',amount_minor:null}]});
  const result=await flow.load(details(),'costs');assert.equal(calls[0][1],'/api/v1/cost-entries?project_id='+id);assert.match(result.message,/不能.*全部当成经营成本/);assert.match(result.records[0].fields[2][1],/未知/);
});
test('only business pools appear in business bonus, production pools are not relabelled',async()=>{
  const {flow}=setup(null,{project_id:id,items:['business','production'].map(bonus_type=>({project:{id},tenant_id:tenant,bonus_type,amount_minor:1000,currency:'CNY',allocations:[]}))});
  const result=await flow.load(details(),'bonus');assert.equal(result.records.length,1);
});
test('wrong-project cost or pool collection is rejected completely',async()=>{
  const {flow}=setup(null,{project_id:id,items:[{project:{id:other},tenant_id:tenant,bonus_type:'business'}]});await assert.rejects(flow.load(details(),'bonus'),/不一致/);
});
test('business participant gap never reuses owners or production participants',async()=>{
  const {flow,calls}=setup();const d={...details(),assignments:[{role_key:'business_owner'},{role_key:'participant'}]};
  const result=await flow.load(d,'personnel');assert.equal(result.state,'unimplemented');assert.equal(result.records.length,0);assert.equal(calls.length,0);
});
function workspaceFixture(load) {
  const nodes=new Map(), selections=[],buttons=Object.keys(tabs).map(key=>({dataset:{realBusinessTab:key},classList:{toggle(){}},setAttribute(){}}));
  const field=selector=>{if(!nodes.has(selector))nodes.set(selector,{hidden:true,textContent:'',replaceChildren(){}});return nodes.get(selector);};
  const workspace=Object.create(BusinessWorkspace.prototype);Object.assign(workspace,{sequence:0,tab:'overview',root:{querySelectorAll:()=>buttons},field,details:()=>details(),flow:{load},selected:(tab,push)=>selections.push([tab,push]),loadClient:async()=>{selections.push(['client-read']);},render:(content,records)=>{content.records=records;}});
  return {workspace,nodes,selections};
}
test('switch changes the active panel and selected URL state before waiting for the read',async()=>{
  let resolve;const {workspace,nodes,selections}=workspaceFixture(()=>new Promise(r=>resolve=r));
  const pending=workspace.select('contracts',true);assert.equal(workspace.tab,'contracts');assert.equal(nodes.get('[data-real-business-reader]').hidden,false);assert.deepEqual(selections,[['contracts',true]]);
  resolve({records:[],message:'real result'});await pending;assert.equal(nodes.get('[data-real-business-status]').textContent,'real result');
});
test('late reply for previous tab cannot replace the currently selected tab',async()=>{
  let resolve;const {workspace,nodes}=workspaceFixture((d,tab)=>tab==='contracts'?new Promise(r=>resolve=r):Promise.resolve({records:[],message:'invoice result'}));
  const pending=workspace.select('contracts');await workspace.select('invoices');resolve({records:[],message:'late contract'});await pending;
  assert.equal(nodes.get('[data-real-business-status]').textContent,'invoice result');
});
test('returning to a fresh generic tab uses its page-memory result and explicit refresh reads again',async()=>{
  let reads=0;const {workspace,nodes}=workspaceFixture(async()=>({records:[{title:'record '+(++reads),fields:[],source:'owner'}],message:'read '+reads}));
  await workspace.select('overview');await workspace.select('overview');
  assert.equal(reads,1);assert.equal(nodes.get('[data-real-business-status]').textContent,'read 1');
  await workspace.select('overview',false,true);assert.equal(reads,2);assert.equal(nodes.get('[data-real-business-status]').textContent,'read 2');
});
test('a stale refresh failure preserves the last visible projection',async()=>{
  let reads=0;const {workspace,nodes}=workspaceFixture(async()=>{reads++;if(reads>1)throw new Error('source unavailable');return {records:[{title:'kept',fields:[],source:'owner'}],message:'last good'};});
  await workspace.select('overview');workspace.readFreshMs=0;await workspace.select('overview');
  assert.equal(reads,2);assert.equal(nodes.get('[data-real-business-content]').records[0].title,'kept');assert.match(nodes.get('[data-real-business-status]').textContent,/仍显示上次结果/);
});
test('read failure clears former facts and remains a retryable failure, not an empty list',async()=>{
  const {workspace,nodes}=workspaceFixture(async()=>{throw new Error('503');});await workspace.select('costs');assert.match(nodes.get('[data-real-business-status]').textContent,/读取未完成.*不表示没有数据/);
});
test('unsaved client edit prevents subtab switch and never submits data',async()=>{
  const {workspace,nodes,selections}=workspaceFixture(async()=>{throw new Error('must not read');});workspace.tab='client';workspace.field('[data-real-client-form]').hidden=false;
  await workspace.select('bonus',true);assert.equal(workspace.tab,'client');assert.equal(selections.length,0);assert.match(nodes.get('[data-real-client-status]').textContent,/先保存或取消/);
});
test('original contract, cash and invoice panels load only the selected owner and never the generic projection',async()=>{
  const {workspace,nodes,selections}=workspaceFixture(async()=>{throw new Error('must not use generic projection');});
  const calls=[];
  for(const [property,tab] of [['contracts','contracts'],['cash','payments'],['invoices','invoices']])workspace[property]={load:async()=>calls.push(tab)};
  for(const tab of ['contracts','payments','invoices']){
    calls.length=0;await workspace.select(tab,true);assert.deepEqual(calls,[tab]);
    assert.equal(nodes.get('[data-real-business-reader]').hidden,true);
    for(const panel of ['contracts','payments','invoices'])assert.equal(nodes.get('[data-real-business-panel="'+panel+'"]').hidden,panel!==tab);
  }
  assert.deepEqual(selections,[['contracts',true],['payments',true],['invoices',true]]);
});
test('an unavailable original owner does not fall back to another service or a tenant-wide collection',async()=>{
  const {workspace,nodes}=workspaceFixture(async()=>{throw new Error('must not use generic projection');});
  workspace.invoices={load:async()=>{throw new Error('Billing unavailable');}};
  await assert.rejects(workspace.select('invoices'),/Billing unavailable/);
  assert.equal(workspace.tab,'invoices');assert.equal(nodes.get('[data-real-business-reader]').hidden,true);
});
test('leaving an original component performs no hidden-owner load',async()=>{
  const {workspace}=workspaceFixture(async()=>({records:[],message:'overview only'}));
  for(const property of ['contracts','cash','invoices'])workspace[property]={load:async()=>{throw new Error('hidden owner must not load');}};
  await workspace.select('overview');assert.equal(workspace.tab,'overview');
});
test('project invalidation also invalidates all three original owner adapters',()=>{
  const {workspace}=workspaceFixture(async()=>({records:[],message:''}));const invalidated=[];
  for(const property of ['bidding','personnel','contracts','cash','invoices'])workspace[property]={invalidate:()=>invalidated.push(property)};
  workspace.invalidate();assert.deepEqual(invalidated,['bidding','personnel','contracts','cash','invoices']);assert.equal(workspace.sequence,1);assert.equal(workspace.productionSequence,1);
});
test('a late generic projection cannot overwrite status after an original owner tab is selected',async()=>{
  let resolve;const {workspace,nodes}=workspaceFixture(()=>new Promise(r=>resolve=r));
  workspace.contracts={load:async()=>{}};
  const pending=workspace.select('overview');await workspace.select('contracts');
  nodes.get('[data-real-business-status]').textContent='owner status';
  resolve({records:[],message:'late overview'});await pending;
  assert.equal(nodes.get('[data-real-business-status]').textContent,'owner status');assert.equal(nodes.get('[data-real-business-reader]').hidden,true);
});
test('production selection requests only the selected native owner and updates URL state before its response',async()=>{
  const {workspace,selections}=workspaceFixture(async()=>{throw new Error('hidden production reader must not load');});
  let resolve;const calls=[];
  workspace.productionLayout={select:(key)=>{calls.push('layout-'+key);return key;}};
  workspace.productionSelected=(key,push)=>selections.push([key,push]);
  workspace.productionCosts={load:async()=>{calls.push('cost');}};
  workspace.productionBonus={load:()=>{calls.push('bonus');return new Promise(r=>resolve=r);}};
  const pending=workspace.production('bonus',true);
  assert.equal(workspace.productionTab,'bonus');assert.deepEqual(calls,['layout-bonus','bonus']);
  assert.deepEqual(selections,[['bonus',true]]);resolve();await pending;
  await workspace.production('cost');assert.deepEqual(calls,['layout-bonus','bonus','layout-cost','cost']);
});
test('navigating an unconnected OA stage surface does not issue an unrelated generic read',async()=>{
  const calls=[];const {workspace,selections}=workspaceFixture(async()=>{calls.push('generic');throw new Error('stage shell must not load the overview projection');});
  workspace.productionLayout={select:key=>key};workspace.productionSelected=(key,push)=>selections.push([key,push]);
  for(const key of ['preliminary','construction','change-completion','test','consultation'])await workspace.production(key,true);
  assert.deepEqual(calls,[]);assert.deepEqual(selections.map(row=>row[0]),['preliminary','construction','change-completion','test','consultation']);
});
test('all connected native stage surfaces dispatch to the same bounded Production stage workspace',async()=>{
  const calls=[];const {workspace}=workspaceFixture(async()=>{throw new Error('native stage must not use generic projection');});
  workspace.productionLayout={select:key=>key};workspace.productionSelected=()=>{};workspace.productionStage={load:async(stage,explicit)=>calls.push([stage,explicit])};
  for(const key of ['scheme','preliminary','construction','change-completion'])await workspace.production(key,true,true);
  assert.deepEqual(calls,[['scheme',true],['preliminary',true],['construction',true],['change-completion',true]]);
});
test('late production overview response cannot overwrite the selected original bonus panel',async()=>{
  let resolve;const {workspace,nodes}=workspaceFixture(()=>new Promise(r=>resolve=r));
  workspace.productionBonus={load:async()=>{}};
  const pending=workspace.production();await workspace.production('bonus');
  nodes.get('[data-real-production-status]').textContent='selected status';
  resolve({records:[],message:'late production overview'});await pending;
  assert.equal(nodes.get('[data-real-production-status]').textContent,'selected status');
});
test('production overview updates the reused OA summary from the owner projection and caches the read',async()=>{
  let reads=0;const {workspace,nodes}=workspaceFixture(async()=>({records:[],message:'owner progress '+(++reads),progress:{complete:false,requiredComplete:2,requiredTotal:3,stageComplete:2,stageTotal:4,typeComplete:0,typeTotal:1,types:[],stages:[]}}));
  workspace.productionLayout={select:key=>key};workspace.productionSelected=()=>{};
  await workspace.production();await workspace.production();
  assert.equal(reads,1);assert.equal(nodes.get('[data-real-production-summary-progress]').textContent,'0/1');assert.equal(nodes.get('[data-real-production-summary-state]').textContent,'进行中');assert.match(nodes.get('[data-real-production-summary-note]').textContent,/2\/4.*Production/);assert.equal(nodes.get('[data-real-production-status]').textContent,'owner progress 1');
});
