/* OA B03: user commands across two explicit owners, never a shared transaction. */
(function (global) {
  'use strict';
  const methods = Object.freeze({comparison:'比选', competitive_negotiation:'竞争性谈判', tender:'招投标'});
  const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  const prefix = '/api/v1/business-engagements';
  class BiddingFlow {
    constructor({projectRequest, projectAuthorize, applicationRequest, evidenceReferenceRequest, tenantID, projectUpdated=()=>{}, randomUUID = () => global.crypto.randomUUID()}) {
      Object.assign(this,{projectRequest,projectAuthorize,applicationRequest,evidenceReferenceRequest,tenantID,projectUpdated,randomUUID});
      this.states = new Map(); this.busy = false;
    }
    validateProject(details) {
      const p=details?.project;
      if (!uuid.test(p?.id||'') || p.tenant_id!==this.tenantID || !Number.isSafeInteger(p.revision) || p.revision<1) throw new Error('项目上下文无效');
      return p.id;
    }
    validateReceipt(value, id) {
      const item=value?.engagement;
      if (!uuid.test(item?.id||'') || item.tenant_id!==this.tenantID || item.project_id!==id || !Number.isSafeInteger(item.revision) || item.revision<1) throw new Error('承接记录与当前项目不一致');
      return value;
    }
    validateSource(value, id) {
      this.validateReceipt(value,id);
      if (!Array.isArray(value.allowed_actions)) throw new Error('承接原生权限回读未完成');
      if(!Array.isArray(value.materials)||value.materials.some(row=>!uuid.test(row.id||'')||row.tenant_id!==this.tenantID||row.engagement_id!==value.engagement.id||!uuid.test(row.evidence_file_id||'')||!Number.isSafeInteger(row.evidence_version)||row.evidence_version<1||!Number.isSafeInteger(row.evidence_source_revision)||row.evidence_source_revision<1||!['tender_document','bid_document','award_document'].includes(row.requirement_key)))throw new Error('材料引用与当前承接记录或租户不一致');
      return value;
    }
    request(path, options) { return this.applicationRequest('business-engagement',path,options); }
    async load(details) {
      const id=this.validateProject(details);
      const list=await this.request(prefix+'?project_id='+encodeURIComponent(id)+'&limit=2');
      if (list.project_id!==id || !Array.isArray(list.items) || list.items.length>1 || !Array.isArray(list.allowed_actions)) throw new Error('承接列表未按当前项目返回，请勿重复新建');
      for (const item of list.items) if (!uuid.test(item.id||'') || item.tenant_id!==this.tenantID || item.project_id!==id) throw new Error('承接列表包含其他项目或租户');
      const source=list.items.length ? this.validateSource(await this.request(prefix+'/'+list.items[0].id),id) : null;
      const links=(details.resource_links||[]).filter(link=>!link.unlinked_at && link.target_service==='business-engagement' && link.resource_type==='business-engagement');
      if (links.length>1 || (links.length && (!source || links[0].resource_id!==source.engagement.id))) throw new Error('项目引用与承接来源不一致，请先核对；不会自动覆盖引用');
      let state=this.states.get(id);
      if (!state) {state={commands:new Map(),intent:null};this.states.set(id,state);}
      Object.assign(state,{project:details,source,linked:links.length===1,actions:source?.allowed_actions||list.allowed_actions,credentialActions:list.allowed_actions});
      return state;
    }
    can(state, action) {
      const permitted=state.actions.includes(action) || (action==='business-engagement.method.select' && state.source?.engagement.applicability==='undetermined' && state.actions.includes('business-engagement.applicability.manage') && state.credentialActions.includes(action));
      return state.project.project.status==='active' && permitted &&
        (!!state.source || state.actions.includes('business-engagement.create')) &&
        (state.linked || state.project.allowed_actions?.includes('project-workspace.resource-link.manage')) &&
        state.source?.engagement.status!=='cancelled';
    }
    async command(state, key, request, payload) {
      let command=state.commands.get(key);
      if (!command) {command={command_id:this.randomUUID(),...payload};state.commands.set(key,command);}
      try {
        const value=await request(command);
        state.commands.delete(key);
        return value;
      } catch(error) {
        // A definitive rejection can be corrected after explicit reread. An
        // uncertain response keeps the exact ID AND original payload.
        if (error.status>=400 && error.status<500 && !error.keepCommand) state.commands.delete(key);
        if(error.status===409)state.conflict=true;
        throw error;
      }
    }
    post(state,key,path,payload) {
      return this.command(state,key,async command=>{
        const id=state.project.project.id;
        const receipt=this.validateReceipt(await this.request(path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(command)}),id);
        // Mutation receipts are domain facts, not executable permission lists.
        // The authenticated native GET is the only source of next-step actions.
        let current;
        try {current=this.validateSource(await this.request(prefix+'/'+receipt.engagement.id),id);}
        catch(error){error.keepCommand=true;throw error;}
        if(current.engagement.id!==receipt.engagement.id)throw new Error('承接原生回读来源不一致');
        if(current.engagement.revision!==receipt.engagement.revision)throw Object.assign(new Error('承接资料已被更新，请重新读取；不会自动覆盖'),{status:409});
        return current;
      },payload);
    }
    accept(state,value,id){state.source=this.validateSource(value,id);state.actions=state.source.allowed_actions;return state.source;}
    canMaterial(state,key) {
      return !!state?.source && state.linked && state.source.engagement.applicability==='applicable' &&
        Object.hasOwn(methods,state.source.engagement.method||'') && this.can(state,'business-engagement.evidence.manage') &&
        state.source.requirements?.some(row=>row.key===key&&row.required===true);
    }
    async material(details,intent) {
      if(this.busy)throw new Error('正在保存，请等待');
      const id=this.validateProject(details),state=this.states.get(id);
      if(!state)throw new Error('请先读取招投标资料');
      if(state.conflict)throw new Error('资料版本冲突，请先重新读取最新资料，不会自动覆盖');
      if(intent?.kind!=='material'||!['append','remove_one'].includes(intent.operation)||!['tender_document','bid_document','award_document'].includes(intent.requirementKey))throw new Error('材料操作无效');
      const normalized={kind:'material',operation:intent.operation,requirementKey:intent.requirementKey};
      if(intent.operation==='append') {
        if(!uuid.test(intent.fileID||'')||!Number.isSafeInteger(intent.version)||intent.version<1)throw new Error('请先完成 Evidence 文件上传');
        Object.assign(normalized,{fileID:intent.fileID,version:intent.version});
      } else {
        if(!uuid.test(intent.materialID||''))throw new Error('材料引用无效');
        normalized.materialID=intent.materialID;
      }
      if(state.intent&&JSON.stringify(state.intent)!==JSON.stringify(normalized))throw new Error('上次操作尚未确认，请先继续原操作或重新读取');
      if(!this.canMaterial(state,normalized.requirementKey))throw new Error('当前账号或业务状态不能修改本项目材料');
      const current=(state.source.materials||[]).filter(row=>!row.superseded_at);
      // An uncertain command must replay its frozen payload, even if a later
      // read already shows the committed result. Never mint a replacement ID.
      if(!state.commands.has('material')) {
        if(normalized.operation==='remove_one'&&!current.some(row=>row.id===normalized.materialID&&row.requirement_key===normalized.requirementKey&&row.engagement_id===state.source.engagement.id&&row.tenant_id===this.tenantID))throw new Error('只能移除当前承接记录的有效材料引用');
        if(normalized.operation==='append'&&current.some(row=>row.requirement_key===normalized.requirementKey&&row.evidence_file_id===normalized.fileID&&row.evidence_version===normalized.version))throw new Error('该文件版本已登记，不会重复追加');
        if(normalized.operation==='append'&&current.length>=100)throw new Error('当前承接材料已达到 100 条上限');
      }
      state.intent=normalized;this.busy=true;
      try {
        const payload={expected_revision:state.source.engagement.revision,operation:normalized.operation,requirement_key:normalized.requirementKey};
        if(normalized.operation==='append'&&!state.commands.has('material')) {
          if(!this.evidenceReferenceRequest)throw new Error('Evidence 原生文件入口尚未配置；已上传文件保留，未登记');
          const reference=await this.evidenceReferenceRequest(normalized.fileID,normalized.version);
          if(reference?.schema_version!=='1'||reference.source_application!=='evidence-service'||reference.resource_type!=='evidence-file'||reference.id!==normalized.fileID||reference.file_id!==normalized.fileID||reference.tenant_id!==this.tenantID||reference.version!==normalized.version||reference.revision!==normalized.version||reference.status!=='available')throw new Error('Evidence 文件版本未确认或不属于当前租户，未登记');
          Object.assign(payload,{evidence_file_id:reference.file_id,evidence_version:reference.version,evidence_source_revision:reference.revision});
        } else if(normalized.operation==='remove_one')payload.material_reference_id=normalized.materialID;
        this.accept(state,await this.post(state,'material',prefix+'/'+state.source.engagement.id+'/materials',payload),id);
        state.intent=null;
        return state;
      } finally {this.busy=false;}
    }
    async apply(details, intent) {
      if (this.busy) throw new Error('正在保存，请等待');
      const id=this.validateProject(details), state=this.states.get(id);
      if (!state) throw new Error('请先读取招投标资料');
      if(state.conflict)throw new Error('资料版本冲突，请先重新读取最新资料，不会自动覆盖');
      if (!['method','applicability'].includes(intent.kind) || (intent.kind==='method' && !Object.hasOwn(methods,intent.method)) || (intent.kind==='applicability' && typeof intent.applicable!=='boolean')) throw new Error('承接操作无效');
      if (state.intent && JSON.stringify(state.intent)!==JSON.stringify(intent)) throw new Error('上次操作尚未确认，请先继续原操作或重新读取');
      const action=intent.kind==='method'?'business-engagement.method.select':'business-engagement.applicability.manage';
      if (!this.can(state,action) || (intent.kind==='method' && state.source?.engagement.applicability!=='applicable' && !state.actions.includes('business-engagement.applicability.manage'))) throw new Error('当前账号不能保存此项目的承接操作或建立项目引用');
      state.intent={...intent};this.busy=true;
      try {
        if (!state.source) this.accept(state,await this.post(state,'create',prefix,{project_id:id,project_source_revision:state.project.project.revision}),id);
        if (!state.linked) {
          await this.projectAuthorize();
          if (!state.commands.has('link')) {
            const fresh=await this.projectRequest('/api/v1/projects/'+id);
            if (this.validateProject(fresh)!==id) throw new Error('项目复读不一致');
            state.project=fresh;
            const existing=(fresh.resource_links||[]).filter(link=>!link.unlinked_at && link.target_service==='business-engagement' && link.resource_type==='business-engagement');
            if (existing.length>1 || (existing.length && existing[0].resource_id!==state.source.engagement.id)) throw new Error('项目已有另一承接引用，不能覆盖');
            if (existing.length===1) state.linked=true;
          }
          if (!state.linked) {
            if (!state.project.allowed_actions?.includes('project-workspace.resource-link.manage')) throw new Error('承接记录已保存，但当前账号不能建立项目引用；记录保留，未重复新建');
            const result=await this.command(state,'link',async command=>{
              const value=await this.projectRequest('/api/v1/projects/'+id+'/resource-links',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(command)});
              if(value.project?.id!==id || value.project.tenant_id!==this.tenantID || value.resource_link?.resource_id!==state.source.engagement.id)throw new Error('项目引用回复尚未确认，请重新读取');
              return value;
            },{expected_revision:state.project.project.revision,operation:'link',target_service:'business-engagement',resource_type:'business-engagement',resource_id:state.source.engagement.id,source_revision:state.source.engagement.revision,relation_key:'engagement-materials',note:''});
            state.project={...state.project,project:result.project,resource_links:[...(state.project.resource_links||[]),result.resource_link]}; state.linked=true;
            this.projectUpdated(state.project);
          }
        }
        const sourceID=state.source.engagement.id;
        const applicable=intent.kind==='method' ? true : intent.applicable;
        if (state.commands.has('applicability') || state.source.engagement.applicability!==(applicable?'applicable':'not_applicable')) {
          if(!state.actions.includes('business-engagement.applicability.manage'))throw new Error('当前账号不能确认招投标适用性；已保存的记录和引用保留');
          const result=await this.post(state,'applicability',prefix+'/'+sourceID+'/applicability',{expected_revision:state.source.engagement.revision,applicable,reason:applicable?'':'当前项目招投标阶段无／不适用'});
          this.accept(state,result,id);
        }
        if (intent.kind==='method' && (state.commands.has('method') || state.source.engagement.method!==intent.method)) {
          if(!state.actions.includes('business-engagement.method.select'))throw new Error('当前原生动作不允许选择方式；不会绕过状态或权限');
          const result=await this.post(state,'method',prefix+'/'+sourceID+'/method',{expected_revision:state.source.engagement.revision,method:intent.method});
          this.accept(state,result,id);
        }
        state.intent=null;state.commands.clear();
        return state;
      } finally {this.busy=false;}
    }
  }
  class BiddingWorkspace {
    constructor({root,flow,details,record=()=>{}}) {
      Object.assign(this,{root,flow,details,record});this.sequence=0;this.state=null;this.message='';
      root.addEventListener('change',event=>{if(event.target.matches('[data-real-bidding-method]'))this.save({kind:'method',method:event.target.value});});
      root.addEventListener('click',event=>{
        if(event.target.closest('[data-real-bidding-applicability]') && this.state)this.save({kind:'applicability',applicable:this.state.source?.engagement.applicability==='not_applicable'});
        if(event.target.closest('[data-real-bidding-retry]') && this.state?.intent)this.save(this.state.intent);
        if(event.target.closest('[data-real-bidding-reload]'))this.load(true);
        const remove=event.target.closest('[data-real-bidding-material-remove]');
        if(remove&&!remove.disabled&&global.confirm('只从本项目招投标资料中移除这份引用，不删除文件或其他记录的引用。确定继续？'))this.save({kind:'material',operation:'remove_one',requirementKey:remove.dataset.requirementKey,materialID:remove.dataset.realBiddingMaterialRemove});
      });
    }
    field(selector) {return this.root.querySelector(selector);}
    invalidate(){this.sequence++;}
    async load(explicit=false) {
      const sequence=++this.sequence, details=this.details(), id=details?.project?.id;
      this.record('flow','读取当前项目的招投标资料','Workbench 组织只读请求；Business Engagement 返回自有承接和材料引用，不因读取新建记录。','waiting');
      this.state=null;this.message='';this.field('[data-real-bidding-method]').value='';this.field('[data-real-bidding-requirements]').replaceChildren();
      this.field('[data-real-bidding-status]').textContent='正在读取…';
      this.disable();
      this.field('[data-real-bidding-retry]').hidden=true;
      if(this.flow.busy){this.reloadAfterSave={explicit};this.field('[data-real-bidding-status]').textContent='正在保存，请等待当前操作结束';return;}
      try {
        const state=await this.flow.load(details);
        if(sequence!==this.sequence || this.details()?.project?.id!==id)return;
        if(explicit){state.commands.clear();state.intent=null;state.conflict=false;this.message='已读取当前事实；不会自动重新提交先前操作。';}
        this.state=state;
        if(state.intent&&!explicit)this.message='先前操作回复尚未确认；可继续原操作，或重新读取当前事实。';
        this.record('workbench','当前招投标资料已复读','只接受当前 Tenant／Project 对应的承接资料；读取没有新增材料或历史。','complete');
        this.render();
      }catch(error){if(sequence===this.sequence && this.details()?.project?.id===id){this.state=null;this.field('[data-real-bidding-status]').textContent='读取未完成：'+error.message;}}
    }
    disable(){for(const element of this.root.querySelectorAll('[data-real-bidding-method],[data-real-bidding-applicability],[data-real-bidding-retry],[data-real-bidding-material-remove]'))element.disabled=true;}
    async save(intent) {
      if(this.flow.busy || !this.state)return;
      const sequence=++this.sequence, details=this.details(), id=details?.project?.id;
      let saved=false;
      this.record('flow',intent.kind==='material'?'保存当前项目的材料引用':'保存当前项目的承接方式／适用性',intent.kind==='material'?'Workbench 提交明确材料动作；BE 拥有引用，Evidence 拥有文件，移除不物理删除文件。':'Workbench 组织 BE 原生命令及必要 Project Link；两个数据所有者分别确认，不共享事务。','waiting');
      this.disable();this.field('[data-real-bidding-status]').textContent='正在保存…';
      try {
        await (intent.kind==='material'?this.flow.material(details,intent):this.flow.apply(details,intent));
        saved=true;
        if(sequence!==this.sequence || this.details()?.project?.id!==id)return;
        this.record('bidding-command',intent.kind==='material'?'材料引用保存':'承接保存',intent.kind==='material'?'Business Engagement 只追加或结束材料引用；Evidence 文件版本和其他项目引用没有被删除。':'当前用户经 Gateway／IAM 向 Business Engagement 保存承接事实；Project 只保存引用，两库分别确认。','complete');
        this.message='已保存。材料齐备不等于中标或签约。';
        // Re-read only. A failed read must never rerun a successful command.
        this.state=await this.flow.load(this.flow.states.get(id).project);
        if(sequence===this.sequence && this.details()?.project?.id===id){this.record('workbench','当前操作与同来源复读已确认','工作台只展示目标原生回执和当前动作；不会替代目标授权或复制业务事实。','complete');this.render();}
      }catch(error){
        if(sequence!==this.sequence || this.details()?.project?.id!==id)return;
        this.record('bidding-command','当前操作尚未全部确认','请求或后续原生复读未完成；保留已确认来源和原命令，不把部分成功说成完整保存。','error');
        const state=this.flow.states.get(id);this.state=state;
        this.message=state?.intent?(intent.kind==='material'?'材料操作回复尚未确认；文件及已确认引用保留：':state.source?'承接记录已存在；后续操作未完成：':'操作回复尚未确认：')+error.message+'。继续原操作使用原命令；重新读取不自动提交。':saved?'保存已确认，复读未完成：'+error.message+'。请重新读取，不必再提交。':'未保存：'+error.message;
        this.render();
      }finally{
        if(this.reloadAfterSave){const pending=this.reloadAfterSave;this.reloadAfterSave=null;await this.load(pending.explicit);}
      }
    }
    render() {
      const state=this.state, source=state?.source, item=source?.engagement, pending=!!state?.intent;
      if(!state)return;
      const select=this.field('[data-real-bidding-method]');
      select.value=item?.method||'';select.disabled=pending||!this.flow.can(state,'business-engagement.method.select')||item?.applicability==='not_applicable'||(item?.applicability!=='applicable'&&!state.actions.includes('business-engagement.applicability.manage'));
      const applicability=this.field('[data-real-bidding-applicability]');
      applicability.textContent=item?.applicability==='not_applicable'?'恢复适用':'本阶段无／不适用';applicability.disabled=pending||!this.flow.can(state,'business-engagement.applicability.manage');
      const retry=this.field('[data-real-bidding-retry]');retry.hidden=!pending||!!state.conflict;retry.disabled=!pending||!!state.conflict;
      this.field('[data-real-bidding-status]').textContent=this.message||(item?.applicability==='not_applicable'?'本阶段无／不适用；历史资料保留。':!item?'请选择承接方式；选择后保存当前项目的招投标资料。':!state.linked?'已有承接资料；下次保存将恢复项目关联，不重复新建。':'当前项目招投标资料已读取。');
      const content=this.field('[data-real-bidding-requirements]');content.replaceChildren();
      if(item?.applicability==='not_applicable')return;
      const names={tender_document:'招标文件',bid_document:'投标文件',award_document:'中标文件'};
      for(const key of Object.keys(names)) {
        const requirement=source?.requirements?.find(row=>row.key===key), materials=(source?.materials||[]).filter(row=>row.requirement_key===key&&!row.superseded_at), material=materials[0];
        // Satisfied can mean "not required", not an uploaded attachment.
        // Only the owning application's current material reference is evidence.
        const line=document.createElement('p');line.textContent=names[key]+'：'+(!item?.method?'待选方式':material?'已登记':requirement?.required?'必传，尚未登记':'当前方式非必传');
        if(material)line.textContent+=' · '+material.display_snapshot;
        content.append(line);
        // Keep all current references visible. The old singular projection is
        // only a compatibility representative, never the complete file list.
        for(const member of materials) {
          const row=document.createElement('div');row.className='bidding-file-slot__file-entry';row.textContent=member.display_snapshot;
          const remove=document.createElement('button');remove.type='button';remove.className='ant-btn bidding-file-slot__action-button';remove.textContent='移除';
          remove.dataset.realBiddingMaterialRemove=member.id;remove.dataset.requirementKey=key;
          // Collections are additive; a legacy source without the new field
          // remains readable and must not receive an unsupported new command.
          remove.disabled=pending||!Array.isArray(requirement?.material_references)||!this.flow.canMaterial(state,key)||!uuid.test(member.id||'');
          row.append(remove);content.append(row);
        }
      }
      const note=document.createElement('p');note.className='project-workspace__card-subtitle';note.textContent='移除只结束本项目材料引用，文件与历史保留；上传、查看和下载仍待 Evidence 文件入口接通。';content.append(note);
    }
  }
  global.ReferenceOABidding={BiddingFlow,BiddingWorkspace,methods};
})(globalThis);
