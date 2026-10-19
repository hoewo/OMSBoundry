import {cents,decimal} from './contract-money.mjs';

const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const action=name=>'approval-audit-result.'+name;
const revision=value=>Number.isSafeInteger(value)&&value>0;
const components=['design_fee','survey_fee','consultation_fee'];
const sourceKey=value=>`${value.application}:${value.resource_type}:${value.resource_id}:${value.revision}`;
const exactKeys=(value,keys)=>value&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
const sameSource=(left,right)=>left?.application===right?.application&&left?.resource_type===right?.resource_type&&left?.resource_id===right?.resource_id&&left?.revision===right?.revision;
const sameEvidence=(left,right)=>left.length===right.length&&right.every(ref=>left.some(value=>value.file_id===ref.file_id&&value.version===ref.version&&value.slot===ref.slot));
const sameComponents=(left,right)=>left.length===right.length&&right.every(item=>left.some(value=>value.kind===item.kind&&value.amount_minor===item.amount_minor));
const sameInstant=(left,right)=>Number.isFinite(Date.parse(left))&&Date.parse(left)===Date.parse(right);
const safeMinor=value=>{const minor=cents(String(value??''));if(minor<0n||minor>BigInt(Number.MAX_SAFE_INTEGER))throw new Error('批复审计金额必须是安全整数范围内的非负金额');return Number(minor);};

export const resultAmount=value=>Number.isSafeInteger(value)?decimal(BigInt(value)):null;

