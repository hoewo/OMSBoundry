import test from 'node:test';
import assert from 'node:assert/strict';
import {BonusAdapter,bonusRatio} from './bonus-adapter.mjs';
const id=n=>'00000000-0000-4000-8000-'+String(n).padStart(12,'0'),profile='reference-oa-bonus-v3',source='reference-oa-production-personnel-v1';
const actions=['read','amount.read','record','withdraw','history.read'].map(v=>'bonus-manual-allocation.'+v);
function fixture(){
 const project={id:id(4),tenant_id:id(2),revision:18},storage=new Map(),calls=[],rows=[];let fail,serial=50;
 const person={id:id(5),revision:2,display_name:'同一人'};
 const candidates=[{tenant_id:id(2),bonus_type:'production',active:true,eligible:true,person,assignment:{id:id(6),revision:3,project_id:id(4),person_id:id(5),role_key:'production_owner'}},...['桥梁','道路'].map((name,i)=>({tenant_id:id(2),bonus_type:'production',active:true,eligible:true,person,assignment_source:source,assignment:{id:id(6+i),revision:4,project_id:id(4),person_id:id(5),role_key:'designer'},discipline:{id:id(10+i),revision:1,display_name:name}}))];
 const options={profile,scope:'production',tenantID:id(2),subjectID:id(1),details:()=>({project}),storage:{getItem:k=>storage.get(k),setItem:(k,v)=>storage.set(k,v),removeItem:k=>storage.delete(k)},newID:()=>id(serial++),applicationRequest:async(app,path,o)=>{
  calls.push({path,...o});if(o?.method==='POST'){if(fail)throw fail;const input=JSON.parse(o.body),batch=id(serial++);const items=input.items.map(x=>{const c=candidates.find(v=>v.assignment.id===x.assignment_ref.id&&(v.assignment_source||'')===(x.assignment_source||''));return {...structuredClone(c),id:id(serial++),batch_id:batch,profile:c.assignment_source?profile:'reference-oa-bonus-v2',project:{id:id(4),revision:18},scope:'production',method:x.method,base_amount_minor:input.base_amount_minor,fixed_amount_minor:x.fixed_amount_minor,ratio_basis_points:x.ratio_basis_points,amount_minor:x.method==='ratio'?bonusRatio(input.base_amount_minor,x.ratio_basis_points):x.fixed_amount_minor,currency:'CNY',occurred_on:input.occurred_on,description:input.description,state:'recorded',revision:1,allowed_actions:actions}});rows.push(...items);return {profile,project_id:id(4),scope:'production',items}}
  return path.includes('/candidates')?{profile,project_id:id(4),project_revision:18,scope:'production',items:candidates}:{profile,project_id:id(4),scope:'production',items:rows,allowed_actions:actions};
 }};
 return {options,candidates,calls,rows,fail(e){fail=e}};
}
const input=keys=>({bonus_category:'production',base_amount:'1.01',occurred_at:'2026-10-04',description:'原专业分配',items:keys.map(assignment_id=>({assignment_id,recipient_id:id(5),allocation_method:'ratio',allocation_ratio:50,amount:'0.00'}))});
test('professional candidates retain same-person same-ID distinct namespaces and profession labels',async()=>{
 const f=fixture(),a=new BonusAdapter(f.options);await a.load();const candidates=await a.candidates();assert.equal(candidates.length,3);assert.equal(new Set(candidates.map(x=>x.uniqueKey)).size,3);assert.deepEqual(candidates.map(x=>x.roleLabel),['生产项目负责人','设计人（桥梁）','设计人（道路）']);
 const result=await a.create(input(candidates.slice(1).map(x=>x.uniqueKey)));assert.deepEqual(result.map(x=>x.amount),['0.51','0.51']);assert.equal(a.statistics().recipient_count,1);const body=JSON.parse(f.calls.find(x=>x.body).body);assert(body.items.every(x=>x.assignment_source===source));assert.deepEqual(body.items.map(x=>x.assignment_ref.id),[id(6),id(7)]);assert(!body.items.some(x=>x.discipline||x.actor||x.allocation_role));
});
test('old profile rejects professional reply and unknown source or missing discipline fails closed',async()=>{
 const f=fixture();await assert.rejects(new BonusAdapter({...f.options,profile:'reference-oa-bonus-v2'}).load());for(const mutate of [v=>v.assignment_source='project',v=>v.discipline=null,v=>v.assignment.role_key='business_owner']){const g=fixture(),a=new BonusAdapter(g.options);await a.load();mutate(g.candidates[1]);await assert.rejects(a.candidates());assert.equal(g.calls.filter(x=>x.body).length,0)}
});
test('known fresh 422 leaves no uncertain command; retry cannot silently reuse stale candidates',async()=>{
 const f=fixture(),a=new BonusAdapter(f.options);await a.load();const candidates=await a.candidates();f.fail(Object.assign(Error('ended'),{status:422}));await assert.rejects(a.create(input([candidates[1].uniqueKey])));assert.equal(a.pending,null);assert.equal(a.candidateValues.size,0);await assert.rejects(a.create(input([candidates[1].uniqueKey])));assert.equal(f.calls.filter(x=>x.body).length,1);
});
test('unknown reply then revoked authorization retains original command across reload',async()=>{
 const f=fixture(),a=new BonusAdapter(f.options);await a.load();const candidates=await a.candidates();f.fail(Object.assign(Error('lost'),{status:503}));await assert.rejects(a.create(input([candidates[1].uniqueKey])));assert.equal(a.pending.uncertain,true);const body=f.calls.find(x=>x.body).body;f.fail(Object.assign(Error('revoked'),{status:403}));const b=new BonusAdapter(f.options);await assert.rejects(b.retry());assert(b.pending);assert.equal(f.calls.filter(x=>x.body).at(-1).body,body);assert.equal(b.pending.uncertain,true);
});
test('pending is uncertain before dispatch; reload during request cannot discard an already committed intent',async()=>{
 const f=fixture();let release,entered;const ready=new Promise(r=>entered=r),blocked=new Promise(r=>release=r),request=f.options.applicationRequest;
 const a=new BonusAdapter({...f.options,applicationRequest:async(...args)=>{if(args[2]?.method==='POST'){entered();await blocked;}return request(...args)}});
 await a.load();const candidates=await a.candidates(),sending=a.create(input([candidates[1].uniqueKey]));await ready;
 const restored=new BonusAdapter(f.options);assert.equal(restored.pending.uncertain,true);const command=restored.pending.fields.command_id;
 f.fail(Object.assign(Error('revoked after possible commit'),{status:403}));await assert.rejects(restored.retry());assert.equal(restored.pending.fields.command_id,command);
 release();await assert.rejects(sending);assert.equal(a.pending,null);
});
