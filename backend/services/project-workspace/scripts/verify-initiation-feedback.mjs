// Exercise the shipped page JavaScript with controlled transport/DOM, no server.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source=readFileSync(new URL('../internal/httpserver/demo_overview.go',import.meta.url),'utf8');
function section(start,end){const a=source.indexOf(start),b=source.indexOf(end,a+start.length);assert(a>=0&&b>a);return source.slice(a,b);}
const api=section('function errorOf(', 'function labelAction(');
const command=section('async function refreshRejectedInitiation(', 'function cookie(');
function scenario(fetch){
 const nodes={};const state={token:'',items:[{id:'fixture',revision:1}],details:{project:{id:'fixture',revision:1},allowed_actions:['create']},listActions:['create']};
 const notices=[],recovered=[],reads=[];
 const context=vm.createContext({state,suiteBase:'/project',fetch,Error,JSON,$:key=>nodes[key]??=( {} ),notice:(...value)=>notices.push(value),renderDetails(){},renderList(){},renderCommercial(){},renderMaterials(){},recoverAccess:error=>recovered.push(error),loadProjects:async()=>reads.push('list'),openProject:async id=>reads.push(id)});
 vm.runInContext(api+command,context);
 return {context,state,nodes,notices,recovered,reads};
}
function response(status,body,redirected=false){return{ok:status<400,status,redirected,text:async()=>typeof body==='string'?body:JSON.stringify(body)}}
const failure={error:{code:'initiation_incomplete',message:'立项资料尚未补全',missing:['business_owner','production_owner']}};
{
 const calls=[],s=scenario(async(path,options)=>{calls.push([path,options.method??'GET']);return options.method==='POST'?response(422,failure):response(200,{project:{id:'fixture',revision:2,status:'pending_completion'},allowed_actions:['read']})});
 await vm.runInContext("command('/api/v1/projects/fixture/complete-initiation',{expected_revision:1})",s.context);
 assert.deepEqual(calls,[['/project/api/v1/projects/fixture/complete-initiation','POST'],['/project/api/v1/projects/fixture','GET']]);
 assert.equal(s.state.details.project.revision,2);assert.equal(s.state.items[0].revision,2);assert.equal(s.state.details.allowed_actions[0],'read');
 assert.match(s.notices.at(-1)[0],/revision 2.*经营负责人、生产负责人/);assert.equal(s.recovered.length,0);assert.equal(s.state.commandInFlight,false);assert.equal(s.nodes['#completeProject'].disabled,false);
}
{
 const s=scenario(async(_path,options)=>options.method==='POST'?response(422,failure):response(503,{error:{code:'dependency_unavailable'}}));
 await vm.runInContext("command('/api/v1/projects/fixture/complete-initiation',{})",s.context);
 assert.equal(s.state.details.project.revision,1);assert.equal(s.state.details.allowed_actions.length,0);assert.equal(s.state.listActions.length,0);assert.equal(s.nodes['#accessRecovery'].hidden,false);assert.match(s.notices.at(-1)[0],/复读失败.*不要重复提交/);
}
{
 const s=scenario(async()=>response(200,'<!doctype html>IAM login',true));
 await assert.rejects(vm.runInContext("api('/api/v1/projects')",s.context),error=>error.status===401);
}
{
 let resolve;const calls=[],s=scenario((path,options)=>{calls.push(options.method??'GET');return new Promise(done=>{resolve=done})});
 const first=vm.runInContext("command('/api/v1/projects/fixture/complete-initiation',{})",s.context);
 await vm.runInContext("command('/api/v1/projects/fixture/complete-initiation',{})",s.context);
 assert.equal(calls.length,1);resolve(response(409,{error:{code:'revision_conflict'}}));await first;assert.equal(s.recovered[0].code,'revision_conflict');
}
{
 const s=scenario(async(_path,options)=>{if(options.method==='POST'){s.state.details={project:{id:'other',revision:8},allowed_actions:['read']};return response(422,failure)}throw new Error('must not reload another selection')});
 await vm.runInContext("command('/api/v1/projects/fixture/complete-initiation',{})",s.context);
 assert.equal(s.state.details.project.id,'other');assert.equal(s.state.details.project.revision,8);
}
console.log('PASS page initiation revision/allowed_actions re-read, refresh failure close, auth redirect, duplicate suppression and selection boundary');
