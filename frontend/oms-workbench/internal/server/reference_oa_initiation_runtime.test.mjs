import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source = readFileSync(new URL('./reference_oa_initiation_runtime.js', import.meta.url), 'utf8');
const context = vm.createContext({});
vm.runInContext(source, context);
const Flow = context.ReferenceOAInitiationFlow;
const id = '33333333-3333-4333-8333-333333333333';
const tenant = '11111111-1111-4111-8111-111111111111';
const person = {person_id: '55555555-5555-4555-8555-555555555555', source_revision: 2};
const actions = ['project-workspace.update', 'project-workspace.initiation.complete', 'project-workspace.assignment.production-owner.manage'];
const fixture = () => ({project: {id, name: '项目', node_type: 'project', project_number: '', summary: '', project_type_keys: [], revision: 1, status: 'draft'}, assignments: [{id: 'business', role_key: 'business_owner', subject_kind: 'person', subject_id: person.person_id}], allowed_actions: actions});

function setup(request) {
  const calls = [];
  let sequence = 0;
  const flow = new Flow({authorize: async () => calls.push({authorize: true}), uuid: () => 'a0000000-0000-4000-8000-' + String(++sequence).padStart(12, '0'),
    request: async (path, options) => {
      const body = options?.body ? JSON.parse(options.body) : null;
      calls.push({path, method: options?.method || 'GET', body});
      return request(path, options, body);
    }});
  return {flow, calls};
}

test('lost activation acknowledgement says unknown, then explicit retry reuses the committed command', async () => {
  let activation = 0, savedID;
  const {flow, calls} = setup(async (path, options, body) => {
    if (path.endsWith('/initiation-info-changes')) return {project: {...fixture().project, ...body.profile, revision: 2}, owners: []};
    activation++;
    if (!savedID) { savedID = body.command_id; throw Object.assign(new Error('ack unavailable'), {status: 503}); }
    assert.equal(body.command_id, savedID);
    return {...flow.details.get(id).project, status: 'active', revision: 3};
  });
  flow.details.set(id, fixture());
  const fields = {name:'项目', project_number:'HNJH2026-998', project_type_keys:['design'], summary:''};
  await assert.rejects(flow.complete(id, fields), /ack unavailable/);
  const failure = flow.completionFailure(id, new Error('ack unavailable'), 1);
  assert.equal(failure.title, '提交结果待确认');
  assert.match(failure.message, /部分资料已确认保存/);
  assert.match(failure.message, /输入已保留.*同一操作.*不要另建项目/);
  await flow.complete(id, fields);
  assert.equal(activation, 2); assert.equal(flow.commands.size, 0);
  assert.equal(calls.filter(c=>c.path?.endsWith('/initiation-info-changes')).length, 1);
});

test('unconfirmed initiation save does not claim that activation or saved profile failed', async () => {
  const {flow} = setup(async () => { throw new Error('connection lost'); }); flow.details.set(id, fixture());
  await assert.rejects(flow.complete(id, {name:'项目', project_number:'HNJH2026-998', project_type_keys:['design'], summary:''}));
  const failure = flow.completionFailure(id, new Error('connection lost'), 1);
  assert.equal(failure.title, '提交结果待确认'); assert.doesNotMatch(failure.message, /资料已确认保存/);
  assert.equal(flow.details.get(id).project.revision, 1);
});

test('definite rejection after confirmed save is not an unknown result', async () => {
  const {flow} = setup(async (path, options, body) => {
    if (path.endsWith('/initiation-info-changes')) return {project:{...fixture().project, ...body.profile, revision:2},owners:[]};
    throw Object.assign(new Error('permission denied'), {status:403});
  }); flow.details.set(id, fixture());
  await assert.rejects(flow.complete(id, {name:'项目', project_number:'HNJH2026-998', project_type_keys:['design'], summary:''}));
  const failure = flow.completionFailure(id, new Error('permission denied'), 1);
  assert.equal(failure.title, '立项尚未完成'); assert.match(failure.message, /部分资料已确认保存/);
  assert.equal(flow.commands.size, 0);
});

test('another project or action cannot contaminate the current completion result', () => {
  const {flow} = setup(()=>{}); flow.details.set(id, fixture());
  flow.commands.set(JSON.stringify(['/api/v1/projects/other/complete-initiation','POST',{}]), {});
  flow.commands.set(JSON.stringify(['/api/v1/projects/'+id+'/counterparty-context','POST',{}]), {});
  const failure = flow.completionFailure(id, new Error('validation rejected'), 1);
  assert.equal(failure.title, '立项尚未完成'); assert.doesNotMatch(failure.message, /部分资料/);
});

