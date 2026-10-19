import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { BiddingAdapter } from './bidding-adapter.mjs';
const context={crypto:{randomUUID:()=>crypto.randomUUID()}};
vm.runInNewContext(fs.readFileSync(new URL('../../internal/server/reference_oa_bidding_runtime.js',import.meta.url),'utf8'),context);
const tenant='11111111-1111-4111-8111-111111111111', projectID='22222222-2222-4222-8222-222222222222', engagementID='33333333-3333-4333-8333-333333333333',fileID='44444444-4444-4444-8444-444444444444';
const clone=value=>JSON.parse(JSON.stringify(value));
function fixture({storage,metadataError,actions=['business-engagement.evidence.manage','business-engagement.method.select','business-engagement.applicability.manage']}={}) {
  const writes=[],evidenceReads=[],uploads=[],commands=new Map();let sequence=0,failAfter=false,failRead=false;
  const details={project:{id:projectID,tenant_id:tenant,revision:4,status:'active'},allowed_actions:[],resource_links:[{target_service:'business-engagement',resource_type:'business-engagement',resource_id:engagementID}]};
  const source={engagement:{id:engagementID,tenant_id:tenant,project_id:projectID,revision:1,status:'collecting',method:'tender',applicability:'applicable'},allowed_actions:actions,materials:[],requirements:[]};
  const requirements=()=>source.requirements=['tender_document','bid_document','award_document'].map(key=>({key,required:key!=='tender_document'||source.engagement.method==='tender',satisfied:source.materials.some(row=>row.requirement_key===key&&!row.superseded_at)}));requirements();
  const reference=(id,version)=>({schema_version:'1',source_application:'evidence-service',resource_type:'evidence-file',id,file_id:id,tenant_id:tenant,version,revision:version,status:'available',file_name:`材料-v${version}.pdf`,size_bytes:17,media_type:'application/pdf'});
  const evidenceReferenceRequest=async(id,version)=>{evidenceReads.push([id,version]);if(metadataError)throw metadataError;return reference(id,version)};
  const request=async(_app,path,options)=>{
    if(!options) {if(failRead)throw Object.assign(new Error('read denied'),{status:403});return clone(path.includes('?')?{project_id:projectID,items:[source.engagement],allowed_actions:actions}:source)}
    const body=JSON.parse(options.body);writes.push(clone(body));
    if(!commands.has(body.command_id)){
      if(body.expected_revision!==source.engagement.revision)throw Object.assign(new Error('conflict'),{status:409});
      source.engagement.revision++;
      if(path.endsWith('/materials')){
        if(body.operation==='append')source.materials.push({id:`55555555-5555-4555-8555-${String(++sequence).padStart(12,'0')}`,tenant_id:tenant,engagement_id:engagementID,requirement_key:body.requirement_key,evidence_file_id:body.evidence_file_id,evidence_version:body.evidence_version,evidence_source_revision:body.evidence_source_revision,created_at:'2026-10-03T00:00:00Z'});
        else source.materials.find(row=>row.id===body.material_reference_id).superseded_at='2026-10-03T01:00:00Z';
      }else if(path.endsWith('/method'))source.engagement.method=body.method;
      commands.set(body.command_id,clone(source.engagement));requirements();
    }
    if(failAfter){failAfter=false;throw new Error('reply lost');}
    return {engagement:clone(commands.get(body.command_id))};
  };
  const flow=new context.ReferenceOABidding.BiddingFlow({tenantID:tenant,projectRequest:()=>{throw new Error('no Project writes')},projectAuthorize:()=>{throw new Error('no Project authorization')},applicationRequest:request,evidenceReferenceRequest,randomUUID:()=>crypto.randomUUID()});
  const transfer={uploadFile:async({file,clientContext})=>{uploads.push({file,clientContext});return {version:{fileId:fileID,version:uploads.length,status:'available',createdAt:'2026-10-03T00:00:00Z'}}},clearCache(){}};
  const create=()=>new BiddingAdapter({flow,details:()=>details,evidenceReferenceRequest,transfer,tenantID:tenant,subjectID:'test-actor',storage});
  const adapter=create();return {adapter,create,flow,details,source,writes,uploads,evidenceReads,reference,setFailAfter:()=>{failAfter=true},setFailRead:value=>{failRead=value}};
}
const memory=()=>{const values=new Map();return {values,getItem:k=>values.get(k)||null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)}};
const upload=async f=>f.adapter.upload(projectID,new Blob(['native Evidence']),'tender');
test('mount/read never creates, links or mutates; original three file lists',async()=>{const f=fixture();await f.adapter.load();assert.equal(f.writes.length,0);assert.equal(f.adapter.info().tender_files.length,0);assert.equal(f.adapter.info().tender_file_required,true)});
test('SDK upload -> native metadata -> append; another version of same File remains separate',async()=>{
  const f=fixture();await f.adapter.load();for(let i=0;i<2;i++){const file=await upload(f);await f.adapter.append(projectID,'tender',file.id)}
  assert.equal(f.uploads.length,2);assert.deepEqual(f.uploads[0].clientContext,{ownerRef:projectID,purpose:'tender_document'});assert.deepEqual(f.writes.map(x=>x.evidence_version),[1,2]);assert.equal(f.adapter.info().tender_files.length,2);assert.notEqual(f.adapter.info().tender_files[0].id,f.adapter.info().tender_files[1].id);assert.equal(f.adapter.pending.size,0);
});
test('single remove uses reference UUID and never invokes Evidence deletion',async()=>{const f=fixture();await f.adapter.load();for(let i=0;i<2;i++){const file=await upload(f);await f.adapter.append(projectID,'tender',file.id)}const [a,b]=f.adapter.info().tender_files;await f.adapter.remove(projectID,'tender',a.id);assert.deepEqual(f.adapter.info().tender_files.map(x=>x.id),[b.id]);assert.equal(f.writes.at(-1).operation,'remove_one');assert.equal(f.writes.at(-1).material_reference_id,a.id);assert.equal(f.uploads.length,2)});
test('cannot infer whole-array replacement or remove foreign reference',async()=>{const f=fixture();await f.adapter.load();await assert.rejects(f.adapter.remove(projectID,'tender',engagementID));assert.equal(f.writes.length,0)});
test('native action denial prevents SDK upload',async()=>{const f=fixture({actions:[]});await f.adapter.load();assert.equal(f.adapter.permissions().canUploadBusinessFile,false);await assert.rejects(upload(f));assert.equal(f.uploads.length,0)});
test('wrong UI Project cannot upload or append',async()=>{const f=fixture();await f.adapter.load();await assert.rejects(f.adapter.upload(engagementID,new Blob(['x']),'bid'));assert.throws(()=>f.adapter.append(engagementID,'bid','x'));assert.equal(f.uploads.length,0);assert.equal(f.writes.length,0)});
test('unknown response keeps original material command ID and payload; retry does not reupload',async()=>{const f=fixture();await f.adapter.load();const file=await upload(f);f.setFailAfter();await assert.rejects(f.adapter.append(projectID,'tender',file.id));assert.equal(f.adapter.pending.size,1);await f.adapter.retry();assert.equal(f.uploads.length,1);assert.deepEqual(f.writes[0],f.writes[1]);assert.equal(f.source.materials.length,1)});
test('explicit reread reconciles committed upload instead of minting duplicate append',async()=>{const f=fixture();await f.adapter.load();const file=await upload(f);f.setFailAfter();await assert.rejects(f.adapter.append(projectID,'tender',file.id));await f.adapter.load(true);assert.equal(f.writes.length,1);assert.equal(f.adapter.state.intent,null);assert.equal(f.adapter.info().tender_files.length,1)});
test('uploaded identity survives reload; scoped session data contains no credentials',async()=>{const storage=memory(),f=fixture({storage});await f.adapter.load();const file=await upload(f);const stored=[...storage.values.values()][0];assert.doesNotMatch(stored,/securityToken|accessKey|password|authorization/);const next=f.create();await next.load();assert.equal(next.pending.size,1);await next.append(projectID,'tender',file.id);assert.equal(f.uploads.length,1);assert.equal(storage.values.size,0)});
test('uncertain native command survives reload with same command ID',async()=>{const storage=memory(),f=fixture({storage});await f.adapter.load();const file=await upload(f);f.setFailAfter();await assert.rejects(f.adapter.append(projectID,'tender',file.id));const next=f.create();await next.load();await next.retry();assert.deepEqual(f.writes[0],f.writes[1]);assert.equal(f.source.materials.length,1)});
test('storage failure is memory-only fallback, not permission bypass',async()=>{const storage={getItem(){throw new Error('disabled')},setItem(){throw new Error('disabled')},removeItem(){throw new Error('disabled')}},f=fixture({storage});await f.adapter.load();const file=await upload(f);await f.adapter.append(projectID,'tender',file.id);assert.equal(f.writes.length,1)});
test('metadata denied after SDK completion retains uploaded identity and performs zero BE writes',async()=>{const f=fixture({metadataError:Object.assign(new Error('metadata denied'),{status:403})});await f.adapter.load();await assert.rejects(upload(f));assert.equal(f.adapter.pending.size,1);assert.equal(f.writes.length,0)});
test('different Tenant metadata never becomes a view or registration',async()=>{const f=fixture();await f.adapter.load();f.adapter.evidenceReferenceRequest=async(id,version)=>({...f.reference(id,version),tenant_id:projectID});await assert.rejects(upload(f));assert.equal(f.writes.length,0)});
test('metadata read failure for a connected file is not rendered as empty materials',async()=>{const f=fixture();await f.adapter.load();const file=await upload(f);await f.adapter.append(projectID,'tender',file.id);f.adapter.evidenceReferenceRequest=async()=>{throw new Error('denied')};await assert.rejects(f.adapter.load());assert.equal(f.writes.length,1)});
test('method update changes requirements through BE; refresh adds zero revisions',async()=>{const f=fixture();await f.adapter.load();await f.adapter.updateMethod(projectID,'comparison');assert.equal(f.adapter.info().tender_file_required,false);const revision=f.source.engagement.revision;await f.adapter.load(true);assert.equal(f.source.engagement.revision,revision)});
test('invalid session identity and old pending receipts cannot issue commands',async()=>{const storage=memory(),f=fixture({storage});storage.setItem(f.adapter.storageKey,JSON.stringify({schema_version:1,saved_at:Date.now(),pending:[['bad',{file_id:'../escape',version:1,category:'tender',created_at:'now'}]]}));assert.equal(f.create().pending.size,0);storage.setItem(f.adapter.storageKey,JSON.stringify({schema_version:1,saved_at:0,pending:[]}));assert.equal(f.create().pending.size,0)});
