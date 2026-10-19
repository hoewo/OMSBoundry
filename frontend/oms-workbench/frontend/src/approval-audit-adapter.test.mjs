import assert from 'node:assert/strict';
import test from 'node:test';
import {ApprovalAuditAdapter} from './approval-audit-adapter.mjs';

const id=n=>`20000000-0000-4000-8000-${String(n).padStart(12,'0')}`;
const all=['create','read','amount.read','correct','withdraw','evidence.manage','history.read'].map(value=>'approval-audit-result.'+value);

function fixture(actions=all){
 const project={id:id(1),tenant_id:id(2),revision:4},records=new Map(),commands=new Map(),calls=[],storage=new Map();let serial=20,lost=false;
 const details=()=>({project,resource_links:[{target_service:'contract-ledger',resource_type:'contract',resource_id:id(4),source_revision:3,relation_key:'当前合同'}]});
 const options={tenantID:id(2),subjectID:id(3),details,newID:()=>id(serial++),storage:{getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,value),removeItem:key=>storage.delete(key)},evidenceReferenceRequest:async(file,version)=>({source_application:'evidence-service',tenant_id:id(2),file_id:file,version,status:'available',size_bytes:12,file_name:'result.pdf',media_type:'application/pdf',created_at:new Date(0).toISOString()}),transfer:{uploadFile:async()=>({version:{status:'available',fileId:id(7),version:2}})},applicationRequest:async(app,path,init={})=>{
  calls.push({app,path,scope:init.approvalScope,body:init.body});
  assert.equal(app,'approval-audit-results');
  assert.equal(init.approvalScope,project.id);
  if(init.method==='POST'){
   const input=JSON.parse(init.body),existing=path.includes('/corrections')?records.get(path.split('/').at(-2)):null;
   if(commands.has(input.command_id))return structuredClone(commands.get(input.command_id));
   const kind=existing?.kind||input.kind||'approval';
   const sourceRef=existing?.source||input.source_ref||{application:'contract-ledger',resource_type:'contract',resource_id:id(4),revision:3},source={...sourceRef,amount_components:existing?.source?.amount_components||[{kind:'design_fee',amount_minor:100},{kind:'survey_fee',amount_minor:20},{kind:'consultation_fee',amount_minor:5}],total_amount_minor:existing?.source?.total_amount_minor||125};
   const amount_components=input.amount_components?.length?input.amount_components:existing?.amount_components||source.amount_components,total_amount_minor=amount_components.reduce((sum,item)=>sum+item.amount_minor,0),evidence=(input.evidence_refs||existing?.evidence||[]).map(file=>({...file,digest:'sha256:test',display_name:'result.pdf'}));
   const confirmedAt=input.external_confirmed_at||existing?.external_confirmed_at||new Date(0).toISOString(),value={id:existing?.id||id(kind==='approval'?8:6),tenant_id:id(2),project:{id:id(1),revision:4},kind,state:path.endsWith('/withdrawal')?'withdrawn':'recorded',revision:(existing?.revision||0)+1,currency:'CNY',total_amount_minor,amount_components,source,evidence,external_confirmed_by:input.external_confirmed_by||existing?.external_confirmed_by||'甲方',external_confirmed_at:confirmedAt.replace('.000Z','Z'),allowed_actions:all};
   records.set(value.id,value);commands.set(input.command_id,structuredClone(value));if(lost){lost=false;throw new Error('committed reply lost');}return structuredClone(value);
  }
  if(path.endsWith('/history'))return {items:[{event_kind:'created',revision:1,occurred_at:new Date(0).toISOString()}]};
  return {project_id:project.id,items:[...records.values()],allowed_actions:actions};
 }};
 return {options,project,records,commands,calls,storage,loseReply:()=>{lost=true}};
}

test('approval and audit use exact project scope and linked immutable source',async()=>{
 const f=fixture(),adapter=new ApprovalAuditAdapter(f.options);await adapter.load();
 assert.equal(adapter.canCreate('approval'),true);
 await adapter.create('approval',{source_key:`contract-ledger:contract:${id(4)}:3`,confirmed_by:'甲方',confirmed_at:new Date(0).toISOString()});
 const input=JSON.parse(f.calls.find(call=>call.body).body);
 assert.deepEqual(input.project_ref,{id:id(1),revision:4});
 assert.deepEqual(input.source_ref,{application:'contract-ledger',resource_type:'contract',resource_id:id(4),revision:3});
 assert(f.calls.every(call=>call.scope===id(1)));
 assert.equal(adapter.canCreate('audit'),true);
 const audit=adapter.input('audit',{use_reference:true,report_file_ids:[],confirmed_by:'审计单位',confirmed_at:new Date(0).toISOString()});
 assert.deepEqual(audit.source_ref,{application:'approval-audit-results',resource_type:'approval-result',resource_id:id(8),revision:1});
});

