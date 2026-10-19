import assert from 'node:assert/strict';
import test from 'node:test';
import {CommissionAdapter} from './commission-adapter.mjs';

const id=n=>`30000000-0000-4000-8000-${String(n).padStart(12,'0')}`;
const actions=['create','read','amount.read','correct','withdraw','assess','evidence.manage','history.read'].map(value=>'outsourcing-engagement.'+value);
const suggestionActions=['create','read','withdraw'].map(value=>'outsourcing-payable-suggestion.'+value);
const memoryStorage=()=>{const values=new Map();return{getItem:key=>values.get(key)||null,setItem:(key,value)=>values.set(key,value),removeItem:key=>values.delete(key)}};

function fixture({collectionActions=actions,partyFails=false,dropCreateResponse=false}={}){
 const project={id:id(1),tenant_id:id(2),revision:5},provider={id:id(4),tenant_id:id(2),revision:2,status:'active',legal_name:'测试受托方'},owner={id:id(5),tenant_id:id(2),revision:3,status:'active',legal_name:'测试本方'},records=new Map(),assessments=new Map(),suggestions=new Map(),receipts=new Map(),calls=[];let dropped=false;
 const response=(input,existing)=>({id:existing?.id||id(8),tenant_id:id(2),project:{id:project.id,revision:project.revision},provider:{id:input.provider_ref.id,revision:input.provider_ref.revision,legal_name:provider.legal_name},contact_name:input.contact_name||'',contact_phone:input.contact_phone||'',scope:input.scope,currency:'CNY',amount_minor:input.amount_minor,state:'recorded',revision:existing?existing.revision+1:1,note:input.note||'',contract:input.contract_ref?{...input.contract_ref,project_id:project.id,party_id:provider.id,state:'confirmed',digest:'sha256:test',display_summary:'测试合同'}:undefined,evidence:(input.evidence_refs||[]).map(ref=>({...ref,digest:'sha256:file',display_name:'委托合同.pdf'})),allowed_actions:actions});
 const options={tenantID:id(2),subjectID:id(3),details:()=>({project,resource_links:[{target_service:'contract-ledger',resource_type:'contract',resource_id:id(7),source_revision:9,relation_key:'外协合同'}]}),newID:(()=>{let n=20;return()=>id(n++)})(),storage:memoryStorage(),applicationRequest:async(app,path,init={})=>{
  calls.push({app,path,scope:init.commissionScope,body:init.body,method:init.method});
  if(app==='party-directory'){if(partyFails)throw new Error('主体目录暂不可用');return{items:[provider,owner],next_cursor:''};}
  assert.equal(app,'cost-outsourcing');assert.equal(init.commissionScope,project.id);
  if(path.startsWith('/api/v1/outsourcing-engagement-assessments'))return{project_id:project.id,items:[...assessments.values()]};
  if(path.startsWith('/api/v1/payable-suggestions')&&!init.method)return{project_id:project.id,items:[...suggestions.values()],allowed_actions:suggestionActions};
  if(path.endsWith('/history'))return{items:[{event_kind:'recorded',revision:1,occurred_at:new Date(0).toISOString(),snapshot:[...records.values()][0]}]};
  if(path.endsWith('/assessments')&&init.method==='POST'){
   const input=JSON.parse(init.body),value={id:id(40),tenant_id:id(2),engagement_id:path.split('/')[4],engagement_revision:input.expected_engagement_revision,rule_key:'reference-oa-five-point-v1',score:input.score,reason:input.reason,revision:input.expected_assessment_revision+1};assessments.set(value.engagement_id,value);return structuredClone(value);
  }
  if(path==='/api/v1/payable-suggestions'&&init.method==='POST'){
   const input=JSON.parse(init.body),source=records.get(input.source_id),value={id:id(41),tenant_id:id(2),project:{id:project.id,revision:project.revision},source:{type:'outsourcing_engagement',id:source.id,revision:source.revision,digest:'sha256:source'},ledger_owner:{id:owner.id,revision:owner.revision,legal_name:owner.legal_name},counterparty:source.provider,amount_minor:source.amount_minor,currency:'CNY',due_on:input.due_on,purpose:input.purpose,state:'proposed',revision:2,request_digest:'sha256:suggestion'};suggestions.set(source.id,value);return structuredClone(value);
  }
  if(path==='/api/v1/outsourcing-engagements'&&init.method==='POST'){
   const input=JSON.parse(init.body);let value=receipts.get(input.command_id);if(!value){value=response(input);receipts.set(input.command_id,value);records.set(value.id,value);}if(dropCreateResponse&&!dropped){dropped=true;throw new Error('响应丢失');}return structuredClone(value);
  }
  if(path.startsWith('/api/v1/outsourcing-engagements/')&&path.endsWith('/corrections')&&init.method==='POST'){
   const input=JSON.parse(init.body),current=records.get(path.split('/')[4]),value=response(input,current);value.id=current.id;records.set(value.id,value);return structuredClone(value);
  }
  if(path.startsWith('/api/v1/outsourcing-engagements')&&!init.method)return{project_id:project.id,items:[...records.values()],allowed_actions:collectionActions,suggestion_allowed_actions:suggestionActions};
  throw new Error(`unexpected ${init.method||'GET'} ${path}`);
 }};
 return{options,records,assessments,suggestions,calls,provider,owner,project};
}

