import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
const context=vm.createContext({document:{createElement:()=>({textContent:'',className:'',dataset:{},children:[],append(child){this.children.push(child);}})}});
vm.runInContext(readFileSync(new URL('./reference_oa_bidding_runtime.js',import.meta.url),'utf8'),context);
const {BiddingFlow,BiddingWorkspace}=context.ReferenceOABidding;
const tenant='11111111-1111-4111-8111-111111111111',projectID='22222222-2222-4222-8222-222222222222',engagementID='33333333-3333-4333-8333-333333333333',other='44444444-4444-4444-8444-444444444444',fileID='55555555-5555-4555-8555-555555555555',secondFile='66666666-6666-4666-8666-666666666666';
const copy=value=>JSON.parse(JSON.stringify(value));
const append=(id=fileID)=>({kind:'material',operation:'append',requirementKey:'bid_document',fileID:id,version:1});
function fixture() {
  let sequence=20,fail=null;
  const calls=[],receipts=new Map();
  const project={project:{id:projectID,tenant_id:tenant,status:'active',revision:4},allowed_actions:[],resource_links:[{target_service:'business-engagement',resource_type:'business-engagement',resource_id:engagementID}]};
  const source={engagement:{id:engagementID,project_id:projectID,tenant_id:tenant,revision:5,status:'collecting_evidence',applicability:'applicable',method:'tender'},allowed_actions:['business-engagement.read','business-engagement.evidence.manage','business-engagement.method.select','business-engagement.applicability.manage'],materials:[],requirements:[]};
  const requirements=()=>{source.requirements=['tender_document','bid_document','award_document'].map(key=>({key,required:true,material_references:source.materials.filter(row=>row.requirement_key===key&&!row.superseded_at)}));};requirements();
  const reference={schema_version:'1',source_application:'evidence-service',resource_type:'evidence-file',id:fileID,file_id:fileID,tenant_id:tenant,status:'available',version:1,revision:1};
  const request=async(app,path,options)=>{
    assert.equal(app,'business-engagement');calls.push({owner:app,path,options});
    if(!options)return path.includes('?')?{project_id:projectID,items:[copy(source.engagement)],allowed_actions:copy(source.allowed_actions)}:copy(source);
    assert.equal(path,`/api/v1/business-engagements/${engagementID}/materials`);
    const payload=JSON.parse(options.body);
    if(receipts.has(payload.command_id))return copy(receipts.get(payload.command_id));
    if(fail?.before){const error=fail.error;fail=null;throw error;}
    assert.equal(payload.expected_revision,source.engagement.revision);
    source.engagement.revision++;
    if(payload.operation==='append')source.materials.push({id:`99999999-9999-4999-8999-${String(++sequence).padStart(12,'0')}`,tenant_id:tenant,engagement_id:engagementID,requirement_key:payload.requirement_key,evidence_file_id:payload.evidence_file_id,evidence_version:payload.evidence_version,evidence_source_revision:payload.evidence_source_revision,display_snapshot:'投标材料.pdf',created_engagement_revision:source.engagement.revision});
    else {assert.equal(payload.operation,'remove_one');const row=source.materials.find(row=>row.id===payload.material_reference_id);assert.ok(row&&!row.superseded_at);row.superseded_at='2026-10-03';row.superseded_engagement_revision=source.engagement.revision;}
    requirements();const receipt=copy(source);delete receipt.allowed_actions;receipts.set(payload.command_id,receipt);
    if(fail?.after){const error=fail.error;fail=null;throw error;}
    return receipt;
  };
  const flow=new BiddingFlow({tenantID:tenant,applicationRequest:request,projectRequest:async()=>{throw new Error('material commands must not write or reread Project');},projectAuthorize:async()=>{throw new Error('no Project authorization needed');},randomUUID:()=>`88888888-8888-4888-8888-${String(++sequence).padStart(12,'0')}`,evidenceReferenceRequest:async(id,version)=>{calls.push({owner:'evidence-read',id,version});return {...reference,id,file_id:id,version,revision:version};}});
  return {flow,project,source,reference,calls,requirements,fail:value=>{fail=value;},writes:()=>calls.filter(row=>row.options)};
}
async function loaded() {const f=fixture();await f.flow.load(f.project);return f;}
test('material action uses the real native capability catalog',()=>{
  const catalog=JSON.parse(readFileSync(new URL('../../../../contracts/access/v1/examples/business-engagement-capabilities.json',import.meta.url),'utf8'));
  assert.ok(catalog.capabilities.some(row=>row.id==='business-engagement.evidence.manage'));
  const native=readFileSync(new URL('../../../../backend/services/business-engagement/internal/httpserver/server.go',import.meta.url),'utf8');
  assert.match(native,/POST \/api\/v1\/business-engagements\/\{engagement_id\}\/materials/);
});
test('two exact completed versions append without replacing and single removal preserves the other',async()=>{
  const f=await loaded();await f.flow.material(f.project,append());await f.flow.material(f.project,append(secondFile));
  assert.equal(f.source.materials.filter(row=>!row.superseded_at).length,2);
  const first=f.source.materials[0];await f.flow.material(f.project,{kind:'material',operation:'remove_one',requirementKey:'bid_document',materialID:first.id});
  assert.equal(f.source.materials[0].superseded_engagement_revision,8);assert.equal(f.source.materials[1].superseded_at,undefined);
  assert.equal(f.calls.filter(row=>row.owner==='evidence-read').length,2);assert.equal(f.writes().length,3);assert.equal(f.project.project.revision,4);
  for(const row of f.writes())assert.equal(JSON.parse(row.options.body).operation==='put'||JSON.parse(row.options.body).operation==='remove',false);
});
test('refresh and an already registered version write zero and do not revisit Evidence',async()=>{
  const f=await loaded();await f.flow.material(f.project,append());const writes=f.writes().length,reads=f.calls.filter(row=>row.owner==='evidence-read').length;
  await f.flow.load(f.project);await assert.rejects(f.flow.material(f.project,append()),/已登记/);assert.equal(f.writes().length,writes);assert.equal(f.calls.filter(row=>row.owner==='evidence-read').length,reads);
});
for(const [field,value] of [['tenant_id',other],['id',other],['file_id',other],['version',2],['revision',2],['status','quarantined'],['source_application','other'],['schema_version','2']])test('unconfirmed Evidence '+field+' fails before BE write',async()=>{
  const f=await loaded(),read=f.flow.evidenceReferenceRequest;f.flow.evidenceReferenceRequest=async(...args)=>({...await read(...args),[field]:value});await assert.rejects(f.flow.material(f.project,append()),/未确认|不属于/);assert.equal(f.writes().length,0);
});
test('Evidence outage keeps the exact File/Version intention, retries read only and never uploads',async()=>{
  const f=await loaded(),read=f.flow.evidenceReferenceRequest;let once=true;
  f.flow.evidenceReferenceRequest=async(...args)=>{if(once){once=false;throw new Error('reference unavailable');}return read(...args);};
  await assert.rejects(f.flow.material(f.project,append()),/unavailable/);assert.equal(f.writes().length,0);
  await assert.rejects(f.flow.material(f.project,append(secondFile)),/上次操作/);
  await f.flow.material(f.project,append());assert.equal(f.writes().length,1);assert.equal(f.source.materials[0].evidence_file_id,fileID);
});
for(const operation of ['append','remove_one'])test('lost '+operation+' reply and later read replay the original command with no extra fact',async()=>{
  const f=await loaded();let intent=append();
  if(operation==='remove_one'){await f.flow.material(f.project,append());intent={kind:'material',operation,requirementKey:'bid_document',materialID:f.source.materials[0].id};}
  const before=f.writes().length;f.fail({after:true,error:new Error('reply lost')});
  await assert.rejects(f.flow.material(f.project,intent),/lost/);const frozen=f.writes().at(-1).options.body;
  await f.flow.load(f.project);await f.flow.material(f.project,intent);
  assert.equal(f.writes().length,before+2);assert.equal(f.writes().at(-1).options.body,frozen);assert.equal(f.source.engagement.revision,operation==='append'?6:7);
});
test('lost native permission reread retains original command even when read denial is 403',async()=>{
  const f=await loaded(),request=f.flow.applicationRequest;let once=true;
  f.flow.applicationRequest=async(app,path,options)=>{if(!options&&!path.includes('?')&&f.writes().length&&once){once=false;throw Object.assign(new Error('native read denied'),{status:403});}return request(app,path,options);};
  await assert.rejects(f.flow.material(f.project,append()),/denied/);const payload=f.writes().at(-1).options.body;
  await f.flow.material(f.project,append());assert.equal(f.writes().at(-1).options.body,payload);assert.equal(f.source.materials.length,1);
});
test('wrong requirement or foreign/superseded reference cannot remove, with no Evidence calls',async()=>{
  const f=await loaded();await f.flow.material(f.project,append());const writes=f.writes().length,reads=f.calls.filter(row=>row.owner==='evidence-read').length;
  for(const intent of [{kind:'material',operation:'remove_one',requirementKey:'award_document',materialID:f.source.materials[0].id},{kind:'material',operation:'remove_one',requirementKey:'bid_document',materialID:other}])await assert.rejects(f.flow.material(f.project,intent),/有效材料/);
  assert.equal(f.writes().length,writes);assert.equal(f.calls.filter(row=>row.owner==='evidence-read').length,reads);
  await f.flow.material(f.project,{kind:'material',operation:'remove_one',requirementKey:'bid_document',materialID:f.source.materials[0].id});
  await assert.rejects(f.flow.material(f.project,{kind:'material',operation:'remove_one',requirementKey:'bid_document',materialID:f.source.materials[0].id}),/有效材料/);assert.equal(f.writes().length,writes+1);
});
for(const scenario of ['read-only','unlinked','not-applicable','unselected','not-required','inactive'])test('material guard '+scenario+' writes zero',async()=>{
  const f=fixture();
  if(scenario==='read-only')f.source.allowed_actions=['business-engagement.read'];
  if(scenario==='unlinked')f.project.resource_links=[];
  if(scenario==='not-applicable')f.source.engagement.applicability='not_applicable';
  if(scenario==='unselected')delete f.source.engagement.method;
  if(scenario==='not-required')f.source.requirements.find(row=>row.key==='bid_document').required=false;
  if(scenario==='inactive')f.project.project.status='cancelled';
  await f.flow.load(f.project);await assert.rejects(f.flow.material(f.project,append()),/不能修改/);assert.equal(f.writes().length,0);assert.equal(f.calls.filter(row=>row.owner==='evidence-read').length,0);
});
test('uncertain material intent prevents method changes and a 409 requires explicit reread',async()=>{
  const f=await loaded();f.fail({before:true,error:Object.assign(new Error('stale'),{status:409})});
  await assert.rejects(f.flow.material(f.project,append()),/stale/);await assert.rejects(f.flow.material(f.project,append()),/重新读取/);await assert.rejects(f.flow.apply(f.project,{kind:'method',method:'comparison'}),/重新读取/);assert.equal(f.writes().length,1);
});
test('cross-record material in a source response fails closed before showing it',async()=>{
  const f=await loaded();await f.flow.material(f.project,append());f.source.materials[0].engagement_id=other;
  await assert.rejects(f.flow.load(f.project),/不一致/);
});
test('workspace renders every current member, supports individual removal and gates legacy sources',async()=>{
  const f=await loaded();await f.flow.material(f.project,append());await f.flow.material(f.project,append(secondFile));
  const fields=Object.fromEntries(['method','status','requirements','applicability','retry','reload'].map(key=>['[data-real-bidding-'+key+']',{textContent:'',value:'',hidden:false,disabled:false,children:[],replaceChildren(){this.children=[];},append(child){this.children.push(child);}}]));
  const root={addEventListener(){},querySelector:selector=>fields[selector],querySelectorAll:()=>['method','applicability','retry'].map(key=>fields['[data-real-bidding-'+key+']'])};
  const ui=new BiddingWorkspace({root,flow:f.flow,details:()=>f.project});await ui.load();
  const rows=()=>fields['[data-real-bidding-requirements]'].children.filter(row=>row.className==='bidding-file-slot__file-entry');
  assert.equal(rows().length,2);assert.equal(rows()[0].children[0].dataset.realBiddingMaterialRemove,f.source.materials[0].id);assert.equal(rows()[1].children[0].disabled,false);
  delete ui.state.source.requirements.find(row=>row.key==='bid_document').material_references;ui.render();assert.equal(rows()[0].children[0].disabled,true);assert.equal(rows()[1].children[0].disabled,true);
});
test('missing Evidence adapter preserves the completed file identity and cannot silently append',async()=>{
  const f=await loaded();delete f.flow.evidenceReferenceRequest;await assert.rejects(f.flow.material(f.project,append()),/尚未配置/);assert.equal(f.writes().length,0);assert.equal(f.flow.states.get(projectID).intent.fileID,fileID);
});
test('a hundred current references reject another append before Evidence or business writes',async()=>{
  const f=fixture();for(let i=0;i<100;i++)f.source.materials.push({id:`77777777-7777-4777-8777-${String(i+1).padStart(12,'0')}`,tenant_id:tenant,engagement_id:engagementID,requirement_key:'award_document',evidence_file_id:secondFile,evidence_version:i+1,evidence_source_revision:i+1});f.requirements();await f.flow.load(f.project);
  await assert.rejects(f.flow.material(f.project,append()),/100 条/);assert.equal(f.writes().length,0);assert.equal(f.calls.filter(row=>row.owner==='evidence-read').length,0);
});
test('uncertain append blocks a different material and method intention',async()=>{
  const f=await loaded();f.fail({after:true,error:new Error('reply lost')});await assert.rejects(f.flow.material(f.project,append()),/lost/);
  await assert.rejects(f.flow.material(f.project,append(secondFile)),/上次操作/);await assert.rejects(f.flow.apply(f.project,{kind:'method',method:'comparison'}),/上次操作/);assert.equal(f.writes().length,1);
});
