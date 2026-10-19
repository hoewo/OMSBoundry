// Original OA view ports -> native owners. This module has no React dependency.
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
export const kinds = Object.freeze({tender:'tender_document', bid:'bid_document', award:'award_document', award_notice:'award_document'});
const categoryFor = key => ({tender_document:'tender',bid_document:'bid',award_document:'award_notice'})[key];
const current = state => (state?.source?.materials || []).filter(row => !row.superseded_at);
export class BiddingAdapter {
  constructor({flow, details, evidenceReferenceRequest, transfer, tenantID, subjectID, storage, changed=()=>{}, record=()=>{}}) {
    Object.assign(this,{flow,details,evidenceReferenceRequest,transfer,tenantID,subjectID,storage,changed,record});
    this.projectID = flow.validateProject(details()); this.files = new Map(); this.pending = new Map(); this.tail = Promise.resolve(); this.state = null;
    this.storageKey = `oms.oa.b03.pending.v1:${tenantID}:${subjectID}:${this.projectID}`;
    this.restorePending();
  }
  assertProject(id) { if(id !== this.projectID) throw new Error('组件项目与原生项目不一致'); }
  exclusive(action) { const result = this.tail.then(action); this.tail = result.catch(()=>{}); return result; }
  requireState() { if(!this.state) throw new Error('请先读取当前项目资料'); return this.state; }
  reference(value, id, version) {
    if(value?.schema_version!=='1'||value.source_application!=='evidence-service'||value.resource_type!=='evidence-file'||value.id!==id||value.file_id!==id||value.tenant_id!==this.tenantID||value.version!==version||value.revision!==version||value.status!=='available'||!Number.isSafeInteger(value.size_bytes)||value.size_bytes<0) throw new Error('Evidence 文件版本未确认或不属于当前租户');
    return value;
  }
  async fileView(id, version, viewID, createdAt, materialID) {
    const ref = this.reference(await this.evidenceReferenceRequest(id,version), id, version);
    return {id:viewID,file_id:id,version,material_reference_id:materialID,original_name:ref.file_name,file_name:ref.file_name,file_size:ref.size_bytes,mime_type:ref.media_type,created_at:createdAt};
  }
  async hydrate() {
    this.dataReady=false;
    const next = new Map();
    // Bounded sequential metadata reads; a denied read is not an empty list.
    for(const row of current(this.state)) next.set(row.id,await this.fileView(row.evidence_file_id,row.evidence_version,row.id,row.created_at,row.id));
    for(const [id,item] of this.pending) next.set(id,await this.fileView(item.file_id,item.version,id,item.created_at));
    this.files=next;this.dataReady=true;
  }
  async load(explicit=false) {
    return this.exclusive(async()=>{
      this.state = await this.flow.load(this.details());
      if(explicit) {
        this.state.commands.clear();this.state.intent=null;this.state.conflict=false;this.restoredCommand=null;this.restoredIntent=null;this.error='';
        // A lost reply can already be committed. Reread reconciles staged IDs,
        // without issuing another append or discarding unregistered files.
        for(const [id,file] of this.pending)if(current(this.state).some(row=>row.evidence_file_id===file.file_id&&row.evidence_version===file.version&&row.requirement_key===kinds[file.category]))this.pending.delete(id);
      }
      else if(this.restoredCommand) {
        this.state.commands.set('material',this.restoredCommand);this.state.intent=this.restoredIntent;
        this.restoredCommand=null;this.restoredIntent=null;
      }
      await this.hydrate();this.persist();this.changed();return this.info();
    });
  }
  info() {
    if(!this.dataReady)throw new Error('文件元数据回读尚未完成，请刷新当前资料；不把读取失败显示为空清单');
    const state=this.requireState(), req=state.source?.requirements||[], item=state.source?.engagement;
    const satisfied=key=>req.some(row=>row.key===key&&row.satisfied);
    const rows=key=>current(state).filter(row=>row.requirement_key===key).map(row=>this.files.get(row.id));
    return {not_applicable:item?.applicability==='not_applicable',engagement_method:item?.method||null,
      tender_file_required:req.some(row=>row.key==='tender_document'&&row.required),tender_file_complete:satisfied('tender_document'),bid_file_complete:satisfied('bid_document'),award_file_complete:satisfied('award_document'),
      tender_files:rows('tender_document'),bid_files:rows('bid_document'),award_notice_files:rows('award_document')};
  }
  permissions() {
    const state=this.state;
    return {canUploadBusinessFile:!!state&&this.dataReady&&!state.intent&&!this.working&&Object.values(kinds).some(key=>this.flow.canMaterial(state,key)),canDeleteBusinessFile:!!state&&this.dataReady&&!state.intent&&!this.working&&Object.values(kinds).some(key=>this.flow.canMaterial(state,key))};
  }
  async action(intent) {
    return this.exclusive(async()=>{
      this.working=true;this.changed();
      this.record('flow','保存当前项目的招投标操作','原 OA 组件只发出明确意图；BE 确认自有承接／材料引用，Evidence 保有文件版本，Project 保有项目引用。','waiting');
      try {
        this.state=await (intent.kind==='material'?this.flow.material(this.details(),intent):this.flow.apply(this.details(),intent));
        if(intent.kind==='material'&&intent.operation==='append') for(const [id,file] of this.pending) if(file.file_id===intent.fileID&&file.version===intent.version) this.pending.delete(id);
        this.persist();await this.hydrate();this.error='';
        this.record('bidding-command','原生保存与复读已确认','材料齐备不是中标或签约；移除材料只结束当前业务引用，不删除 Evidence 文件。','complete');
        return this.info();
      } catch(error) {
        this.state=this.flow.states.get(this.projectID)||this.state;this.error=error.message;
        this.record('bidding-command','操作尚未全部确认','已上传文件和已确认来源保留；继续原操作复用原命令，不自动重新上传或覆盖历史。','error');throw error;
      } finally {this.working=false;this.persist();this.changed();}
    });
  }
  updateMethod(id, method) {this.assertProject(id);return this.action({kind:'method',method});}
  updateApplicability(applicable) {return this.action({kind:'applicability',applicable});}
  async upload(id,file,category) {
    this.assertProject(id);
    return this.exclusive(async()=>{
      if(!kinds[category]||!this.flow.canMaterial(this.requireState(),kinds[category])||this.state.intent)throw new Error('当前权限／状态不能上传，请先处理未确认操作');
      this.working=true;this.changed();
      try {
        this.record('evidence-service','文件 → Evidence','使用既有 File Transfer SDK，申请有界上传、直传并完成校验；此时尚未登记 BE 材料。','waiting');
        const {version}=await this.transfer.uploadFile({file,clientContext:{ownerRef:this.projectID,purpose:kinds[category]}});
        if(version.status!=='available'||!uuid.test(version.fileId)||!Number.isSafeInteger(version.version)||version.version<1)throw new Error('上传版本尚未确认，未登记材料');
        const viewID=`upload:${version.fileId}:${version.version}`;
        const item={file_id:version.fileId,version:version.version,category,created_at:version.createdAt};
        // Save identity before metadata reread, so a failed reread can resume.
        this.pending.set(viewID,item);this.persist();
        const view=await this.fileView(item.file_id,item.version,viewID,item.created_at);
        this.files.set(viewID,view);return view;
      } finally {this.working=false;this.changed();}
    });
  }
  append(id,type,viewID) {
    this.assertProject(id);const file=this.pending.get(viewID);
    if(!file||kinds[file.category]!==kinds[type])return Promise.reject(new Error('只能登记当前项目已上传的这一文件版本'));
    return this.action({kind:'material',operation:'append',requirementKey:kinds[type],fileID:file.file_id,version:file.version});
  }
  remove(id,type,referenceID) {
    this.assertProject(id);
    if(!current(this.requireState()).some(row=>row.id===referenceID&&row.requirement_key===kinds[type]))return Promise.reject(new Error('只能移除当前项目的这一有效引用'));
    return this.action({kind:'material',operation:'remove_one',requirementKey:kinds[type],materialID:referenceID});
  }
  retry() {const intent=this.requireState().intent;if(!intent)throw new Error('没有需要继续的原命令');return this.action({...intent});}
  descriptor(viewID) {
    const file=this.files.get(viewID);if(!file)throw new Error('文件引用尚未读取');
    return {fileId:file.file_id,version:file.version,fileName:file.original_name,sizeBytes:file.file_size,mediaType:file.mime_type};
  }
  persist() {
    try {
      const command=this.state?.commands.get('material');
      if(!this.pending.size&&!command){this.storage?.removeItem(this.storageKey);return;}
      this.storage?.setItem(this.storageKey,JSON.stringify({schema_version:1,saved_at:Date.now(),pending:[...this.pending],command:command||null,intent:command?this.state.intent:null}));
    }catch{/* Storage disabled: continue in memory, never weaken IAM. */}
  }
  restorePending() {
    try {
      const data=JSON.parse(this.storage?.getItem(this.storageKey)||'null');
      if(!data||data.schema_version!==1||!Number.isSafeInteger(data.saved_at)||Math.abs(Date.now()-data.saved_at)>86400000||!Array.isArray(data.pending)||data.pending.length>100)return;
      for(const [key,file] of data.pending) {
        if(!uuid.test(file?.file_id||'')||!Number.isSafeInteger(file.version)||file.version<1||key!==`upload:${file.file_id}:${file.version}`||!kinds[file.category]||typeof file.created_at!=='string')throw new Error('invalid pending identity');
        this.pending.set(key,file);
      }
      const c=data.command,i=data.intent;
      const common=c&&i&&uuid.test(c.command_id||'')&&Number.isSafeInteger(c.expected_revision)&&c.expected_revision>0&&i.kind==='material'&&c.requirement_key===i.requirementKey&&Object.values(kinds).includes(i.requirementKey)&&c.operation===i.operation;
      const append=common&&i.operation==='append'&&uuid.test(i.fileID||'')&&Number.isSafeInteger(i.version)&&i.version>0&&c.evidence_file_id===i.fileID&&c.evidence_version===i.version&&c.evidence_source_revision===i.version;
      const remove=common&&i.operation==='remove_one'&&uuid.test(i.materialID||'')&&c.material_reference_id===i.materialID;
      if(append||remove){this.restoredCommand={command_id:c.command_id,expected_revision:c.expected_revision,operation:c.operation,requirement_key:c.requirement_key,...(append?{evidence_file_id:i.fileID,evidence_version:i.version,evidence_source_revision:i.version}:{material_reference_id:i.materialID})};this.restoredIntent=append?{kind:'material',operation:'append',requirementKey:i.requirementKey,fileID:i.fileID,version:i.version}:{kind:'material',operation:'remove_one',requirementKey:i.requirementKey,materialID:i.materialID};}
    }catch{this.pending.clear();this.restoredCommand=null;this.restoredIntent=null;}
  }
}
