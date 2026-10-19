import test from 'node:test';
import assert from 'node:assert/strict';
import {ContributorAdapter,contributorAction,contributorRole} from './contributor-adapter.mjs';
const tenant='11111111-1111-4111-8111-111111111111', project='22222222-2222-4222-8222-222222222222', subject='33333333-3333-4333-8333-333333333333', person='44444444-4444-4444-8444-444444444444', assignment='55555555-5555-4555-8555-555555555555', command='66666666-6666-4666-8666-666666666666';
const clone=v=>structuredClone(v);
const memory=()=>{const data=new Map();return {getItem:k=>data.get(k),setItem:(k,v)=>data.set(k,v),removeItem:k=>data.delete(k),data}};
function fixture({actions=[contributorAction],storage=memory()}={}) {
 const value={project:{id:project,tenant_id:tenant,revision:4,status:'active'},allowed_actions:actions,assignments:[]};
 let current=value,failAfter=false,failRead=false,forcedStatus=0,resultTransform=v=>v;
 const writes=[],commands=new Map();
 const people=[{id:person,tenant_id:tenant,revision:2,display_name:'张工',status:'active'}];
 const request=async(path,options)=>{
  if(!options){if(failRead)throw new Error('read unavailable');return clone(value)}
  const c=JSON.parse(options.body);writes.push(c);
  if(forcedStatus)throw Object.assign(new Error('rejected'),{status:forcedStatus});
  if(!commands.has(c.command_id)) {
   assert.equal(c.expected_revision,value.project.revision);value.project.revision++;
   let a;
   if(c.operation==='assign') {a={id:assignment,project_id:project,tenant_id:tenant,subject_kind:'person',subject_id:c.subject_id,source_revision:c.source_revision,role_key:contributorRole,display_snapshot:'张工'};value.assignments.push(a)}
   else {a=value.assignments.find(a=>a.id===c.assignment_id);a.ended_at='2026-10-03T12:00:00Z'}
   commands.set(c.command_id,{project:clone(value.project),assignment:clone(a)});
  }
  if(failAfter){failAfter=false;throw new Error('reply lost')}
  return resultTransform(clone(commands.get(c.command_id)));
 };
 const create=()=>new ContributorAdapter({details:()=>current,request,authorize:async()=>{},applicationRequest:async()=>clone(people),tenantID:tenant,subjectID:subject,storage,newID:()=>command});
 return {adapter:create(),create,value,people,writes,storage,setAfter:()=>failAfter=true,setRead:v=>failRead=v,setStatus:v=>forcedStatus=v,setResult:f=>resultTransform=f,switch:()=>current={project:{id:person,tenant_id:tenant}}};
}
const add=async f=>{await f.adapter.load();await f.adapter.users();await f.adapter.create(project,{role:'business_personnel',user_id:person})};
test('original list model uses explicit purpose, with no invented organization facts',async()=>{const f=fixture();await add(f);assert.equal(f.writes[0].role_key,contributorRole);assert.equal(f.writes[0].source_revision,2);assert.equal(f.adapter.members()[0].user.real_name,'张工');assert.equal(f.adapter.members()[0].user.username,undefined);assert.equal(f.adapter.members()[0].user.department,undefined)});
test('old broad or production participant grants never expand',async()=>{for(const action of ['project-workspace.assignment.manage','project-workspace.assignment.participant.manage']){const f=fixture({actions:[action]});await assert.rejects(add(f),/没有经营参与人管理权限/);assert.equal(f.writes.length,0)}});
test('duplicate add is rejected before POST',async()=>{const f=fixture();await add(f);await assert.rejects(f.adapter.create(project,{role:'business_personnel',user_id:person}),/已在/);assert.equal(f.writes.length,1)});
test('read or repeated refresh produces zero writes',async()=>{const f=fixture();await f.adapter.load();await f.adapter.users();await f.adapter.reconcile();await f.adapter.load();assert.equal(f.writes.length,0)});
test('different tenant people and duplicate directory rows fail closed',async()=>{for(const mutate of [f=>f.people[0].tenant_id=person,f=>f.people.push(clone(f.people[0]))]){const f=fixture();mutate(f);await assert.rejects(f.adapter.users(),/范围或版本/);assert.equal(f.writes.length,0)}});
test('stopped person is never selected',async()=>{const f=fixture();f.people[0].status='inactive';await assert.rejects(add(f),/有效人员/);assert.equal(f.writes.length,0)});
test('unknown reply retains command and reload retries exactly once with same payload',async()=>{const f=fixture();f.setAfter();await assert.rejects(add(f),/reply lost/);assert.ok(f.adapter.pending);const next=f.create();await next.load();await next.retry();assert.deepEqual(f.writes[0],f.writes[1]);assert.equal(f.value.assignments.length,1);assert.equal(next.pending,null)});
test('explicit refresh can reconcile committed unknown reply without another POST',async()=>{const f=fixture();f.setAfter();await assert.rejects(add(f));await f.adapter.reconcile();assert.equal(f.adapter.pending,null);assert.equal(f.writes.length,1)});
test('409 cannot auto retry and explicit refresh discards the rejected intent',async()=>{const f=fixture();f.setStatus(409);await assert.rejects(add(f));assert.equal(f.adapter.conflict,true);await assert.rejects(f.adapter.retry(),/刷新/);assert.equal(f.writes.length,1);f.setStatus(0);await f.adapter.reconcile();assert.equal(f.adapter.pending,null)});
test('definitive authorization failure never retains a write for automatic retry',async()=>{const f=fixture();f.setStatus(403);await assert.rejects(add(f));assert.equal(f.adapter.pending,null);assert.equal(f.writes.length,1)});
test('foreign or malformed result is uncertain, never success',async()=>{for(const change of [v=>v.assignment.subject_id=subject,v=>v.assignment.tenant_id=person,v=>v.project.revision++]){const f=fixture();f.setResult(v=>{change(v);return v});await assert.rejects(add(f),/尚未确认/);assert.ok(f.adapter.pending)}});
test('readback failure after confirmed save does not retain command or suggest duplicate creation',async()=>{const f=fixture();await f.adapter.load();await f.adapter.users();const original=f.adapter.request;f.adapter.request=async(p,o)=>{const r=await original(p,o);if(o)f.setRead(true);return r};await assert.rejects(f.adapter.create(project,{role:'business_personnel',user_id:person}),/已保存/);assert.equal(f.adapter.pending,null);assert.equal(f.writes.length,1)});
test('move out preserves assignment history and hides it from current list',async()=>{const f=fixture();await add(f);f.adapter.newID=()=> '77777777-7777-4777-8777-777777777777';await f.adapter.remove(assignment);assert.equal(f.adapter.members().length,0);assert.equal(f.value.assignments.length,1);assert.ok(f.value.assignments[0].ended_at);assert.equal(f.writes[1].operation,'end')});
test('switching project never posts stale work to the new project',async()=>{const f=fixture();await f.adapter.load();await f.adapter.users();f.switch();await assert.rejects(f.adapter.create(project,{role:'business_personnel',user_id:person}),/项目已切换/);assert.equal(f.writes.length,0)});
test('one fresh capability per write, no redundant renewal on guarded readback',async()=>{
 const f=fixture(),events=[],request=f.adapter.request;
 f.adapter.authorize=async()=>events.push('renew');
 f.adapter.request=async(path,options)=>{events.push(options?'POST':'GET');return request(path,options)};
 await add(f);
 assert.deepEqual(events,['renew','GET','GET','renew','POST','GET']);
 await f.adapter.load({renewCredential:false});assert.equal(events.filter(e=>e==='renew').length,2);
 await f.adapter.reconcile();assert.equal(events.filter(e=>e==='renew').length,3);
});
test('credential renewal rejection happens before POST and is never bypassed',async()=>{
 const f=fixture();await f.adapter.load();await f.adapter.users();
 f.adapter.authorize=async()=>{throw Object.assign(new Error('renewal denied'),{status:403})};
 await assert.rejects(f.adapter.create(project,{role:'business_personnel',user_id:person}),/renewal denied/);
 assert.equal(f.writes.length,0);assert.equal(f.adapter.pending,null);
});