test('a confirmed active project with failed detail read is never labelled incomplete or unknown', () => {
  const {flow} = setup(()=>{}); flow.details.set(id, {project:{...fixture().project,status:'active',revision:3}});
  const failure=flow.completionFailure(id,new Error('read unavailable'),2);
  assert.equal(failure.title,'立项已完成，详情读取失败'); assert.match(failure.message,/重新打开此项目.*不要再次创建/);
});

function navigationFixture(href) {
  const fields = new Map(), calls = [];
  const state = {URL, location: {href}, document: {querySelectorAll: () => []}};
  state.history = {state: {keep: 'unchanged'}, pushState(value, _, url) { this.state = value; state.location.href = String(url); calls.push(['push', String(url)]); }, replaceState(value, _, url) { this.state = value; state.location.href = String(url); calls.push(['replace', String(url)]); }};
  const local = vm.createContext(state); vm.runInContext(source, local);
  const controller = Object.create(local.ReferenceOAInitiationController.prototype);
  controller.field = selector => { if (!fields.has(selector)) fields.set(selector, {value: '', hidden: false, disabled: false}); return fields.get(selector); };
  controller.showView = view => calls.push(['view', view]);
  controller.drawer = (kind, open) => calls.push(['drawer', kind, open]);
  controller.profileDrawer = open => calls.push(['profile', open]);
  controller.populatePeople = () => {};
  controller.loadPeople = async () => {};
  return {controller, state, calls};
}

