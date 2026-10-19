// Exercise the actual shipped picker with a small DOM/transport fixture.
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const source=readFileSync(new URL('../internal/httpserver/demo_overview.go',import.meta.url),'utf8');
const start=source.indexOf('function resetAssignmentDirectory('),end=source.indexOf('async function refreshPermissions(',start);
assert(start>0&&end>start);
const script=source.slice(start,end);
const full=source.match(/const projectDemoJavaScript = `([\s\S]*?)`\s*$/)?.[1];
assert(full);new vm.Script(full);
const tenant='11111111-1111-4111-8111-111111111111';
const person='55555555-5555-4555-8555-555555555555';
const group='66666666-6666-4666-8666-666666666666';
const active={id:person,tenant_id:tenant,display_name:'验收人员',reference_code:'VERIFY-P',status:'active',revision:4,iam_subject_id:'must-not-store'};
function node(){return {value:'',checked:false,disabled:false,hidden:false,required:false,textContent:'',children:[],replaceChildren(){this.children=[];this.value=''},append(value){this.children.push(value)}}}
function scenario({suiteBase='/project',fetch=async()=>({ok:true,status:200,text:async()=>JSON.stringify([active])}),authorize=async()=>{}}={}){
 const nodes={},get=key=>nodes[key]??=node();
 const state={details:{project:{id:'fixture',tenant_id:tenant,revision:3}},commandInFlight:false,assignmentDirectory:{sequence:0,projectID:'',kind:'',items:[],loading:false,message:''}};
 get('#assignmentForm [name="subject_kind"]').value='person';get('#assignmentForm [name="role_key"]').value='business_owner';
 const manualNodes=[node(),node()],calls=[],notices=[];let allowed=true;
 const context=vm.createContext({state,suiteBase,fetch,authorizeApplication:authorize,Error,JSON,Number,Set,document:{createElement:node},$:get,$$:()=>manualNodes,canAssign:()=>allowed,errorOf:(response,data)=>Object.assign(new Error(data.error?.message||'denied'),{status:response.status}),uuid:()=> 'command',notice:(...args)=>notices.push(args),command:async(path,body)=>{calls.push({path,body:JSON.parse(JSON.stringify(body))})}});
 vm.runInContext(script,context);
 const run=code=>vm.runInContext(code,context);
 const form={elements:{role_key:get('#assignmentForm [name="role_key"]'),subject_kind:get('#assignmentForm [name="subject_kind"]'),subject_id:get('#assignmentForm [name="subject_id"]'),source_revision:get('#assignmentForm [name="source_revision"]')}};
 context.event={target:form,preventDefault(){}};
 return{context,state,nodes,get,run,calls,notices,deny(){allowed=false}};
}
let cases=0;
{
 const s=scenario();s.context.data=[active,{...active,id:group,status:'inactive'}];
 const data=JSON.parse(JSON.stringify(s.run("normalizeAssignmentCandidates(data,'person',state.details.project.tenant_id)")));
 assert.deepEqual(data,[{id:person,revision:4,name:'验收人员',code:'VERIFY-P'}]);assert(!JSON.stringify(data).includes('iam_subject'));cases++;
 s.context.data=[{id:group,tenant_id:tenant,name:'验收编组',group_type_key:'design',status:'active',revision:9}];
 assert.equal(s.run("normalizeAssignmentCandidates(data,'group',state.details.project.tenant_id)[0].revision"),9);cases++;
 for(const change of [{tenant_id:'other'},{id:'not-an-id'},{revision:0},{revision:1.5},{display_name:''}]){s.context.data=[{...active,...change}];assert.throws(()=>s.run("normalizeAssignmentCandidates(data,'person',state.details.project.tenant_id)"))}
 s.context.data=[active,active];assert.throws(()=>s.run("normalizeAssignmentCandidates(data,'person',state.details.project.tenant_id)"));cases++;
}
{
 const authorizations=[],fetches=[],s=scenario({authorize:async app=>authorizations.push(app),fetch:async(path,options)=>{fetches.push({path,options});return{ok:true,status:200,text:async()=>JSON.stringify([active])}}});
 await s.run('loadAssignmentCandidates()');assert.deepEqual(authorizations,['people-groups']);assert.equal(fetches[0].path,'/people/api/v1/persons');assert.equal(fetches[0].options.credentials,'same-origin');assert(!fetches[0].options.headers);assert.equal(s.get('#submitAssignment').disabled,true);
 s.get('#assignmentCandidate').value=person;s.run('renderAssignmentPicker()');assert.equal(s.get('#submitAssignment').disabled,false);
 await s.run('submitAssignment(event)');assert.deepEqual(s.calls[0].body,{command_id:'command',expected_revision:3,operation:'assign',subject_kind:'person',subject_id:person,source_revision:4,role_key:'business_owner'});assert.equal(s.calls[0].path,'/api/v1/projects/fixture/assignments');cases++;
 s.get('#assignmentCandidate').value='forged';await s.run('submitAssignment(event)');assert.equal(s.calls.length,1);cases++;
 s.get('#assignmentForm [name="subject_kind"]').value='group';s.run('resetAssignmentDirectory();renderAssignmentCandidates()');assert.equal(s.state.assignmentDirectory.items.length,0);assert.equal(s.get('#assignmentCandidate').value,'');assert.equal(s.get('#submitAssignment').disabled,true);cases++;
}
{
 let fetched=0;const s=scenario({authorize:async()=>{throw new Error('no People capability')},fetch:async()=>{fetched++}});
 await s.run('loadAssignmentCandidates()');assert.equal(fetched,0);assert.equal(s.get('#assignmentManualMode').checked,false);assert.equal(s.get('#submitAssignment').disabled,true);assert.match(s.get('#assignmentReferenceFeedback').textContent,/读取失败.*没有自动提交/);cases++;
}
for(const change of ['project','kind','permission']){
 let resolve,markStarted;const started=new Promise(done=>{markStarted=done}),s=scenario({fetch:()=>new Promise(done=>{resolve=done;markStarted()})});
 const pending=s.run('loadAssignmentCandidates()');await started;assert.equal(s.get('#submitAssignment').disabled,true);
 if(change==='project')s.state.details.project.id='other';else if(change==='kind')s.get('#assignmentForm [name="subject_kind"]').value='group';else s.deny();
 s.run('resetAssignmentDirectory();renderAssignmentCandidates()');resolve({ok:true,status:200,text:async()=>JSON.stringify([active])});await pending;assert.equal(s.state.assignmentDirectory.items.length,0);assert.equal(s.get('#assignmentCandidate').value,'');assert.equal(s.get('#submitAssignment').disabled,true);cases++;
}
{
 const s=scenario({suiteBase:''});s.run('resetAssignmentDirectory();renderAssignmentCandidates()');assert.equal(s.get('#assignmentManualMode').checked,true);assert.equal(s.get('#assignmentDirectory').hidden,true);s.get('#assignmentForm [name="subject_id"]').value=person;s.get('#assignmentForm [name="source_revision"]').value='4';await s.run('submitAssignment(event)');assert.equal(s.calls[0].body.subject_id,person);assert.equal(s.calls[0].body.source_revision,4);cases++;
 s.state.commandInFlight=true;await s.run('submitAssignment(event)');assert.equal(s.calls.length,1);cases++;
 s.state.commandInFlight=false;s.deny();await s.run('submitAssignment(event)');assert.equal(s.calls.length,1);assert.match(s.notices.at(-1)[0],/当前权限不能/);cases++;
}
console.log('PASS '+cases+' shipped picker cases: active references, strict IDs/tenant/revision, exact payload, no privilege fields, denial, late responses, explicit local fallback and no duplicate command');
