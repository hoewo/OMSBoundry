import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
const context=vm.createContext({document:{createElement:()=>({textContent:'',className:'',dataset:{},children:[],append(child){this.children.push(child);}})}});
vm.runInContext(readFileSync(new URL('./reference_oa_bidding_runtime.js',import.meta.url),'utf8'),context);
const {BiddingFlow,BiddingWorkspace}=context.ReferenceOABidding;
const tenant='11111111-1111-4111-8111-111111111111', id='22222222-2222-4222-8222-222222222222', resource='33333333-3333-4333-8333-333333333333', other='44444444-4444-4444-8444-444444444444';
const capabilities=['create','read','applicability.manage','method.select'].map(key=>'business-engagement.'+key);
const projectLinkCapability='project-workspace.resource-link.manage';
const clone=value=>JSON.parse(JSON.stringify(value));
function setup({existing=false,linked=false,denied=false,fault=null}={}) {
  let project={project:{id,tenant_id:tenant,status:'active',revision:3},allowed_actions:denied?[]:[projectLinkCapability],resource_links:[]};
  let source=existing?{engagement:{id:resource,tenant_id:tenant,project_id:id,revision:1,status:'draft',applicability:'undetermined'},allowed_actions:denied?['business-engagement.read']:capabilities,requirements:[],materials:[]}:null;
  if(linked)project.resource_links.push({target_service:'business-engagement',resource_type:'business-engagement',resource_id:resource});
  const calls=[],receipts=new Map();let sequence=10, fail=fault;
  const details=()=>clone(project);
  const app=async(application,path,options)=>{
    assert.equal(application,'business-engagement');calls.push({owner:'business',path,options});
    if(!options)return path.includes('?')?{project_id:id,items:source?[clone(source.engagement)]:[],allowed_actions:denied?['business-engagement.read']:capabilities}:clone(source);
    const payload=JSON.parse(options.body),kind=path.endsWith('/applicability')?'applicability':path.endsWith('/method')?'method':'create';
    if(fail?.stage===kind&&!fail.after){const error=fail.error;fail=null;throw error;}
    if(receipts.has(payload.command_id))return clone(receipts.get(payload.command_id));
    if(kind==='create'){assert.equal(source,null);source={engagement:{id:resource,tenant_id:tenant,project_id:id,revision:1,status:'draft',applicability:'undetermined'},allowed_actions:capabilities,requirements:[],materials:[]};}
    else {assert.equal(payload.expected_revision,source.engagement.revision);source.engagement.revision++;
      if(kind==='applicability'){source.engagement.applicability=payload.applicable?'applicable':'not_applicable';source.engagement.status=payload.applicable?'draft':'not_applicable';}
      else{source.engagement.method=payload.method;source.engagement.status='collecting_evidence';}}
    // Native command replies contain domain Details, not allowed_actions.
    // Only the subsequent authenticated GET returns current executable actions.
    const reply=clone(source);delete reply.allowed_actions;
    receipts.set(payload.command_id,reply);
    if(fail?.stage===kind&&fail.after){const error=fail.error;fail=null;throw error;}
    return clone(reply);
  };
  const projectRequest=async(path,options)=>{
    calls.push({owner:'project',path,options});if(!options)return clone(project);
    const payload=JSON.parse(options.body);
    if(fail?.stage==='link'&&!fail.after){const error=fail.error;fail=null;throw error;}
    if(receipts.has(payload.command_id))return clone(receipts.get(payload.command_id));
    assert.equal(payload.expected_revision,project.project.revision);
    assert.equal(payload.resource_id,resource);assert.equal(payload.operation,'link');
    project.project.revision++;const link={id:other,target_service:payload.target_service,resource_type:payload.resource_type,resource_id:payload.resource_id};project.resource_links.push(link);
    const result={project:clone(project.project),resource_link:clone(link)};receipts.set(payload.command_id,result);
    if(fail?.stage==='link'&&fail.after){const error=fail.error;fail=null;throw error;}
    return clone(result);
  };
  const flow=new BiddingFlow({tenantID:tenant,applicationRequest:app,projectRequest,projectAuthorize:async()=>calls.push({owner:'authorization'}),randomUUID:()=>`99999999-9999-4999-8999-${String(++sequence).padStart(12,'0')}`,projectUpdated:value=>{project=clone(value);}});
  return {flow,calls,details,source:()=>clone(source),setFault:value=>{fail=value;},writes:()=>calls.filter(call=>call.options)};
}
test('exact Project discovery reads only and never creates or links',async()=>{
  const f=setup();await f.flow.load(f.details());assert.equal(f.writes().length,0);assert.match(f.calls[0].path,/project_id=.*&limit=2/);
});
test('Project link fixture agrees with the versioned native capability catalog and server',()=>{
  const catalog=JSON.parse(readFileSync(new URL('../../../../contracts/access/v1/examples/project-workspace-capabilities.json',import.meta.url),'utf8'));
  assert.ok(catalog.capabilities.some(item=>item.id===projectLinkCapability));
  const native=readFileSync(new URL('../../../../backend/services/project-workspace/internal/auth/auth.go',import.meta.url),'utf8');
  assert.match(native,/ActionResourceManage\s*=\s*"project-workspace\.resource-link\.manage"/);
  assert.equal(catalog.capabilities.some(item=>item.id==='project-workspace.resource.manage'),false);
});
test('native resource-link.manage enables a fresh Project and the invented resource.manage does not',async()=>{
  const f=setup();let state=await f.flow.load(f.details());
  assert.equal(f.flow.can(state,'business-engagement.method.select'),true);
  assert.equal(f.flow.can(state,'business-engagement.applicability.manage'),true);
  const details=f.details();details.allowed_actions=['project-workspace.resource.manage'];
  state=await f.flow.load(details);
  assert.equal(f.flow.can(state,'business-engagement.method.select'),false);
  await assert.rejects(f.flow.apply(details,{kind:'method',method:'comparison'}),/不能保存/);
  assert.equal(f.writes().length,0);
});
for(const method of ['comparison','competitive_negotiation','tender'])test('native method '+method+' creates, links, sets applicability then method',async()=>{
  const f=setup();await f.flow.load(f.details());const state=await f.flow.apply(f.details(),{kind:'method',method});
  assert.equal(state.source.engagement.method,method);assert.equal(state.linked,true);assert.equal(f.writes().length,4);
  assert.deepEqual(f.writes().map(call=>call.owner),['business','project','business','business']);
  const refreshed=await f.flow.load(f.details());assert.equal(refreshed.linked,true);assert.equal(refreshed.source.engagement.method,method);
});
test('existing source without Link is reused rather than recreated',async()=>{
  const f=setup({existing:true});await f.flow.load(f.details());await f.flow.apply(f.details(),{kind:'method',method:'comparison'});
  assert.equal(f.writes().some(call=>call.path==='/api/v1/business-engagements'),false);
});
test('domain-only command receipts are followed by native GET before the next action',async()=>{
  const native=readFileSync(new URL('../../../../backend/services/business-engagement/internal/httpserver/server.go',import.meta.url),'utf8');
  assert.match(native,/func \(s \*Server\) writeResult[\s\S]*?writeJSON\(w, status, value\)/);
  const f=setup();await f.flow.load(f.details());await f.flow.apply(f.details(),{kind:'method',method:'comparison'});
  for(let i=0;i<f.calls.length;i++)if(f.calls[i].owner==='business'&&f.calls[i].options){
    assert.equal(f.calls[i+1].owner,'business');assert.equal(f.calls[i+1].options,undefined);
    assert.equal(f.calls[i+1].path,'/api/v1/business-engagements/'+resource);
  }
});
for(const stage of ['create','applicability','method'])test('lost native read after '+stage+' retains exact command for replay',async()=>{
  const f=setup(),request=f.flow.applicationRequest;let fail=true;
  f.flow.applicationRequest=async(app,path,options)=>{
    if(!options&&!path.includes('?')&&fail){
      const last=f.writes().at(-1),kind=last?.path.endsWith('/method')?'method':last?.path.endsWith('/applicability')?'applicability':'create';
      if(kind===stage){fail=false;throw Object.assign(new Error('native read denied'),{status:403});}
    }
    return request(app,path,options);
  };
  await f.flow.load(f.details());const intent={kind:'method',method:'comparison'};
  await assert.rejects(f.flow.apply(f.details(),intent),/native read denied/);
  assert.ok(f.flow.states.get(id).commands.has(stage));
  await f.flow.apply(f.details(),intent);
  const replies=f.writes().filter(call=>call.owner==='business'&&(stage==='create'?call.path==='/api/v1/business-engagements':call.path.endsWith('/'+stage)));
  assert.equal(replies.length,2);assert.equal(replies[0].options.body,replies[1].options.body);
  assert.equal(f.source().engagement.revision,3);assert.equal(f.details().resource_links.length,1);
});
test('a concurrently advanced native read stops with conflict and never applies the next action',async()=>{
  const f=setup(),request=f.flow.applicationRequest;
  f.flow.applicationRequest=async(app,path,options)=>{
    const value=await request(app,path,options);
    if(!options&&!path.includes('?')&&f.writes().length)value.engagement.revision++;
    return value;
  };
  await f.flow.load(f.details());await assert.rejects(f.flow.apply(f.details(),{kind:'method',method:'comparison'}),/已被更新/);
  assert.equal(f.writes().length,1);assert.equal(f.flow.states.get(id).conflict,true);
});
test('not-applicable and restoration retain source identity and history revision',async()=>{
  const f=setup({existing:true,linked:true});await f.flow.load(f.details());await f.flow.apply(f.details(),{kind:'applicability',applicable:false});
  assert.equal(f.source().engagement.applicability,'not_applicable');await f.flow.apply(f.details(),{kind:'applicability',applicable:true});
  assert.equal(f.source().engagement.id,resource);assert.equal(f.source().engagement.revision,3);
});
test('read-only user has no executable mutation and writes zero',async()=>{
  const f=setup({denied:true});const state=await f.flow.load(f.details());assert.equal(f.flow.can(state,'business-engagement.method.select'),false);
  await assert.rejects(f.flow.apply(f.details(),{kind:'method',method:'comparison'}),/不能保存/);assert.equal(f.writes().length,0);
});
test('unlinked source without Project linking permission cannot start a create',async()=>{
  const f=setup();const details=f.details();details.allowed_actions=[];await f.flow.load(details);
  await assert.rejects(f.flow.apply(details,{kind:'applicability',applicable:false}),/不能保存/);assert.equal(f.writes().length,0);
});
for(const stage of ['create','link','applicability','method'])test('lost '+stage+' acknowledgement retries the exact command without duplicate facts',async()=>{
  const f=setup({fault:{stage,after:true,error:new Error('network reply lost')}});await f.flow.load(f.details());
  const intent={kind:'method',method:'tender'};await assert.rejects(f.flow.apply(f.details(),intent),/lost/);await f.flow.apply(f.details(),intent);
  const commands=f.writes().map(call=>({stage:call.owner==='project'?'link':call.path.endsWith('/method')?'method':call.path.endsWith('/applicability')?'applicability':'create',body:call.options.body})).filter(call=>call.stage===stage);
  assert.equal(commands.length,2);assert.equal(commands[0].body,commands[1].body);
  assert.equal(f.details().resource_links.length,1);assert.equal(f.source().engagement.revision,3);
});
test('known Link failure retains created source and recovery does not recreate',async()=>{
  const f=setup({fault:{stage:'link',error:Object.assign(new Error('source unavailable'),{status:503})}});await f.flow.load(f.details());
  const intent={kind:'method',method:'comparison'};await assert.rejects(f.flow.apply(f.details(),intent),/unavailable/);
  assert.equal(f.source().engagement.revision,1);await f.flow.apply(f.details(),intent);
  assert.equal(f.writes().filter(call=>call.path==='/api/v1/business-engagements').length,1);
});
test('uncertain command rejects changing the intent payload',async()=>{
  const f=setup({fault:{stage:'method',after:true,error:new Error('lost')}});await f.flow.load(f.details());
  await assert.rejects(f.flow.apply(f.details(),{kind:'method',method:'comparison'}));const count=f.writes().length;
  await assert.rejects(f.flow.apply(f.details(),{kind:'method',method:'tender'}),/上次操作/);assert.equal(f.writes().length,count);
});
test('409 conflict cannot be retried with a silently updated revision',async()=>{
  const f=setup({fault:{stage:'method',error:Object.assign(new Error('stale'),{status:409})}});await f.flow.load(f.details());
  const intent={kind:'method',method:'comparison'};await assert.rejects(f.flow.apply(f.details(),intent),/stale/);const count=f.writes().length;
  await assert.rejects(f.flow.apply(f.details(),intent),/重新读取/);assert.equal(f.writes().length,count);
});
test('draft Project cannot create an engagement',async()=>{
  const f=setup();const details=f.details();details.project.status='draft';await f.flow.load(details);
  await assert.rejects(f.flow.apply(details,{kind:'method',method:'comparison'}),/不能保存/);assert.equal(f.writes().length,0);
});
test('unknown method is rejected before any business mutation',async()=>{
  const f=setup();await f.flow.load(f.details());await assert.rejects(f.flow.apply(f.details(),{kind:'method',method:'invented'}),/无效/);assert.equal(f.writes().length,0);
});
for(const response of [
  {project_id:other,items:[],allowed_actions:capabilities},
  {project_id:id,items:[{id:resource,tenant_id:tenant,project_id:other}],allowed_actions:capabilities},
  {project_id:id,items:[{},{}],allowed_actions:capabilities},
  {items:[],allowed_actions:capabilities}
])test('inconsistent or old list response fails closed',async()=>{
  const f=setup();f.flow.applicationRequest=async()=>response;await assert.rejects(f.flow.load(f.details()));assert.equal(f.writes().length,0);
});
test('link to a different source is never overwritten',async()=>{
  const f=setup({existing:true,linked:true});const details=f.details();details.resource_links[0].resource_id=other;
  await assert.rejects(f.flow.load(details),/不一致/);assert.equal(f.writes().length,0);
});
test('malformed mutation reply keeps its command id for safe replay',async()=>{
  const f=setup();await f.flow.load(f.details());f.flow.applicationRequest=async()=>({engagement:{id:other}});
  await assert.rejects(f.flow.apply(f.details(),{kind:'method',method:'comparison'}),/不一致/);
  assert.ok(f.flow.states.get(id).commands.has('create'));
});
test('cross-tenant project is rejected before a source request',async()=>{
  const f=setup();const details=f.details();details.project.tenant_id=other;await assert.rejects(f.flow.load(details),/无效/);assert.equal(f.calls.length,0);
});