test('entering initiation clears the previous project and tab before any refresh', () => {
  const {controller, state} = navigationFixture('https://oatest.antuinfo.cn/workbench/?project_id=old&tab=business&return_to=%2Fbilling%2F');
  controller.projectID = 'old'; controller.enterInitiations();
  const url = new URL(state.location.href);
  assert.equal(url.searchParams.get('project_id'), null); assert.equal(url.searchParams.get('tab'), null);
  assert.equal(url.searchParams.get('view'), 'initiation'); assert.equal(url.searchParams.get('return_to'), '/billing/');
  assert.equal(controller.projectID, null); assert.equal(state.history.state.keep, 'unchanged');
});
test('opening completion identifies the new project in URL and refresh restores that drawer read-only', async () => {
  const {controller, state, calls} = navigationFixture('https://oatest.antuinfo.cn/workbench/?project_id=old&tab=basic');
  let reads = 0; controller.flow = {busy:false, load:async () => { reads++; return fixture(); }, can:() => true};
  await controller.openCompletion(id);
  assert.equal(new URL(state.location.href).searchParams.get('project_id'), id);
  assert.equal(new URL(state.location.href).searchParams.get('view'), 'initiation');
  assert.equal(new URL(state.location.href).searchParams.has('tab'), false);
  const pushes = calls.filter(value => value[0] === 'push').length;
  await controller.restoreLocation();
  assert.equal(controller.projectID, id); assert.equal(reads, 2);
  assert.equal(calls.filter(value => value[0] === 'push').length, pushes);
  assert.equal(calls.at(-1)[0], 'drawer'); assert.equal(calls.at(-1)[2], true);
});
test('cancel completion returns to initiation with no stale project deep link', () => {
  const {controller, state, calls} = navigationFixture('https://oatest.antuinfo.cn/workbench/?project_id=new&view=initiation');
  controller.closeCompletion();
  assert.equal(new URL(state.location.href).searchParams.has('project_id'), false);
  assert.equal(new URL(state.location.href).searchParams.get('view'), 'initiation');
  assert.equal(calls[0][2], false); assert.equal(calls.at(-1)[1], 'initiation-reference');
});
test('reloading an already activated completion closes the stale drawer only after same-project detail succeeds', async () => {
  const {controller,calls}=navigationFixture('https://oatest.antuinfo.cn/workbench/?view=initiation&project_id='+id);
  controller.flow={busy:false,load:async()=>({project:{...fixture().project,status:'active',revision:3}})};
  controller.openDetail=async value=>{assert.equal(value,id);calls.push(['detail',value]);};
  await controller.openCompletion(id);
  assert.equal(calls[0][0],'detail'); assert.deepEqual(calls.at(-1),['drawer','complete',false]);
});
test('failed detail read after activation preserves the completion drawer and input', async () => {
  const {controller,calls}=navigationFixture('https://oatest.antuinfo.cn/workbench/?view=initiation&project_id='+id);
  controller.flow={busy:false,load:async()=>({project:{...fixture().project,status:'active',revision:3}})};
  controller.openDetail=async()=>{throw new Error('detail unavailable');};
  await assert.rejects(controller.openCompletion(id),/detail unavailable/);
  assert.equal(calls.some(call=>call[0]==='drawer'),false);
});
test('back/forward restores initiation list, project list or active detail without adding history', async () => {
  for (const [href, expected] of [['?view=initiation', 'initiation-reference'], ['', 'list'], ['?project_id=active&tab=business', 'active']]) {
    const {controller, calls} = navigationFixture('https://oatest.antuinfo.cn/workbench/' + href);
    controller.returnToList = push => { assert.equal(push, false); calls.push(['view', 'list']); };
    controller.openDetail = async (value, options) => { assert.equal(options.history, false); calls.push(['view', value]); };
    await controller.restoreLocation();
    assert.equal(calls.at(-1)[1], expected); assert.equal(calls.some(value => value[0] === 'push'), false);
  }
});
test('legacy draft deep links resume completion instead of showing an active detail', async () => {
  const {controller} = navigationFixture('https://oatest.antuinfo.cn/workbench/?project_id=' + id);
  controller.flow = {load: async () => fixture()}; let resumed = false;
  controller.openCompletion = async (value, options) => { assert.equal(value, id); assert.equal(options.history, false); resumed = true; };
  await controller.openDetail(id, {history:false}); assert.equal(resumed, true);
});
test('successful detail URL removes the completion hint without discarding other query context', () => {
  const {controller, state} = navigationFixture('https://oatest.antuinfo.cn/workbench/?project_id=' + id + '&view=initiation&return_to=%2Fbilling%2F');
  controller.detailTab = 'basic'; controller.setURL(id);
  const url = new URL(state.location.href);
  assert.equal(url.searchParams.get('view'), null); assert.equal(url.searchParams.get('tab'), 'basic');
  assert.equal(url.searchParams.get('project_id'), id); assert.equal(url.searchParams.get('return_to'), '/billing/');
});
test('production URL preserves its selected native tab and clears it outside production',()=>{
  const {controller,state}=navigationFixture('https://oatest.antuinfo.cn/workbench/?return_to=%2Fbilling%2F&business_tab=bonus');
  controller.businessWorkspace={productionTab:'bonus',tab:'contracts'};
  controller.detailTab='production';controller.setURL(id,true);
  let url=new URL(state.location.href);assert.equal(url.searchParams.get('production_tab'),'bonus');assert(!url.searchParams.has('business_tab'));assert.equal(url.searchParams.get('return_to'),'/billing/');
  controller.detailTab='business';controller.setURL(id);url=new URL(state.location.href);
  assert(!url.searchParams.has('production_tab'));assert.equal(url.searchParams.get('business_tab'),'contracts');
  controller.setURL(null);url=new URL(state.location.href);assert(!url.searchParams.has('production_tab'));assert(!url.searchParams.has('business_tab'));
});

test('active owner editor submits an explicit selected blank as clear and retains the other owner', async () => {
  const {controller} = navigationFixture('https://oatest.antuinfo.cn/workbench/?project_id=' + id);
  const details = {assignments:[{role_key:'business_owner',subject_id:person.person_id},{role_key:'production_owner',subject_id:id}]};
  const commands = [];
  controller.editProjectID = id;
  controller.flow = {details:new Map([[id,details]]),can:() => false,changeBasicInfo:async (...args) => commands.push(args)};
  controller.record = () => {}; controller.openDetail = async () => {};
  controller.field('[data-real-profile-business-owner]').value = person.person_id;
  controller.field('[data-real-profile-production-owner]').value = '';
  await controller.saveProfile({disabled:false});
  assert.equal(commands.length,1); assert.equal(commands[0][0],id); assert.equal(commands[0][1],null);
  assert.deepEqual(JSON.parse(JSON.stringify(commands[0][2])),[{role_key:'production_owner',operation:'clear'}]);
});
test('unchanged or disabled owner selections cannot produce a clear operation', async () => {
  for (const disabled of [false,true]) {
    const {controller} = navigationFixture('https://oatest.antuinfo.cn/workbench/?project_id=' + id);
    const commands = [];
    controller.editProjectID = id;
    controller.flow = {details:new Map([[id,{assignments:[{role_key:'production_owner',subject_id:person.person_id}]}]]),can:() => false,changeBasicInfo:async (...args) => commands.push(args)};
    controller.record = () => {}; controller.openDetail = async () => {};
    const select = controller.field('[data-real-profile-production-owner]');
    select.value = disabled ? '' : person.person_id; select.disabled = disabled;
    await controller.saveProfile({disabled:false});
    assert.equal(commands.length,1); assert.equal(commands[0][2].length,0);
  }
});