export class ApprovalAuditAdapter{
 constructor(options){
  Object.assign(this,{newID:()=>crypto.randomUUID(),changed:()=>{},...options});
  this.projectID=this.details()?.project?.id;this.context=null;this.native=new Map();this.files=new Map();this.staged=new Map();this.pending=null;this.working=false;this.conflict=false;this.error='';this.tail=Promise.resolve();
  if(![this.projectID,this.tenantID,this.subjectID].every(value=>uuid.test(value||'')))throw new Error('批复审计身份／项目无效');
  this.key=`oms.oa.approval-audit.pending.v1:${this.tenantID}:${this.subjectID}:${this.projectID}`;
  try{
   const pending=JSON.parse(this.storage?.getItem(this.key)||'null');if(this.validPending(pending))this.pending=pending;
   const files=JSON.parse(this.storage?.getItem(this.key+':files')||'[]');if(!Array.isArray(files)||files.length>40)throw Error();
   for(const file of files)if(uuid.test(file.file_id||'')&&revision(file.version)&&['approval','audit'].includes(file.kind)&&typeof file.id==='string')this.staged.set(file.id,file);
  }catch{this.pending=null;this.staged.clear();}
 }
 assertCurrent(){const project=this.details()?.project;if(project?.id!==this.projectID||project.tenant_id!==this.tenantID)throw new Error('项目已切换，请重新打开批复审计');}
 request(path,options={}){this.assertCurrent();return this.applicationRequest('approval-audit-results',path,{...options,approvalScope:this.projectID});}
 persist(){try{if(this.pending)this.storage?.setItem(this.key,JSON.stringify(this.pending));else this.storage?.removeItem(this.key);this.storage?.setItem(this.key+':files',JSON.stringify([...this.staged.values()]));}catch{}}
 exclusive(task){const next=this.tail.then(async()=>{this.working=true;this.changed();try{return await task();}finally{this.working=false;this.changed();}});this.tail=next.catch(()=>{});return next;}
 validate(value,id){
  if(!uuid.test(value?.id||'')||(id&&value.id!==id)||value.tenant_id!==this.tenantID||value.project?.id!==this.projectID||!revision(value.project.revision)||!['approval','audit'].includes(value.kind)||!['recorded','withdrawn'].includes(value.state)||!revision(value.revision)||!Array.isArray(value.allowed_actions)||!Array.isArray(value.evidence))throw new Error('批复审计记录身份、状态或版本未完整回读');
  if(!value.source||!uuid.test(value.source.resource_id||'')||!revision(value.source.revision)||!String(value.external_confirmed_by||'').trim()||!Number.isFinite(Date.parse(value.external_confirmed_at)))throw new Error('批复审计冻结来源或外部确认未完整回读');
  if(value.kind==='approval'&&(value.source.application!=='contract-ledger'||!['contract','settlement'].includes(value.source.resource_type)))throw new Error('批复来源不是精确合同／结算 revision');
  if(value.kind==='audit'&&(value.source.application!=='approval-audit-results'||value.source.resource_type!=='approval-result'))throw new Error('审计来源不是精确批复 revision');
  if(value.allowed_actions.includes(action('amount.read'))){
   if(value.currency!=='CNY'||!Number.isSafeInteger(value.total_amount_minor)||!Array.isArray(value.amount_components)||value.amount_components.length!==3)throw new Error('批复审计精确金额未完整回读');
   for(const component of value.amount_components)if(!components.includes(component.kind)||!Number.isSafeInteger(component.amount_minor)||component.amount_minor<0)throw new Error('批复审计金额构成无效');
  }else if(value.total_amount_minor!==undefined||value.amount_components!==undefined||value.currency!==undefined)throw new Error('批复审计金额未正确脱敏');
  const seen=new Set();for(const file of value.evidence){const key=file.file_id+':'+file.version+':'+file.slot;if(!uuid.test(file.file_id||'')||!revision(file.version)||!['approval_document','audit_document'].includes(file.slot)||seen.has(key))throw new Error('批复审计文件引用无效');seen.add(key);}
  return value;
 }
 sources(){
  const project=this.details()?.project,links=Array.isArray(this.details()?.resource_links)?this.details().resource_links:[],values=[];
  for(const link of links){if(link.unlinked_at||link.target_service!=='contract-ledger'||!['contract','settlement'].includes(link.resource_type)||!uuid.test(link.resource_id||'')||!revision(link.source_revision))continue;values.push({application:'contract-ledger',resource_type:link.resource_type,resource_id:link.resource_id,revision:link.source_revision,label:link.relation_key||`${link.resource_type} revision ${link.source_revision}`});}
  if(!project||project.id!==this.projectID)return[];return values;
 }
 current(kind){return[...this.native.values()].find(value=>value.kind===kind)||null;}
 approval(){const value=this.current('approval');return value?.state==='recorded'?value:null;}
 canCreate(kind){if(this.pending||this.working||!this.context?.allowed_actions.includes(action('create')))return false;if(kind==='approval')return this.current('approval')?.state!=='recorded'&&this.sources().length>0;return!!this.approval()&&this.current('audit')?.state!=='recorded';}
 evidenceAllowed(kind){const current=this.current(kind);return this.context?.allowed_actions.includes(action('evidence.manage'))||current?.allowed_actions.includes(action('evidence.manage'));}
 canEvidence(kind){return !this.pending&&!this.working&&!!this.transfer&&!!this.evidenceAllowed(kind)&&(kind==='approval'||!!this.approval());}
 async readFile(file){
  if(!this.evidenceReferenceRequest)throw new Error('Evidence 元数据端口未配置');
  const value=await this.evidenceReferenceRequest(file.file_id,file.version);
  if(value?.source_application!=='evidence-service'||value.tenant_id!==this.tenantID||value.file_id!==file.file_id||value.version!==file.version||value.status!=='available'||!Number.isSafeInteger(value.size_bytes)||value.size_bytes<0||!value.file_name?.trim())throw new Error('批复审计 Evidence 版本／元数据不一致');
  const id=file.id||`approval-audit-file:${this.projectID}:${file.file_id}:${file.version}:${file.slot}`;
  const view={id,file_id:file.file_id,version:file.version,slot:file.slot,original_name:value.file_name,file_name:value.file_name,file_size:value.size_bytes,mime_type:value.media_type,created_at:value.created_at};this.files.set(id,view);return view;
 }
 async load(){
  this.assertCurrent();const value=await this.request('/api/v1/approval-audit-results?project_id='+encodeURIComponent(this.projectID));
  if(value?.project_id!==this.projectID||!Array.isArray(value.items)||!Array.isArray(value.allowed_actions))throw new Error('批复审计列表或原生动作未完整读取');
  const native=new Map();for(const raw of value.items){const item=this.validate(raw);if([...native.values()].some(previous=>previous.kind===item.kind))throw new Error('同类批复审计当前投影重复');native.set(item.id,item);for(const file of item.evidence)await this.readFile(file);}
  for(const file of this.staged.values())await this.readFile(file);
  this.native=native;this.context=value;this.assertCurrent();this.changed();return this.list();
 }
 view(value){
  const amountVisible=value.allowed_actions.includes(action('amount.read')),sourceComponents=value.source?.amount_components||[];
  return {id:value.id,kind:value.kind,state:value.state,revision:value.revision,total_amount:resultAmount(value.total_amount_minor),components:(value.amount_components||[]).map(item=>({kind:item.kind,amount:resultAmount(item.amount_minor)})),source:value.source,evidence:value.evidence,report_files:value.evidence.map(file=>this.files.get(`approval-audit-file:${this.projectID}:${file.file_id}:${file.version}:${file.slot}`)).filter(Boolean),confirmed_by:value.external_confirmed_by,confirmed_at:value.external_confirmed_at,override_reason:value.override_reason||'',uses_source_amount:amountVisible&&sameComponents(value.amount_components||[],sourceComponents),can_correct:!this.pending&&!this.working&&value.state==='recorded'&&amountVisible&&value.allowed_actions.includes(action('correct')),can_withdraw:!this.pending&&!this.working&&value.state==='recorded'&&value.allowed_actions.includes(action('withdraw')),can_history:value.allowed_actions.includes(action('history.read'))};
 }
 list(){this.assertCurrent();return[...this.native.values()].sort((left,right)=>left.kind==='approval'?-1:right.kind==='approval'?1:0).map(value=>this.view(value));}
 stagedFiles(kind){return[...this.staged.values()].filter(file=>file.kind===kind).map(file=>this.files.get(file.id)).filter(Boolean);}
 descriptor(id){const file=this.files.get(id);if(!file)throw new Error('批复审计文件版本尚未读取');return {fileId:file.file_id,version:file.version,fileName:file.original_name,sizeBytes:file.file_size,mediaType:file.mime_type};}
 async upload(kind,file){return this.exclusive(async()=>{if(!['approval','audit'].includes(kind)||this.pending||!this.transfer||!this.evidenceAllowed(kind)||(kind==='audit'&&!this.approval()))throw new Error('没有批复审计文件权限，或请先继续未确认操作');const {version}=await this.transfer.uploadFile({file,clientContext:{ownerRef:this.projectID,purpose:`${kind}_document`}});if(version.status!=='available'||!uuid.test(version.fileId||'')||!revision(version.version))throw new Error('批复审计 Evidence 上传未完成');const staged={id:`approval-audit-upload:${this.projectID}:${version.fileId}:${version.version}:${kind}`,file_id:version.fileId,version:version.version,slot:`${kind}_document`,kind};this.staged.set(staged.id,staged);this.persist();return this.readFile(staged);});}
 amountInput(form){if(form.use_reference!==false)return[];const result=components.map(kind=>({kind,amount_minor:safeMinor(form[kind])}));if(!String(form.override_reason||'').trim())throw new Error('手工金额与冻结来源可能不同，必须填写金额说明');return result;}
 evidenceInput(kind,ids){const result=[];for(const id of ids||[]){const file=this.files.get(id);if(!file||file.slot!==`${kind}_document`)throw new Error('批复审计报告文件版本尚未完整读取');result.push({file_id:file.file_id,version:file.version,slot:file.slot});}return result;}
 input(kind,form,current){
  const project=this.details()?.project;if(!project||!revision(project.revision))throw new Error('项目 revision 尚未读取');let source;
  if(current)source={application:current.source.application,resource_type:current.source.resource_type,resource_id:current.source.resource_id,revision:current.source.revision};
  else if(kind==='audit'){const approval=this.approval();if(!approval)throw new Error('先登记有效批复，再登记审计');source={application:'approval-audit-results',resource_type:'approval-result',resource_id:approval.id,revision:approval.revision};}
  else{source=this.sources().find(value=>sourceKey(value)===form.source_key);if(!source)throw new Error('请选择当前项目已连接的合同／结算来源');source={application:source.application,resource_type:source.resource_type,resource_id:source.resource_id,revision:source.revision};}
  const confirmedBy=String(form.confirmed_by||'').trim(),confirmedAt=new Date(form.confirmed_at||'');if(!confirmedBy||Number.isNaN(confirmedAt.valueOf()))throw new Error('外部确认人和确认时间必填');
  const base={currency:'CNY',amount_components:this.amountInput(form),source_ref:source,evidence_refs:this.evidenceInput(kind,form.report_file_ids),override_reason:String(form.override_reason||'').trim(),external_confirmed_by:confirmedBy,external_confirmed_at:confirmedAt.toISOString()};
  if(current)return{command_id:this.newID(),expected_revision:current.revision,reason:String(form.reason||'').trim(),...base};
  const previous=this.current(kind);return{command_id:this.newID(),expected_revision:previous?.state==='withdrawn'?previous.revision:0,project_ref:{id:this.projectID,revision:project.revision},kind,...base};
 }
 validPending(p){
  if(p?.profile!=='reference-oa-approval-audit-v1'||p.tenant_id!==this.tenantID||p.subject_id!==this.subjectID||p.project_id!==this.projectID||!['create','correct','withdraw'].includes(p.kind)||!uuid.test(p.fields?.command_id||''))return false;
  const fields=p.fields;if(p.kind==='withdraw')return uuid.test(p.id||'')&&exactKeys(fields,['command_id','expected_revision','reason'])&&revision(fields.expected_revision)&&typeof fields.reason==='string'&&!!fields.reason.trim();
  const createKeys=['command_id','expected_revision','project_ref','kind','currency','amount_components','source_ref','evidence_refs','override_reason','external_confirmed_by','external_confirmed_at'];
  const correctKeys=['command_id','expected_revision','reason','currency','amount_components','source_ref','evidence_refs','override_reason','external_confirmed_by','external_confirmed_at'];
  if(!exactKeys(fields,p.kind==='create'?createKeys:correctKeys)||!['approval','audit'].includes(fields.kind||p.result_kind)||!Number.isSafeInteger(fields.expected_revision)||fields.expected_revision<0||fields.currency!=='CNY'||!Array.isArray(fields.amount_components)||!Array.isArray(fields.evidence_refs)||!fields.external_confirmed_by?.trim()||Number.isNaN(Date.parse(fields.external_confirmed_at)))return false;
  if(p.kind==='create'&&(!exactKeys(fields.project_ref,['id','revision'])||fields.project_ref.id!==this.projectID||!revision(fields.project_ref.revision)))return false;
  if(p.kind==='correct'&&(!uuid.test(p.id||'')||!revision(fields.expected_revision)||!fields.reason?.trim()))return false;
  if(fields.amount_components.length!==0&&fields.amount_components.length!==3)return false;for(const item of fields.amount_components)if(!components.includes(item.kind)||!Number.isSafeInteger(item.amount_minor)||item.amount_minor<0)return false;
  const resultKind=fields.kind||p.result_kind;if(!uuid.test(fields.source_ref?.resource_id||'')||!revision(fields.source_ref?.revision))return false;if(resultKind==='approval'&&(fields.source_ref.application!=='contract-ledger'||!['contract','settlement'].includes(fields.source_ref.resource_type)))return false;if(resultKind==='audit'&&(fields.source_ref.application!=='approval-audit-results'||fields.source_ref.resource_type!=='approval-result'))return false;for(const file of fields.evidence_refs)if(!uuid.test(file.file_id||'')||!revision(file.version)||file.slot!==`${resultKind}_document`)return false;return true;
 }
 start(kind,id,resultKind,fields){this.pending={profile:'reference-oa-approval-audit-v1',tenant_id:this.tenantID,subject_id:this.subjectID,project_id:this.projectID,kind,id,result_kind:resultKind,fields};this.persist();return this.continue();}
 async continue(){
  const pending=this.pending;if(!this.validPending(pending))throw new Error('待续批复审计命令无效');
  try{
   const path='/api/v1/approval-audit-results'+(pending.kind==='create'?'':`/${pending.id}/${pending.kind==='correct'?'corrections':'withdrawal'}`);
   const value=this.validate(await this.request(path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(pending.fields)}),pending.id);
   if(value.revision!==pending.fields.expected_revision+1)throw new Error('批复审计版本回执未确认');
   if(pending.kind==='withdraw'){if(value.state!=='withdrawn')throw new Error('批复审计撤回回执未确认');}
   else if(value.kind!==(pending.fields.kind||pending.result_kind)||value.state!=='recorded'||!sameSource(value.source,pending.fields.source_ref)||!sameEvidence(value.evidence,pending.fields.evidence_refs)||value.external_confirmed_by!==pending.fields.external_confirmed_by||!sameInstant(value.external_confirmed_at,pending.fields.external_confirmed_at)||(pending.fields.amount_components.length&&!sameComponents(value.amount_components||[],pending.fields.amount_components)))throw new Error('批复审计回执与原意图不一致');
   await this.load();if(!this.native.has(value.id))throw new Error('批复审计最终列表未确认');
   if(pending.kind!=='withdraw')for(const [id,file]of this.staged)if(value.evidence.some(item=>item.file_id===file.file_id&&item.version===file.version&&item.slot===file.slot))this.staged.delete(id);
   this.pending=null;this.conflict=false;this.error='';this.persist();this.changed();return this.view(this.native.get(value.id));
  }catch(error){this.error=error instanceof Error?error.message:'批复审计操作未确认';this.conflict=error?.status===409;if(pending.kind==='create'&&[400,401,403,404,415,422,429].includes(error?.status))this.pending=null;this.persist();this.changed();throw error;}
 }
 create(kind,form){return this.exclusive(async()=>{if(this.pending||!this.context?.allowed_actions.includes(action('create'))||(kind==='approval'&&(this.current('approval')?.state==='recorded'||!this.sources().length))||(kind==='audit'&&(!this.approval()||this.current('audit')?.state==='recorded')))throw new Error('当前缺少登记权限或业务前置记录');return this.start('create',undefined,kind,this.input(kind,form));});}
 correct(id,form){return this.exclusive(async()=>{const current=this.native.get(id);if(this.pending||!current||!current.allowed_actions.includes(action('correct'))||!current.allowed_actions.includes(action('amount.read'))||!String(form.reason||'').trim())throw new Error('没有纠正权限，或纠正原因未填写');return this.start('correct',id,current.kind,this.input(current.kind,form,current));});}
 withdraw(id,reason){return this.exclusive(async()=>{const current=this.native.get(id);if(this.pending||!current||!current.allowed_actions.includes(action('withdraw'))||!String(reason||'').trim())throw new Error('没有撤回权限，或撤回原因未填写');return this.start('withdraw',id,current.kind,{command_id:this.newID(),expected_revision:current.revision,reason:String(reason).trim()});});}
 retry(){return this.exclusive(()=>{if(this.conflict)throw new Error('版本冲突须重新读取核对，不能自动覆盖');return this.continue();});}
 async history(id){const current=this.native.get(id);if(!current?.allowed_actions.includes(action('history.read')))throw new Error('没有批复审计历史权限');const value=await this.request(`/api/v1/approval-audit-results/${id}/history`);if(!Array.isArray(value?.items))throw new Error('批复审计历史未完整读取');return value.items.map(item=>({...item,snapshot:this.view(this.validate(item.snapshot,id))}));}
}
