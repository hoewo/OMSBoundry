// Production owns reference facts; Evidence API + transfer ports own bytes.
// Durable recovery stores only bounded identities/receipts, never credentials or file bytes.
export const stageProfile='reference-oa-stage-materials-v1';
export const stageTypeKey='reference-oa-stage-set-v1';
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const rev=v=>Number.isSafeInteger(v)&&v>0;
const action=n=>'production-work-item.stage-material.'+n;
const copy=v=>JSON.parse(JSON.stringify(v));
export function stageScore(value){
 if(value==null||value==='')return undefined;
 if(typeof value!=='string'||value.length>128||!/^\d+(?:\.\d+)?$/.test(value))throw Error('评分必须是精确十进制字符串');
 const [i,f='']=value.split('.'),whole=BigInt(i);if(whole>100n||(whole===100n&&/[1-9]/.test(f)))throw Error('评分必须在 0–100 之间');
 const fraction=f.replace(/0+$/,'');return whole.toString()+(fraction?'.'+fraction:'');
}
export function stagePendingStore(storage,key){
 if(!storage||typeof key!=='string'||!key)throw Error('阶段续办存储必须显式配置');
 return {load(){const text=storage.getItem(key);if(!text)return null;try{return JSON.parse(text)}catch{throw Error('阶段续办意图损坏，未启用写入')}},save(value){storage.setItem(key,JSON.stringify(value))},clear(){storage.removeItem(key)}};
}
async function fingerprint(file){
 if(!(file instanceof Blob)||!file.name?.trim()||file.size<=0||file.size>200*1024*1024)throw Error('请选择非空且不超过 200 MB 的阶段文件');
 const digest=await crypto.subtle.digest('SHA-256',await file.arrayBuffer());
 return {name:file.name,size:file.size,media_type:file.type||'application/octet-stream',sha256:[...new Uint8Array(digest)].map(v=>v.toString(16).padStart(2,'0')).join('')};
}
export function createNativeStagePort(options){
 const {projectID,tenantID,subjectID,request,pendingStore,readFile,upload,transfer}=options;
 const projectTypeKey='shared',typeKey=stageTypeKey,stage=options.stage||'scheme',definition={
 scheme:{key:'scheme',min:2},preliminary:{key:'preliminary-design',min:3},construction:{key:'construction-drawing',min:4},
  change:{key:'change',min:5,mainOnly:true,replace:true,extensions:['pdf','doc','docx','xls','xlsx','png','jpg','jpeg','dwg','dxf']},
  completion:{key:'completion',min:5,mainOnly:true,replace:true,extensions:['pdf','doc','docx','xls','xlsx','zip','rar','png','jpg','jpeg','dwg','dxf']},
  test:{key:'test',min:6,mainOnly:true,extensions:['pdf','doc','docx','xls','xlsx','png','jpg','jpeg','dwg','dxf']},
  consultation:{key:'report',min:6,mainOnly:true,extensions:['pdf','doc','docx','xls','xlsx','png','jpg','jpeg','dwg','dxf']},
 }[stage],stageKey=definition?.key,newID=options.newID||(()=>crypto.randomUUID());
 if(!definition)throw Error('阶段原生端口尚未登记');
 if(![projectID,tenantID,subjectID].every(v=>uuid.test(v||''))||![request,readFile,upload].every(v=>typeof v==='function')||!pendingStore)throw Error('阶段原生 Project／Tenant／Subject／端口未配置');
 let value=null,context=null,workItem=null,working=false,tail=Promise.resolve();const files=new Map(),selected=new Map();
 const assertCurrent=()=>{options.assertCurrent?.();};
 const path=id=>'/api/v1/production-work-items/'+id+'/stages/'+stageKey+'/material-collection';
 const validateFileName=name=>{if(!definition.mainOnly)return;const extension=String(name||'').toLowerCase().split('.').pop();if(!definition.extensions.includes(extension))throw Error('当前阶段文件类型不受支持')};
 const validateItem=item=>{if(!uuid.test(item?.id||'')||item.tenant_id!==tenantID||item.project_id!==projectID||item.type_key!==typeKey||!rev(item.revision)||![1,2,3,4,5,6].includes(item.type_definition_version)||!['draft','in-progress','completed','cancelled'].includes(item.status))throw Error('阶段任务真实身份／版本未确认');return item};
 const validate=result=>{
  if(result?.schema_version!=='1.0'||result.profile!==stageProfile||result.stage?.stage_key!==stageKey||!Array.isArray(result.main_files)||!Array.isArray(result.review_sheets)||!Array.isArray(result.allowed_actions)||result.main_files.length>100||result.review_sheets.length>100)throw Error('阶段集合响应不完整，不代表空资料');
  validateItem(result.work_item);if(![2,3,4,5,6].includes(result.work_item.type_definition_version)||result.work_item.type_definition_version<definition.min||result.stage.tenant_id!==tenantID||result.stage.work_item_id!==result.work_item.id)throw Error('阶段集合版本／所属任务不匹配');
  if(definition.mainOnly&&(result.review_sheets.length!==0||result.score!==null))throw Error('当前阶段响应包含不允许的校审或评分');
  if(result.score!==null&&stageScore(result.score)!==result.score)throw Error('阶段评分响应不是精确值');
  const ids=new Set();for(const [kind,rows]of [['main-file',result.main_files],['review-sheet',result.review_sheets]]){
   let previous=null;for(const row of rows){if(!uuid.test(row.id||'')||ids.has(row.id)||row.tenant_id!==tenantID||row.work_item_id!==result.work_item.id||row.stage_key!==stageKey||row.kind!==kind||!uuid.test(row.file_id||'')||!rev(row.version)||row.source_revision!==row.version||!rev(row.created_revision)||row.created_revision>result.work_item.revision||!row.display_name?.trim())throw Error('阶段资料身份／版本回读无效');
    if(!Number.isInteger(row.batch_position)||row.batch_position<1||row.batch_position>20||previous&&(row.created_revision<previous.created_revision||row.created_revision===previous.created_revision&&row.batch_position<=previous.batch_position))throw Error('阶段资料批内顺序未确认');
    ids.add(row.id);previous=row;
   }
  }
  return result;
 };
 const pending=()=>{
  const p=pendingStore.load();if(!p)return null;
  if(p.schema!=='stage-intent-v1'||p.profile!==stageProfile||p.tenant_id!==tenantID||p.subject_id!==subjectID||p.project_id!==projectID||p.type_key!==typeKey||p.stage!==stage||!['save','withdraw','replace'].includes(p.kind)||!uuid.test(p.command_id||'')||!Array.isArray(p.files)||p.files.length>40)throw Error('阶段续办意图不属于当前会话／项目／类型／阶段');
  if(p.work_item_id&&!uuid.test(p.work_item_id))throw Error('续办任务身份无效');
  if(p.expected_revision!==undefined&&!rev(p.expected_revision))throw Error('续办 revision 无效');
  if(p.score!==undefined&&stageScore(p.score)!==p.score)throw Error('续办评分无效');
  if(p.kind==='withdraw'&&(!uuid.test(p.reference_id||'')||!p.work_item_id||!rev(p.expected_revision)||p.reason!=='原 OA 阶段资料移出'))throw Error('移出续办意图无效');
  if(p.kind==='replace'&&(!definition.replace||!uuid.test(p.reference_id||'')||!uuid.test(p.withdraw_command_id||'')||!p.work_item_id||!rev(p.expected_revision)||p.files.length!==1||p.files[0].kind!=='main-file'||p.score!==undefined||p.registered!==undefined&&p.registered!==true||p.registered===true&&!uuid.test(p.new_reference_id||'')))throw Error('替换续办意图无效');
  if(p.create&&(!uuid.test(p.create.command_id||'')||p.create.project_id!==projectID||!rev(p.create.project_source_revision)||p.create.type_key!==typeKey||![2,3,4,5,6].includes(p.create.type_definition_version)))throw Error('任务创建续办意图无效');
  const upgrades=p.upgrades||(p.upgrade?[p.upgrade]:[]);if(!Array.isArray(upgrades)||upgrades.length>4)throw Error('阶段定义升级续办意图无效');
  for(let i=0;i<upgrades.length;i++){const u=upgrades[i];if(!uuid.test(u.command_id||'')||![3,4,5,6].includes(u.target_version)||!rev(u.expected_revision)||u.completed!==undefined&&u.completed!==true||i&&u.target_version!==upgrades[i-1].target_version+1||i&&u.expected_revision!==upgrades[i-1].expected_revision+1)throw Error('阶段定义升级续办意图无效')}
  for(const f of p.files){if(!['main-file','review-sheet'].includes(f.kind)||!f.name?.trim()||!rev(f.size)||f.size>200*1024*1024||!/^[0-9a-f]{64}$/.test(f.sha256)||!uuid.test(f.create_key||'')||!uuid.test(f.complete_key||''))throw Error('文件续办身份无效');validateFileName(f.name);if(definition.mainOnly&&f.kind!=='main-file')throw Error('当前阶段只接受主文件');if(f.reference&&(!uuid.test(f.reference.file_id||'')||!rev(f.reference.version)||f.reference.source_revision!==f.reference.version))throw Error('已上传 File 身份无效');}
  return p;
 };
 const persist=p=>pendingStore.save(copy(p));
 const exclusive=fn=>{const p=tail.then(async()=>{assertCurrent();working=true;options.changed?.();try{return await fn()}finally{working=false;options.changed?.()}});tail=p.catch(()=>{});return p};
 const rowView=async row=>{const meta=await readFile(row.file_id,row.version);assertCurrent();if(meta.resource_type!=="evidence-file"||meta.tenant_id!==tenantID||meta.file_id!==row.file_id||meta.version!==row.version||meta.revision!==row.source_revision||meta.status!=='available'||!Number.isSafeInteger(meta.size_bytes)||meta.size_bytes<0||!meta.display_name?.trim())throw Error('阶段 Evidence 版本尚未确认');const id=`stage-file:${projectID}:${typeKey}:${stage}:${row.file_id}:${row.version}`,file={id,file_id:row.file_id,version:row.version,original_name:meta.display_name,file_name:meta.display_name,file_size:meta.size_bytes,mime_type:meta.media_type,created_at:meta.created_at};files.set(id,file);return {id:row.id,project_id:projectID,file_id:row.file_id,file,stage,created_at:row.created_at}};
 const toView=async result=>{const main_files=[],review_sheets=[];for(const row of result.main_files)main_files.push(await rowView(row));for(const row of result.review_sheets)review_sheets.push(await rowView(row));return {stage,main_files,review_sheets,review_sheet:review_sheets.at(-1)||null,score:result.score}};
 const read=async()=>{
  assertCurrent();const query='/api/v1/production-stage-work-items?project_id='+projectID+'&type_key='+typeKey,list=await request(query);
  if(list?.schema_version!=='1.0'||list.profile!==stageProfile||list.tenant_id!==tenantID||list.project_id!==projectID||list.type_key!==typeKey||!Array.isArray(list.items)||!Array.isArray(list.allowed_actions)||list.items.length>1)throw Error('阶段任务列表未按精确 Project／类型完整读取');
  if(list.items.some(item=>![2,3,4,5,6].includes(item.type_definition_version)||item.type_key!==typeKey))throw Error('此项目已有旧版单成果任务；需制定保留历史的转换，不能并行建立阶段真相');
  context=list;workItem=list.items.length?validateItem(list.items[0]):null;
  if(!workItem){value=null;return {stage,main_files:[],review_sheets:[],review_sheet:null,score:null}}
  if(workItem.type_definition_version<definition.min){value=null;return {stage,main_files:[],review_sheets:[],review_sheet:null,score:null}}
  const result=validate(await request(path(workItem.id)));const view=await toView(result);assertCurrent();value=result;workItem=result.work_item;return view;
 };
 const can=name=>!!(value?.allowed_actions||context?.allowed_actions||[]).includes(action(name));
 const verifySave=(result,p)=>{
  validate(result);if(result.work_item.id!==p.work_item_id||result.work_item.revision!==p.expected_revision+1)throw Error('阶段保存 revision 回复未确认，请继续原命令');
  for(const f of p.files){const rows=f.kind==='main-file'?result.main_files:result.review_sheets;if(!rows.some(row=>row.file_id===f.reference.file_id&&row.version===f.reference.version&&row.created_revision===result.work_item.revision))throw Error('阶段保存文件集合未全部确认')}
  if(p.score!==undefined&&result.score!==p.score)throw Error('阶段评分保存未确认');return result;
 };
 const continuePending=async p=>{
  assertCurrent();
  if(p.kind==='save'||p.kind==='replace'){
   if(!p.work_item_id){const created=await request('/api/v1/production-work-items','POST',p.create);const item=validateItem(created.work_item);if(item.type_definition_version!==p.create.type_definition_version||item.revision!==1)throw Error('阶段任务创建回复未确认');p.work_item_id=item.id;p.expected_revision=1;persist(p)}
   const upgrades=p.upgrades||(p.upgrade?[p.upgrade]:[]);for(const upgrade of upgrades)if(!upgrade.completed){const upgraded=await request('/api/v1/production-work-items/'+p.work_item_id+'/stage-material-definition-upgrade','POST',{command_id:upgrade.command_id,expected_revision:upgrade.expected_revision,target_version:upgrade.target_version}),item=validateItem(upgraded?.work_item);if(item.id!==p.work_item_id||item.type_definition_version!==upgrade.target_version||item.revision!==upgrade.expected_revision+1)throw Error('阶段定义升级回复未确认，请继续原命令');upgrade.completed=true;p.expected_revision=item.revision;workItem=item;persist(p)}
   for(const f of p.files){if(f.reference)continue;const key=JSON.stringify([f.kind,f.name,f.size,f.sha256]),file=selected.get(key);const reference=await upload(file,f,()=>{assertCurrent();persist(p)});assertCurrent();if(!uuid.test(reference?.file_id||'')||!rev(reference.version)||reference.source_revision!==reference.version)throw Error('Evidence 上传身份未确认');f.reference=reference;persist(p)}
   if(p.kind==='save'){
    const body={command_id:p.command_id,expected_revision:p.expected_revision,main_files:p.files.filter(f=>f.kind==='main-file').map(f=>f.reference),review_sheets:p.files.filter(f=>f.kind==='review-sheet').map(f=>f.reference),...(p.score!==undefined?{score:p.score}:{})};
    const result=verifySave(await request(path(p.work_item_id),'POST',body),p);
    await read();if(!value||value.work_item.id!==result.work_item.id||value.work_item.revision<result.work_item.revision||p.files.some(f=>![...value.main_files,...value.review_sheets].some(row=>row.file_id===f.reference.file_id&&row.version===f.reference.version)))throw Error('阶段最终回读尚未确认，请继续原命令');
   }else{
    if(!p.registered){
     const body={command_id:p.command_id,expected_revision:p.expected_revision,main_files:[p.files[0].reference],review_sheets:[]};
     const result=verifySave(await request(path(p.work_item_id),'POST',body),p),row=result.main_files.find(row=>row.file_id===p.files[0].reference.file_id&&row.version===p.files[0].reference.version&&row.created_revision===result.work_item.revision);
     if(!row||!uuid.test(row.id||''))throw Error('替换的新资料引用未确认，请继续原命令');
     p.registered=true;p.new_reference_id=row.id;p.expected_revision=result.work_item.revision;workItem=result.work_item;persist(p);
    }
    const result=validate(await request(path(p.work_item_id)+'/'+p.reference_id+'/withdraw','POST',{command_id:p.withdraw_command_id,expected_revision:p.expected_revision,reason:'原 OA 辅助资料替换'}));
    if(result.work_item.id!==p.work_item_id||result.work_item.revision!==p.expected_revision+1||result.main_files.some(f=>f.id===p.reference_id)||!result.main_files.some(f=>f.id===p.new_reference_id))throw Error('替换旧引用结束未确认，请继续原命令');
    await read();if(value.main_files.some(f=>f.id===p.reference_id)||!value.main_files.some(f=>f.id===p.new_reference_id))throw Error('替换最终回读尚未确认，请继续原命令');
   }
  }else{
   const result=validate(await request(path(p.work_item_id)+'/'+p.reference_id+'/withdraw','POST',{command_id:p.command_id,expected_revision:p.expected_revision,reason:p.reason}));
   if(result.work_item.id!==p.work_item_id||result.work_item.revision!==p.expected_revision+1||[...result.main_files,...result.review_sheets].some(f=>f.id===p.reference_id))throw Error('资料移出回复未确认');await read();if([...value.main_files,...value.review_sheets].some(f=>f.id===p.reference_id))throw Error('资料移出最终回读未确认');
  }
  assertCurrent();pendingStore.clear();selected.clear();options.changed?.();
 };
 return {
  projectID,projectTypeKey,stage,files,transfer,maxUploadBytes:200*1024*1024,
  readStage:read,
  async permissions(){if(!context)await read();pending();return {read:can('read'),upload:can('register')&&(!workItem?context.allowed_actions.includes('production-work-item.create'):workItem.status!=='cancelled'),download:can('read'),withdraw:!!workItem&&workItem.status!=='cancelled'&&can('withdraw')}},
  pendingOperation(){return pending()},isWorking(){return working},
  retryPending(){return exclusive(async()=>{const p=pending();if(!p)throw Error('没有待续阶段命令');await continuePending(p)})},
  saveStage(input){return exclusive(async()=>{
   if(definition.mainOnly&&((input.reviewSheets||[]).length||input.score!==undefined&&input.score!==null&&input.score!==''))throw Error('当前阶段只允许上传主文件，不含校审或评分');
   const old=pending();if(old){
    if(old.kind!=='save'||stageScore(input.score)!==old.score)throw Error('有尚未确认的阶段操作，请先继续原命令；不能替换原意图');
    const rows=[];for(const [kind,blobs]of [['main-file',input.mainFiles||[]],['review-sheet',input.reviewSheets||[]]])for(const blob of blobs){const info=await fingerprint(blob);rows.push({kind,...info});selected.set(JSON.stringify([kind,info.name,info.size,info.sha256]),blob)}
    if(rows.length!==old.files.length||rows.some((f,i)=>f.kind!==old.files[i].kind||f.name!==old.files[i].name||f.size!==old.files[i].size||f.sha256!==old.files[i].sha256))throw Error('不能替换原意图；请继续原阶段命令');
    await continuePending(old);return;
   }
   if(!can('register')||workItem?.status==='cancelled'||(!workItem&&!context?.allowed_actions.includes('production-work-item.create')))throw Error('当前不能登记阶段资料');
   const score=stageScore(input.score);const rows=[];
   for(const [kind,blobs]of [['main-file',input.mainFiles||[]],['review-sheet',input.reviewSheets||[]]]){if(blobs.length>20)throw Error('每批每类最多 20 份资料');for(const file of blobs){const info=await fingerprint(file);validateFileName(info.name);const key=JSON.stringify([kind,info.name,info.size,info.sha256]);if(rows.some(f=>JSON.stringify([f.kind,f.name,f.size,f.sha256])===key))throw Error('本次选择存在重复文件');selected.set(key,file);rows.push({kind,...info,create_key:newID(),complete_key:newID()})}}
   if(!rows.length&&(score===undefined||score===value?.score))return;
   if(!value?.main_files.length&&!rows.some(f=>f.kind==='main-file'))throw Error('请先上传主文件');
   const upgrades=[];if(workItem)for(let target=workItem.type_definition_version+1,expected=workItem.revision;target<=definition.min;target++,expected++)upgrades.push({command_id:newID(),expected_revision:expected,target_version:target});
   const p={schema:'stage-intent-v1',profile:stageProfile,tenant_id:tenantID,subject_id:subjectID,project_id:projectID,type_key:typeKey,stage,kind:'save',command_id:newID(),files:rows,...(score!==undefined?{score}:{}),...(workItem?{work_item_id:workItem.id,expected_revision:workItem.revision,...(upgrades.length?{upgrades}:{})}:{create:{command_id:newID(),project_id:projectID,project_source_revision:options.projectRevision(),type_key:typeKey,type_definition_version:6}})};
   persist(p);await continuePending(p);
  })},
  replaceReference(id,file){return exclusive(async()=>{if(!definition.replace)throw Error('当前阶段没有原 OA 单文件替换');if(pending())throw Error('请先继续上一笔未确认操作');if(!value||!can('register')||!can('withdraw')||workItem.status==='cancelled'||!value.main_files.some(row=>row.id===id))throw Error('当前不能替换该资料');const info=await fingerprint(file);validateFileName(info.name);const key=JSON.stringify(['main-file',info.name,info.size,info.sha256]);selected.set(key,file);const p={schema:'stage-intent-v1',profile:stageProfile,tenant_id:tenantID,subject_id:subjectID,project_id:projectID,type_key:typeKey,stage,kind:'replace',command_id:newID(),withdraw_command_id:newID(),work_item_id:workItem.id,expected_revision:workItem.revision,reference_id:id,files:[{kind:'main-file',...info,create_key:newID(),complete_key:newID()}]};persist(p);await continuePending(p)})},
  withdrawReference(id){return exclusive(async()=>{if(pending())throw Error('请先继续上一笔未确认操作');if(!value||!can('withdraw')||workItem.status==='cancelled'||![...value.main_files,...value.review_sheets].some(f=>f.id===id))throw Error('当前不能移出该阶段资料');const p={schema:'stage-intent-v1',profile:stageProfile,tenant_id:tenantID,subject_id:subjectID,project_id:projectID,type_key:typeKey,stage,kind:'withdraw',command_id:newID(),work_item_id:workItem.id,expected_revision:workItem.revision,reference_id:id,reason:'原 OA 阶段资料移出',files:[]};persist(p);await continuePending(p)})},
  async reselectPending(files){const p=pending();if(!p)throw Error('没有待续阶段文件');for(const file of files){const info=await fingerprint(file);for(const f of p.files)if(f.name===info.name&&f.size===info.size&&f.sha256===info.sha256)selected.set(JSON.stringify([f.kind,f.name,f.size,f.sha256]),file)}},
  descriptor(id){const f=files.get(id);if(!f)throw Error('阶段文件版本尚未读取');return {fileId:f.file_id,version:f.version,fileName:f.file_name,sizeBytes:f.file_size,mediaType:f.mime_type}},
 };
}