const contactFixture = () => ({project_id:id,project_revision:3,project_status:'active',context:null,allowed_actions:['project-workspace.counterparty-context.read','project-workspace.counterparty-context.manage']});
test('client name suggestion query is encoded and read-only, empty input does not authorize or request',async()=>{
 const {flow,calls}=setup(async()=>({schema_version:'project-counterparty-name-suggestions-v1',names:['甲方%_单位']}));
 assert.equal((await flow.suggestClientNames('  ')).length,0);assert.equal(calls.length,0);
 const names=await flow.suggestClientNames(' 甲方%_ ');assert.equal(names[0],'甲方%_单位');assert.equal(calls[1].path,'/api/v1/counterparty-context/name-suggestions?q='+encodeURIComponent('甲方%_')+'&limit=10');assert.equal(calls[1].method,'GET');assert.equal(calls.filter(v=>v.body).length,0);
});
test('inline historical name completion prioritizes prefixes, supports fuzzy, and never changes the input by itself',()=>{
 const complete=context.referenceOANameCompletion;
 const names=['历史甲方单位','甲方单位'];assert.equal(complete('甲方',names).value,'甲方单位');assert.equal(complete('甲方',names).suffix,'单位');assert.equal(complete('历史',names).suffix,'甲方单位');assert.equal(complete('方单位',names).suffix,' → 历史甲方单位');assert.equal(complete('甲方单位',['甲方单位']),null);assert.equal(complete(' 甲方',names),null);assert.equal(complete('甲方 ',names),null);assert.equal(complete('',names),null);
 assert.equal(complete('甲方单位',names).value,'历史甲方单位'); // Equal candidates are excluded, not other containing names: same as original OA.
});
test('malformed or unrelated name suggestions fail closed and cause no write',async()=>{
 for(const names of [['无关'],['甲方','甲方'],['甲方',12],Array(11).fill('甲方')]){const {flow,calls}=setup(async()=>({schema_version:'project-counterparty-name-suggestions-v1',names}));await assert.rejects(flow.suggestClientNames('甲方'),/响应无效/);assert.equal(calls.filter(v=>v.body).length,0)}
});
test('project client editor saves only local contact facts and refreshes same project',async()=>{
 const state=contactFixture();const {flow,calls}=setup(async(path,options,body)=>options?{...state,project_revision:4,context:{schema_version:'project-counterparty-context-v1',role:'client',name:body.name,contact_name:body.contact_name,contact_phone:body.contact_phone}}:state);
 await flow.loadCounterparty(id);await flow.saveCounterparty(id,{name:' 甲方 ',contact_name:' 李工 ',contact_phone:' +86 (0571) 123-456 '});
 const write=calls.find(v=>v.body);assert.equal(write.path,'/api/v1/projects/'+id+'/counterparty-context');assert.equal(write.body.name,'甲方');assert.equal(write.body.expected_revision,3);assert.equal('party_id' in write.body,false);assert.equal(calls.some(v=>v.path?.includes('party-directory')),false);assert.equal(flow.contacts.get(id).project_revision,4);
});
test('client editor validation and view-only state produce zero writes',async()=>{
 const state=contactFixture();const {flow,calls}=setup(async()=>state);await flow.loadCounterparty(id);
 await assert.rejects(flow.saveCounterparty(id,{name:' ',contact_phone:''}),/甲方名称/);await assert.rejects(flow.saveCounterparty(id,{name:'甲方',contact_phone:'abc'}),/正确/);
 state.allowed_actions=['project-workspace.counterparty-context.read'];await flow.loadCounterparty(id);await assert.rejects(flow.saveCounterparty(id,{name:'甲方'}),/没有编辑/);assert.equal(calls.filter(v=>v.body).length,0);
});
test('uncertain client save repeats the same command, revision and contact payload',async()=>{
 const state=contactFixture();let writes=0;const {flow,calls}=setup(async(path,options,body)=>{if(!options)return state;if(++writes===1)throw new Error('response lost');return {...state,project_revision:4,context:{schema_version:'project-counterparty-context-v1',role:'client',name:body.name}}});await flow.loadCounterparty(id);const fields={name:'甲方',contact_name:'',contact_phone:''};await assert.rejects(flow.saveCounterparty(id,fields),/response lost/);await flow.saveCounterparty(id,fields);const commands=calls.filter(v=>v.body);assert.deepEqual(commands[0].body,commands[1].body);
});
test('failed client read is not accepted as empty, and mismatched project is rejected',async()=>{
 const {flow}=setup(async()=>{throw new Error('unavailable')});await assert.rejects(flow.loadCounterparty(id),/unavailable/);assert.equal(flow.contacts.has(id),false);
 const wrong=setup(async()=>({...contactFixture(),project_id:person.person_id}));await assert.rejects(wrong.flow.loadCounterparty(id),/契约/);assert.equal(wrong.flow.contacts.has(id),false);
});
test('definite client revision conflict retains snapshot until explicit read',async()=>{
 let current=contactFixture();const {flow,calls}=setup(async(path,options)=>{if(options)throw Object.assign(new Error('stale'),{status:409});return current});await flow.loadCounterparty(id);await assert.rejects(flow.saveCounterparty(id,{name:'甲方'}),/stale/);assert.equal(flow.contacts.get(id).project_revision,3);current={...current,project_revision:5};await flow.loadCounterparty(id);assert.equal(flow.contacts.get(id).project_revision,5);assert.equal(calls.filter(v=>v.body).length,1);
});

