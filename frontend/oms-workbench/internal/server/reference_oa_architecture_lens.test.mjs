import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
const source=readFileSync(new URL('./reference_oa_architecture_lens.js',import.meta.url),'utf8');
const context=vm.createContext({});vm.runInContext(source,context);
const create=context.ReferenceOAArchitectureLens.create;
const tenant='11111111-1111-4111-8111-111111111111',subject='22222222-2222-4222-8222-222222222222',other='33333333-3333-4333-8333-333333333333';
function storage(){const values=new Map(),writes=[];return {values,writes,getItem:key=>values.get(key),setItem(key,value){values.set(key,value);writes.push({key,value});}};}
function trace(store=storage()){return {store,model:create({tenant,subject,enabled:true,storage:store})};}
test('new BE step is actually displayed instead of discarded by fixed Project keys',()=>{
  const {model}=trace();model.record('flow','保存招投标资料','Workbench → Gateway／IAM → Business Engagement','waiting');model.record('business-engagement','BE 原生接口已回复','只拥有承接和材料引用，不拥有 Project 或 Evidence 字节。','complete');model.record('bidding-command','承接保存已确认','Project 只保存引用。','complete');
  assert.deepEqual(Array.from(model.steps(),row=>row.key),['flow','business-engagement','bidding-command']);
});
test('the next operation clears former flow, owner and failure steps',()=>{
  const {model}=trace();model.record('project','立项','Project 自有库','complete');model.record('iam','授权失败','未提交目标','error');model.record('flow','读取招投标','只读','waiting');assert.equal(model.steps().length,1);assert.equal(model.steps()[0].title,'读取招投标');
});
test('account and tenant isolate browser explanation storage, legacy global key is ignored',()=>{
  const store=storage();store.values.set('omsbizfoundry-architecture-lens-v1',JSON.stringify([{key:'project',title:'旧流程',copy:'旧复制',state:'complete'}]));const a=trace(store).model;a.record('flow','新流程','无业务值','waiting');
  assert.equal(create({tenant,subject:other,enabled:true,storage:store}).steps().some(row=>row.title==='新流程'),false);assert.equal(create({tenant:other,subject,enabled:true,storage:store}).steps().some(row=>row.title==='新流程'),false);assert.match(store.writes[0].key,new RegExp(tenant+':'+subject+'$'));
});
test('non-admin or invalid identity does not persist explanations',()=>{
  for(const options of [{tenant,subject,enabled:false},{tenant:'wrong',subject,enabled:true},{tenant,subject:'wrong',enabled:true}]){const store=storage();create({...options,storage:store}).record('project','不可写','只读','complete');assert.equal(store.writes.length,0);}
});
test('blocked storage never throws through a business operation and keeps in-memory steps',()=>{
  const {model}=trace({getItem(){throw new Error('blocked');},setItem(){throw new Error('quota');}});assert.doesNotThrow(()=>model.record('business-engagement','已回复','业务操作不依赖日志存储','complete'));assert.equal(model.steps()[0].key,'business-engagement');
});
test('invalid or unbounded cached content is ignored',()=>{
  for(const value of ['{',JSON.stringify({version:1,steps:[]}),JSON.stringify({version:2,steps:[{key:'secret',title:'a',copy:'b',state:'complete'}]}),JSON.stringify({version:2,steps:Array(100).fill({key:'project',title:'a',copy:'b',state:'complete'})})]){const store=storage();store.values.set('omsbizfoundry-architecture-lens-v2:'+tenant+':'+subject,value);const {model}=trace(store);assert.equal(model.steps().every(row=>row.state==='waiting'),true);}
});
test('recording rejects arbitrary keys, oversized strings and unknown state',()=>{
  const {model,store}=trace();for(const entry of [['headers','x','y','complete'],['project','x'.repeat(101),'y','complete'],['project','x','y'.repeat(401),'complete'],['project','x','y','mystery']])model.record(...entry);assert.equal(store.writes.length,0);
});
test('repeated owner steps are bounded and returned state cannot mutate the stored explanation',()=>{
  const {model}=trace();for(let i=0;i<100;i++)model.record('business-engagement','BE '+i,'只读','complete');assert.equal(model.steps().length,1);const rows=model.steps();rows[0].copy='changed';assert.equal(model.steps()[0].copy,'只读');
});
test('late request from the previous operation cannot contaminate the current explanation',()=>{
  const {model}=trace();model.record('flow','旧操作','旧','waiting');const old=model.epoch();model.record('flow','当前操作','当前','waiting');model.record('business-engagement','旧请求回复','旧','complete',old);assert.equal(model.steps().length,1);assert.equal(model.steps()[0].title,'当前操作');
});
test('valid current explanation survives same-account refresh without raw requests or credentials',()=>{
  const {model,store}=trace();model.record('flow','保存材料引用','BE 拥有引用；Evidence 拥有文件。','waiting');model.record('bidding-command','未完全确认','原命令保留，不自动重传。','error');const restored=trace(store).model;assert.deepEqual(Array.from(restored.steps(),row=>row.key),['flow','bidding-command']);const payload=JSON.parse(store.writes.at(-1).value);assert.deepEqual(Object.keys(payload),['version','steps']);assert.deepEqual(Object.keys(payload.steps[0]),['key','title','copy','state']);
});
test('actual Workbench renderer displays BE explanation via textContent and safe storage failure',()=>{
  const go=readFileSync(new URL('./server.go',import.meta.url),'utf8'),start=go.indexOf('const architectureLens=document.querySelector'),end=go.indexOf('const workRoot=',start);assert.ok(start>0&&end>start);
  function node(){return {dataset:{},hidden:true,children:[],append(...values){this.children.push(...values);},replaceChildren(){this.children=[];},addEventListener(){},setAttribute(){}};}
  const mask=node(),root=node(),button=node(),identity=node();Object.assign(identity.dataset,{tenantId:tenant,subjectId:subject});
  const bindings={'[data-architecture-lens]':mask,'[data-architecture-lens-steps]':root,'[data-architecture-lens-open]':button,'[data-initiation-workspace]':identity};
  const browser=vm.createContext({document:{querySelector:key=>bindings[key],querySelectorAll:()=>[],createElement:node},sessionStorage:{getItem(){throw new Error('blocked');},setItem(){throw new Error('blocked');}}});vm.runInContext(source,browser);vm.runInContext(go.slice(start,end),browser);
  browser.recordArchitectureStep('flow','保存招投标','当前操作','waiting');browser.recordArchitectureStep('business-engagement','Business Engagement 原生接口已回复','BE 仅拥有材料引用','complete');
  assert.equal(root.children.length,2);assert.equal(root.children[1].children[1].children[0].textContent,'Business Engagement 原生接口已回复');assert.equal(root.children[1].children[1].children[1].textContent,'BE 仅拥有材料引用');assert.equal(go.slice(start,end).includes('innerHTML'),false);
});
test('browser request trace describes ownership but does not record command body, paths or secrets',()=>{
  const go=readFileSync(new URL('./server.go',import.meta.url),'utf8'),start=go.indexOf('async function businessApplicationFetch'),end=go.indexOf('function realInitiationItem',start),bridge=go.slice(start,end);
  assert.match(bridge,/traceEpoch=architectureTrace\?\.epoch/);for(const line of bridge.split('\n').filter(line=>line.includes('recordArchitectureStep(')))assert.doesNotMatch(line,/options\.body|JSON\.stringify|csrf_token|authorization\.url|data\.error|recordArchitectureStep\([^\n]*path[,)]/);
});