// Uses generic Evidence/transfer ports, not an OSS SDK or physical object path.
export function durableStageUpload({api,uploader}){
 return async(file,intent,persist)=>{
  if(intent.reference)return intent.reference;
  let session;
  if(!intent.receipt){
   if(!file)throw Error('请重新选择原文件继续；已完成上传的文件不会重复上传');
   session=await api.createUploadSession({fileName:intent.name,sizeBytes:intent.size,mediaType:intent.media_type,sha256:intent.sha256,idempotencyKey:intent.create_key,clientContext:{purpose:'production_stage_material'}});
   if(!uuid.test(session.sessionId||'')||!uuid.test(session.fileId||'')||!rev(session.version)||session.status!=='pending'||Date.parse(session.expiresAt)<=Date.now()||!session.authorization?.objectKey||session.authorization.objectKey!==session.authorization.scope?.objectKey)throw Error('原 Evidence 上传会话未确认或已过期，不自动创建替代文件');
   if(intent.session&&(intent.session.id!==session.sessionId||intent.session.file_id!==session.fileId||intent.session.version!==session.version))throw Error('Evidence 原会话身份变化');
   intent.session={id:session.sessionId,file_id:session.fileId,version:session.version};persist();
   const receipt=await uploader.upload({file,session});intent.receipt={...(receipt.etag?{etag:receipt.etag}:{}),...(receipt.parts?{parts:receipt.parts.map(p=>({partNumber:p.partNumber,etag:p.etag,...(p.sizeBytes!==undefined?{sizeBytes:p.sizeBytes}:{})}))}:{})};persist();
  }
  if(!intent.session||!uuid.test(intent.session.id||'')||!uuid.test(intent.session.file_id||'')||!rev(intent.session.version))throw Error('完成上传的原会话身份无效');
  const v=await api.completeUpload(intent.session.id,{idempotencyKey:intent.complete_key,observedSizeBytes:intent.size,observedSha256:intent.sha256,...intent.receipt});
  if(v.status!=='available'||v.fileId!==intent.session.file_id||v.version!==intent.session.version||v.sizeBytes!==intent.size||v.sha256!==intent.sha256)throw Error('Evidence 完成回执与原文件不一致');
  return {file_id:v.fileId,version:v.version,source_revision:v.version};
 };
}