test('one basic-info save atomically requests core and independently authorized owners', async () => {
  const initial = fixture(); initial.project = {...initial.project,status:'active',project_number:'OA-001',project_type_keys:['design'],revision:5}; initial.allowed_actions = ['project-workspace.profile.correct','project-workspace.assignment.business-owner.manage','project-workspace.assignment.production-owner.manage'];
  const {flow,calls} = setup(async (path,options,body) => options ? {project:{...initial.project,...body.profile,revision:6},owners:[]} : initial);
  await flow.load(id);
  await flow.changeBasicInfo(id,{name:'新名称',project_number:'OA-001',project_type_keys:['design']},[{role_key:'business_owner',operation:'set',person},{role_key:'production_owner',operation:'clear'}]);
  const writes=calls.filter(value=>value.body);assert.equal(writes.length,1);assert.equal(writes[0].path,'/api/v1/projects/'+id+'/basic-info-changes');assert.equal(writes[0].body.expected_revision,5);assert.equal(writes[0].body.profile.name,'新名称');assert.equal(writes[0].body.owners.length,2);assert.equal(flow.details.get(id).project.revision,6);
});
test('owner-only edit does not require or transmit core fields, and cannot change other owner role', async () => {
  const initial=fixture();initial.project.status='active';initial.allowed_actions=['project-workspace.assignment.production-owner.manage'];
  const {flow,calls}=setup(async(path,options)=>options?{project:initial.project,owners:[]}:initial);await flow.load(id);
  await flow.changeBasicInfo(id,null,[{role_key:'production_owner',operation:'set',person}]);
  assert.equal('profile' in calls.find(value=>value.body).body,false);
  await assert.rejects(flow.changeBasicInfo(id,null,[{role_key:'business_owner',operation:'clear'}]),/没有该类/);
  assert.equal(calls.filter(value=>value.body).length,1);
});
test('basic-info uncertain response reuses complete atomic command and retains input revision',async()=>{
  const initial=fixture();initial.project.status='active';initial.allowed_actions=['project-workspace.assignment.manage'];let attempt=0;
  const {flow,calls}=setup(async(path,options)=>{if(!options)return initial;if(++attempt===1)throw new Error('response lost');return {project:{...initial.project,revision:2},owners:[]}});await flow.load(id);
  const owners=[{role_key:'production_owner',operation:'set',person}];await assert.rejects(flow.changeBasicInfo(id,null,owners),/response lost/);await flow.changeBasicInfo(id,null,owners);
  const writes=calls.filter(value=>value.body);assert.equal(writes[0].body.command_id,writes[1].body.command_id);assert.equal(writes[1].body.expected_revision,1);
});
test('basic-info validates all roles and state before sending any write',async()=>{
  const initial=fixture();initial.project.status='active';initial.allowed_actions=['project-workspace.assignment.manage'];const {flow,calls}=setup(async()=>initial);await flow.load(id);
  await assert.rejects(flow.changeBasicInfo(id,null,[{role_key:'participant',operation:'clear'}]),/类型无效/);
  await assert.rejects(flow.changeBasicInfo(id,null,[{role_key:'production_owner',operation:'set'}]),/请选择/);
  await assert.rejects(flow.changeBasicInfo(id,null,[{role_key:'production_owner',operation:'clear'},{role_key:'production_owner',operation:'clear'}]),/重复/);
  assert.equal(calls.filter(value=>value.body).length,0);
});

