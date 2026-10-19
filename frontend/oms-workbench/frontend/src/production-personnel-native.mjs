// Explicit native adapter for the copied D01 UI, not imported by main workspace.
// No credentials/Actor/display snapshots enter command bodies. URLs are relative
// to the caller's configured authenticated Project transport.
const PROFILE='reference-oa-production-personnel-v1';
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const manage='project-workspace.reference-oa-personnel.manage',catalogueManage='project-workspace.reference-oa-discipline.manage';
const knownRejected=status=>[400,401,403,404,409,422].includes(status);
const copy=v=>JSON.parse(JSON.stringify(v));
/** @type {(revision:number)=>Promise<void>} */
const noEnrollment=async()=>{};
export function personnelPendingStore(storage,key){
 if(!storage||typeof key!=='string'||!key)throw Error('必须提供当前可信会话／项目的恢复键');
 return {load(){const value=storage.getItem(key);if(!value)return null;try{return JSON.parse(value)}catch{throw Error('生产人员续办意图损坏，须人工复核')}},save(value){storage.setItem(key,JSON.stringify(value))},clear(){storage.removeItem(key)}};
}
export function createNativePersonnelPort({projectID,request,candidates,pendingStore,enroll=noEnrollment,commandID=()=>crypto.randomUUID()}){
 if(!uuid.test(projectID)||typeof request!=='function'||typeof candidates!=='function'||!pendingStore)throw Error('原生生产人员端口必须配置 Project、受保护传输、People 候选及续办存储');
 const base='/api/v1/projects/'+projectID+'/reference-oa/',catalogue='/api/v1/reference-oa/production-disciplines';
 let view=null,enrollment=null;const people=new Map(),disciplines=new Map();
 const same=id=>{if(id!==projectID)throw Error('生产人员 Project 上下文不一致')};
 const read=async()=>{const v=await request(base+'production-personnel');if(v.profile!==PROFILE||v.project_id!==projectID||!Number.isSafeInteger(v.project_revision)||v.project_revision<1||!Array.isArray(v.facts)||!Array.isArray(v.allowed_actions))throw Error('原生生产人员响应无效');view=v;return v};
 const revision=()=>{if(!view)throw Error('尚未读取生产人员真实版本');return view.project_revision};
 const personRef=id=>{const v=people.get(id);if(!v)throw Error('人员候选尚未读取或已失效，请重新读取');return {id:v.id,revision:v.revision}};
 const disciplineRef=id=>{const v=disciplines.get(id);if(!v||v.state!=='active')throw Error('专业候选尚未读取或已停用');return {id:v.id,revision:v.revision}};
 const ids=values=>[...(values||[])].sort().map(personRef);
const roles=(role,values)=>({role,operation:'replace',people:ids(values)});
 const write=async(path,makeIntention,purpose)=>{
  const fingerprint=JSON.stringify({path,purpose});let pending=pendingStore.load();const freshAttempt=!pending;
  if(pending&&(pending.profile!==PROFILE||pending.project_id!==projectID||pending.fingerprint!==fingerprint))throw Error('另一个生产人员命令结果尚不明确，请先原意图继续，不能自动改写');
  if(!pending){const intention=typeof makeIntention==='function'?makeIntention():makeIntention;const id=commandID();if(!uuid.test(id))throw Error('生产人员命令 ID 无效');pending={profile:PROFILE,project_id:projectID,fingerprint,path,body:{...copy(intention),command_id:id}};pendingStore.save(pending)}
  let committedReply=false;try{const result=await request(path,'POST',pending.body);committedReply=true;if(path===catalogue){if(!uuid.test(result?.id)||!Number.isSafeInteger(result.revision)||!['active','archived'].includes(result.state))throw Error('专业命令回复不明确，请继续原命令')}else if(result?.profile!==PROFILE||result.project_id!==projectID||!Number.isSafeInteger(result.project_revision))throw Error('人员命令回复不明确，请继续原命令');pendingStore.clear();if(result?.project_revision&&view)view={...view,project_revision:result.project_revision,participant_not_applicable:result.participant_not_applicable};return result}
  catch(e){if(freshAttempt&&!committedReply&&knownRejected(e.status))pendingStore.clear();throw e}
 };
 const catalogueCommand=async(operation,id,input={})=>{
  const result=await write(catalogue,()=>{const old=id?disciplines.get(id):null;if(id&&!old)throw Error('专业版本尚未读取，请重新读取');return {operation,expected_revision:old?.revision||0,...(id?{id}:{}),...(operation==='create'||operation==='update'?{name:input.name,description:input.description||''}:{})}}, {operation,id,...input});
  disciplines.set(result.id,result);return {id:result.id,name:result.name,description:result.description,is_active:result.state==='active'};
 };
 return {
  projectID,
  pendingOperation(){const p=pendingStore.load();if(!p)return null;if(p.profile!==PROFILE||p.project_id!==projectID||![base+'production-personnel',catalogue].includes(p.path)||!uuid.test(p.body?.command_id))throw Error('续办命令不属于当前 Project／受保护端口');return {operation:p.body.operation,command_id:p.body.command_id}},
  async retryPending(){const p=pendingStore.load();if(!p)return null;this.pendingOperation();const f=JSON.parse(p.fingerprint);if(f.path!==p.path)throw Error('续办命令指纹无效');return write(p.path,null,f.purpose)},
  async listMembers(id){same(id);const v=await read();return v.facts.filter(f=>!f.ended_at).map(f=>({id:f.id,project_id:projectID,user_id:f.person.id,role:f.role,...(f.discipline?{discipline_id:f.discipline.id,discipline:{id:f.discipline.id,name:f.discipline.name}}:{}),user:{id:f.person.id,real_name:f.person.name,employment_status:'active'}}))},
  async permissions(id){same(id);let v;try{v=await read()}catch(e){if(e.status!==404)throw e;try{const c=await request(base+'production-profile');if(c.profile!==PROFILE||c.project_id!==projectID||!Number.isSafeInteger(c.project_revision)||c.project_revision<1||!['draft','active','pending_completion','completed','cancelled'].includes(c.project_status)||typeof c.installed!=='boolean'||!Array.isArray(c.allowed_actions))throw Error('生产人员初始化权限响应无效');if(c.installed)throw Error('生产人员资料已初始化但读取不一致，请重新读取');enrollment=c;return {can_manage:c.allowed_actions.includes(manage),can_manage_disciplines:false,participant_not_applicable:false,not_enrolled:true}}catch(contextError){if(contextError.status===403)return {can_manage:false,can_manage_disciplines:false,participant_not_applicable:false,not_enrolled:true};throw contextError}}let catalogueActions=[];try{const c=await request(catalogue);if(c.profile!==PROFILE||!Array.isArray(c.allowed_actions))throw Error('原生专业权限响应无效');catalogueActions=c.allowed_actions}catch(e){if(![401,403,404].includes(e.status))throw e}enrollment=null;return {can_manage:v.allowed_actions.includes(manage),can_manage_disciplines:catalogueActions.includes(catalogueManage),participant_not_applicable:v.participant_not_applicable===true}},
  async initialize(){if(!enrollment||enrollment.installed||!enrollment.allowed_actions.includes(manage))throw Error('当前账号没有初始化生产人员的原生权限');await enroll(enrollment.project_revision)},
  async candidates(id){same(id);const values=await candidates(id);if(!Array.isArray(values))throw Error('People 候选响应无效');people.clear();return values.map(v=>{if(!uuid.test(v.id)||!Number.isSafeInteger(v.revision)||v.revision<1||v.status!=='active'||!v.display_name)throw Error('People 候选不是有效真实 Person／revision');people.set(v.id,v);return {id:v.id,real_name:v.display_name,employment_status:'active'}})},
  async listDisciplines(includeInactive){const path=includeInactive?catalogue+'?include_archived=true':base+'production-disciplines';const result=await request(path);if(result.profile!==PROFILE||!Array.isArray(result.items))throw Error('原生专业目录响应无效');for(const v of result.items){if(!uuid.test(v.id)||!Number.isSafeInteger(v.revision)||v.revision<1||!['active','archived'].includes(v.state))throw Error('专业目录版本无效');disciplines.set(v.id,v)}return result.items.map(v=>({id:v.id,name:v.name,description:v.description,is_active:v.state==='active'}))},
  async saveDiscipline(id,input){same(id);return write(base+'production-personnel',()=>({profile:PROFILE,expected_project_revision:revision(),operation:'replace_discipline',...(input.original_discipline_id?{source_discipline_id:input.original_discipline_id}:{}),target_discipline:disciplineRef(input.discipline_id),changes:[roles('designer',input.designer_ids),input.participant_operation==='preserve'?{role:'participant',operation:'preserve'}:roles('participant',input.participant_ids),roles('reviewer',input.reviewer_id?[input.reviewer_id]:[]),roles('auditor',input.auditor_id?[input.auditor_id]:[])]}),{operation:'replace_discipline',...input,designer_ids:[...(input.designer_ids||[])].sort(),...(input.participant_operation==='preserve'?{participant_ids:undefined}:{participant_ids:[...(input.participant_ids||[])].sort()})})},
  async saveReviewers(id,input){same(id);return write(base+'production-personnel',()=>({profile:PROFILE,expected_project_revision:revision(),operation:'replace_reviewers',changes:[roles('approver',input.approver_id?[input.approver_id]:[]),roles('estimator',input.estimator_id?[input.estimator_id]:[])]}),{operation:'replace_reviewers',...input})},
  async endDiscipline(id,disciplineID,input){same(id);return write(base+'production-personnel',()=>({profile:PROFILE,expected_project_revision:revision(),operation:'end_discipline',source_discipline_id:disciplineID,participant_operation:input.participant_operation}),{operation:'end_discipline',disciplineID,...input})},
  createDiscipline:input=>catalogueCommand('create',null,input),updateDiscipline:(id,input)=>catalogueCommand('update',id,input),deleteDiscipline:async id=>{await catalogueCommand('archive',id)},
 };
}