test('commission create freezes contact, party and exact contract/evidence input',async()=>{
 const f=fixture(),adapter=new CommissionAdapter(f.options);await adapter.load();assert.equal(adapter.canCreate(),true);
 const file={id:`commission-file:${id(6)}:2:outsourcing_contract`,file_id:id(6),version:2,slot:'outsourcing_contract',original_name:'委托合同.pdf',file_name:'委托合同.pdf',file_size:12,mime_type:'application/pdf'};adapter.files.set(file.id,file);
 await adapter.create({provider_id:f.provider.id,contact_name:'李工',contact_phone:'13800000000',scope:'外业协作',amount:'12.34',contract_key:`contract-ledger:contract:${id(7)}:9`,contract_file_ids:[file.id],note:'OA 委托'});
 const input=JSON.parse(f.calls.find(call=>call.method==='POST'&&call.path==='/api/v1/outsourcing-engagements').body);
 assert.deepEqual(input.project_ref,{id:f.project.id,revision:5});assert.deepEqual(input.provider_ref,{id:f.provider.id,revision:2});assert.equal(input.amount_minor,1234);assert.equal(input.contact_name,'李工');assert.equal(input.contact_phone,'13800000000');assert.equal(input.contract_ref.resource_id,id(7));assert.deepEqual(input.evidence_refs,[{file_id:id(6),version:2,slot:'outsourcing_contract'}]);
 assert.equal(adapter.list()[0].contact_name,'李工');assert(f.calls.filter(call=>call.app==='cost-outsourcing').every(call=>call.scope===f.project.id));
});

test('commission latest assessment projection and payable suggestion stay owner-native',async()=>{
 const f=fixture(),adapter=new CommissionAdapter(f.options);await adapter.load();await adapter.create({provider_id:f.provider.id,scope:'外业协作',amount:'50.00'});const row=adapter.list()[0];
 await adapter.assess(row.id,5,'成果完整');assert.equal(adapter.summary().average_score,5);
 await adapter.propose(row.id,{ledger_owner_id:f.owner.id,due_on:'2026-10-31',purpose:'外协应付款'});const updated=adapter.list()[0];assert.equal(updated.suggestion.state,'proposed');assert.equal(updated.suggestion.amount_minor,5000);assert.equal(updated.score,5);
 assert.equal(f.calls.filter(call=>call.path.startsWith('/api/v1/outsourcing-engagement-assessments')).length,4,'每轮 load 只用一个批量评分投影，不按委托 N+1');
});

test('unknown create response keeps original command and retry does not duplicate',async()=>{
 const f=fixture({dropCreateResponse:true}),adapter=new CommissionAdapter(f.options);await adapter.load();await assert.rejects(adapter.create({provider_id:f.provider.id,scope:'外业协作',amount:'1.00'}),/响应丢失/);assert(adapter.pending);const command=adapter.pending.fields.command_id;await adapter.retry();assert.equal(f.records.size,1);const posts=f.calls.filter(call=>call.method==='POST'&&call.path==='/api/v1/outsourcing-engagements').map(call=>JSON.parse(call.body).command_id);assert.deepEqual(posts,[command,command]);assert.equal(adapter.pending,null);
});

test('commission receipt mismatch identifies the frozen field',async()=>{
 const f=fixture(),original=f.options.applicationRequest;f.options.applicationRequest=async(app,path,init={})=>{const value=await original(app,path,init);if(app==='cost-outsourcing'&&path==='/api/v1/outsourcing-engagements'&&init.method==='POST')return{...value,amount_minor:value.amount_minor+1};return value};
 const adapter=new CommissionAdapter(f.options);await adapter.load();await assert.rejects(adapter.create({provider_id:f.provider.id,scope:'外业协作',amount:'1.00'}),/原意图不一致：金额/);assert(adapter.pending);
});

test('omitted optional contact values equal the empty original intent',async()=>{
 const f=fixture(),original=f.options.applicationRequest;f.options.applicationRequest=async(app,path,init={})=>{const value=await original(app,path,init);if(app==='cost-outsourcing'&&path==='/api/v1/outsourcing-engagements'&&init.method==='POST'){delete value.contact_name;delete value.contact_phone;}return value};
 const adapter=new CommissionAdapter(f.options);await adapter.load();const created=await adapter.create({provider_id:f.provider.id,scope:'外业协作',amount:'1.00'});assert.equal(created.contact_name,'');assert.equal(created.contact_phone,'');assert.equal(adapter.pending,null);
});

test('commission missing create capability stays read only',async()=>{
 const f=fixture({collectionActions:['outsourcing-engagement.read']}),adapter=new CommissionAdapter(f.options);await adapter.load();assert.equal(adapter.canCreate(),false);await assert.rejects(adapter.create({}),/权限|目录/);assert.equal(f.calls.filter(call=>call.body).length,0);
});

test('party directory failure does not hide existing commission facts',async()=>{
 const f=fixture({partyFails:true});f.records.set(id(8),{id:id(8),tenant_id:id(2),project:{id:id(1),revision:5},provider:{id:id(4),revision:2,legal_name:'测试受托方'},scope:'外业协作',currency:'CNY',amount_minor:100,state:'recorded',revision:1,note:'',evidence:[],allowed_actions:actions});const adapter=new CommissionAdapter(f.options);await adapter.load();assert.equal(adapter.list().length,1);assert.equal(adapter.canCreate(),false);assert.match(adapter.partyError,/目录/);
});