test('dispatch sends only name, node type and versioned business owner', async () => {
  const {flow, calls} = setup(async () => ({id, status: 'draft', revision: 1}));
  await flow.dispatch({name: ' 项目 ', node_type: 'project', business_owner: person});
  const action = calls.find(value => value.body);
  assert.equal(action.path, '/api/v1/initiation-dispatches');
  assert.equal(action.body.name, '项目');
  assert.equal(action.body.business_owner.source_revision, 2);
  assert.equal('project_number' in action.body, false);
  assert.equal('project_type_keys' in action.body, false);
});

test('uncertain dispatch retry reuses command ID instead of creating duplicates', async () => {
  let attempt = 0;
  const {flow, calls} = setup(async () => { if (++attempt === 1) throw new Error('network lost'); return {id}; });
  const input = {name: '项目', node_type: 'project', business_owner: person};
  await assert.rejects(flow.dispatch(input), /network lost/);
  await flow.dispatch(input);
  const writes = calls.filter(value => value.body);
  assert.equal(writes[0].body.command_id, writes[1].body.command_id);
});

test('completion fills details then activates the same project without a production owner', async () => {
  const details = fixture();
  const {flow, calls} = setup(async (path, options, body) => {
    if (!options) return details;
    if (path.endsWith('/initiation-info-changes')) return {project: {...details.project, ...body.profile, revision: 2}, owners: []};
    return {...details.project, revision: 3, status: 'active'};
  });
  await flow.load(id);
  const active = await flow.complete(id, {name: '项目', project_number: 'OA-001', summary: '说明', project_type_keys: ['design', 'consultation']});
  assert.equal(active.id, id);
  assert.equal(active.status, 'active');
  const writes = calls.filter(value => value.body);
  assert.deepEqual(writes.map(value => value.method), ['POST', 'POST']);
  assert.equal(writes[0].path.endsWith('/initiation-info-changes'), true);
  assert.equal(writes[1].body.expected_revision, 2);
  assert.equal(writes[1].body.initiation_profile, 'business-owner-v1');
  assert.equal(calls.some(value => value.path?.endsWith('/assignments')), false);
});

test('lost activation response retries activation, not already saved details', async () => {
  let activation = 0;
  const initial = fixture();
  const {flow, calls} = setup(async (path, options, body) => {
    if (!options) return initial;
    if (path.endsWith('/initiation-info-changes')) return {project: {...initial.project, ...body.profile, revision: 2}, owners: []};
    if (++activation === 1) throw new Error('activation response lost');
    return {...initial.project, revision: 3, status: 'active'};
  });
  await flow.load(id);
  const fields = {name: '项目', project_number: 'OA-001', summary: '', project_type_keys: ['design']};
  await assert.rejects(flow.complete(id, fields), /activation response lost/);
  assert.equal(flow.details.get(id).project.revision, 2);
  await flow.complete(id, fields);
  assert.equal(calls.filter(value => value.path?.endsWith('/initiation-info-changes')).length, 1);
  const activations = calls.filter(value => value.path?.endsWith('/complete-initiation'));
  assert.equal(activations[0].body.command_id, activations[1].body.command_id);
});

test('later owner assignment consumes active revision and never covers existing history', async () => {
  const initial = fixture();
  initial.project.status = 'active'; initial.project.revision = 4;
  const {flow, calls} = setup(async (path, options, body) => options ? {project: {...initial.project, revision: 5}, assignment: {subject_kind: 'person', subject_id: body.subject_id, role_key: 'production_owner'}} : initial);
  await flow.load(id);
  await flow.assignProduction(id, person);
  const write = calls.find(value => value.body);
  assert.equal(write.body.expected_revision, 4);
  await assert.rejects(flow.assignProduction(id, {...person, person_id: '66666666-6666-4666-8666-666666666666'}), /不能覆盖历史/);
  assert.equal(calls.filter(value => value.body).length, 1);
});

