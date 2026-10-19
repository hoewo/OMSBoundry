import {cents,decimal,sumMoney,rateBPS,percentFromBPS,previewAmount,previewFeeRows} from './contract-money.mjs';
import {commercialRelation,validateCommercialLinkReceipt} from './commercial-relations.mjs';
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const revision=v=>Number.isSafeInteger(v)&&v>0;
const chains=['client_to_qualification','qualification_to_our_company'];
export const relationFor=chain=>commercialRelation('contract',chain);
export class ContractAdapter {
  constructor({details,projectRequest,projectAuthorize,applicationRequest,evidenceReferenceRequest,transfer,tenantID,subjectID,storage,updated=()=>{},changed=()=>{},newID=()=>crypto.randomUUID()}) {
    Object.assign(this,{details,projectRequest,projectAuthorize,applicationRequest,evidenceReferenceRequest,transfer,tenantID,subjectID,storage,updated,changed,newID});
    this.projectID=details()?.project?.id;
    if(![this.projectID,tenantID,subjectID].every(v=>uuid.test(v||'')))throw new Error('合同身份或项目上下文无效');
    this.native=new Map();this.value=[];this.pending=null;this.tail=Promise.resolve();this.working=false;this.files=new Map();this.uploaded=new Map();
    this.key=`oms.oa.b04.pending.v1:${tenantID}:${subjectID}:${this.projectID}`;
    try{const v=JSON.parse(storage?.getItem(this.key)||'null');if(this.validStoredPending(v))this.pending=v;}catch{/* storage is untrusted, never authority */}
    this.uploadKey=this.key+':files';try{const saved=JSON.parse(storage?.getItem(this.uploadKey)||'null');if(Array.isArray(saved)&&saved.length<=100)for(const row of saved){if(!uuid.test(row.contract_id||'')||!uuid.test(row.file_id||'')||!revision(row.version)||!['contract','amendment','finalization'].includes(row.kind)||(row.kind==='amendment'&&!uuid.test(row.amendment_id||'')))throw new Error('invalid staged file');this.uploaded.set('contract-upload:'+row.file_id+':'+row.version,row)}}catch{this.uploaded.clear()}
  }
  assertCurrent(){if(this.details()?.project?.id!==this.projectID||this.details()?.project?.tenant_id!==this.tenantID)throw new Error('项目已切换，请重新打开合同页');}
  request(path,options){this.assertCurrent();return this.applicationRequest('contract-ledger',path,options);}
  hasInvalidLink(){const p=this.pending;return p?.operation==='save'&&p.stage==='link'&&p.link?.target_service==='contract-ledger'&&p.link.resource_type==='contract'&&p.link.resource_id===p.contract_id&&p.link.relation_key==='reference-oa-contract-'+p.data?.chain_type&&!/^[a-z][a-z0-9-]{0,63}$/.test(p.link.relation_key);}
  repairInvalidLink(){return this.exclusive(async()=>{this.assertCurrent();if(!this.hasInvalidLink())throw new Error('没有已知无效的旧关联；不重写未知命令');const p=this.pending;p.rejected_link=p.link;delete p.link;this.persist();return this.continue();});}
  validPending(p){return !!p&&p.schema_version==='1'&&p.project_id===this.projectID&&p.tenant_id===this.tenantID&&p.subject_id===this.subjectID&&['save','overlay','cancel','native','confirmation','material'].includes(p.operation)&&uuid.test(p.command_id||'')&&(!p.contract_id||uuid.test(p.contract_id))&&(!p.overlay_command_id||uuid.test(p.overlay_command_id))&&(!p.link?.command_id||uuid.test(p.link.command_id))&&(!p.data||chains.includes(p.data.chain_type))&&(p.operation!=='material'||(uuid.test(p.contract_id||'')&&Number.isSafeInteger(p.fields?.expected_revision)&&p.fields.expected_revision>=0&&['contract','amendment','finalization'].includes(p.fields.kind)&&['command','read'].includes(p.stage)&&(p.fields.operation==='append'?uuid.test(p.fields.file_id||'')&&revision(p.fields.version):p.fields.operation==='remove'&&uuid.test(p.fields.material_id||''))))&&(p.operation!=='confirmation'||(uuid.test(p.contract_id||'')&&uuid.test(p.amendment_id||'')&&revision(p.fields?.expected_overlay_revision)&&revision(p.fields?.expected_contract_revision)&&p.fields.amendment?.id===p.amendment_id&&['command','read'].includes(p.stage)))&&(p.operation!=='native'||(uuid.test(p.contract_id||'')&&revision(p.expected_draft_version)&&['party.add','party.remove','effective-date','activate'].includes(p.native_kind)&&['command','read'].includes(p.stage)));}
  persist(){try{if(this.pending)this.storage?.setItem(this.key,JSON.stringify(this.pending));else this.storage?.removeItem(this.key);}catch{}}
  validStoredPending(p){return this.validPending(p)||(p?.operation==='settlement'&&p.schema_version==='1'&&p.project_id===this.projectID&&p.tenant_id===this.tenantID&&p.subject_id===this.subjectID&&uuid.test(p.command_id||'')&&uuid.test(p.contract_id||'')&&['command','read'].includes(p.stage)&&revision(p.fields?.expected_contract_revision)&&Number.isSafeInteger(p.fields?.expected_settlement_sequence)&&p.fields.expected_settlement_sequence>=0&&['progress','final'].includes(p.fields.kind)&&typeof p.fields.amount==='string'&&typeof p.fields.confirmed_on==='string'&&Array.isArray(p.fields.confirming_party_positions));}
  canManage(){return !this.pending&&!this.working&&!!this.context?.allowed_operations.includes('draft.create')&&!!this.details()?.allowed_actions?.includes('project-workspace.resource-link.manage');}
  canEdit(id){return !this.pending&&!this.working&&!!this.native.get(id)?.allowed_operations.includes('draft.update');}
  exclusive(fn){const result=this.tail.then(async()=>{this.working=true;this.changed();try{return await fn()}finally{this.working=false;this.changed();}});this.tail=result.catch(()=>{});return result;}
  validateNative(v,id,chain){
    const c=v?.contract,d=v?.reference_oa_draft;
    if(v?.schema_version!=='1'||v.source_application!=='contract-ledger'||v.tenant_id!==this.tenantID||c?.id!==id||!revision(c.draft_version)||!Array.isArray(v.allowed_operations)||!Array.isArray(v.allowed_actions))throw new Error('合同原生范围或版本不一致');
    if(!d)throw new Error('该合同尚未登记参考 OA 两链及分项；不会伪造空字段');
    const confirmations=v.reference_oa_confirmations||[],seenConfirm=new Set();if(!Array.isArray(confirmations))throw new Error('正式协议关联未完整读取');for(const row of confirmations){if(row.contract_id!==id||row.tenant_id!==this.tenantID||!uuid.test(row.amendment_id||'')||!uuid.test(row.instrument_id||'')||!revision(row.contract_revision)||seenConfirm.has(row.amendment_id))throw new Error('正式协议关联范围不一致');seenConfirm.add(row.amendment_id)}
    const settlements=v.settlements||[],seenSettlement=new Set();if(!Array.isArray(settlements))throw new Error('正式结算未完整读取');for(const row of settlements){if(!uuid.test(row.id||'')||!revision(row.sequence)||seenSettlement.has(row.sequence)||!revision(row.source_revision)||row.source_revision>c.current_revision||!['progress','final'].includes(row.kind)||row.currency!=='CNY'||typeof row.amount!=='string'||cents(row.amount)<0n)throw new Error('正式结算来源或版本不一致');seenSettlement.add(row.sequence)}
    if(d.schema_version!=='1'||d.contract_id!==id||d.tenant_id!==this.tenantID||d.project_id!==this.projectID||!chains.includes(d.chain_type)||(chain&&d.chain_type!==chain)||!revision(d.revision)||!Array.isArray(d.amendments))throw new Error('合同客户扩展的项目或两链不一致');
    for(const fee of Object.values(d.fees||{}))cents(fee);
    if(Object.keys(d.fees||{}).length!==3||!['design_fee','survey_fee','consultation_fee'].every(k=>typeof d.fees[k]==='string'))throw new Error('合同分项字段不完整');
    if(c.status==='draft'&&cents(sumMoney(...Object.values(d.fees)))!==cents(String(c.total_amount).replace(/(\.\d*?)0+$/,'$1').replace(/\.$/,'')))throw new Error('独立合同草稿已变更，客户分项与 Base 金额不同；请核对后显式修订');
    const seen=new Set();for(const a of d.amendments){if(!uuid.test(a.id||'')||seen.has(a.id)||!['adjustment','revision','no_amount_change'].includes(a.change_type)||!a.amendment_number||!/^\d{4}-\d{2}-\d{2}$/.test(a.signed_on))throw new Error('协议草稿不完整');seen.add(a.id);for(const k of ['design_fee','survey_fee','consultation_fee'])cents(a.fees?.[k]);}
    return v;
  }
  view(v){
    const c=v.contract,d=v.reference_oa_draft;
    const amendments=d.amendments.filter(a=>!a.cancelled).map(a=>({id:a.id,contract_id:c.id,amendment_number:a.amendment_number,sign_date:a.signed_on,change_type:a.change_type,...a.fees,contract_rate:percentFromBPS(a.rate_bps),description:a.description,amendment_amount:sumMoney(...Object.values(a.fees)),created_at:a.signed_on,updated_at:a.signed_on,amendment_files:[]}));
    const result={id:c.id,project_id:this.projectID,chain_type:d.chain_type,contract_number:c.contract_number,sign_date:c.signed_on,contract_rate:percentFromBPS(d.rate_bps),...d.fees,contract_amount:sumMoney(...Object.values(d.fees)),settlement_follows_contract_amount:d.settlement_follows_contract_amount,settlement_amount:d.settlement_amount??undefined,amendments,contract_files:[],finalization_files:[],created_at:c.created_at,updated_at:c.updated_at,native_status:c.status,can_edit:c.status==='draft'&&v.allowed_operations.includes('draft.update')&&!this.pending};
    result.contract_files=this.materialFiles(c.id,'contract');result.finalization_files=this.materialFiles(c.id,'finalization');
    result.native_settlement=[...(v.settlements||[])].sort((a,b)=>b.sequence-a.sequence)[0];
    Object.assign(result,previewFeeRows(result));for(const row of result.amendments){row.amendment_files=this.materialFiles(c.id,'amendment',row.id);row.native_confirmation=(v.reference_oa_confirmations||[]).find(c=>c.amendment_id===row.id);row.can_edit=v.allowed_operations.some(op=>['draft.update','reference-oa.amendment.manage'].includes(op))&&!row.native_confirmation}result.current_effective_amount=previewAmount(result);return result;
  }
  async load(){
    this.assertCurrent();await this.projectAuthorize();
    const context=await this.request('/api/v1/contracts/workspace-context');if(context?.schema_version!=='1'||context.source_application!=='contract-ledger'||context.tenant_id!==this.tenantID||!Array.isArray(context.allowed_operations))throw new Error('合同权限上下文不完整');this.context=context;
    const p=await this.projectRequest('/api/v1/projects/'+this.projectID);
    if(p?.project?.id!==this.projectID||p.project.tenant_id!==this.tenantID||!Array.isArray(p.resource_links)||!Array.isArray(p.allowed_actions))throw new Error('项目合同引用读取不完整');
    this.updated(p);const native=new Map(),ids=new Set();
    for(const link of p.resource_links){if(link.unlinked_at||link.target_service!=='contract-ledger'||link.resource_type!=='contract')continue;
      const chain=chains.find(c=>relationFor(c)===link.relation_key);if(!chain)throw new Error('现有合同关联尚无明确两链映射，不能隐藏或默认甲方合同');
      if(!uuid.test(link.resource_id||'')||ids.has(link.resource_id)||link.project_id!==this.projectID||link.tenant_id!==this.tenantID)throw new Error('合同关联范围或两链重复');ids.add(link.resource_id);
      const value=this.validateNative(await this.request('/api/v1/contracts/'+link.resource_id+'/workspace'),link.resource_id,chain);native.set(link.resource_id,value);
    }
    this.assertCurrent();this.native=native;this.value=[...native.values()].filter(v=>v.contract.status!=='cancelled').map(v=>this.view(v));this.changed();return this.value;
  }
  list(){this.assertCurrent();return this.value;}
  async get(id){const v=this.native.get(id);if(!v)throw new Error('当前合同尚未加载');const next=this.validateNative(await this.request('/api/v1/contracts/'+id+'/workspace'),id,v.reference_oa_draft.chain_type);this.native.set(id,next);return this.view(next);}
  amendments(id){const v=this.native.get(id);if(!v)throw new Error('协议所属合同尚未读取');return this.view(v).amendments;}
  input(data){
    if(!chains.includes(data.chain_type)||!data.contract_number?.trim()||!/^\d{4}-\d{2}-\d{2}$/.test(data.sign_date))throw new Error('请填写合同编号、签订日期和明确合同链');
    const fees={design_fee:decimal(cents(data.design_fee)),survey_fee:decimal(cents(data.survey_fee)),consultation_fee:decimal(cents(data.consultation_fee))};if(Object.values(fees).some(v=>cents(v)<0n))throw new Error('原合同分项不能为负数');
    if([...(data.contract_file_ids||[]),...(data.finalization_file_ids||[])].length)throw new Error('合同材料登记尚未接通，文件不能伪装保存成功');
    return {...data,...fees,contract_number:data.contract_number.trim(),contract_amount:sumMoney(...Object.values(fees)),rate_bps:rateBPS(data.contract_rate)};
  }
  async save(id,data){return this.exclusive(async()=>{
    this.assertCurrent();if(this.pending)throw new Error('上次合同操作尚未确认，请继续原操作');
    const existing=id?this.native.get(id):null;if(id&&!existing)throw new Error('合同未加载');
    if(existing&&!existing.allowed_operations.includes('draft.update'))throw new Error('正式合同不能覆盖原始草稿');
    if(!id&&!this.details()?.allowed_actions?.includes('project-workspace.resource-link.manage'))throw new Error('没有项目关联管理权限');
    const normalized=this.input({...existing&&this.view(existing),...data});
    if(existing&&existing.reference_oa_draft.chain_type!==normalized.chain_type)throw new Error('合同链不能更换');
    this.pending={schema_version:'1',operation:'save',command_id:this.newID(),overlay_command_id:this.newID(),project_id:this.projectID,tenant_id:this.tenantID,subject_id:this.subjectID,contract_id:id||null,creating:!id,data:normalized,expected_draft_version:existing?.contract.draft_version,expected_overlay_revision:existing?.reference_oa_draft.revision??0,amendments:existing?.reference_oa_draft.amendments||[],stage:'core'};
    if(existing){this.pending.core_fields={number_namespace:existing.contract.number_namespace,title:existing.contract.title,category:existing.contract.category,currency:existing.contract.currency,effective_on:existing.contract.effective_on};if(existing.contract.contract_number===normalized.contract_number&&existing.contract.signed_on===normalized.sign_date&&cents(String(existing.contract.total_amount).replace(/(\.\d*?)0+$/,'$1').replace(/\.$/,''))===cents(normalized.contract_amount)){this.pending.stage='overlay';this.pending.draft_version=existing.contract.draft_version;}}
    this.persist();return this.continue();
  });}
  overlayBody(p){return {command_id:p.overlay_command_id||p.command_id,expected_revision:p.expected_overlay_revision,expected_draft_version:p.draft_version||p.expected_draft_version,...(p.expected_contract_revision?{expected_contract_revision:p.expected_contract_revision}:{}),project_id:this.projectID,chain_type:p.data.chain_type,fees:{design_fee:p.data.design_fee,survey_fee:p.data.survey_fee,consultation_fee:p.data.consultation_fee},rate_bps:p.data.rate_bps??null,settlement_follows_contract_amount:p.data.settlement_follows_contract_amount,settlement_amount:p.data.settlement_follows_contract_amount?null:p.data.settlement_amount??null,amendments:p.amendments};}
  async continue(){
    this.assertCurrent();const p=this.pending;if(!this.validStoredPending(p))throw new Error('待续命令无效，请重新核对');
    try {
      if(p.operation==='settlement'){
        if(p.stage==='command'){const v=await this.request('/api/v1/contracts/'+p.contract_id+'/settlements',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:p.command_id,...p.fields})});if(v?.tenant_id!==this.tenantID||v.receipt?.contract_id!==p.contract_id||!uuid.test(v.receipt.settlement_id||'')||v.receipt.sequence!==p.fields.expected_settlement_sequence+1)throw new Error('正式结算回执尚未确认');p.stage='read';this.persist();this.changed()}
      } else if(p.operation==='material'){
        if(p.stage==='command'){const v=await this.request('/api/v1/contracts/'+p.contract_id+'/reference-oa-materials',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:p.command_id,...p.fields})});if(v?.tenant_id!==this.tenantID||v.result?.materials?.contract_id!==p.contract_id||v.result.materials.revision!==p.fields.expected_revision+1)throw new Error('材料登记回执尚未确认');if(p.fields.operation==='append')this.uploaded.delete('contract-upload:'+p.fields.file_id+':'+p.fields.version);this.persistFiles();p.stage='read';this.persist();this.changed()}
      } else if(p.operation==='confirmation'){
        if(p.stage==='command'){const v=await this.request('/api/v1/contracts/'+p.contract_id+'/reference-oa-amendments/'+p.amendment_id+'/confirm',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:p.command_id,...p.fields})});const receipt=v?.confirmation;if(v?.tenant_id!==this.tenantID||receipt?.contract_id!==p.contract_id||receipt?.amendment_id!==p.amendment_id||!uuid.test(receipt.instrument_id||'')||receipt.contract_revision!==p.fields.expected_contract_revision+1)throw new Error('协议正式确认回执尚未读取');p.stage='read';this.persist();this.changed()}
      } else if(p.operation==='native'){
        if(p.stage==='command'){
          const suffix=p.native_kind==='party.add'?'/parties':p.native_kind==='party.remove'?'/parties/'+p.fields.party_link_id+'/remove':p.native_kind==='activate'?'/activate':'';
          const fields={...p.fields};delete fields.party_link_id;
          const v=await this.request('/api/v1/contracts/'+p.contract_id+suffix,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:p.command_id,expected_draft_version:p.expected_draft_version,...fields})});
          if(v?.tenant_id!==this.tenantID||v?.receipt?.contract_id!==p.contract_id||(p.native_kind==='activate'?!revision(v.receipt.revision):!revision(v.receipt.draft_version)))throw new Error('原生合同操作回执尚未确认');
          p.stage='read';this.persist();this.changed();
        }
      } else if(p.operation==='cancel'){
        if(p.stage!=='read'){
        const v=await this.request('/api/v1/contracts/'+p.contract_id+'/cancel',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:p.command_id,expected_draft_version:p.expected_draft_version,reason:'移出参考 OA 合同草稿'})});
        if(v?.receipt?.contract_id!==p.contract_id)throw new Error('取消结果尚未确认');
        p.stage='read';this.persist();
        }
      } else {
        if(p.stage==='core'){
          const body={command_id:p.command_id,number_namespace:'ref-oa-'+this.tenantID+'-'+(p.data.chain_type===chains[0]?'client':'company'),title:p.data.contract_number,currency:'CNY',effective_on:'',category:'',...p.core_fields,contract_number:p.data.contract_number,total_amount:p.data.contract_amount,signed_on:p.data.sign_date,...(p.creating?{}:{expected_draft_version:p.expected_draft_version})};
          const value=await this.request('/api/v1/contracts'+(p.creating?'':'/'+p.contract_id),{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
          const receipt=value?.receipt;if(value?.tenant_id!==this.tenantID||!uuid.test(receipt?.contract_id||'')||!revision(receipt.draft_version)||(!p.creating&&receipt.contract_id!==p.contract_id))throw new Error('合同核心保存回执尚未确认');
          p.contract_id=receipt.contract_id;p.draft_version=receipt.draft_version;p.stage='overlay';this.persist();this.changed();
        }
        if(p.stage==='overlay'){
          const v=await this.request('/api/v1/contracts/'+p.contract_id+'/reference-oa-draft',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(this.overlayBody(p))});
          if(v?.tenant_id!==this.tenantID||v?.result?.draft?.contract_id!==p.contract_id||v.result.draft.project_id!==this.projectID||v.result.draft.chain_type!==p.data.chain_type||v.result.draft.revision!==p.expected_overlay_revision+1)throw new Error('客户扩展保存回执尚未确认');
          p.stage=p.creating?'link':'read';this.persist();this.changed();
        }
        if(p.stage==='link'){
          if(!p.link){await this.projectAuthorize();const current=await this.projectRequest('/api/v1/projects/'+this.projectID);if(current?.project?.id!==this.projectID||current.project.tenant_id!==this.tenantID||!revision(current.project.revision))throw new Error('项目关联版本尚未读取');this.updated(current);
            p.link={command_id:this.newID(),expected_revision:current.project.revision,operation:'link',target_service:'contract-ledger',resource_type:'contract',resource_id:p.contract_id,source_revision:p.draft_version,relation_key:relationFor(p.data.chain_type),note:''};this.persist();}
          await this.projectAuthorize();const result=await this.projectRequest('/api/v1/projects/'+this.projectID+'/resource-links',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(p.link)});
          validateCommercialLinkReceipt(result,this.projectID,this.tenantID,p.link);p.stage='read';this.persist();
        }
      }
      // A committed mutation plus a read failure remains resumable read-only.
      if(p.operation==='cancel')p.stage='read';
      await this.load();
      this.pending=null;this.persist();this.value=[...this.native.values()].filter(v=>v.contract.status!=='cancelled').map(v=>this.view(v));this.changed();return this.value.find(c=>c.id===p.contract_id);
    }catch(error){this.error=error.message;this.conflict=error.status===409;
      // A definite rejection of a native single-transaction command has no
      // saved fact to continue. Keep 409/unknown replies and committed reads;
      // release rejected input so the user can correct it without a reload.
      if((['native','confirmation','material','settlement'].includes(p.operation)&&p.stage==='command'||p.operation==='overlay'&&p.stage==='overlay')&&[400,401,403,404,415,422,429].includes(error.status)){this.pending=null;this.conflict=false;}
      this.persist();this.changed();throw error;}
  }
  retry(){return this.exclusive(()=>{if(this.conflict)throw new Error('版本冲突须先核对；不会自动覆盖');return this.continue();});}
  canManageAmendments(id){const v=this.native.get(id);return !this.pending&&!this.working&&!!v&&(v.allowed_operations.includes('draft.update')||v.allowed_operations.includes('reference-oa.amendment.manage'));}
  canRecordSettlement(id){return !this.pending&&!this.working&&!!this.native.get(id)?.allowed_operations.includes('contract.settlement.record');}
  recordSettlement(id,data){return this.exclusive(async()=>{
    this.assertCurrent();const v=this.native.get(id);if(this.pending||!v?.allowed_operations.includes('contract.settlement.record'))throw new Error('当前合同没有结算确认权限或尚有未确认操作');
    const settlements=v.settlements;if(!Array.isArray(settlements)||!revision(v.contract.current_revision))throw new Error('正式结算来源版本未完整读取');let sequence=0;for(const row of settlements){if(!revision(row.sequence)||!uuid.test(row.id||'')||row.kind==='final')throw new Error('已有最终结算或结算版本不完整');sequence=Math.max(sequence,row.sequence)}
    const positions=[...(data.confirming_party_positions||[])].sort((a,b)=>a-b),allowed=(v.party_snapshots||[]).filter(p=>p.revision===v.contract.current_revision).map(p=>p.position);
    if(!positions.length||positions.some((p,i)=>!revision(p)||!allowed.includes(p)||(i>0&&p===positions[i-1])))throw new Error('请选择当前正式合同版本的确认主体');
    const amount=decimal(cents(data.amount));if(cents(amount)<0n||!['progress','final'].includes(data.kind)||!data.statement_number?.trim()||!data.title?.trim()||!data.basis?.trim()||!/^\d{4}-\d{2}-\d{2}$/.test(data.confirmed_on||''))throw new Error('请填写完整的正式结算信息');
    this.pending={schema_version:'1',operation:'settlement',command_id:this.newID(),project_id:this.projectID,tenant_id:this.tenantID,subject_id:this.subjectID,contract_id:id,stage:'command',fields:{expected_contract_revision:v.contract.current_revision,expected_settlement_sequence:sequence,kind:data.kind,statement_number:data.statement_number.trim(),title:data.title.trim(),amount,confirmed_on:data.confirmed_on,basis:data.basis.trim(),confirming_party_positions:positions}};this.persist();return this.continue();
  });}
  canConfirmAmendment(id,amendment){const v=this.native.get(id);return !this.pending&&!this.working&&v?.allowed_operations.includes('reference-oa.amendment.confirm')&&!v.reference_oa_confirmations?.some(c=>c.amendment_id===amendment);}
  confirmAmendment(id,amendment,effectiveOn){return this.exclusive(async()=>{this.assertCurrent();const v=this.native.get(id);if(this.pending||!v?.allowed_operations.includes('reference-oa.amendment.confirm'))throw new Error('当前合同不能确认协议');const row=v.reference_oa_draft.amendments.find(a=>a.id===amendment&&!a.cancelled);if(!row||v.reference_oa_confirmations?.some(c=>c.amendment_id===amendment))throw new Error('协议已确认或不可用');if(!/^\d{4}-\d{2}-\d{2}$/.test(effectiveOn||''))throw new Error('请显式填写协议生效日期');this.pending={schema_version:'1',operation:'confirmation',command_id:this.newID(),project_id:this.projectID,tenant_id:this.tenantID,subject_id:this.subjectID,contract_id:id,amendment_id:amendment,stage:'command',fields:{expected_overlay_revision:v.reference_oa_draft.revision,expected_contract_revision:v.contract.current_revision,effective_on:effectiveOn,amendment:structuredClone(row)}};this.persist();return this.continue()});}
  async partyCandidates(){const v=await this.request('/api/v1/contracts/party-candidates');if(v?.schema_version!=='1'||v.source_application!=='contract-ledger'||v.tenant_id!==this.tenantID||v.directory_kind!=='contract_local'||!Array.isArray(v.parties))throw new Error('合同主体目录范围不完整');const seen=new Set();for(const p of v.parties){if(!uuid.test(p.id||'')||seen.has(p.id)||!revision(p.revision)||!['organization','individual'].includes(p.kind)||!p.legal_name?.trim())throw new Error('合同主体目录记录无效');seen.add(p.id)}return v.parties;}
  prepareNative(id,kind,fields){return this.exclusive(async()=>{
    this.assertCurrent();if(this.pending)throw new Error('上次合同操作尚未确认');const v=this.native.get(id),ops=v?.allowed_operations||[];
    const required={'party.add':'draft.party.add','party.remove':'draft.party.remove','effective-date':'draft.update','activate':'contract.activate'}[kind];if(!required||!ops.includes(required))throw new Error('当前合同没有此原生操作权限');
    let payload;
    if(kind==='party.add'){if(!uuid.test(fields.party_id||'')||!fields.role?.trim()||fields.role.trim().length>100)throw new Error('请选择主体并填写合同内角色');if(v.parties?.some(p=>p.active&&p.party_id===fields.party_id&&p.role.toLowerCase()===fields.role.trim().toLowerCase()))throw new Error('该主体与合同角色已存在');payload={party_id:fields.party_id,role:fields.role.trim()};}
    else if(kind==='party.remove'){if(!uuid.test(fields.party_link_id||'')||!v.parties?.some(p=>p.id===fields.party_link_id&&p.active))throw new Error('签约主体关联不属于当前合同');payload={party_link_id:fields.party_link_id};}
    else if(kind==='effective-date'){if(!/^\d{4}-\d{2}-\d{2}$/.test(fields.effective_on||''))throw new Error('请填写生效日期');const c=v.contract;payload={number_namespace:c.number_namespace,contract_number:c.contract_number,title:c.title,category:c.category,currency:c.currency,total_amount:c.total_amount,signed_on:c.signed_on,effective_on:fields.effective_on};}
    else payload={};
    this.pending={schema_version:'1',operation:'native',command_id:this.newID(),project_id:this.projectID,tenant_id:this.tenantID,subject_id:this.subjectID,contract_id:id,expected_draft_version:v.contract.draft_version,native_kind:kind,fields:payload,stage:'command'};this.persist();return this.continue();
  });}
  cancel(id){return this.exclusive(async()=>{if(this.pending)throw new Error('上次操作尚未完成');const v=this.native.get(id);if(!v?.allowed_operations.includes('draft.cancel'))throw new Error('仅未确认合同草稿可取消；历史不会删除');this.pending={schema_version:'1',operation:'cancel',command_id:this.newID(),project_id:this.projectID,tenant_id:this.tenantID,subject_id:this.subjectID,contract_id:id,expected_draft_version:v.contract.draft_version,stage:'cancel'};this.persist();await this.continue();});}
  async saveAmendment(id,amendmentID,data){return this.exclusive(async()=>{
    if(this.pending)throw new Error('上次操作尚未完成');const v=this.native.get(id);if(!v||!v.allowed_operations.some(op=>['draft.update','reference-oa.amendment.manage'].includes(op)))throw new Error('当前没有协议意图管理权限');if(v.reference_oa_confirmations?.some(c=>c.amendment_id===amendmentID))throw new Error('已确认协议需要追加更正，不能按草稿编辑');
    if((data.amendment_file_ids||[]).length)throw new Error('协议材料登记尚未接通');
    const old=v.reference_oa_draft.amendments.find(a=>a.id===amendmentID);if(amendmentID&&(!old||old.cancelled))throw new Error('协议草稿不可编辑');
      const row={id:amendmentID||this.newID(),amendment_number:data.amendment_number,signed_on:data.sign_date,change_type:data.change_type,fees:{design_fee:data.design_fee,survey_fee:data.survey_fee,consultation_fee:data.consultation_fee},rate_bps:rateBPS(data.contract_rate),description:data.description||'',cancelled:false};
      for(const fee of Object.values(row.fees))cents(fee);
    this.prepareOverlay(id,v,[...v.reference_oa_draft.amendments.filter(a=>a.id!==amendmentID),row]);return this.continue();
  });}
  prepareOverlay(id,v,amendments){const data={...this.view(v),rate_bps:v.reference_oa_draft.rate_bps};this.pending={schema_version:'1',operation:'overlay',command_id:this.newID(),project_id:this.projectID,tenant_id:this.tenantID,subject_id:this.subjectID,contract_id:id,data,amendments,expected_overlay_revision:v.reference_oa_draft.revision,expected_draft_version:v.contract.draft_version,expected_contract_revision:v.contract.status==='active'?v.contract.current_revision:undefined,stage:'overlay'};this.persist();}
  cancelAmendment(id,amendmentID){return this.exclusive(async()=>{if(this.pending)throw new Error('上次操作尚未完成');const v=this.native.get(id);if(!v?.allowed_operations.some(op=>['draft.update','reference-oa.amendment.manage'].includes(op))||v.reference_oa_confirmations?.some(c=>c.amendment_id===amendmentID))throw new Error('已确认协议不能删除');const rows=v.reference_oa_draft.amendments;if(!rows.some(a=>a.id===amendmentID&&!a.cancelled))throw new Error('协议不属于当前合同');this.prepareOverlay(id,v,rows.map(a=>a.id===amendmentID?{...a,cancelled:true}:a));return this.continue();});}
  materialFiles(id,kind,amendmentID=''){
    const state=this.native.get(id)?.reference_oa_materials;if(!state)return [];
    if(state.contract_id!==id||state.tenant_id!==this.tenantID||!Number.isSafeInteger(state.revision)||state.revision<0||!Array.isArray(state.items))throw new Error('合同文件引用范围或版本未读取');
    const seen=new Set(),views=[];for(const row of state.items){if(!uuid.test(row.id||'')||seen.has(row.id))throw new Error('合同材料身份不一致');seen.add(row.id);const ref=row.file;if(!uuid.test(ref?.file_id||'')||ref.tenant_id!==this.tenantID||ref.id!==ref.file_id||!revision(ref.version)||ref.version!==ref.revision||ref.status!=='available'||ref.source_application!=='evidence-service'||!Number.isSafeInteger(ref.size_bytes)||ref.size_bytes<0)throw new Error('合同文件版本未完整读取');const view={id:'contract-material:'+row.id,file_id:ref.file_id,version:ref.version,material_reference_id:row.id,original_name:ref.file_name,file_name:ref.file_name,file_size:ref.size_bytes,mime_type:ref.media_type,created_at:row.created_at};this.files.set(view.id,view);if(!row.removed_at&&row.kind===kind&&(row.amendment_id||'')===amendmentID)views.push(view)}return views;
  }
  stagedFiles(id,kind,amendmentID=''){return [...this.uploaded].filter(([,v])=>v.contract_id===id&&v.kind===kind&&(v.amendment_id||'')===amendmentID).map(([key,row])=>({...row,id:key,original_name:row.file_name||'已上传，文件元数据待回读',file_size:row.size_bytes||0,mime_type:row.media_type||'',version:row.version}));}
  persistFiles(){try{if(this.uploaded.size)this.storage?.setItem(this.uploadKey,JSON.stringify([...this.uploaded.values()]));else this.storage?.removeItem(this.uploadKey)}catch{}}
  canManageMaterials(id){return !this.pending&&!this.working&&!!this.transfer&&!!this.native.get(id)?.allowed_operations.includes('reference-oa.material.manage');}
  descriptor(id){const f=this.files.get(id);if(!f)throw new Error('文件版本尚未读取');return {fileId:f.file_id,version:f.version,fileName:f.original_name,sizeBytes:f.file_size,mediaType:f.mime_type};}
  async uploadMaterial(id,kind,amendmentID,file){return this.exclusive(async()=>{
    if(this.pending||!this.native.get(id)?.allowed_operations.includes('reference-oa.material.manage')||!this.transfer)throw new Error('没有当前合同附件权限或先处理未确认操作');this.assertCurrent();
    const {version}=await this.transfer.uploadFile({file,clientContext:{ownerRef:id,purpose:'contract-'+kind}});if(version.status!=='available'||!uuid.test(version.fileId||'')||!revision(version.version))throw new Error('Evidence 上传尚未完成');
    const key='contract-upload:'+version.fileId+':'+version.version;const row={contract_id:id,kind,amendment_id:amendmentID||'',file_id:version.fileId,version:version.version,created_at:version.createdAt};this.uploaded.set(key,row);this.persistFiles();
    const ref=await this.evidenceReferenceRequest(row.file_id,row.version);if(ref?.tenant_id!==this.tenantID||ref.file_id!==row.file_id||ref.id!==row.file_id||ref.version!==row.version||ref.revision!==row.version||ref.status!=='available'||ref.source_application!=='evidence-service')throw new Error('文件已上传，元数据未确认；保留版本，勿重复上传');Object.assign(row,{file_name:ref.file_name,size_bytes:ref.size_bytes,media_type:ref.media_type});this.persistFiles();const view={...row,id:key,original_name:ref.file_name,file_size:ref.size_bytes,mime_type:ref.media_type};this.files.set(key,view);this.changed();return view;
  });}
  mutateMaterial(id,kind,amendmentID,viewID,operation){return this.exclusive(async()=>{
    if(this.pending||!this.native.get(id)?.allowed_operations.includes('reference-oa.material.manage'))throw new Error('先处理未确认操作或当前无材料权限');const state=this.native.get(id).reference_oa_materials;if(!state)throw new Error('材料版本未读取');let fields={expected_revision:state.revision,operation,kind,...(amendmentID?{amendment_id:amendmentID}:{})};
    if(operation==='append'){const file=this.uploaded.get(viewID);if(!file||file.contract_id!==id||file.kind!==kind||(file.amendment_id||'')!==(amendmentID||''))throw new Error('只能登记本合同刚上传的文件版本');fields={...fields,file_id:file.file_id,version:file.version}}else{const row=state.items.find(m=>'contract-material:'+m.id===viewID&&!m.removed_at&&m.kind===kind&&(m.amendment_id||'')===(amendmentID||''));if(!row)throw new Error('只能移出当前合同的该引用');fields={...fields,material_id:row.id}}
    this.pending={schema_version:'1',operation:'material',command_id:this.newID(),project_id:this.projectID,tenant_id:this.tenantID,subject_id:this.subjectID,contract_id:id,fields,stage:'command'};this.persist();return this.continue();
  });}
}
