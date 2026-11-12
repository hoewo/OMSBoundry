(()=>{
  'use strict';

  const root=document.querySelector('[data-oms-v1]');
  const ownerAPI=globalThis.OMSPhase1OwnerAPI;
  const model=globalThis.OMSInternalModel;
  if(!root||!model||!ownerAPI?.app?.('people-groups')||!ownerAPI?.app?.('payroll'))return;

  const safe=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));
  const uuid=()=>globalThis.crypto?.randomUUID?.()||'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g,value=>{const random=Math.random()*16|0,part=value==='x'?random:(random&3|8);return part.toString(16)});
  const today=()=>{const value=new Date();return new Date(value.getTime()-value.getTimezoneOffset()*60000).toISOString().slice(0,10)};
  const currentMonth=()=>today().slice(0,7);
  const date=value=>value?String(value).slice(0,10).replaceAll('-','/'):'—';
  const dateTime=value=>{if(!value)return '—';const parsed=new Date(value);return Number.isNaN(parsed.getTime())?String(value):new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}).format(parsed)};
  const minorInput=value=>{try{const raw=BigInt(String(value||0)),negative=raw<0n,absolute=negative?-raw:raw;return (negative?'-':'')+String(absolute/100n)+'.'+String(absolute%100n).padStart(2,'0')}catch{return '0.00'}};
  const minorJSON=value=>{const raw=BigInt(String(value)),number=Number(raw);if(!Number.isSafeInteger(number))throw new Error('金额超出当前版本可安全处理的范围');return number};
  const field=(label,control,wide=false)=>'<label class="oms-v1__form-field'+(wide?' is-wide':'')+'"><span>'+safe(label)+'</span>'+control+'</label>';
  const option=(value,label,current)=>'<option value="'+safe(value)+'" '+(String(value)===String(current)?'selected':'')+'>'+safe(label)+'</option>';
  const formStatus=copy=>'<div class="oms-v1__form-status is-wide" data-oms-v1-form-status>'+safe(copy||'')+'</div>';
  const tag=(label,tone='')=>'<span class="oms-v1__internal-tag '+(tone?'is-'+tone:'')+'">'+safe(label)+'</span>';
  const show=detail=>root.dispatchEvent(new CustomEvent('oms-v1:show-dialog',{detail}));
  const close=()=>root.dispatchEvent(new CustomEvent('oms-v1:close-dialog'));
  const api=(path,options)=>ownerAPI.api('payroll',path,options);
  const post=(path,value)=>api(path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(value)});
  const put=(path,value)=>api(path,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(value)});
  const components={pension:'养老保险',medical:'医疗保险（含生育）',unemployment:'失业保险',work_injury:'工伤保险',maternity:'生育保险（已并入医保）',housing_fund:'住房公积金'};
  const componentOrder=['pension','medical','unemployment','work_injury','maternity','housing_fund'];
  const labels={review_pending:'待核对',ready_to_pay:'待发放',partially_paid:'部分发放',paid:'已发放',zero_pending:'待确认零发放',no_payment_required:'无需发放',recovery_pending:'待扣回',voided:'已作废'};
  const statusLabel=value=>labels[value]||value||'—';
  const statusTone=value=>value==='paid'||value==='no_payment_required'?'positive':value==='ready_to_pay'||value==='partially_paid'||value==='zero_pending'?'warning':value==='voided'?'muted':'';
  const actorLabel=run=>{const id=String(run?.created_by_subject_id||'');if(!id)return '—';return id===String(root.dataset.subjectId||'')&&root.dataset.displayName?root.dataset.displayName:'账号 '+id.slice(0,8)};
  const runPath=run=>'/api/v1/payroll-periods/'+encodeURIComponent(run.id);
  const linePath=(run,line)=>runPath(run)+'/lines/'+encodeURIComponent(line.id);
  const remaining=line=>BigInt(String(line.net_payable_minor||0))-BigInt(String(line.allocated_paid_minor||0));
  const paymentReady=line=>line.status==='ready_to_pay'||line.status==='partially_paid';
  const editable=line=>line.status==='review_pending';
  const selectable=line=>editable(line)||paymentReady(line);

  const state={loading:true,error:'',runs:[],schemes:[],people:[],compensations:[],selected:new Set(),selectable:[],page:1,pageSize:20,sort:{key:'pay_date',direction:'desc'}};

  function personDepartment(personID){const person=state.people.find(item=>item.id===personID);return person?.department||''}
  function compare(left,right){if(typeof left==='bigint'||typeof right==='bigint'){const a=typeof left==='bigint'?left:BigInt(String(left||0)),b=typeof right==='bigint'?right:BigInt(String(right||0));return a===b?0:a>b?1:-1}return String(left??'').localeCompare(String(right??''),'zh-CN',{numeric:true,sensitivity:'base'})}
  function payrollRows(){const rows=[];for(const run of state.runs)for(const line of run.lines||[])rows.push({run,line,department:personDepartment(line.person_id)});return rows}
  function sorted(items){const {key,direction}=state.sort;return [...items].sort((left,right)=>{let a,b;if(key==='member'){a=left.line.person_display_name;b=right.line.person_display_name}else if(key==='payable'){a=BigInt(String(left.line.net_payable_minor||0));b=BigInt(String(right.line.net_payable_minor||0))}else if(key==='status'){a=left.line.status;b=right.line.status}else if(key==='month'){a=left.run.period;b=right.run.period}else{a=left.run.planned_pay_date;b=right.run.planned_pay_date}const result=compare(a,b);return direction==='asc'?result:-result})}
  function emptyRow(title,copy){return '<tr><td colspan="11"><div class="oms-v1__empty"><strong>'+safe(title)+'</strong><span>'+safe(copy)+'</span></div></td></tr>'}
  function schemeSort(left,right){return Number(right.scheme_kind==='custom')-Number(left.scheme_kind==='custom')||String(right.effective_month||'').localeCompare(String(left.effective_month||''))||Number(right.revision||0)-Number(left.revision||0)}
  function percentInput(value){return (Number(value||0)/100).toFixed(2)}
  function percentBPS(value){const raw=String(value??'').trim();if(!/^(?:100(?:\.0{1,2})?|\d{1,2}(?:\.\d{1,2})?)$/.test(raw))throw new Error('缴费比例必须是 0 到 100 之间、最多两位小数的百分比');return Math.round(Number(raw)*100)}
  function nonnegativeMinor(value,label){const amount=model.decimalMinor(value);if(amount===null||amount<0n)throw new Error(label+'必须是大于等于 0、最多两位小数的金额');return minorJSON(amount)}

  async function load(){
    state.loading=true;state.error='';render();
    try{
      const [runs,schemes,people,compensations]=await Promise.all([
        api('/api/v1/payroll-periods'),
        api('/api/v1/contribution-schemes'),
        ownerAPI.api('people-groups','/api/v1/persons'),
        api('/api/v1/compensation-profiles'),
      ]);
      state.runs=Array.isArray(runs?.items)?runs.items:[];
      state.schemes=Array.isArray(schemes)?schemes.sort(schemeSort):[];
      state.people=Array.isArray(people)?people:[];
      state.compensations=Array.isArray(compensations)?compensations:[];
    }catch(error){state.error=error instanceof Error?error.message:'工资数据暂不可用';state.runs=[];state.schemes=[]}
    state.loading=false;render();
  }

  function renderSchemeState(){
    const target=root.querySelector('[data-internal-scheme-state]'),button=target?.querySelector('[data-internal-benefit-scheme]');
    if(!target)return;
    target.classList.toggle('is-warning',state.schemes.length===0||Boolean(state.error));
    const custom=state.schemes.find(item=>item.scheme_kind==='custom');
    target.querySelector('p').textContent=state.error?'工资服务暂不可用：'+state.error:custom?'当前使用本公司缴费方案；新工资表会冻结所选方案和比例。':state.schemes.length?'已提供北京官方口径参考方案；工伤档位和公积金比例请按本公司情况确认。':'缴费方案尚未初始化，请完成 Payroll 数据库迁移。';
    if(button)button.textContent=custom?'设置方案':'确认方案';
  }
  function renderPager(total){
    const target=root.querySelector('[data-internal-pager="payroll"]'),pages=Math.max(1,Math.ceil(total/state.pageSize));
    state.page=Math.min(state.page,pages);
    target.innerHTML='<span>第 '+state.page+' / '+pages+' 页</span><div><button type="button" data-prev '+(state.page<=1?'disabled':'')+'>‹</button><button type="button" class="is-current">'+state.page+'</button><button type="button" data-next '+(state.page>=pages?'disabled':'')+'>›</button></div><select aria-label="每页数量"><option value="20" '+(state.pageSize===20?'selected':'')+'>20 条/页</option><option value="50" '+(state.pageSize===50?'selected':'')+'>50 条/页</option><option value="100" '+(state.pageSize===100?'selected':'')+'>100 条/页</option></select>';
    target.querySelector('[data-prev]').addEventListener('click',()=>{state.page--;render()});
    target.querySelector('[data-next]').addEventListener('click',()=>{state.page++;render()});
    target.querySelector('select').addEventListener('change',event=>{state.pageSize=Number(event.target.value)||20;state.page=1;render()});
    return {start:(state.page-1)*state.pageSize,end:state.page*state.pageSize};
  }
  function bindSort(){
    for(const button of root.querySelectorAll('[data-internal-sort="payroll"]')){
      const active=state.sort.key===button.dataset.key;
      button.classList.toggle('is-active',active);button.dataset.sortDirection=active?state.sort.direction:'';
      button.onclick=()=>{state.sort={key:button.dataset.key,direction:active&&state.sort.direction==='asc'?'desc':'asc'};render()};
    }
  }
  function syncBatchActions(items){
    const existing=new Set(items.map(item=>item.line.id));for(const id of state.selected)if(!existing.has(id))state.selected.delete(id);
    const rows=items.filter(item=>state.selected.has(item.line.id)),sameRun=rows.length>0&&rows.every(item=>item.run.id===rows[0].run.id),allEditable=sameRun&&rows.every(item=>editable(item.line)),allPayable=sameRun&&rows.every(item=>paymentReady(item.line));
    const bar=root.querySelector('[data-internal-payroll-batch]');bar.querySelector('strong').textContent=String(rows.length);
    bar.querySelector('[data-internal-payroll-select-all]').disabled=!state.selectable.length;
    bar.querySelector('[data-internal-payroll-clear]').disabled=rows.length===0;
    bar.querySelector('[data-internal-payroll-performance]').disabled=!allEditable;
    bar.querySelector('[data-internal-payroll-paid]').disabled=!allPayable;
  }
  function render(){
    renderSchemeState();
    const search=root.querySelector('[data-internal-payroll-search]').value.trim().toLowerCase(),status=root.querySelector('[data-internal-payroll-status]').value,month=root.querySelector('[data-internal-payroll-month]').value;
    let items=payrollRows().filter(item=>(!search||[item.line.person_display_name,item.line.person_reference_code,item.department,item.run.period,item.run.label].some(value=>String(value||'').toLowerCase().includes(search)))&&(status==='active'?item.line.status!=='voided':!status||item.line.status===status)&&(!month||item.run.period===month));
    items=sorted(items);root.querySelector('[data-internal-payroll-total]').textContent=String(items.length);
    const range=renderPager(items.length),body=root.querySelector('[data-internal-payroll-body]');
    if(state.loading)body.innerHTML=emptyRow('正在读取工资记录','请稍候。');
    else if(state.error)body.innerHTML=emptyRow('工资数据暂不可用',state.error);
    else if(!items.length)body.innerHTML=emptyRow('暂无工资记录','新建月度工资表后，可录入绩效、核对并确认实际发放。');
    else body.innerHTML=items.slice(range.start,range.end).map(item=>{
      const canSelect=selectable(item.line),selected=state.selected.has(item.line.id),missing=item.line.payment_account_missing?' '+tag('缺工资账户','warning'):'',action=editable(item.line)?'编辑':paymentReady(item.line)?'发放':item.line.status==='zero_pending'?'确认':'查看';
      return '<tr><td><input type="checkbox" data-select-owner-payroll="'+item.line.id+'" '+(canSelect?'':'disabled')+' '+(selected?'checked':'')+' aria-label="选择 '+safe(item.line.person_display_name)+'"></td><td>'+safe(item.run.period)+'</td><td><strong>'+safe(item.line.person_display_name)+'</strong></td><td class="is-number is-money">'+model.money(item.line.fixed_salary_minor)+'</td><td class="is-number is-money">'+model.money(item.line.performance_minor)+'</td><td class="is-number is-money">'+model.money(item.line.employee_contribution_minor)+'</td><td class="is-number is-money">'+model.money(item.line.net_payable_minor)+'</td><td>'+date(item.run.planned_pay_date)+'</td><td>'+tag(statusLabel(item.line.status),statusTone(item.line.status))+missing+'</td><td>'+safe(actorLabel(item.run))+'</td><td><span class="oms-v1__row-actions"><button type="button" data-owner-payroll-detail="'+item.line.id+'">详情</button><button type="button" data-owner-payroll-action="'+item.line.id+'">'+action+'</button></span></td></tr>';
    }).join('');
    state.selectable=items.filter(item=>selectable(item.line));
    for(const box of body.querySelectorAll('[data-select-owner-payroll]'))box.addEventListener('change',()=>{
      const item=items.find(value=>value.line.id===box.dataset.selectOwnerPayroll);if(!item)return;
      if(box.checked){for(const selected of [...state.selected]){const selectedItem=items.find(value=>value.line.id===selected);if(selectedItem&&selectedItem.run.id!==item.run.id)state.selected.delete(selected)}state.selected.add(item.line.id)}else state.selected.delete(item.line.id);
      render();
    });
    for(const button of body.querySelectorAll('[data-owner-payroll-detail]'))button.addEventListener('click',()=>{const item=payrollRows().find(value=>value.line.id===button.dataset.ownerPayrollDetail);if(item)openDetail(item.run,item.line)});
    for(const button of body.querySelectorAll('[data-owner-payroll-action]'))button.addEventListener('click',()=>{const item=payrollRows().find(value=>value.line.id===button.dataset.ownerPayrollAction);if(!item)return;if(editable(item.line))openEdit(item.run,item.line);else if(paymentReady(item.line))openPayment(item.run,item.line);else openDetail(item.run,item.line)});
    bindSort();syncBatchActions(items);
  }

  function schemeFacts(scheme){
    const rows=[...(scheme.components||[])].sort((left,right)=>componentOrder.indexOf(left.key)-componentOrder.indexOf(right.key)).map(component=>{const individualFixed=Number(component.individual_fixed_minor||0),employerFixed=Number(component.employer_fixed_minor||0),fixed=(individualFixed||employerFixed)?'；固定额 个人 '+model.money(individualFixed)+' / 单位 '+model.money(employerFixed):'';return '<div><dt>'+safe(components[component.key]||component.key)+'</dt><dd>个人 '+percentInput(component.individual_rate_bps)+'% · 单位 '+percentInput(component.employer_rate_bps)+'%'+fixed+'</dd></div>'}).join('');
    const action=scheme.scheme_kind==='standard'?'以此创建本公司方案':'编辑本公司方案';
    return '<article class="oms-v1__scheme-item"><header><div><strong>'+safe(scheme.title)+'</strong><span>'+safe(scheme.region_name||scheme.region_code||'未设置地区')+' · '+safe(scheme.effective_month)+'</span></div>'+tag(scheme.scheme_kind==='standard'?'官方参考':'本公司方案')+'</header><dl class="oms-v1__record-facts">'+rows+'</dl><footer><small>来源核验：'+safe(date(scheme.authority_verified_at))+' · revision '+safe(scheme.revision)+'</small><button type="button" data-owner-scheme-edit="'+safe(scheme.id)+'">'+action+'</button></footer></article>';
  }
  function openSchemes(){
    const body=state.schemes.length?'<div class="oms-v1__scheme-guidance"><strong>先用参考方案，再按公司实际调整</strong><span>北京参考值可直接用于体验。工伤单位费率按最低行业档 0.2%、住房公积金按最低可选比例 5% 预填；正式发薪前应由工资管理员核对。</span></div><section class="oms-v1__scheme-list">'+state.schemes.map(schemeFacts).join('')+'</section>':'<div class="oms-v1__empty"><strong>缴费方案尚未初始化</strong><span>Payroll 数据库缺少北京参考方案迁移；完成服务升级后，本页面会出现可直接确认和调整的默认方案。</span></div>';
    show({kicker:'工资设置',title:'五险一金缴费方案',dismissOnly:true,body});
    for(const button of root.querySelectorAll('[data-owner-scheme-edit]'))button.addEventListener('click',()=>{const scheme=state.schemes.find(item=>item.id===button.dataset.ownerSchemeEdit);if(scheme)openSchemeEditor(scheme)});
  }
  function schemeEditorRows(scheme){
    const byKey=new Map((scheme.components||[]).map(item=>[item.key,item]));
    return componentOrder.map(key=>{const item=byKey.get(key);if(!item)return '';const locked=key==='maternity'?' readonly':'';return '<div class="oms-v1__scheme-rate-row"><strong>'+safe(components[key])+'</strong><label><span>个人比例</span><span class="oms-v1__input-suffix"><input name="'+key+'_individual_rate" inputmode="decimal" required value="'+percentInput(item.individual_rate_bps)+'"'+locked+'><i>%</i></span></label><label><span>单位比例</span><span class="oms-v1__input-suffix"><input name="'+key+'_employer_rate" inputmode="decimal" required value="'+percentInput(item.employer_rate_bps)+'"'+locked+'><i>%</i></span></label><label><span>个人固定额</span><span class="oms-v1__input-prefix"><i>¥</i><input name="'+key+'_individual_fixed" inputmode="decimal" required value="'+minorInput(item.individual_fixed_minor||0)+'"'+locked+'></span></label><label><span>单位固定额</span><span class="oms-v1__input-prefix"><i>¥</i><input name="'+key+'_employer_fixed" inputmode="decimal" required value="'+minorInput(item.employer_fixed_minor||0)+'"'+locked+'></span></label></div>'}).join('');
  }
  function openSchemeEditor(scheme){
    const social=(scheme.components||[]).find(item=>item.base_kind==='social'),housing=(scheme.components||[]).find(item=>item.base_kind==='housing'),custom=scheme.scheme_kind==='custom';
    const source=custom?state.schemes.find(item=>item.id===scheme.source_scheme_id&&item.scheme_kind==='standard'):scheme;
    if(!source){show({kicker:'工资设置',title:'无法编辑方案',dismissOnly:true,body:'<div class="oms-v1__empty"><strong>标准来源不可用</strong><span>当前自定义方案缺少可追溯的标准来源，请联系管理员恢复来源 revision。</span></div>'});return}
    show({kicker:'工资设置',title:custom?'编辑本公司缴费方案':'确认北京参考方案',submit:custom?'保存新版本':'保存本公司方案',enabled:true,body:'<form class="oms-v1__form-grid">'+field('方案名称 *','<input name="title" required maxlength="160" value="'+safe(custom?scheme.title:'本公司北京五险一金方案')+'">')+field('生效月份 *','<input name="effective_month" type="month" required value="'+safe(custom?scheme.effective_month:currentMonth())+'">')+field('社保基数下限（元） *','<input name="social_floor" inputmode="decimal" required value="'+minorInput(social?.base_floor_minor||0)+'">')+field('社保基数上限（元） *','<input name="social_cap" inputmode="decimal" required value="'+minorInput(social?.base_cap_minor||0)+'">')+field('公积金基数下限（元） *','<input name="housing_floor" inputmode="decimal" required value="'+minorInput(housing?.base_floor_minor||0)+'">')+field('公积金基数上限（元） *','<input name="housing_cap" inputmode="decimal" required value="'+minorInput(housing?.base_cap_minor||0)+'">')+'<div class="oms-v1__scheme-rate-editor is-wide"><div class="oms-v1__scheme-rate-head"><strong>缴费比例与固定额</strong><span>金额均按人民币／月</span></div>'+schemeEditorRows(scheme)+'</div>'+field('设置说明 *','<textarea name="reason" required maxlength="500">'+safe(custom?'核对并调整本公司实际缴费方案':'基于北京官方公开口径建立；已核对本公司工伤档位和公积金比例')+'</textarea>',true)+'<label class="oms-v1__inline-check is-wide"><input name="confirmed" type="checkbox" value="yes" required><span>我已核对工伤行业档位、公积金比例和生效月份；这份方案用于后续工资计算。</span></label><div class="oms-v1__form-note is-wide"><strong>北京参考：</strong>养老 16%／8%，医疗（含生育）9.8%／2%＋个人固定 3 元，失业 0.5%／0.5%；工伤按最低档 0.2% 预填，公积金按最低可选 5% 预填。后两项必须结合本公司实际确认。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{
      if(formData.get('confirmed')!=='yes')throw new Error('请先确认本公司实际适用的工伤档位和公积金比例');
      const floors={social:nonnegativeMinor(formData.get('social_floor'),'社保基数下限'),housing:nonnegativeMinor(formData.get('housing_floor'),'公积金基数下限')},caps={social:nonnegativeMinor(formData.get('social_cap'),'社保基数上限'),housing:nonnegativeMinor(formData.get('housing_cap'),'公积金基数上限')};
      if(caps.social<floors.social||caps.housing<floors.housing)throw new Error('缴费基数上限不能低于下限');
      const next=(scheme.components||[]).map(item=>({...item,individual_rate_bps:percentBPS(formData.get(item.key+'_individual_rate')),employer_rate_bps:percentBPS(formData.get(item.key+'_employer_rate')),individual_fixed_minor:nonnegativeMinor(formData.get(item.key+'_individual_fixed'),components[item.key]+'个人固定额'),employer_fixed_minor:nonnegativeMinor(formData.get(item.key+'_employer_fixed'),components[item.key]+'单位固定额'),base_floor_minor:floors[item.base_kind],base_cap_minor:caps[item.base_kind]}));
      const payload={command_id:uuid(),title:String(formData.get('title')||'').trim(),effective_month:String(formData.get('effective_month')||''),components:next,reason:String(formData.get('reason')||'').trim()};
      if(custom)await put('/api/v1/contribution-schemes/custom/'+encodeURIComponent(scheme.id),{...payload,expected_revision:scheme.revision});else await post('/api/v1/contribution-schemes/custom',{...payload,source_scheme_id:source.id,source_scheme_revision:source.revision});
      close();await load();openSchemes();
    }});
  }
  function eligibleMembers(run=null){
    const compensationIDs=new Set(state.compensations.map(item=>item.person_id)),existing=new Set((run?.lines||[]).map(item=>item.person_id));
    return state.people.filter(item=>item.status==='active'&&compensationIDs.has(item.id)&&!existing.has(item.id));
  }
  function memberChoices(members,name='person_id'){return '<div class="oms-v1__sync-member-list">'+members.map(member=>'<label><input type="checkbox" name="'+name+'" value="'+safe(member.id)+'" checked><span><strong>'+safe(member.display_name)+'</strong><small>'+safe([member.department,member.position_title].filter(Boolean).join(' · ')||member.reference_code||'已配置薪酬档案')+'</small></span></label>').join('')+'</div>'}
  function openCreate(){
    if(!state.schemes.length){openSchemes();return}
    const members=eligibleMembers();if(!members.length){show({kicker:'工资管理',title:'新建工资表',dismissOnly:true,body:'<div class="oms-v1__empty"><strong>没有可生成工资的在职成员</strong><span>请先在成员管理中补齐成员和薪酬档案。</span></div>'});return}
    const scheme=state.schemes[0];
    show({kicker:'工资管理',title:'新建月度工资表',submit:'生成工资表',enabled:true,body:'<form class="oms-v1__form-grid">'+field('工资月份 *','<input name="period" type="month" required value="'+currentMonth()+'">')+field('计划发薪日期 *','<input name="planned_pay_date" type="date" required value="'+today()+'">')+field('工资表名称 *','<input name="label" required maxlength="200" value="'+currentMonth()+' 月工资">')+field('缴费方案 *','<select name="scheme_id">'+state.schemes.map(item=>option(item.id,item.title+' · '+item.effective_month,scheme.id)).join('')+'</select>')+'<div class="is-wide">'+memberChoices(members)+'</div><div class="oms-v1__form-note is-wide">生成时会冻结成员、薪酬、工资账户与缴费方案快照；后续档案变化必须手动同步，不能静默改写本期工资。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{
      const selected=formData.getAll('person_id').map(String);if(!selected.length)throw new Error('请至少选择一名成员');
      const selectedScheme=state.schemes.find(item=>item.id===String(formData.get('scheme_id')));if(!selectedScheme)throw new Error('请选择有效的缴费方案');
      await post('/api/v1/payroll-periods',{command_id:uuid(),period:String(formData.get('period')),label:String(formData.get('label')||'').trim(),planned_pay_date:String(formData.get('planned_pay_date')),contribution_scheme_id:selectedScheme.id,contribution_scheme_revision:selectedScheme.revision,person_ids:selected});
      close();await load();
    }});
  }
  function openSync(){
    const runs=state.runs.filter(run=>run.run_kind==='regular'&&run.status!=='voided').sort((a,b)=>String(b.period).localeCompare(String(a.period)));
    if(!runs.length){show({kicker:'工资管理',title:'同步新增成员',dismissOnly:true,body:'<div class="oms-v1__empty"><strong>暂无可同步的工资表</strong><span>请先新建月度工资表。</span></div>'});return}
    const initial=runs[0];
    show({kicker:'工资管理',title:'同步新增成员',submit:'加入工资表',enabled:true,body:'<form class="oms-v1__form-grid">'+field('工资表 *','<select name="run_id">'+runs.map(run=>option(run.id,run.label+' · '+run.period,initial.id)).join('')+'</select>',true)+'<div class="oms-v1__sync-member-panel is-wide" data-owner-payroll-sync-panel></div>'+field('同步原因 *','<input name="reason" required maxlength="500" value="工资表生成后补入成员">',true)+'<div class="oms-v1__form-note is-wide">只追加当前在职、已配置薪酬且尚未进入本工资表的成员。任何成功或结果未知的付款都会阻止同步。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{
      const run=state.runs.find(item=>item.id===String(formData.get('run_id')));if(!run)throw new Error('请选择有效工资表');
      const ids=formData.getAll('person_id').map(String);if(!ids.length)throw new Error('请至少选择一名新增成员');
      await post(runPath(run)+'/members:sync',{command_id:uuid(),expected_revision:run.revision,person_ids:ids,reason:String(formData.get('reason')||'').trim()});close();await load();
    }});
    const form=root.querySelector('[data-oms-v1-dialog-body] form'),select=form.elements.run_id,panel=form.querySelector('[data-owner-payroll-sync-panel]'),submit=root.querySelector('[data-oms-v1-dialog-submit]');
    const update=()=>{const run=state.runs.find(item=>item.id===select.value),members=run?eligibleMembers(run):[];panel.innerHTML=members.length?memberChoices(members):'<div class="oms-v1__empty"><strong>没有待同步成员</strong><span>当前在职且已配置薪酬的成员均已进入该工资表。</span></div>';submit.disabled=!members.length};
    select.addEventListener('change',update);update();
  }

  function adjustmentsFromForm(formData,reason){
    const supplement=model.decimalMinor(formData.get('supplement')),recovery=model.decimalMinor(formData.get('recovery'));if(supplement===null||recovery===null||supplement<0n||recovery<0n)throw new Error('调整金额必须大于等于 0，且最多保留两位小数');
    const values=[];if(supplement>0n)values.push({id:uuid(),kind:'manual',amount_minor:minorJSON(supplement),reason});if(recovery>0n)values.push({id:uuid(),kind:'manual',amount_minor:minorJSON(-recovery),reason});return values;
  }
  function adjustmentTotals(line){let supplement=0n,recovery=0n;for(const item of line.adjustments||[]){const amount=BigInt(String(item.amount_minor||0));if(amount>0n)supplement+=amount;else recovery-=amount}return {supplement,recovery}}
  function openEdit(run,line){
    if(!editable(line)){openDetail(run,line);return}
    const totals=adjustmentTotals(line);
    show({kicker:'工资明细',title:line.person_display_name+' · '+run.period,submit:'保存并重算',enabled:true,body:'<form class="oms-v1__form-grid">'+field('当月绩效（元） *','<input name="performance" required inputmode="decimal" value="'+safe(minorInput(line.performance_minor))+'">')+field('个人所得税（元） *','<input name="tax" required inputmode="decimal" value="'+safe(minorInput(line.tax_minor))+'">')+field('补贴／补发（元）','<input name="supplement" inputmode="decimal" value="'+safe(minorInput(totals.supplement))+'">')+field('扣减／扣回（元）','<input name="recovery" inputmode="decimal" value="'+safe(minorInput(totals.recovery))+'">')+field('个税结果说明','<input name="tax_note" maxlength="500" value="'+safe(line.tax_source_note||'')+'" placeholder="例如：外部个税计算结果">',true)+field('修改原因 *','<input name="reason" required maxlength="500" value="工资明细核对调整">',true)+'<div class="oms-v1__form-note is-wide">固定工资、缴费基数和比例来自已冻结快照；本页只录入绩效、个税结果与本期调整，并以整数分重新计算。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{
      const performance=model.decimalMinor(formData.get('performance')),tax=model.decimalMinor(formData.get('tax')),reason=String(formData.get('reason')||'').trim();if(performance===null||tax===null||performance<0n||tax<0n)throw new Error('绩效和个税必须大于等于 0，且最多保留两位小数');
      await put(linePath(run,line),{command_id:uuid(),expected_run_revision:run.revision,expected_line_revision:line.revision,performance_minor:minorJSON(performance),tax_minor:minorJSON(tax),tax_source_kind:'manual',tax_source_note:String(formData.get('tax_note')||'').trim(),adjustments:adjustmentsFromForm(formData,reason),reason});close();await load();
    }});
  }
  function contributionFacts(line){return (line.contributions||[]).map(item=>'<div><dt>'+safe(components[item.key]||item.key)+'</dt><dd>个人 '+model.money(item.individual_minor)+' · 单位 '+model.money(item.employer_minor)+'</dd></div>').join('')||'<div><dt>缴费明细</dt><dd>无</dd></div>'}
  function actionButtons(run,line){
    const values=[];
    if(run.status!=='voided'&&line.status==='review_pending')values.push('<button type="button" data-owner-sync-compensation>同步薪酬档案</button>','<button type="button" data-owner-sync-payment>同步工资账户</button>');
    if(run.run_kind==='regular'&&['paid','no_payment_required','recovery_pending'].includes(line.status))values.push('<button type="button" data-owner-adjustment>新建补发／扣回</button>');
    if(run.status!=='voided')values.push('<button type="button" data-owner-void-run>作废工资表</button>');
    return values.length?'<div class="oms-v1__record-inline-actions">'+values.join('')+'</div>':'';
  }
  function openDetail(run,line){
    const completeReview=run.status==='review_pending',confirmZero=line.status==='zero_pending',enabled=completeReview||confirmZero,submit=confirmZero?'确认无需发放':'完成核对';
    const remain=remaining(line),body='<form><section class="oms-v1__record-detail"><div class="oms-v1__record-status"><span>'+safe(run.label)+'</span><strong>'+safe(statusLabel(line.status))+'</strong></div><div class="oms-v1__record-metrics"><article><span>应发合计</span><strong>'+model.money(line.gross_minor)+'</strong></article><article><span>个人五险一金</span><strong>'+model.money(line.employee_contribution_minor)+'</strong></article><article><span>个人所得税</span><strong>'+model.money(line.tax_minor)+'</strong></article><article class="is-positive"><span>实发金额</span><strong>'+model.money(line.net_payable_minor)+'</strong><small>已核销 '+model.money(line.allocated_paid_minor)+'</small></article></div><h3 class="oms-v1__record-subtitle">缴费明细</h3><dl class="oms-v1__record-facts">'+contributionFacts(line)+'</dl><h3 class="oms-v1__record-subtitle">冻结快照与发放</h3><dl class="oms-v1__record-facts"><div><dt>固定工资</dt><dd>'+model.money(line.fixed_salary_minor)+'</dd></div><div><dt>绩效</dt><dd>'+model.money(line.performance_minor)+'</dd></div><div><dt>计划发薪日</dt><dd>'+date(run.planned_pay_date)+'</dd></div><div><dt>工资账户</dt><dd>'+safe(line.masked_payment_account||'未配置')+'</dd></div><div><dt>开户行</dt><dd>'+safe(line.payment_bank_name||'—')+'</dd></div><div><dt>尚待核销</dt><dd>'+model.money(remain)+'</dd></div><div><dt>超额未核销</dt><dd>'+model.money(line.overage_minor)+'</dd></div><div><dt>成员快照 revision</dt><dd>'+safe(line.person_source_revision)+'</dd></div><div><dt>工资行 revision</dt><dd>'+safe(line.revision)+'</dd></div><div><dt>更新时间</dt><dd>'+dateTime(line.updated_at)+'</dd></div></dl></section>'+actionButtons(run,line)+(enabled?field(confirmZero?'确认原因 *':'核对说明 *','<input name="reason" required maxlength="500" value="'+(confirmZero?'确认本期实发为零':'工资明细已完成核对')+'">',true):'')+formStatus('')+'</form>';
    show({kicker:'工资明细',title:line.person_display_name+' · '+run.period,submit,enabled,onSubmit:enabled?async formData=>{const reason=String(formData.get('reason')||'').trim();if(confirmZero)await post(linePath(run,line)+'/confirm-no-payment',{command_id:uuid(),expected_run_revision:run.revision,expected_line_revision:line.revision,reason});else await post(runPath(run)+'/review',{command_id:uuid(),expected_revision:run.revision,reason});close();await load()}:null,dismissOnly:!enabled,body});
    const dialog=root.querySelector('[data-oms-v1-dialog-body]');
    dialog.querySelector('[data-owner-void-run]')?.addEventListener('click',()=>openVoid(run));
    dialog.querySelector('[data-owner-sync-compensation]')?.addEventListener('click',()=>openProfileSync(run,line,'compensation'));
    dialog.querySelector('[data-owner-sync-payment]')?.addEventListener('click',()=>openProfileSync(run,line,'payment-profile'));
    dialog.querySelector('[data-owner-adjustment]')?.addEventListener('click',()=>openAdjustment(run,line));
  }
  function openVoid(run){show({kicker:'工资管理',title:'作废 '+run.label,submit:'确认作废',enabled:true,body:'<form>'+field('作废原因 *','<textarea name="reason" required maxlength="500" placeholder="请说明作废原因"></textarea>',true)+'<div class="oms-v1__form-note">作废保留工资表和全部 revision；已有成功或结果未知的付款时，服务会拒绝作废。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{await post(runPath(run)+'/void',{command_id:uuid(),expected_revision:run.revision,reason:String(formData.get('reason')||'').trim()});close();await load()}})}
  async function openProfileSync(run,line,kind){
    const title=kind==='compensation'?'同步薪酬档案':'同步工资账户',preview=await api(linePath(run,line)+'/'+kind+'-sync-preview'),before=preview.before,after=preview.after;
    show({kicker:'工资快照',title,submit:'确认同步',enabled:true,body:'<form><section class="oms-v1__record-detail"><dl class="oms-v1__record-facts"><div><dt>成员</dt><dd>'+safe(line.person_display_name)+'</dd></div><div><dt>来源档案 revision</dt><dd>'+safe(preview.source_profile_revision)+'</dd></div>'+(kind==='compensation'?'<div><dt>固定工资</dt><dd>'+model.money(before.fixed_salary_minor)+' → '+model.money(after.fixed_salary_minor)+'</dd></div><div><dt>社保基数</dt><dd>'+model.money(before.social_insurance_base_minor)+' → '+model.money(after.social_insurance_base_minor)+'</dd></div>':'<div><dt>工资账户</dt><dd>'+safe(before.masked_payment_account||'未配置')+' → '+safe(after.masked_payment_account||'未配置')+'</dd></div><div><dt>开户行</dt><dd>'+safe(before.payment_bank_name||'—')+' → '+safe(after.payment_bank_name||'—')+'</dd></div>')+'</dl></section>'+field('同步原因 *','<input name="reason" required maxlength="500" value="'+title+'">',true)+formStatus('')+'</form>',onSubmit:async formData=>{const suffix=kind==='compensation'?'compensation:sync':'payment-profile:sync';await post(linePath(run,line)+'/'+suffix,{command_id:uuid(),expected_run_revision:preview.payroll_run_revision,expected_line_revision:preview.payroll_line_revision,preview_hash:preview.preview_hash,reason:String(formData.get('reason')||'').trim()});close();await load()}});
  }
  function openAdjustment(run,line){
    show({kicker:'工资调整',title:line.person_display_name+' · 补发／扣回',submit:'建立调整批次',enabled:true,body:'<form class="oms-v1__form-grid">'+field('调整方向 *','<select name="direction">'+option('supplement','补发','supplement')+option('recovery','扣回','supplement')+'</select>')+field('金额（元） *','<input name="amount" required inputmode="decimal" placeholder="0.00">')+field('所属月份 *','<input name="period" type="month" required value="'+currentMonth()+'">')+field('计划处理日期 *','<input name="planned_pay_date" type="date" required value="'+today()+'">')+field('批次名称 *','<input name="label" required maxlength="200" value="'+safe(line.person_display_name)+'工资调整">',true)+field('调整原因 *','<textarea name="reason" required maxlength="500"></textarea>',true)+'<div class="oms-v1__form-note is-wide">调整批次引用当前工资行及 revision，不改写原工资事实；补发可继续进入发放，扣回保持待扣回状态。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{const amount=model.decimalMinor(formData.get('amount'));if(amount===null||amount<=0n)throw new Error('调整金额必须大于 0，且最多保留两位小数');const reason=String(formData.get('reason')||'').trim();await post('/api/v1/payroll-adjustment-runs',{command_id:uuid(),period:String(formData.get('period')),label:String(formData.get('label')||'').trim(),planned_pay_date:String(formData.get('planned_pay_date')),items:[{source_payroll_line_id:line.id,source_payroll_line_revision:line.revision,direction:String(formData.get('direction')),amount_minor:minorJSON(amount),reason}]});close();await load()}});
  }

  function paymentFields(line,prefix=''){const amount=minorInput(remaining(line));return field('实际发放金额（元） *','<input name="'+prefix+'amount" required inputmode="decimal" value="'+safe(amount)+'">')+field('超额确认','<label class="oms-v1__inline-check"><input type="checkbox" name="'+prefix+'overage_confirmed"> 金额超过未发部分时仍确认</label>')+field('超额原因','<input name="'+prefix+'overage_reason" maxlength="500" placeholder="仅超额时必填">',true)}
  function paymentInput(formData,run,line,prefix='',occurredOn=today(),note=''){
    const amount=model.decimalMinor(formData.get(prefix+'amount'));if(amount===null||amount<=0n)throw new Error(line.person_display_name+' 的发放金额必须大于 0');const overage=amount>remaining(line),confirmed=formData.get(prefix+'overage_confirmed')==='on',reason=String(formData.get(prefix+'overage_reason')||'').trim();if(overage&&(!confirmed||!reason))throw new Error(line.person_display_name+' 的超额发放必须勾选确认并填写原因');return {command_id:uuid(),expected_run_revision:run.revision,expected_line_revision:line.revision,actual_amount_minor:minorJSON(amount),occurred_on:occurredOn,note,overage_confirmed:confirmed,overage_reason:reason}}
  function openPayment(run,line){
    if(line.payment_account_missing){show({kicker:'工资发放',title:line.person_display_name,dismissOnly:true,body:'<div class="oms-v1__empty"><strong>缺少工资账户快照</strong><span>请先在成员管理中维护账户，再从工资详情执行“同步工资账户”。</span></div>'});return}
    show({kicker:'工资发放',title:line.person_display_name+' · '+run.period,submit:'确认实际发放',enabled:true,body:'<form class="oms-v1__form-grid">'+paymentFields(line)+field('实际发薪日期 *','<input name="occurred_on" type="date" required value="'+today()+'">')+field('备注','<textarea name="note" maxlength="500" placeholder="选填"></textarea>',true)+'<div class="oms-v1__form-note is-wide">确认后由 Payroll 向 Money 提交实际资金与核销；结果未知时不会自动重试，请刷新后核对。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{await post(linePath(run,line)+'/payments',paymentInput(formData,run,line,'',String(formData.get('occurred_on')),String(formData.get('note')||'').trim()));close();await load()}});
  }
  function selectedRows(){const rows=payrollRows().filter(item=>state.selected.has(item.line.id));if(!rows.length)throw new Error('请先选择工资记录');if(!rows.every(item=>item.run.id===rows[0].run.id))throw new Error('批量操作只能选择同一工资表');return rows}
  function openBatchImport(){
    let rows;try{rows=selectedRows()}catch(error){show({kicker:'工资管理',title:'批量录入',dismissOnly:true,body:'<div class="oms-v1__empty"><strong>无法批量录入</strong><span>'+safe(error.message)+'</span></div>'});return}if(!rows.every(item=>editable(item.line)))return;
    const run=rows[0].run;
    show({kicker:'工资管理',title:'批量录入绩效与个税',submit:'生成预览',enabled:true,body:'<form><div class="oms-v1__batch-form">'+rows.map(item=>'<div class="oms-v1__batch-payroll-row"><span class="oms-v1__batch-member"><strong>'+safe(item.line.person_display_name)+'</strong><small>'+safe(run.period)+' · line revision '+safe(item.line.revision)+'</small></span><div class="oms-v1__batch-inputs"><label><span>当月绩效（元）</span><input name="performance_'+item.line.id+'" aria-label="'+safe(item.line.person_display_name)+'当月绩效（元）" inputmode="decimal" required value="'+safe(minorInput(item.line.performance_minor))+'"></label><label><span>个人所得税（元）</span><input name="tax_'+item.line.id+'" aria-label="'+safe(item.line.person_display_name)+'个人所得税（元）" inputmode="decimal" required value="'+safe(minorInput(item.line.tax_minor))+'"></label></div></div>').join('')+'</div>'+field('个税来源说明','<input name="tax_note" maxlength="500" value="批量导入核对">',true)+'<div class="oms-v1__form-note">先生成服务端预览；所有行匹配、revision 和金额校验通过后才允许一次性确认，不会部分改写工资表。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{
      const values=rows.map(item=>{const performance=model.decimalMinor(formData.get('performance_'+item.line.id)),tax=model.decimalMinor(formData.get('tax_'+item.line.id));if(performance===null||tax===null||performance<0n||tax<0n)throw new Error(item.line.person_display_name+' 的绩效或个税金额不正确');return {person_id:item.line.person_id,expected_line_revision:item.line.revision,performance_minor:minorJSON(performance),tax_minor:minorJSON(tax),tax_source_note:String(formData.get('tax_note')||'').trim(),adjustments:item.line.adjustments||[]}});
      const preview=await post(runPath(run)+'/imports:preview',{expected_run_revision:run.revision,rows:values});openImportPreview(run,preview);
    }});
  }
  function openImportPreview(run,preview){
    const rows=(preview.items||[]).map(item=>'<tr><td>'+safe(item.after?.person_display_name||item.before?.person_display_name||item.member_reference_code||item.provided_person_id||'—')+'</td><td>'+safe(item.match_status)+'</td><td>'+safe((item.changed_fields||[]).join('、')||'无变化')+'</td><td>'+safe(item.error_message||'—')+'</td></tr>').join('');
    show({kicker:'工资导入预览',title:preview.confirmable?'预览校验通过':'预览存在问题',submit:'确认写入',enabled:Boolean(preview.confirmable),dismissOnly:!preview.confirmable,body:'<form><div class="oms-v1__table-wrap"><table class="oms-v1__table"><thead><tr><th>成员</th><th>匹配</th><th>变更字段</th><th>问题</th></tr></thead><tbody>'+rows+'</tbody></table></div>'+field('提交原因 *','<input name="reason" required maxlength="500" value="批量录入绩效与个税">',true)+'<div class="oms-v1__form-note">预览有效期有限；确认时会再次核对工资表 revision，过期或被他人修改后必须重新预览。</div>'+formStatus('')+'</form>',onSubmit:preview.confirmable?async formData=>{await post(runPath(run)+'/imports:commit',{command_id:uuid(),preview_id:preview.id,preview_hash:preview.preview_hash,expected_run_revision:preview.payroll_run_revision,reason:String(formData.get('reason')||'').trim()});state.selected.clear();close();await load()}:null});
  }
  function openBatchPayment(){
    let rows;try{rows=selectedRows()}catch(error){show({kicker:'工资发放',title:'批量确认',dismissOnly:true,body:'<div class="oms-v1__empty"><strong>无法批量发放</strong><span>'+safe(error.message)+'</span></div>'});return}if(!rows.every(item=>paymentReady(item.line)))return;
    const missing=rows.filter(item=>item.line.payment_account_missing);if(missing.length){show({kicker:'工资发放',title:'暂不能批量确认',dismissOnly:true,body:'<div class="oms-v1__empty"><strong>所选成员缺少工资账户</strong><span>'+safe(missing.map(item=>item.line.person_display_name).join('、'))+'：请先在成员管理中维护账户，再从工资详情执行“同步工资账户”。</span></div>'});return}
    const run=rows[0].run;
    show({kicker:'工资发放',title:'批量确认 '+rows.length+' 人',submit:'提交批量发放',enabled:true,body:'<form class="oms-v1__form-grid">'+field('实际发薪日期 *','<input name="occurred_on" type="date" required value="'+today()+'">')+field('统一备注','<input name="note" maxlength="500" value="工资实际发放确认">')+'<div class="oms-v1__batch-payment is-wide">'+rows.map(item=>'<section><header><strong>'+safe(item.line.person_display_name)+'</strong><span>未发 '+model.money(remaining(item.line))+'</span></header>'+paymentFields(item.line,item.line.id+'_')+'</section>').join('')+'</div><div class="oms-v1__form-note is-wide">批量操作逐人提交 Money；成功项不会因其他成员失败而回滚，结果未知项也不会自动重试。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{
      const occurredOn=String(formData.get('occurred_on')),note=String(formData.get('note')||'').trim(),items=rows.map(item=>({line_id:item.line.id,input:paymentInput(formData,run,item.line,item.line.id+'_',occurredOn,note)})),result=await post(runPath(run)+'/payments:batch',{items});
      state.selected.clear();await load();const summary=result.summary||{};show({kicker:'工资发放',title:'批量处理结果',dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-metrics"><article class="is-positive"><span>成功</span><strong>'+safe(summary.succeeded||0)+'</strong></article><article><span>失败</span><strong>'+safe(summary.failed||0)+'</strong></article><article><span>结果未知</span><strong>'+safe(summary.unknown||0)+'</strong></article><article><span>未执行</span><strong>'+safe(summary.unexecuted||0)+'</strong></article></div></section><div class="oms-v1__form-note">成功记录已经保留；失败或结果未知记录请按列表状态核对后续办。</div>'});
    }});
  }

  // Member and compensation facts can change while this single-page workbench stays open.
  // Refresh the owners before deciding who is eligible instead of using the boot-time snapshot.
  root.querySelector('[data-internal-new-payroll]').addEventListener('click',async()=>{await load();openCreate()});
  root.querySelector('[data-internal-payroll-sync]').addEventListener('click',async()=>{await load();openSync()});
  root.querySelector('[data-internal-benefit-scheme]').addEventListener('click',openSchemes);
  root.querySelector('[data-internal-payroll-performance]').addEventListener('click',openBatchImport);
  root.querySelector('[data-internal-payroll-paid]').addEventListener('click',openBatchPayment);
  root.querySelector('[data-internal-payroll-select-all]').addEventListener('click',()=>{state.selected.clear();const first=state.selectable[0];if(first)for(const item of state.selectable)if(item.run.id===first.run.id)state.selected.add(item.line.id);render()});
  root.querySelector('[data-internal-payroll-clear]').addEventListener('click',()=>{state.selected.clear();render()});
  root.querySelector('[data-internal-payroll-search]').addEventListener('input',()=>{state.page=1;state.selected.clear();render()});
  for(const control of root.querySelectorAll('[data-internal-payroll-status],[data-internal-payroll-month]'))control.addEventListener('change',()=>{state.page=1;state.selected.clear();render()});

  void load();
})();