test('missing native completion capability causes zero writes', async () => {
  const initial = fixture(); initial.allowed_actions = [];
  const {flow, calls} = setup(async () => initial);
  await flow.load(id);
  await assert.rejects(flow.complete(id, {name: '项目', project_number: 'OA-001', project_type_keys: ['design']}), /没有完成/);
  assert.equal(calls.filter(value => value.body).length, 0);
});

test('pending completion can resume without replaying already saved fields', async () => {
  const initial = fixture();
  Object.assign(initial.project, {status: 'pending_completion', revision: 3, project_number: 'OA-001', project_type_keys: ['design']});
  const {flow, calls} = setup(async (path, options) => options ? {...initial.project, status: 'active', revision: 4} : initial);
  await flow.load(id);
  await flow.complete(id, {name: '项目', project_number: 'OA-001', project_type_keys: ['design']});
  const writes = calls.filter(value => value.body);
  assert.equal(writes.length, 1);
  assert.equal(writes[0].body.expected_revision, 3);
  assert.equal(writes[0].path.endsWith('/complete-initiation'), true);
});

test('an active project cannot resubmit completion using stale action projection', async () => {
  const initial = fixture(); initial.project.status = 'active';
  const {flow, calls} = setup(async () => initial);
  await flow.load(id);
  await assert.rejects(flow.complete(id, {name: '项目', project_number: 'OA-001', project_type_keys: ['design']}), /不处于待立项/);
  assert.equal(calls.filter(value => value.body).length, 0);
});

test('legacy missing business owner fails before any write and can be explicitly repaired together with production replacement', async () => {
  const initial = fixture(); initial.project.status = 'pending_completion'; initial.project.revision = 6;
  const old = {id:'old', role_key:'production_owner',subject_kind:'person',subject_id:'66666666-6666-4666-8666-666666666666',display_snapshot:'原生产'};
  initial.assignments = [old]; initial.allowed_actions = [...actions,'project-workspace.assignment.business-owner.manage'];
  const {flow,calls} = setup(async(path, options, body) => {
    if (!options) return initial;
    if (path.endsWith('/initiation-info-changes')) return {project:{...initial.project,...body.profile,revision:7},owners:body.owners.map(v=>({role_key:v.role_key,changed:true,previous:v.role_key==='production_owner'?{...old,ended_at:'now'}:undefined,current:{id:v.role_key,role_key:v.role_key,subject_kind:'person',subject_id:v.person.person_id}}))};
    return {...initial.project,status:'active',revision:8};
  });
  await flow.load(id); const fields = {name:'项目',project_number:'OA-101',project_type_keys:['consultation','survey']};
  await assert.rejects(flow.complete(id,fields),/缺少经营负责人/); assert.equal(calls.filter(v=>v.body).length,0);
  await flow.complete(id,{...fields,business_owner:person,production_owner:person});
  const writes = calls.filter(v=>v.body); assert.equal(writes.length,2); assert.equal(writes[0].body.expected_revision,6); assert.equal(writes[0].body.owners.length,2); assert.equal(writes[1].body.expected_revision,7);
  assert.equal(flow.details.get(id).assignments.find(v=>v.id==='old').ended_at,'now'); assert.equal(flow.details.get(id).project.status,'active');
  assert.equal(calls.some(v=>v.path?.endsWith('/assignments')),false);
});
test('all initiation permissions are checked before profile writes, and missing recovery rights never auto-assign',async()=>{
  const initial=fixture();initial.assignments=[];
  const {flow,calls}=setup(async()=>initial);await flow.load(id);
  await assert.rejects(flow.complete(id,{name:'项目',project_number:'OA-101',project_type_keys:['design'],business_owner:person}),/调整权限/);
  assert.equal(calls.filter(v=>v.body).length,0);
});
test('same existing production owner or blank optional selection preserves the owner without a command',async()=>{
  for(const selection of [null,person]){
    const initial=fixture();Object.assign(initial.project,{status:'pending_completion',revision:6,project_number:'OA-101',project_type_keys:['design']});
    initial.assignments.push({id:'production',role_key:'production_owner',subject_kind:'person',subject_id:person.person_id,display_snapshot:'原快照'});
    const {flow,calls}=setup(async(path,options)=>options?{...initial.project,status:'active',revision:7}:initial);await flow.load(id);
    await flow.complete(id,{name:'项目',project_number:'OA-101',project_type_keys:['design'],production_owner:selection});
    assert.equal(calls.filter(v=>v.body).length,1);assert.equal(flow.details.get(id).assignments[1].display_snapshot,'原快照');
  }
});
test('uncertain initiation save retries the same atomic command; no activation happens until the save is confirmed',async()=>{
  const initial=fixture();let attempts=0;
  const {flow,calls}=setup(async(path,options,body)=>{if(!options)return initial;if(path.endsWith('/initiation-info-changes')){if(++attempts===1)throw new Error('save response lost');return {project:{...initial.project,...body.profile,revision:2},owners:[]}}return {...initial.project,status:'active',revision:3}});
  await flow.load(id);const fields={name:'项目',project_number:'OA-101',project_type_keys:['design']};
  await assert.rejects(flow.complete(id,fields),/save response lost/);assert.equal(calls.some(v=>v.path?.endsWith('/complete-initiation')),false);
  await flow.complete(id,fields);const saves=calls.filter(v=>v.path?.endsWith('/initiation-info-changes'));assert.deepEqual(saves[0].body,saves[1].body);
});

