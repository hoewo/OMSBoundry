// Original personnel UI view port. Project is the sole assignment fact owner.
export const contributorRole = 'commercial_contributor';
export const contributorAction = 'project-workspace.assignment.commercial-contributor.manage';
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const revision = value => Number.isSafeInteger(value) && value > 0;
export class ContributorAdapter {
  constructor({details, request, authorize, applicationRequest, tenantID, subjectID, storage, updated=()=>{}, changed=()=>{}, newID=()=>crypto.randomUUID()}) {
    Object.assign(this,{details,request,authorize,applicationRequest,tenantID,subjectID,storage,updated,changed,newID});
    this.projectID=details()?.project?.id;
    if(!uuid.test(this.projectID||'')||!uuid.test(tenantID||'')||!uuid.test(subjectID||''))throw new Error('人员工作面的身份或项目上下文无效');
    this.value=null;this.persons=new Map();this.pending=null;this.conflict=false;this.tail=Promise.resolve();
    this.key=`oms.oa.b02.pending.v1:${tenantID}:${subjectID}:${this.projectID}`;
    try { const saved=JSON.parse(storage?.getItem(this.key)||'null');if(this.validCommand(saved))this.pending=saved; }catch{/* Untrusted/disabled session storage is never authority. */}
  }
  assertCurrent(){if(this.details()?.project?.id!==this.projectID||this.details()?.project?.tenant_id!==this.tenantID)throw new Error('当前项目已切换，请重新打开人员栏目');}
  validCommand(c){return !!c&&uuid.test(c.command_id||'')&&revision(c.expected_revision)&&((c.operation==='assign'&&c.subject_kind==='person'&&c.role_key===contributorRole&&uuid.test(c.subject_id||'')&&revision(c.source_revision))||(c.operation==='end'&&uuid.test(c.assignment_id||'')&&c.end_reason==='移出经营参与人'));}
  validate(value){
    if(value?.project?.id!==this.projectID||value.project.tenant_id!==this.tenantID||!revision(value.project.revision)||!Array.isArray(value.assignments)||!Array.isArray(value.allowed_actions))throw new Error('项目名单的范围或版本不一致');
    const ids=new Set();
    for(const row of value.assignments){if(!uuid.test(row.id||'')||row.project_id!==this.projectID||row.tenant_id!==this.tenantID||ids.has(row.id))throw new Error('人员分工不属于当前项目或重复');ids.add(row.id);if(row.role_key===contributorRole&&(row.subject_kind!=='person'||!uuid.test(row.subject_id||'')||!revision(row.source_revision)||!row.display_snapshot))throw new Error('商业参与分工引用无效');}
    return value;
  }
  canManage(){return !!this.value&&this.value.project.status!=='cancelled'&&this.value.allowed_actions.includes(contributorAction)&&!this.pending&&!this.working;}
  async load({renewCredential=true}={}){
    // Workbench entry/manual refresh renews the credential. Follow-up GETs use
    // that same signed capability; the target still authorizes every request.
    // Never turn a read failure into an automatic business-command retry.
    this.assertCurrent();if(renewCredential)await this.authorize();
    const value=this.validate(await this.request('/api/v1/projects/'+this.projectID));
    this.assertCurrent();this.value=value;this.updated(value);return this.members();
  }
  members(){if(!this.value)throw new Error('项目名单尚未完整读取');return this.value.assignments.filter(a=>a.role_key===contributorRole&&!a.ended_at).map(a=>({id:a.id,project_id:this.projectID,user_id:a.subject_id,role:'business_personnel',is_active:true,user:{id:a.subject_id,real_name:a.display_snapshot}}));}
  async users(){
    this.assertCurrent();this.persons.clear();
    const values=await this.applicationRequest('people-groups','/api/v1/persons');this.assertCurrent();
    if(!Array.isArray(values))throw new Error('人员目录未完整返回');
    const next=new Map(),seen=new Set();
    for(const p of values){if(!uuid.test(p.id||'')||p.tenant_id!==this.tenantID||!revision(p.revision)||typeof p.display_name!=='string'||!p.display_name.trim()||seen.has(p.id))throw new Error('人员目录的范围或版本无效');seen.add(p.id);if(p.status==='active')next.set(p.id,p);}
    this.persons=next;
    // No source for account/department/company positions: do not invent them.
    return [...next.values()].map(p=>({id:p.id,real_name:p.display_name}));
  }
  persist(){try{if(this.pending)this.storage?.setItem(this.key,JSON.stringify(this.pending));else this.storage?.removeItem(this.key);}catch{/* In-memory continuation remains available. */}}
  exclusive(action){const result=this.tail.then(async()=>{this.working=true;this.changed();try{return await action();}finally{this.working=false;this.changed();}});this.tail=result.catch(()=>{});return result;}
  async create(projectID,input){return this.exclusive(async()=>{
    this.assertCurrent();if(projectID!==this.projectID||input?.role!=='business_personnel'||!uuid.test(input.user_id||''))throw new Error('人员与项目不一致');
    if(this.pending)throw new Error('上次操作尚未确认，请继续原操作或刷新核对');
    await this.load({renewCredential:false});if(!this.value.allowed_actions.includes(contributorAction)||this.value.project.status==='cancelled')throw new Error('没有经营参与人管理权限');
    if(this.members().some(row=>row.user_id===input.user_id))throw new Error('该人员已在经营参与人列表中');
    const person=this.persons.get(input.user_id);if(!person)throw new Error('请选择当前已加载的有效人员');
    return this.commit({command_id:this.newID(),expected_revision:this.value.project.revision,operation:'assign',subject_kind:'person',subject_id:person.id,source_revision:person.revision,role_key:contributorRole});
  });}
  async remove(id){return this.exclusive(async()=>{
    this.assertCurrent();if(this.pending)throw new Error('上次操作尚未确认，请先核对');await this.load({renewCredential:false});
    if(!this.value.allowed_actions.includes(contributorAction)||this.value.project.status==='cancelled')throw new Error('没有经营参与人管理权限');
    if(!this.members().some(row=>row.id===id))throw new Error('只能移出当前有效经营参与人');
    return this.commit({command_id:this.newID(),expected_revision:this.value.project.revision,operation:'end',assignment_id:id,end_reason:'移出经营参与人'});
  });}
  async commit(command){
    this.assertCurrent();if(!this.validCommand(command))throw new Error('人员命令无效');
    this.pending=command;this.persist();this.changed();
    try{
      await this.authorize();this.assertCurrent();
      const result=await this.request('/api/v1/projects/'+this.projectID+'/assignments',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(command)});
      const a=result?.assignment, p=result?.project;
      if(p?.id!==this.projectID||p.tenant_id!==this.tenantID||a?.project_id!==this.projectID||a.tenant_id!==this.tenantID||a.role_key!==contributorRole||a.subject_kind!=='person'||!uuid.test(a.id||'')||p.revision!==command.expected_revision+1||(command.operation==='assign'?(a.subject_id!==command.subject_id||a.source_revision!==command.source_revision||!!a.ended_at):(a.id!==command.assignment_id||!a.ended_at)))throw new Error('提交结果尚未确认，请继续原操作');
      this.pending=null;this.conflict=false;this.persist();
    }catch(error){
      this.conflict=error.status===409;
      if([400,401,403,404,422,429].includes(error.status)){this.pending=null;this.persist();}
      throw error;
    }
    try{await this.load({renewCredential:false});}catch(error){throw new Error('人员操作已保存，名单回读失败；请刷新核对，不要再次添加。'+error.message);}
  }
  async retry(){return this.exclusive(async()=>{if(!this.pending||this.conflict)throw new Error('请先刷新核对名单');return this.commit(this.pending);});}
  async reconcile(){return this.exclusive(async()=>{
    await this.load();
    if(this.pending){const c=this.pending;const found=this.value.assignments.some(a=>a.role_key===contributorRole&&(c.operation==='assign'?a.subject_id===c.subject_id&&!a.ended_at:a.id===c.assignment_id&&!!a.ended_at));
      // GET normally hides ended rows. A successful end is also reconciled by
      // absence from a fully validated active assignment list.
      if(found||(c.operation==='end'&&!this.value.assignments.some(a=>a.id===c.assignment_id))||this.conflict){this.pending=null;this.conflict=false;this.persist();}
    }
    this.changed();return this.members();
  });}
}