test('approval adapter cannot turn missing capability into a write',async()=>{
 const f=fixture(['approval-audit-result.read']),adapter=new ApprovalAuditAdapter(f.options);await adapter.load();
 assert.equal(adapter.canCreate('approval'),false);
 await assert.rejects(adapter.create('approval',{source_key:'x',confirmed_by:'甲方',confirmed_at:new Date(0).toISOString()}),/权限|前置/);
 assert.equal(f.calls.filter(call=>call.body).length,0);
});

test('approval adapter rejects project switch before another request',async()=>{
 const f=fixture(),adapter=new ApprovalAuditAdapter(f.options);await adapter.load();
 f.options.details=()=>({project:{id:id(7),tenant_id:id(2),revision:1}});adapter.details=f.options.details;
 await assert.rejects(adapter.load(),/切换/);
 assert.equal(f.calls.length,1);
});

test('manual amounts and multiple Evidence versions remain exact in correction intent',async()=>{
 const f=fixture(),adapter=new ApprovalAuditAdapter(f.options);await adapter.load();
 const staged=await adapter.upload('approval',{name:'result.pdf',size:12,type:'application/pdf'});assert.equal(staged.file_id,id(7));
 await adapter.create('approval',{source_key:`contract-ledger:contract:${id(4)}:3`,use_reference:false,design_fee:'1.01',survey_fee:'2.02',consultation_fee:'3.03',override_reason:'书面调整',report_file_ids:[staged.id],confirmed_by:'甲方',confirmed_at:new Date(0).toISOString()});
 const current=adapter.current('approval'),view=adapter.list()[0];assert.equal(view.total_amount,'6.06');assert.equal(view.report_files.length,1);
 const correction=adapter.input('approval',{use_reference:false,design_fee:'1.01',survey_fee:'2.02',consultation_fee:'3.04',override_reason:'再次书面调整',report_file_ids:[view.report_files[0].id],confirmed_by:'甲方二次确认',confirmed_at:new Date(1).toISOString(),reason:'追加纠正'},current);
 assert.deepEqual(correction.amount_components,[{kind:'design_fee',amount_minor:101},{kind:'survey_fee',amount_minor:202},{kind:'consultation_fee',amount_minor:304}]);
 assert.deepEqual(correction.evidence_refs,[{file_id:id(7),version:2,slot:'approval_document'}]);
 assert.equal(correction.expected_revision,1);assert.equal(correction.reason,'追加纠正');
});

test('committed unknown reply retains and replays the exact Results command',async()=>{
 const f=fixture(),adapter=new ApprovalAuditAdapter(f.options);await adapter.load();f.loseReply();
 await assert.rejects(adapter.create('approval',{source_key:`contract-ledger:contract:${id(4)}:3`,use_reference:true,report_file_ids:[],confirmed_by:'甲方',confirmed_at:new Date(0).toISOString()}),/lost/);
 assert(adapter.pending);const first=f.calls.find(call=>call.body).body,reloaded=new ApprovalAuditAdapter(f.options);assert(reloaded.pending);
 await reloaded.retry();const second=f.calls.filter(call=>call.body).at(-1).body;assert.equal(second,first);assert.equal(f.records.size,1);assert.equal(f.commands.size,1);assert.equal(reloaded.pending,null);
});

test('tampered pending source type cannot issue an audit command',async()=>{
 const f=fixture(),adapter=new ApprovalAuditAdapter(f.options);await adapter.load();await adapter.create('approval',{source_key:`contract-ledger:contract:${id(4)}:3`,use_reference:true,report_file_ids:[],confirmed_by:'甲方',confirmed_at:new Date(0).toISOString()});
 const fields=adapter.input('audit',{use_reference:true,report_file_ids:[],confirmed_by:'审计',confirmed_at:new Date(0).toISOString()});adapter.pending={profile:'reference-oa-approval-audit-v1',tenant_id:id(2),subject_id:id(3),project_id:id(1),kind:'create',result_kind:'audit',fields:{...fields,source_ref:{...fields.source_ref,resource_type:'approval_audit_result'}}};
 const before=f.calls.filter(call=>call.body).length;await assert.rejects(adapter.continue(),/待续/);assert.equal(f.calls.filter(call=>call.body).length,before);
});