test('people candidates reject other tenants, stale versions and duplicate IDs', () => {
  const value = {id: person.person_id, tenant_id: tenant, revision: 2, status: 'active', display_name: ' 李晨 '};
  assert.equal(context.normalizeReferenceOAPeople([value], tenant)[0].name, '李晨');
  for (const values of [[{...value, tenant_id: id}], [{...value, revision: 0}], [value, value]]) {
    assert.throws(() => context.normalizeReferenceOAPeople(values, tenant), /不一致/);
  }
});

function activeFixture() {
  const details = fixture();
  Object.assign(details.project, {status: 'active', revision: 4, project_number: 'OA-EDIT-001', project_type_keys: ['design']});
  details.allowed_actions = ['project-workspace.profile.correct'];
  return details;
}
const editedFields = () => ({name: '更正后的项目', project_number: 'OA-EDIT-001', project_type_keys: ['design', 'consultation'], summary: '更正概况', correction_reason: '修改项目基本资料'});

test('active basic edit uses new native action and preserves owners', async () => {
  const initial = activeFixture(); initial.assignments = [{role_key: 'business_owner', subject_id: person.person_id}];
  const {flow, calls} = setup(async (path, options, body) => options ? {...initial.project, name: body.name, revision: 5} : initial);
  await flow.load(id);
  const result = await flow.correctProfile(id, editedFields());
  assert.equal(result.revision, 5);
  assert.equal(flow.details.get(id).assignments[0].subject_id, person.person_id);
  const write = calls.find(value => value.body);
  assert.equal(write.path, '/api/v1/projects/' + id + '/profile-corrections');
  assert.equal(write.method, 'POST');
  assert.equal(write.body.expected_revision, 4);
  assert.equal(write.body.correction_reason, '修改项目基本资料');
});

test('old update, invalid state or empty types cannot submit active edit', async () => {
  for (const kind of ['old capability', 'cancelled', 'empty types', 'missing reason']) {
    const initial = activeFixture(), fields = editedFields();
    if (kind === 'old capability') initial.allowed_actions = ['project-workspace.update'];
    if (kind === 'cancelled') initial.project.status = 'cancelled';
    if (kind === 'empty types') fields.project_type_keys = [];
    if (kind === 'missing reason') fields.correction_reason = '';
    const {flow, calls} = setup(async () => initial);
    await flow.load(id);
    await assert.rejects(flow.correctProfile(id, fields));
    assert.equal(calls.filter(value => value.body).length, 0);
  }
});

test('uncertain correction retry keeps expected revision and command ID', async () => {
  const initial = activeFixture(); let writes = 0;
  const {flow, calls} = setup(async (path, options) => {
    if (!options) return initial;
    if (++writes === 1) throw new Error('saved response lost');
    return {...initial.project, revision: 5};
  });
  await flow.load(id);
  await assert.rejects(flow.correctProfile(id, editedFields()), /saved response lost/);
  await flow.correctProfile(id, editedFields());
  const commands = calls.filter(value => value.body);
  assert.equal(commands[0].body.command_id, commands[1].body.command_id);
  assert.equal(commands[1].body.expected_revision, 4);
});

test('definite conflict never overwrites the revision until explicit reload', async () => {
  const initial = activeFixture();
  const {flow, calls} = setup(async (path, options) => {
    if (!options) return initial;
    throw Object.assign(new Error('revision conflict'), {status: 409});
  });
  await flow.load(id);
  await assert.rejects(flow.correctProfile(id, editedFields()), /revision conflict/);
  assert.equal(flow.details.get(id).project.revision, 4);
  assert.equal(calls.filter(value => value.body).length, 1);
});