function workspace(f) {
  const fields=Object.fromEntries(['method','status','requirements','applicability','retry','reload'].map(key=>['[data-real-bidding-'+key+']',{textContent:'',value:'',hidden:false,disabled:false,children:[],replaceChildren(){this.children=[];},append(child){this.children.push(child);}}]));
  const root={addEventListener(){},querySelector:selector=>fields[selector],querySelectorAll:()=>['method','applicability','retry'].map(key=>fields['[data-real-bidding-'+key+']'])};
  const records=[];
  const ui=new BiddingWorkspace({root,flow:f.flow,details:f.details,record:(...args)=>records.push(args)});
  return {ui,fields,records};
}
test('browser workspace confirms save but never resubmits on reread failure',async()=>{
  const f=setup();const w=workspace(f);await w.ui.load();
  const request=f.flow.applicationRequest;
  f.flow.applicationRequest=async(app,path,options)=>{if(!options&&path.includes('?')&&f.writes().length)throw new Error('read unavailable');return request(app,path,options);};
  await w.ui.save({kind:'method',method:'tender'});
  assert.match(w.fields['[data-real-bidding-status]'].textContent,/保存已确认，复读未完成/);
  assert.equal(f.writes().length,4);assert.equal(w.fields['[data-real-bidding-retry]'].hidden,true);
  const saved=w.records.find(row=>row[0]==='bidding-command'&&row[3]==='complete');assert.deepEqual(saved.slice(0,2),['bidding-command','承接保存']);
});
test('browser workspace clears prior facts when navigation occurs during save and reloads after completion',async()=>{
  const f=setup({existing:true,linked:true});let details=f.details();const originalDetails=f.details;f.details=()=>details;
  const w=workspace(f);await w.ui.load();let release;
  const request=f.flow.applicationRequest;
  f.flow.applicationRequest=async(app,path,options)=>{if(options&&path.endsWith('/method'))await new Promise(resolve=>{release=resolve;});return request(app,path,options);};
  const save=w.ui.save({kind:'method',method:'tender'});
  while(!release)await Promise.resolve();
  details={...originalDetails(),project:{...originalDetails().project,id:other}};
  await w.ui.load();
  assert.equal(w.ui.state,null);assert.equal(w.fields['[data-real-bidding-method]'].value,'');assert.equal(w.fields['[data-real-bidding-requirements]'].children.length,0);assert.equal(w.fields['[data-real-bidding-method]'].disabled,true);
  release();await save;
  assert.match(w.fields['[data-real-bidding-status]'].textContent,/读取未完成/);assert.equal(w.records.some(row=>row[0]==='bidding-command'&&row[3]==='complete'),false);
});
test('browser workspace requires explicit reload after conflict and does not autosubmit',async()=>{
  const f=setup({fault:{stage:'method',error:Object.assign(new Error('stale'),{status:409})}});const w=workspace(f);await w.ui.load();
  await w.ui.save({kind:'method',method:'comparison'});
  assert.equal(w.fields['[data-real-bidding-retry]'].hidden,true);assert.equal(w.fields['[data-real-bidding-method]'].disabled,true);
  const count=f.writes().length;await w.ui.load(true);assert.equal(f.writes().length,count);assert.equal(w.ui.state.conflict,false);
});
test('browser workspace never labels a rejected local operation as saved',async()=>{
  const f=setup();const w=workspace(f);await w.ui.load();await w.ui.save({kind:'method',method:'invalid'});
  assert.match(w.fields['[data-real-bidding-status]'].textContent,/未保存：/);assert.equal(f.writes().length,0);
});
test('optional requirement satisfaction never invents a registered attachment',async()=>{
  const f=setup({existing:true,linked:true}),w=workspace(f);await w.ui.load();
  w.ui.state.source.engagement.method='comparison';
  w.ui.state.source.requirements=[{key:'tender_document',required:false,satisfied:true},{key:'bid_document',required:true,satisfied:false},{key:'award_document',required:true,satisfied:false}];
  w.ui.render();
  const lines=w.fields['[data-real-bidding-requirements]'].children.map(row=>row.textContent);
  assert.equal(lines[0],'招标文件：当前方式非必传');
  assert.equal(lines[1],'投标文件：必传，尚未登记');
  assert.equal(lines[2],'中标文件：必传，尚未登记');
  assert.equal(f.writes().length,0);
});
test('only a current material reference can render an attachment as registered',async()=>{
  const f=setup({existing:true,linked:true}),w=workspace(f);await w.ui.load();
  w.ui.state.source.engagement.method='tender';
  w.ui.state.source.requirements=[{key:'bid_document',required:true,satisfied:true}];
  w.ui.state.source.materials=[{requirement_key:'bid_document',display_snapshot:'旧投标文件.pdf',superseded_at:'2026-10-02'},{requirement_key:'award_document',display_snapshot:'中标通知.pdf'}];
  w.ui.render();
  const lines=w.fields['[data-real-bidding-requirements]'].children.map(row=>row.textContent);
  assert.equal(lines[1],'投标文件：必传，尚未登记');
  assert.equal(lines[2],'中标文件：已登记 · 中标通知.pdf');
  assert.equal(lines[3],'中标通知.pdf');
});
test('native undetermined draft permits only applicability first, then newly returned method action',async()=>{
  const f=setup({existing:true});const request=f.flow.applicationRequest;
  f.flow.applicationRequest=async(app,path,options)=>{
    const value=await request(app,path,options);
    if(value.engagement)value.allowed_actions=capabilities.filter(action=>action!=='business-engagement.method.select'||value.engagement.applicability==='applicable');
    return value;
  };
  const state=await f.flow.load(f.details());assert.equal(state.actions.includes('business-engagement.method.select'),false);
  assert.equal(f.flow.can(state,'business-engagement.method.select'),true);
  await f.flow.apply(f.details(),{kind:'method',method:'tender'});
  assert.deepEqual(f.writes().filter(call=>call.owner==='business').map(call=>call.path.split('/').at(-1)),['applicability','method']);
});
test('undetermined draft with no method credential cannot propose method, and missing next action never writes it',async()=>{
  const f=setup({existing:true});const request=f.flow.applicationRequest;
  f.flow.applicationRequest=async(app,path,options)=>{
    const value=await request(app,path,options);
    if(value.engagement)value.allowed_actions=capabilities.filter(action=>action!=='business-engagement.method.select');
    if(path.includes('?'))value.allowed_actions=capabilities.filter(action=>action!=='business-engagement.method.select');
    return value;
  };
  let state=await f.flow.load(f.details());assert.equal(f.flow.can(state,'business-engagement.method.select'),false);
  await assert.rejects(f.flow.apply(f.details(),{kind:'method',method:'tender'}),/不能保存/);assert.equal(f.writes().length,0);
  state.credentialActions=capabilities;
  await assert.rejects(f.flow.apply(f.details(),{kind:'method',method:'tender'}),/不允许选择/);
  assert.equal(f.writes().some(call=>call.path.endsWith('/method')),false);
});
