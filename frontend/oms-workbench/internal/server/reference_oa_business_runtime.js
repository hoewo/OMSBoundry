/* Same-project OA readers. All facts remain at their source applications.
 * No simulated record, implicit link, grant, financial total or business write.
 */
(function (global) {
  'use strict';
  const tabs = Object.freeze({overview: '总览', client: '甲方信息', personnel: '经营参与人', bidding: '招投标信息', contracts: '合同信息', payments: '支付信息', invoices: '开票信息', costs: '经营成本', bonus: '奖金分配'});
  const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  const labels = {active:'有效', draft:'草稿', pending:'待处理', 'in-progress':'进行中', completed:'已完成', recorded:'已登记', confirmed:'已确认', published:'已发布', withdrawn:'已撤回', superseded:'已取代', ready:'材料齐备', incomplete:'材料待补齐', cancelled:'已取消', not_applicable:'不适用', required:'适用', direct:'直接承接', bidding:'招投标', design:'设计', 'design-v1':'设计', 'consultation-v1':'咨询', available:'可读取', revision_changed:'来源已更新', unavailable:'来源暂不可用', invalid:'引用失效', receipt:'收款', payment:'付款', receivable:'应收', payable:'应付'};
  const label = value => labels[value] || value || '未设置';
  function minorAmount(value, currency) {
    // JSON int64 outside the exact JS range must never appear as a rounded amount.
    if (!Number.isSafeInteger(value)) return value == null ? '未获金额读取权限或金额未知' : '金额超出页面精确表示范围';
    const n = BigInt(value), absolute = n < 0n ? -n : n;
    return (currency || '') + ' ' + (n < 0n ? '-' : '') + String(absolute / 100n).replace(/\B(?=(\d{3})+(?!\d))/g, ',') + '.' + String(absolute % 100n).padStart(2, '0');
  }
  function decimalAmount(value, currency) {
    if (typeof value !== 'string' || !/^-?\d+(?:\.\d+)?$/.test(value)) return '金额未知';
    const [whole, fraction] = value.split('.');
    return (currency || '') + ' ' + whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',') + (fraction == null ? '' : '.' + fraction);
  }
  const missing = (message, state = 'unconnected') => ({state, message, records: []});
  class BusinessFlow {
    constructor({projectRequest, projectAuthorize, applicationRequest, tenantID}) {
      this.projectRequest = projectRequest; this.projectAuthorize = projectAuthorize;
      this.applicationRequest = applicationRequest; this.tenantID = tenantID;
    }
    validateProject(details) {
      if (!uuid.test(details?.project?.id || '') || details.project.tenant_id !== this.tenantID) throw new Error('项目上下文或租户不一致');
      return details.project.id;
    }
    async commercial(details, tab) {
      const id = this.validateProject(details);
      if (!details.allowed_actions?.includes('project-workspace.commercial-summary.read')) return missing('当前账号没有本项目经营金额读取权限，未读取合同、支付或开票金额。', 'denied');
      await this.projectAuthorize();
      const value = await this.projectRequest('/api/v1/projects/' + id + '/commercial-summary');
      if (value.project_id !== id || !Array.isArray(value.segments)) throw new Error('经营汇总响应与当前项目不一致');
      const types = {contracts:['contract'], payments:['obligation','transaction'], invoices:['invoice_registration']};
      const segments = value.segments.filter(item => !types[tab] || types[tab].includes(item.resource_type));
      const records = segments.map(item => {
        const fields = [['业务状态', label(item.business_status)], ['读取状态', label(item.state)]];
        if (item.state === 'available' || item.state === 'revision_changed') {
          const c = item.contract, o = item.obligation, t = item.transaction, i = item.invoice_registration;
          if (c) fields.push(['当前有效合同金额', decimalAmount(c.current_effective_amount,c.currency)]);
          if (o) fields.push(['方向',label(o.direction)],['义务金额',decimalAmount(o.effective_amount,o.currency)],['已核销',decimalAmount(o.allocated_amount,o.currency)],['未清金额',decimalAmount(o.outstanding_amount,o.currency)]);
          if (t) fields.push(['收付类型',label(t.transaction_type)],['发生日期',t.occurred_on],['实际资金',decimalAmount(t.effective_amount,t.currency)],['未核销',decimalAmount(t.unallocated_amount,t.currency)]);
          if (i) fields.push(['开票日期',i.issued_on],['开票金额',minorAmount(i.amount_minor,i.currency)]);
        }
        if (item.error_code) fields.push(['读取问题',item.error_code]);
        return {title:item.display_summary || ({contract:'合同',obligation:'确认义务',transaction:'资金记录',invoice_registration:'开票登记'}[item.resource_type] || '经营记录'), fields, source:item.source_application, revision:item.source_revision, state:item.state};
      });
      return {state: records.length ? 'available' : 'unconnected', records, message: records.length ? '读取本项目已连接的真实记录；不同金额含义不混合加总。' : '本项目尚未连接相应经营记录。这不表示来源系统没有记录；新增、连接及原 OA 编辑表单尚待接通。'};
    }
    async linked(details, application, resourceType, path, key) {
      const id = this.validateProject(details);
      const links = (details.resource_links || []).filter(link => !link.unlinked_at && link.target_service === application && link.resource_type === resourceType);
      if (!links.length) return missing('本项目尚未连接' + (application === 'business-engagement' ? '招投标承接记录' : '生产任务') + '；不能据此认定没有源记录。原 OA 新增、文件及编辑操作尚待接通。');
      const records = [];
      // Serial reads keep low-resource D1 quiet; failure stays local to one record.
      for (const link of links) {
        if (!uuid.test(link.resource_id || '')) throw new Error('业务引用标识无效');
        try {
          const value = await this.applicationRequest(application, path + '/' + link.resource_id);
          const item = value[key];
          if (item?.project_id !== id || item.id !== link.resource_id || item.tenant_id !== this.tenantID) throw new Error('源记录的项目、租户或标识不一致');
          const fields = [['状态',label(item.status)]];
          if (key === 'engagement') {
            fields.push(['适用性',label(item.applicability)],['承接方式',label(item.method)]);
            if (item.not_applicable_reason) fields.push(['不适用说明',item.not_applicable_reason]);
            for (const material of (value.materials || []).filter(m => !m.superseded_at)) fields.push([material.requirement_key, material.display_snapshot + ' · 文件版本 ' + material.evidence_version]);
            if (value.missing_requirements?.length) fields.push(['待补材料',value.missing_requirements.join('、')]);
          } else {
            fields.push(['生产类型',label(item.type_key)],['当前阶段',label(item.current_stage_key)]);
            for (const stage of value.stages || []) fields.push([label(stage.stage_key),label(stage.status)]);
          }
          records.push({title:key === 'engagement' ? '招投标与经营承接' : label(item.type_key) + '生产任务',fields,source:application,revision:item.revision,state:'available'});
        } catch (error) {
          records.push({title:key === 'engagement' ? '承接记录读取未完成' : '生产任务读取未完成',fields:[['原因',error.message]],source:application,state:'unavailable'});
        }
      }
      return {state:'available',records,message:'已连接记录逐项读取；缺权或来源故障不会伪装为空记录。当前页面只读。'};
    }
    async collection(details, tab) {
      const id = this.validateProject(details), bonus = tab === 'bonus', application = bonus ? 'bonus-distribution' : 'cost-outsourcing';
      const value = await this.applicationRequest(application, (bonus ? '/api/v1/bonus-pools' : '/api/v1/cost-entries') + '?project_id=' + encodeURIComponent(id));
      if (value.project_id !== id || !Array.isArray(value.items)) throw new Error('来源列表响应与当前项目不一致');
      for (const item of value.items) if (item.project?.id !== id || item.tenant_id !== this.tenantID) throw new Error('来源记录的项目或租户不一致');
      const items = value.items.filter(item => !bonus || item.bonus_type === 'business');
      const records = items.map(item => ({title:bonus ? '经营奖池' : label(item.category), source:application, revision:item.revision, state:'available', fields:bonus ? [['状态',label(item.state)],['奖池金额',minorAmount(item.amount_minor,item.currency)],['未分配',minorAmount(item.unallocated_minor,item.currency)],...((item.allocations || []).map(a => [a.person?.display_name || '分配记录',minorAmount(a.amount_minor,a.currency)]))] : [['状态',label(item.state)],['发生日期',item.occurred_on],['金额',minorAmount(item.amount_minor,item.currency)],['说明',item.description || '未填写']]}));
      return {state:'available',records,message:bonus ? '本项目真实经营奖池；新建、分配和发布表单尚未接通。' : '读取本项目成本原记录。经营／生产归属尚未映射，当前不能把这些记录全部当成经营成本或计算经营合计；录入表单尚未接通。'};
    }
    async production(details) {
      const id=this.validateProject(details),value=await this.applicationRequest('production-operations','/api/v1/production-stage-progress?project_id='+encodeURIComponent(id)+'&type_key=reference-oa-stage-set-v1',{productionStageScope:id});
      if(value?.schema_version!=='1.0'||value?.profile!=='reference-oa-production-progress-v1'||value?.tenant_id!==this.tenantID||value?.project_id!==id||!Array.isArray(value.items))throw new Error('生产进度响应与当前项目不一致');
      const stageLabels={scheme:'方案阶段','preliminary-design':'初步设计','construction-drawing':'施工图设计',change:'变更洽商',completion:'竣工验收',test:'试验阶段',report:'咨询／勘察／测绘报告'};
      if(value.items.length>1)throw new Error('同一项目存在多套生产阶段真相，拒绝合并显示');
      const records=value.items.map(item=>{
        if(!uuid.test(item?.work_item_id||'')||!Array.isArray(item.stages)||!Array.isArray(item.type_progress)||!Array.isArray(item.project_type_keys)||!Number.isInteger(item.revision)||!Number.isInteger(item.type_definition_version))throw new Error('生产进度项目契约无效');
        const fields=[['状态',label(item.status)],['定义版本','v'+String(item.type_definition_version)],['冻结项目 revision',String(item.project_source_revision)],['必需阶段进度',String(item.required_complete_count)+'/'+String(item.required_stage_count)]];
        for(const stage of item.stages){
          if(!Object.hasOwn(stageLabels,stage.stage_key)||!Number.isInteger(stage.main_file_count)||!Number.isInteger(stage.review_file_count))throw new Error('生产阶段投影契约无效');
          fields.push([stageLabels[stage.stage_key],label(stage.status)+' · 主文件 '+String(stage.main_file_count)+' 份'+(stage.review_file_count?' · 校审 '+String(stage.review_file_count)+' 份':'')]);
        }
        return {title:'生产任务 · revision '+String(item.revision),fields,source:'production-operations',revision:item.revision,state:'available'};
      });
      const item=records.length?value.items[0]:null,typeProgress=item?.type_progress||[];
      for(const line of typeProgress)if(!['consultation','design','mapping','survey'].includes(line?.project_type_key)||!Number.isInteger(line.required_stage_count)||!Number.isInteger(line.required_complete_count))throw new Error('生产类型进度契约无效');
      return {state:records.length?'available':'unconnected',records,message:records.length?'进度由 Production 自有 Work Item、阶段和有效资料一次投影；普通读取不回查 Project。':'当前项目尚无 Production 工作项；这不表示 Project 类型未设置。',progress:{complete:Boolean(value.production_complete),requiredComplete:Number(value.required_complete_count)||0,requiredTotal:Number(value.required_stage_count)||0,stageComplete:Number(value.completed_stage_count)||0,stageTotal:Number(value.applicable_stage_count)||0,typeComplete:typeProgress.filter(line=>line.complete).length,typeTotal:typeProgress.length,types:typeProgress,stages:item?.stages||[]}};
    }
    async load(details, tab) {
      this.validateProject(details);
      if (['overview','contracts','payments','invoices'].includes(tab)) return this.commercial(details,tab);
      if (tab === 'bidding') return this.linked(details,'business-engagement','business-engagement','/api/v1/business-engagements','engagement');
      if (tab === 'production') return this.production(details);
      if (['costs','bonus'].includes(tab)) return this.collection(details,tab);
      if (tab === 'personnel') return missing('经营参与人的准确类型与分权尚未实现，添加／移除不可用。这里不会把经营负责人或生产参与人冒充经营参与人。', 'unimplemented');
      throw new Error('未知经营栏目');
    }
  }
  class BusinessWorkspace {
    constructor({root, flow, details, loadClient, selected, bidding, personnel, contracts, cash, invoices, costs, productionCosts, approvalAudit, commission, bonus, productionBonus, productionPersonnel, productionStage, productionLayout, productionSelected}) {
      this.root=root; this.flow=flow; this.details=details; this.loadClient=loadClient; this.selected=selected; this.sequence=0; this.tab='overview';
      this.readFreshMs=30000;this.readCache=new Map();this.readInflight=new Map();
      this.bidding=bidding;
      this.personnel=personnel;
      this.contracts=contracts;
      this.cash=cash;
      this.invoices=invoices;
      this.costs=costs;this.productionCosts=productionCosts;
      this.approvalAudit=approvalAudit;this.commission=commission;
      this.bonus=bonus;this.productionBonus=productionBonus;
      this.productionPersonnel=productionPersonnel;
      this.productionStage=productionStage;
      this.productionLayout=productionLayout;this.productionSelected=productionSelected;this.productionTab='overview';
      root.addEventListener('click', event => {
        const tab = event.target.closest('[data-real-business-tab]');
        if (tab) this.select(tab.dataset.realBusinessTab,true);
        if (event.target.closest('[data-real-business-reload]')) this.select(this.tab,false,true);
        if (event.target.closest('[data-real-production-reload]')) this.production(this.productionTab,false,true);
      });
      root.addEventListener('keydown', event => {
        const button=event.target.closest('[data-real-business-tab]'); if (!button || !['ArrowLeft','ArrowRight','Home','End'].includes(event.key)) return;
        const buttons=[...root.querySelectorAll('[data-real-business-tab]')], current=buttons.indexOf(button);
        const index=event.key==='Home'?0:event.key==='End'?buttons.length-1:(current+(event.key==='ArrowRight'?1:-1)+buttons.length)%buttons.length;
        event.preventDefault(); buttons[index].focus(); this.select(buttons[index].dataset.realBusinessTab,true);
      });
    }
    field(selector) { return this.root.querySelector(selector); }
    cache(area,id,tab) { return area+':'+id+':'+tab; }
    cachedRead(key,explicit,load) {
      this.readCache ||= new Map();this.readInflight ||= new Map();if(!Number.isFinite(this.readFreshMs))this.readFreshMs=30000;
      const cached=this.readCache.get(key),fresh=cached&&Date.now()-cached.loadedAt<this.readFreshMs;
      if(!explicit&&fresh)return {cached,task:null};
      let task=this.readInflight.get(key);
      if(!task){let loaded;try{loaded=load();}catch(error){loaded=Promise.reject(error);}task=Promise.resolve(loaded).then(value=>{this.readCache.set(key,{value,loadedAt:Date.now()});return value;}).finally(()=>{if(this.readInflight.get(key)===task)this.readInflight.delete(key);});this.readInflight.set(key,task);}
      return {cached,task};
    }
    skeleton(content) {
      if(typeof document==='undefined'){content.replaceChildren();content.skeleton=true;return;}
      const shell=document.createElement('div'),label=document.createElement('div');shell.className='workspace-read-skeleton';label.className='workspace-read-skeleton__label';label.textContent='正在读取当前项目资料…';shell.append(label);
      for(const width of ['82%','94%','76%','58%']){const line=document.createElement('div');line.className='workspace-read-skeleton__line';line.style.width=width;shell.append(line);}
      content.replaceChildren(shell);
    }
    productionSummary(progress) {
      const count=this.field('[data-real-production-summary-progress]'),state=this.field('[data-real-production-summary-state]'),note=this.field('[data-real-production-summary-note]');
      if(!count||!state||!note)return;
      count.textContent=String(progress?.typeComplete||0)+'/'+String(progress?.typeTotal||0);
      state.textContent=progress?.complete?'全部完成':progress?.stageTotal?'进行中':'未建立';
      note.textContent=String(progress?.stageComplete||0)+'/'+String(progress?.stageTotal||0)+' 个适用阶段已完成 · 来源 Production';
      const badge=this.field('[data-real-production-overview-badge]');if(badge)badge.textContent=progress?.complete?'全部完成':progress?.stageTotal?'已启动':'未建立';
      for(const row of this.root.querySelectorAll('[data-real-production-overview-stage]'))row.hidden=true;
      for(const stage of progress?.stages||[]){const key=stage.stage_key,row=this.field('[data-real-production-overview-stage="'+key+'"]'),value=this.field('[data-real-production-overview-stage-value="'+key+'"]');if(!row||!value)continue;row.hidden=false;value.textContent=label(stage.status)+' · 主文件 '+String(stage.main_file_count)+' 份'+(stage.review_file_count?' · 校审 '+String(stage.review_file_count)+' 份':'');value.className='production-info__overview-value production-info__overview-value--'+(stage.status==='completed'?'ready':'pending');}
      for(const line of progress?.types||[]){const key='type-'+line.project_type_key,row=this.field('[data-real-production-overview-stage="'+key+'"]'),value=this.field('[data-real-production-overview-stage-value="'+key+'"]');if(!row||!value)continue;row.hidden=false;value.textContent=String(line.required_complete_count)+'/'+String(line.required_stage_count)+' 个必需阶段'+(line.complete?' · 已完成':'');value.className='production-info__overview-value production-info__overview-value--'+(line.complete?'ready':'pending');}
    }
invalidate() { this.sequence++; this.productionSequence=(this.productionSequence||0)+1; this.readCache?.clear();this.readInflight?.clear();this.bidding?.invalidate(); this.personnel?.invalidate();this.contracts?.invalidate();this.cash?.invalidate();this.invoices?.invalidate();this.costs?.invalidate();this.productionCosts?.invalidate();this.approvalAudit?.invalidate();this.commission?.invalidate();this.bonus?.invalidate();this.productionBonus?.invalidate();this.productionPersonnel?.invalidate();this.productionStage?.invalidate(); }
    async select(tab, updateURL=false, explicit=false) {
      tab = Object.hasOwn(tabs,tab) ? tab : 'overview';
      const form=this.field('[data-real-client-form]');
      if (this.tab==='client' && tab!=='client' && form && !form.hidden) { this.field('[data-real-client-status]').textContent='请先保存或取消甲方编辑，再切换栏目。'; return; }
      const sequence=++this.sequence, details=this.details(), id=details?.project?.id;
      this.tab=tab;
      for (const button of this.root.querySelectorAll('[data-real-business-tab]')) { const active=button.dataset.realBusinessTab===tab; button.classList.toggle('ant-tabs-tab-active',active); button.setAttribute('aria-selected',String(active)); button.tabIndex=active?0:-1; }
      this.field('[data-real-business-panel="client"]').hidden=tab!=='client';
      const biddingPanel=this.field('[data-real-business-panel="bidding"]');if(biddingPanel)biddingPanel.hidden=tab!=='bidding';
      const personnelPanel=this.field('[data-real-business-panel="personnel"]');if(personnelPanel)personnelPanel.hidden=tab!=='personnel';
      const contractPanel=this.field('[data-real-business-panel="contracts"]');if(contractPanel)contractPanel.hidden=tab!=='contracts';
      const cashPanel=this.field('[data-real-business-panel="payments"]');if(cashPanel)cashPanel.hidden=tab!=='payments';
      const invoicePanel=this.field('[data-real-business-panel="invoices"]');if(invoicePanel)invoicePanel.hidden=tab!=='invoices';
      const costPanel=this.field('[data-real-business-panel="costs"]');if(costPanel)costPanel.hidden=tab!=='costs';
      const bonusPanel=this.field('[data-real-business-panel="bonus"]');if(bonusPanel)bonusPanel.hidden=tab!=='bonus';
      this.field('[data-real-business-reader]').hidden=tab==='client'||(tab==='bidding'&&!!this.bidding)||(tab==='personnel'&&!!this.personnel)||(tab==='contracts'&&!!this.contracts)||(tab==='payments'&&!!this.cash)||(tab==='invoices'&&!!this.invoices)||(tab==='costs'&&!!this.costs)||(tab==='bonus'&&!!this.bonus);
      this.selected(tab,updateURL);
      if (tab==='client') return this.loadClient(explicit);
      if(tab==='bidding'&&this.bidding)return this.bidding.load(explicit);
      if(tab==='personnel'&&this.personnel)return this.personnel.load(explicit);
      if(tab==='contracts'&&this.contracts)return this.contracts.load(explicit);
      if(tab==='payments'&&this.cash)return this.cash.load(explicit);
      if(tab==='invoices'&&this.invoices)return this.invoices.load(explicit);
      if(tab==='costs'&&this.costs)return this.costs.load(explicit);
      if(tab==='bonus'&&this.bonus)return this.bonus.load(explicit);
      this.field('[data-real-business-title]').textContent=tabs[tab];
      this.field('[data-real-business-description]').textContent='当前项目内读取，不跳转其他系统。';
      const status=this.field('[data-real-business-status]'), content=this.field('[data-real-business-content]');
      const read=this.cachedRead(this.cache('business',id,tab),explicit,()=>this.flow.load(details,tab));
      if(read.cached){status.textContent=read.cached.value.message+(read.task?' · 正在后台更新…':'');this.render(content,read.cached.value.records);}
      else {status.textContent='正在读取…';this.skeleton(content);}
      if(!read.task)return;
      try {
        const value=await read.task;
        if (sequence!==this.sequence || this.details()?.project?.id!==id) return;
        status.textContent=value.message;
        this.render(content,value.records);
      } catch (error) {
        if (sequence!==this.sequence || this.details()?.project?.id!==id) return;
        if(read.cached){status.textContent=read.cached.value.message+' · 后台更新失败，可手动刷新；当前仍显示上次结果。';this.render(content,read.cached.value.records);}
        else {content.replaceChildren(); status.textContent='读取未完成：'+error.message+'。请刷新重试；这不表示没有数据。';}
      }
    }
    async production(tab='overview', updateURL=false, explicit=false) {
      const sequence=(this.productionSequence||0)+1; this.productionSequence=sequence;
      const details=this.details(), id=details?.project?.id;
      tab=this.productionLayout?this.productionLayout.select(tab,details):(['cost','bonus'].includes(tab)?tab:'overview');
      this.productionTab=tab;
      this.productionSelected?.(tab,updateURL);
      if(tab==='cost'&&this.productionCosts)return this.productionCosts.load(explicit);
      if(tab==='approval-audit'&&this.approvalAudit)return this.approvalAudit.load(explicit);
      if(tab==='commission'&&this.commission)return this.commission.load(explicit);
      if(tab==='bonus'&&this.productionBonus)return this.productionBonus.load(explicit);
      if(tab==='personnel'&&this.productionPersonnel)return this.productionPersonnel.load(explicit);
      if(['scheme','preliminary','construction','change-completion','test','consultation'].includes(tab)&&this.productionStage)return this.productionStage.load(tab,explicit);
      // These original OA work surfaces are intentionally navigable before
      // their owner ports exist. Do not issue an unrelated generic production
      // read merely because the user selected a visible stage shell.
      if(['scheme','preliminary','construction','change-completion','test','consultation'].includes(tab))return;
      const status=this.field('[data-real-production-status]'), content=this.field('[data-real-production-content]');
      if(!status||!content)return;
      const read=this.cachedRead(this.cache('production',id,tab),explicit,()=>this.flow.load(details,'production'));
      if(read.cached){this.productionSummary(read.cached.value.progress);status.textContent=read.cached.value.message+(read.task?' · 正在后台更新…':'');this.render(content,read.cached.value.records);}else{status.textContent='正在读取…';this.skeleton(content);}
      if(!read.task)return;
      try { const value=await read.task; if (sequence!==this.productionSequence || this.details()?.project?.id!==id) return; this.productionSummary(value.progress);const currentStatus=this.field('[data-real-production-status]'),currentContent=this.field('[data-real-production-content]');if(currentStatus)currentStatus.textContent=value.message;if(currentContent)this.render(currentContent,value.records); }
      catch(error) { if (sequence===this.productionSequence && this.details()?.project?.id===id){if(read.cached){status.textContent=read.cached.value.message+' · 后台更新失败，可手动刷新；当前仍显示上次结果。';this.render(content,read.cached.value.records);}else{content.replaceChildren();status.textContent='读取未完成：'+error.message+'。请刷新重试。';}} }
    }
    render(content,records) {
      content.replaceChildren();
      for (const item of records) {
        const article=document.createElement('article'), title=document.createElement('h4'), facts=document.createElement('dl'), source=document.createElement('small');
        article.className='real-business-record'; title.textContent=item.title;
        for (const [name,value] of item.fields) { const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=name;dd.textContent=value ?? '未知';facts.append(dt,dd); }
        source.textContent='来源：'+item.source+(item.revision?' · revision '+item.revision:'');article.append(title,facts,source);content.append(article);
      }
    }
  }
  global.ReferenceOABusiness = {BusinessFlow,BusinessWorkspace,tabs,minorAmount,decimalAmount};
})(globalThis);
