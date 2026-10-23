(()=>{
  'use strict';

  const root=document.querySelector('[data-oms-v1]'),model=globalThis.OMSInternalModel;
  if(!root||!model)return;

  // This branch deliberately keeps the candidate workflow local-only.  If it is
  // accidentally included in an authenticated/public build, fail closed before
  // reading or writing browser business data.
  if(document.body.dataset.demoPreview!=='true'){
    const page=root.querySelector('[data-oms-v1-page="internal"]');
    if(page)page.innerHTML='<header class="oms-v1__page-head"><div><h1 data-oms-v1-page-title>公司内部管理</h1><p>本功能仍处于本地验证阶段</p></div></header><div class="oms-v1__empty"><strong>尚未接入正式业务服务</strong><span>请使用本地预览验证交互；当前页面不会在正式环境保存成员、工资或财务申请。</span></div>';
    return;
  }

  const safe=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));
  const uuid=()=>globalThis.crypto?.randomUUID?.()||'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g,value=>{const random=Math.random()*16|0,part=value==='x'?random:(random&3|8);return part.toString(16)});
  const today=()=>{const now=new Date();return new Date(now.getTime()-now.getTimezoneOffset()*60000).toISOString().slice(0,10)};
  const now=()=>new Date().toISOString();
  const currentMonth=()=>today().slice(0,7);
  const date=value=>value?String(value).slice(0,10).replaceAll('-','/'): '—';
  const dateTime=value=>{if(!value)return '—';const parsed=new Date(value);return Number.isNaN(parsed.getTime())?String(value):new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}).format(parsed)};
  const clone=value=>JSON.parse(JSON.stringify(value));
  const percentToBps=value=>{const match=String(value??'').trim().match(/^(\d{1,3})(?:\.(\d{1,2}))?$/);if(!match)return null;const bps=Number(match[1])*100+Number((match[2]||'').padEnd(2,'0'));return bps<=10000?bps:null};
  const bpsToPercent=value=>(Number(value||0)/100).toFixed(2);
  const minorInput=value=>{try{const raw=BigInt(String(value||0)),negative=raw<0n,absolute=negative?-raw:raw;return (negative?'-':'')+String(absolute/100n)+'.'+String(absolute%100n).padStart(2,'0')}catch{return '0.00'}};
  const compare=(a,b)=>{if(typeof a==='bigint'||typeof b==='bigint'){const left=typeof a==='bigint'?a:BigInt(a),right=typeof b==='bigint'?b:BigInt(b);return left===right?0:left>right?1:-1}return typeof a==='number'&&typeof b==='number'?a-b:String(a??'').localeCompare(String(b??''),'zh-CN',{numeric:true,sensitivity:'base'})};

  const defaultScheme=()=>({
    title:'默认演示方案（请按参保地核对）',confirmed:false,effective_from:today(),updated_at:now(),operator:'本地操作人',
    rates:{
      pension:{employee_bps:800,employer_bps:1600},medical:{employee_bps:200,employer_bps:980},
      unemployment:{employee_bps:50,employer_bps:50},injury:{employee_bps:0,employer_bps:20},
      maternity:{employee_bps:0,employer_bps:80},housing:{employee_bps:1200,employer_bps:1200},
    },
  });
  const emptyStore=()=>({version:1,members:[],payroll_periods:[],finance_records:[],benefit_scheme:defaultScheme()});
  const storeKey='oms-v1.company-internal.v1.'+(root.dataset.tenantId||'local');
  function readStore(){
    try{
      const value=JSON.parse(localStorage.getItem(storeKey)||'null');
      if(value?.version===1&&Array.isArray(value.members)&&Array.isArray(value.payroll_periods)&&Array.isArray(value.finance_records))return {...emptyStore(),...value};
    }catch{}
    return emptyStore();
  }
  let data=readStore();
  const view={pages:{members:1,payroll:1,finance:1},pageSize:{members:20,payroll:20,finance:20},sorts:{members:{key:'updated_at',direction:'desc'},payroll:{key:'pay_date',direction:'desc'},finance:{key:'created_at',direction:'desc'}},selectedPayroll:new Set(),selectedFinance:new Set()};
  const persist=()=>localStorage.setItem(storeKey,JSON.stringify(data));
  const show=detail=>root.dispatchEvent(new CustomEvent('oms-v1:show-dialog',{detail}));
  const close=()=>root.dispatchEvent(new CustomEvent('oms-v1:close-dialog'));
  const formStatus=copy=>'<div class="oms-v1__form-status is-wide" data-oms-v1-form-status>'+safe(copy||'')+'</div>';
  const field=(label,control,wide=false)=>'<label class="oms-v1__form-field'+(wide?' is-wide':'')+'"><span>'+safe(label)+'</span>'+control+'</label>';
  const option=(value,label,current)=>'<option value="'+safe(value)+'" '+(String(value)===String(current)?'selected':'')+'>'+safe(label)+'</option>';
  const tag=(label,tone='')=>'<span class="oms-v1__internal-tag '+(tone?'is-'+tone:'')+'">'+safe(label)+'</span>';
  const selectedValues=(set,records)=>records.filter(record=>set.has(record.id));

  function sorted(items,key,accessor){const sort=view.sorts[key];return [...items].sort((left,right)=>{const result=compare(accessor(left,sort.key),accessor(right,sort.key));return sort.direction==='asc'?result:-result})}
  function pager(key,total,render){
    const target=root.querySelector('[data-internal-pager="'+key+'"]'),size=view.pageSize[key],pages=Math.max(1,Math.ceil(total/size)),current=Math.min(view.pages[key],pages);
    view.pages[key]=current;
    target.innerHTML='<span>第 '+current+' / '+pages+' 页</span><div><button type="button" data-prev '+(current<=1?'disabled':'')+'>‹</button><button type="button" class="is-current">'+current+'</button><button type="button" data-next '+(current>=pages?'disabled':'')+'>›</button></div><select aria-label="每页数量"><option value="20" '+(size===20?'selected':'')+'>20 条/页</option><option value="50" '+(size===50?'selected':'')+'>50 条/页</option><option value="100" '+(size===100?'selected':'')+'>100 条/页</option></select>';
    target.querySelector('[data-prev]').addEventListener('click',()=>{view.pages[key]=Math.max(1,current-1);render()});
    target.querySelector('[data-next]').addEventListener('click',()=>{view.pages[key]=Math.min(pages,current+1);render()});
    target.querySelector('select').addEventListener('change',event=>{view.pageSize[key]=Number(event.target.value)||20;view.pages[key]=1;render()});
    return {start:(current-1)*size,end:current*size};
  }
  function bindSort(scope,render){
    for(const button of root.querySelectorAll('[data-internal-sort="'+scope+'"]')){
      const sort=view.sorts[scope],active=sort.key===button.dataset.key;
      button.classList.toggle('is-active',active);button.dataset.sortDirection=active?sort.direction:'';
      button.onclick=()=>{view.sorts[scope]={key:button.dataset.key,direction:active&&sort.direction==='asc'?'desc':'asc'};render()};
    }
  }
  function statusLabel(status){return {active:'在职',inactive:'停用',planned:'待发放',confirm:'待确认',partial:'部分发放',paid:'已发放',draft:'草稿',pending:'待支付'}[status]||status}
  function financeStatusLabel(status){return {draft:'草稿',pending:'待支付',paid:'已支付'}[status]||status}
  function statusTone(status){return status==='paid'||status==='active'?'positive':status==='confirm'||status==='pending'||status==='partial'?'warning':status==='inactive'?'muted':''}
  function emptyRow(columns,title,copy){return '<tr><td colspan="'+columns+'"><div class="oms-v1__empty"><strong>'+safe(title)+'</strong><span>'+safe(copy)+'</span></div></td></tr>'}

  function memberForm(member){
    const value=member||{name:'',department:'',position:'',joined_on:today(),fixed_salary_minor:'0',social_base_minor:'0',housing_base_minor:'0',status:'active',operator:'本地操作人'};
    return '<form class="oms-v1__form-grid">'+
      field('姓名 *','<input name="name" required maxlength="80" value="'+safe(value.name)+'" placeholder="请输入成员姓名">')+
      field('状态','<select name="status">'+option('active','在职',value.status)+option('inactive','停用',value.status)+'</select>')+
      field('部门','<input name="department" maxlength="80" value="'+safe(value.department)+'" placeholder="选填">')+
      field('岗位','<input name="position" maxlength="80" value="'+safe(value.position)+'" placeholder="选填">')+
      field('入职日期','<input name="joined_on" type="date" value="'+safe(value.joined_on||today())+'">')+
      field('固定工资（元） *','<input name="fixed_salary" inputmode="decimal" required value="'+safe(minorInput(value.fixed_salary_minor))+'">')+
      field('社保缴费基数（元） *','<input name="social_base" inputmode="decimal" required value="'+safe(minorInput(value.social_base_minor))+'">')+
      field('公积金缴费基数（元） *','<input name="housing_base" inputmode="decimal" required value="'+safe(minorInput(value.housing_base_minor))+'">')+
      field('操作人 *','<input name="operator" required maxlength="80" value="'+safe(value.operator||'本地操作人')+'">')+
      '<div class="oms-v1__form-note is-wide">绩效不写入成员固定档案，在每个月的工资表中单独录入。金额按分保存并显示两位小数。</div>'+formStatus('')+'</form>';
  }
  function openMemberEditor(member=null){
    show({kicker:'公司成员',title:member?'编辑成员':'新增成员',submit:member?'保存修改':'保存成员',enabled:true,body:memberForm(member),onSubmit:async formData=>{
      const fixed=model.decimalMinor(formData.get('fixed_salary')),social=model.decimalMinor(formData.get('social_base')),housing=model.decimalMinor(formData.get('housing_base'));
      if(fixed===null||social===null||housing===null||BigInt(fixed)<0n||BigInt(social)<0n||BigInt(housing)<0n)throw new Error('工资和缴费基数必须是大于等于 0、最多两位小数的金额');
      const record={id:member?.id||uuid(),name:String(formData.get('name')||'').trim(),department:String(formData.get('department')||'').trim(),position:String(formData.get('position')||'').trim(),joined_on:String(formData.get('joined_on')||today()),fixed_salary_minor:fixed,social_base_minor:social,housing_base_minor:housing,status:String(formData.get('status')||'active'),operator:String(formData.get('operator')||'').trim(),created_at:member?.created_at||now(),updated_at:now()};
      if(member)data.members=data.members.map(item=>item.id===member.id?record:item);else data.members.push(record);
      persist();close();renderMembers();
    }});
  }
  function openMemberDetail(member){
    show({kicker:'成员详情',title:member.name,dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-status"><span>当前状态</span><strong>'+safe(statusLabel(member.status))+'</strong></div><div class="oms-v1__record-metrics"><article><span>固定工资</span><strong>'+model.money(member.fixed_salary_minor)+'</strong></article><article><span>社保基数</span><strong>'+model.money(member.social_base_minor)+'</strong></article><article><span>公积金基数</span><strong>'+model.money(member.housing_base_minor)+'</strong></article></div><dl class="oms-v1__record-facts"><div><dt>部门</dt><dd>'+safe(member.department||'—')+'</dd></div><div><dt>岗位</dt><dd>'+safe(member.position||'—')+'</dd></div><div><dt>入职日期</dt><dd>'+date(member.joined_on)+'</dd></div><div><dt>最近操作人</dt><dd>'+safe(member.operator||'—')+'</dd></div><div><dt>更新时间</dt><dd>'+dateTime(member.updated_at)+'</dd></div></dl></section>'});
  }
  function renderMembers(){
    const search=root.querySelector('[data-internal-member-search]').value.trim().toLowerCase(),status=root.querySelector('[data-internal-member-status]').value;
    let items=data.members.filter(item=>(!search||[item.name,item.department,item.position].some(value=>String(value||'').toLowerCase().includes(search)))&&(!status||item.status===status));
    items=sorted(items,'members',(item,key)=>key==='fixed_salary'?BigInt(item.fixed_salary_minor):item[key]);
    const range=pager('members',items.length,renderMembers),body=root.querySelector('[data-internal-member-body]');
    root.querySelector('[data-internal-member-total]').textContent=String(items.length);
    body.innerHTML=items.length?items.slice(range.start,range.end).map(item=>'<tr><td><strong title="'+safe(item.name)+'">'+safe(item.name)+'</strong></td><td>'+safe(item.department||'—')+'</td><td>'+safe(item.position||'—')+'</td><td class="is-number is-money">'+model.money(item.fixed_salary_minor)+'</td><td class="is-number is-money">'+model.money(item.social_base_minor)+'</td><td class="is-number is-money">'+model.money(item.housing_base_minor)+'</td><td>'+tag(statusLabel(item.status),statusTone(item.status))+'</td><td>'+safe(item.operator||'—')+'</td><td><span class="oms-v1__row-actions"><button type="button" data-detail="'+item.id+'">详情</button><button type="button" data-edit="'+item.id+'">编辑</button></span></td></tr>').join(''):emptyRow(9,'暂无公司成员','新增成员后可生成工资表。');
    for(const button of body.querySelectorAll('[data-detail]'))button.addEventListener('click',()=>openMemberDetail(data.members.find(item=>item.id===button.dataset.detail)));
    for(const button of body.querySelectorAll('[data-edit]'))button.addEventListener('click',()=>openMemberEditor(data.members.find(item=>item.id===button.dataset.edit)));
    bindSort('members',renderMembers);
  }

  function schemeForm(){
    const scheme=data.benefit_scheme,rows=model.components.map(([key,label])=>'<div class="oms-v1__rate-row"><strong>'+safe(label)+'</strong><label>个人 %<input name="'+key+'_employee" inputmode="decimal" required value="'+bpsToPercent(scheme.rates[key]?.employee_bps)+'"></label><label>单位 %<input name="'+key+'_employer" inputmode="decimal" required value="'+bpsToPercent(scheme.rates[key]?.employer_bps)+'"></label></div>').join('');
    return '<form>'+field('方案名称 *','<input name="title" required maxlength="100" value="'+safe(scheme.title)+'">')+field('生效日期','<input name="effective_from" type="date" value="'+safe(scheme.effective_from||today())+'">')+'<div class="oms-v1__rate-table">'+rows+'</div>'+field('操作人 *','<input name="operator" required maxlength="80" value="'+safe(scheme.operator||'本地操作人')+'">')+'<div class="oms-v1__form-note">比例会按万分比精确保存；每项金额以分为单位四舍五入。不同参保地、年度和人员类别的比例可能不同，请按实际政策核对后再保存。已经生成的工资表保留原比例快照，不随本方案静默变化。</div>'+formStatus('')+'</form>';
  }
  function openScheme(){show({kicker:'工资设置',title:'五险一金缴费方案',submit:'确认方案',enabled:true,body:schemeForm(),onSubmit:async formData=>{const rates={};for(const [key] of model.components){const employee=percentToBps(formData.get(key+'_employee')),employer=percentToBps(formData.get(key+'_employer'));if(employee===null||employer===null)throw new Error('比例必须是 0 到 100 之间、最多两位小数的百分比');rates[key]={employee_bps:employee,employer_bps:employer}}data.benefit_scheme={title:String(formData.get('title')||'').trim(),confirmed:true,effective_from:String(formData.get('effective_from')||today()),operator:String(formData.get('operator')||'').trim(),updated_at:now(),rates};persist();close();renderPayroll()}})}

  function createPayrollPeriod(formData){
    if(!data.benefit_scheme.confirmed)throw new Error('请先核对并确认五险一金缴费方案');
    const month=String(formData.get('month')||''),payDate=String(formData.get('pay_date')||''),operator=String(formData.get('operator')||'').trim();
    if(data.payroll_periods.some(period=>period.month===month))throw new Error('该月份工资表已经存在');
    const members=data.members.filter(member=>member.status==='active');if(!members.length)throw new Error('当前没有在职成员，无法生成工资表');
    const period={id:uuid(),month,pay_date:payDate,operator,created_at:now(),updated_at:now(),scheme_snapshot:clone(data.benefit_scheme),lines:members.map(member=>({id:uuid(),member_id:member.id,member_name:member.name,department:member.department,member_snapshot:clone(member),...model.payrollLine(member,'0',data.benefit_scheme),paid_at:'',payment_date:'',paid_operator:'',payment_remark:''}))};
    data.payroll_periods.push(period);persist();close();renderPayroll();
  }
  function openPayrollCreate(){
    if(!data.benefit_scheme.confirmed){openScheme();return}
    show({kicker:'工资管理',title:'新建月度工资表',submit:'生成工资表',enabled:true,body:'<form class="oms-v1__form-grid">'+field('工资月份 *','<input name="month" type="month" required value="'+currentMonth()+'">')+field('计划发薪日期 *','<input name="pay_date" type="date" required value="'+today()+'">')+field('操作人 *','<input name="operator" required maxlength="80" value="本地操作人">')+'<div class="oms-v1__form-note is-wide">系统会读取当前在职成员的固定工资和缴费基数，并保存成员与比例快照。到达发薪日只转为“待确认”，不会在没有银行回执时自动登记为已发放。</div>'+formStatus('')+'</form>',onSubmit:async formData=>createPayrollPeriod(formData)});
  }
  function payrollRows(){const rows=[];for(const period of data.payroll_periods)for(const line of period.lines)rows.push({period,line,status:model.lineStatus(line,period.pay_date,today())});return rows}
  function recalculateLine(period,line,performanceMinor){Object.assign(line,model.payrollLine(line.member_snapshot,performanceMinor,period.scheme_snapshot));period.updated_at=now()}
  function openPayrollEdit(period,line){
    if(line.paid_at){openPayrollDetail(period,line);return}
    show({kicker:'工资明细',title:line.member_name+' · '+period.month,submit:'保存设置',enabled:true,body:'<form class="oms-v1__form-grid">'+field('计划发薪日期 *','<input name="pay_date" type="date" required value="'+safe(period.pay_date)+'">')+field('当月绩效（元） *','<input name="performance" required inputmode="decimal" value="'+safe(minorInput(line.performance_minor))+'">')+field('操作人 *','<input name="operator" required maxlength="80" value="'+safe(period.operator||'本地操作人')+'">')+'<div class="oms-v1__form-note is-wide">修改计划发薪日期会同步到本月全部成员；修改绩效只影响当前成员。当前计算仅扣除个人五险一金，尚未计算个人所得税。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{const performance=model.decimalMinor(formData.get('performance'));if(performance===null||BigInt(performance)<0n)throw new Error('绩效必须是大于等于 0、最多两位小数的金额');period.pay_date=String(formData.get('pay_date')||period.pay_date);period.operator=String(formData.get('operator')||'').trim();recalculateLine(period,line,performance);persist();close();renderPayroll()}});
  }
  function contributionFacts(line,side){return model.components.map(([key,label])=>'<div><dt>'+safe(label)+'</dt><dd>'+model.money(line[side+'_contributions'][key])+'</dd></div>').join('')}
  function openPayrollDetail(period,line){const status=model.lineStatus(line,period.pay_date,today());show({kicker:'工资明细',title:line.member_name+' · '+period.month,dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-status"><span>当前状态</span><strong>'+safe(statusLabel(status))+'</strong></div><div class="oms-v1__record-metrics"><article><span>固定工资</span><strong>'+model.money(line.fixed_salary_minor)+'</strong></article><article><span>当月绩效</span><strong>'+model.money(line.performance_minor)+'</strong></article><article><span>个人五险一金</span><strong>'+model.money(line.employee_contribution_minor)+'</strong></article><article class="is-positive"><span>本期发放金额</span><strong>'+model.money(line.payable_minor)+'</strong><small>未计算个税</small></article></div><h3 class="oms-v1__record-subtitle">个人承担</h3><dl class="oms-v1__record-facts">'+contributionFacts(line,'employee')+'</dl><h3 class="oms-v1__record-subtitle">单位承担</h3><dl class="oms-v1__record-facts">'+contributionFacts(line,'employer')+'</dl><dl class="oms-v1__record-facts"><div><dt>计划发薪日</dt><dd>'+date(period.pay_date)+'</dd></div><div><dt>实际确认日期</dt><dd>'+date(line.payment_date)+'</dd></div><div><dt>操作人</dt><dd>'+safe(line.paid_operator||period.operator||'—')+'</dd></div><div><dt>确认时间</dt><dd>'+dateTime(line.paid_at)+'</dd></div></dl></section>'})}
  function openPerformanceBatch(){const rows=selectedValues(view.selectedPayroll,payrollRows().map(item=>({id:item.line.id,...item}))).filter(item=>!item.line.paid_at);if(!rows.length)return;show({kicker:'工资管理',title:'批量录入绩效',submit:'保存绩效',enabled:true,body:'<form><div class="oms-v1__batch-form">'+rows.map(item=>'<label><span><strong>'+safe(item.line.member_name)+'</strong><small>'+safe(item.period.month)+'</small></span><input name="performance_'+item.line.id+'" inputmode="decimal" required value="'+safe(minorInput(item.line.performance_minor))+'"></label>').join('')+'</div>'+formStatus('')+'</form>',onSubmit:async formData=>{for(const item of rows){const value=model.decimalMinor(formData.get('performance_'+item.line.id));if(value===null||BigInt(value)<0n)throw new Error(item.line.member_name+' 的绩效金额不正确')}for(const item of rows)recalculateLine(item.period,item.line,model.decimalMinor(formData.get('performance_'+item.line.id)));persist();view.selectedPayroll.clear();close();renderPayroll()}})}
  function openPayrollPaidBatch(){const rows=payrollRows().filter(item=>view.selectedPayroll.has(item.line.id)&&!item.line.paid_at);if(!rows.length)return;show({kicker:'工资管理',title:'确认已发放 '+rows.length+' 人',submit:'确认已发放',enabled:true,body:'<form class="oms-v1__form-grid">'+field('实际发薪日期 *','<input name="payment_date" type="date" required value="'+today()+'">')+field('操作人 *','<input name="operator" required maxlength="80" value="本地操作人">')+field('备注','<textarea name="remark" maxlength="500" placeholder="选填"></textarea>',true)+'<div class="oms-v1__form-note is-wide">此操作表示工资已经在系统外实际发放；确认后保留操作人和时间，不会发起银行付款。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{const at=now();for(const item of rows){item.line.paid_at=at;item.line.payment_date=String(formData.get('payment_date')||today());item.line.paid_operator=String(formData.get('operator')||'').trim();item.line.payment_remark=String(formData.get('remark')||'').trim();item.period.updated_at=at}persist();view.selectedPayroll.clear();close();renderPayroll()}})}
  function renderPayroll(){
    const search=root.querySelector('[data-internal-payroll-search]').value.trim().toLowerCase(),statusFilter=root.querySelector('[data-internal-payroll-status]').value,month=root.querySelector('[data-internal-payroll-month]').value;
    let items=payrollRows().filter(item=>(!search||[item.line.member_name,item.line.department,item.period.month,item.period.operator].some(value=>String(value||'').toLowerCase().includes(search)))&&(!statusFilter||item.status===statusFilter)&&(!month||item.period.month===month));
    items=sorted(items,'payroll',(item,key)=>key==='payable'?BigInt(item.line.payable_minor):key==='member'?item.line.member_name:key==='status'?item.status:item.period[key]||item.line[key]);
    const range=pager('payroll',items.length,renderPayroll),body=root.querySelector('[data-internal-payroll-body]'),schemeState=root.querySelector('[data-internal-scheme-state]');
    schemeState.classList.toggle('is-warning',!data.benefit_scheme.confirmed);schemeState.querySelector('p').textContent=data.benefit_scheme.confirmed?'当前方案：'+data.benefit_scheme.title+'，生效日期 '+date(data.benefit_scheme.effective_from)+'。':'缴费比例尚未确认；生成工资表前请先核对参保地政策。';
    root.querySelector('[data-internal-payroll-total]').textContent=String(items.length);
    body.innerHTML=items.length?items.slice(range.start,range.end).map(item=>'<tr><td><input type="checkbox" data-select-payroll="'+item.line.id+'" '+(item.line.paid_at?'disabled':'')+' '+(view.selectedPayroll.has(item.line.id)?'checked':'')+' aria-label="选择 '+safe(item.line.member_name)+'"></td><td>'+safe(item.period.month)+'</td><td><strong>'+safe(item.line.member_name)+'</strong></td><td class="is-number is-money">'+model.money(item.line.fixed_salary_minor)+'</td><td class="is-number is-money">'+model.money(item.line.performance_minor)+'</td><td class="is-number is-money">'+model.money(item.line.employee_contribution_minor)+'</td><td class="is-number is-money">'+model.money(item.line.payable_minor)+'</td><td>'+date(item.period.pay_date)+'</td><td>'+tag(statusLabel(item.status),statusTone(item.status))+'</td><td>'+safe(item.line.paid_operator||item.period.operator||'—')+'</td><td><span class="oms-v1__row-actions"><button type="button" data-payroll-detail="'+item.line.id+'">详情</button><button type="button" data-payroll-edit="'+item.line.id+'">'+(item.line.paid_at?'查看':'编辑')+'</button></span></td></tr>').join(''):emptyRow(11,'暂无工资记录','新增月度工资表后，可录入绩效并批量确认发放。');
    for(const box of body.querySelectorAll('[data-select-payroll]'))box.addEventListener('change',()=>{box.checked?view.selectedPayroll.add(box.dataset.selectPayroll):view.selectedPayroll.delete(box.dataset.selectPayroll);syncPayrollActions()});
    for(const button of body.querySelectorAll('[data-payroll-detail],[data-payroll-edit]'))button.addEventListener('click',()=>{const id=button.dataset.payrollDetail||button.dataset.payrollEdit,item=payrollRows().find(value=>value.line.id===id);button.dataset.payrollDetail?openPayrollDetail(item.period,item.line):openPayrollEdit(item.period,item.line)});
    bindSort('payroll',renderPayroll);syncPayrollActions();
  }
  function syncPayrollActions(){const count=view.selectedPayroll.size,bar=root.querySelector('[data-internal-payroll-batch]');bar.hidden=count===0;bar.querySelector('strong').textContent=String(count)}

  function financeForm(record){const value=record||{type:'reimbursement',title:'',applicant:'',payee:'',amount_minor:'0',business_date:today(),expected_pay_date:today(),status:'draft',operator:'本地操作人',remark:''};return '<form class="oms-v1__form-grid">'+field('业务类型 *','<select name="type">'+option('reimbursement','报销',value.type)+option('payment_request','付款申请',value.type)+'</select>')+field('事项名称 *','<input name="title" required maxlength="120" value="'+safe(value.title)+'" placeholder="例如：差旅费报销">')+field('申请人 *','<input name="applicant" required maxlength="80" value="'+safe(value.applicant)+'">')+field('收款方 *','<input name="payee" required maxlength="120" value="'+safe(value.payee)+'">')+field('申请金额（元） *','<input name="amount" required inputmode="decimal" value="'+safe(minorInput(value.amount_minor))+'">')+field('发生日期 *','<input name="business_date" type="date" required value="'+safe(value.business_date||today())+'">')+field('计划支付日期','<input name="expected_pay_date" type="date" value="'+safe(value.expected_pay_date||today())+'">')+field('保存状态','<select name="status">'+option('draft','草稿',value.status)+option('pending','待支付',value.status)+'</select>')+field('操作人 *','<input name="operator" required maxlength="80" value="'+safe(value.operator||'本地操作人')+'">')+field('备注','<textarea name="remark" maxlength="1000" placeholder="选填">'+safe(value.remark)+'</textarea>',true)+'<div class="oms-v1__form-note is-wide">本期不做审批流；保存为“待支付”后，实际付款完成再回到系统确认。已支付记录不允许静默覆盖。</div>'+formStatus('')+'</form>'}
  function openFinanceEditor(record=null){show({kicker:'财务管理',title:record?'编辑申请':'新增财务申请',submit:record?'保存修改':'保存申请',enabled:true,body:financeForm(record),onSubmit:async formData=>{const amount=model.decimalMinor(formData.get('amount'));if(amount===null||BigInt(amount)<=0n)throw new Error('申请金额必须大于 0，且最多保留两位小数');const item={id:record?.id||uuid(),type:String(formData.get('type')||'reimbursement'),title:String(formData.get('title')||'').trim(),applicant:String(formData.get('applicant')||'').trim(),payee:String(formData.get('payee')||'').trim(),amount_minor:amount,business_date:String(formData.get('business_date')||today()),expected_pay_date:String(formData.get('expected_pay_date')||''),status:String(formData.get('status')||'draft'),operator:String(formData.get('operator')||'').trim(),remark:String(formData.get('remark')||'').trim(),created_at:record?.created_at||now(),updated_at:now(),paid_at:record?.paid_at||'',payment_date:record?.payment_date||'',paid_operator:record?.paid_operator||'',payment_remark:record?.payment_remark||''};if(record)data.finance_records=data.finance_records.map(value=>value.id===record.id?item:value);else data.finance_records.push(item);persist();close();renderFinance()}})}
  function financeTypeLabel(type){return type==='payment_request'?'付款申请':'报销'}
  function openFinanceDetail(record){show({kicker:'财务申请',title:record.title,dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-status"><span>'+safe(financeTypeLabel(record.type))+'</span><strong>'+safe(financeStatusLabel(record.status))+'</strong></div><div class="oms-v1__record-metrics"><article class="is-primary"><span>申请金额</span><strong>'+model.money(record.amount_minor)+'</strong></article></div><dl class="oms-v1__record-facts"><div><dt>申请人</dt><dd>'+safe(record.applicant)+'</dd></div><div><dt>收款方</dt><dd>'+safe(record.payee)+'</dd></div><div><dt>发生日期</dt><dd>'+date(record.business_date)+'</dd></div><div><dt>计划支付日期</dt><dd>'+date(record.expected_pay_date)+'</dd></div><div><dt>实际支付日期</dt><dd>'+date(record.payment_date)+'</dd></div><div><dt>操作人</dt><dd>'+safe(record.paid_operator||record.operator||'—')+'</dd></div><div><dt>确认时间</dt><dd>'+dateTime(record.paid_at)+'</dd></div></dl>'+(record.remark?'<div class="oms-v1__record-remark"><span>备注</span><p>'+safe(record.remark)+'</p></div>':'')+'</section>'})}
  function openFinancePaidBatch(){const records=data.finance_records.filter(record=>view.selectedFinance.has(record.id)&&record.status==='pending');if(!records.length)return;show({kicker:'财务管理',title:'确认已支付 '+records.length+' 笔',submit:'确认已支付',enabled:true,body:'<form class="oms-v1__form-grid">'+field('实际支付日期 *','<input name="payment_date" type="date" required value="'+today()+'">')+field('操作人 *','<input name="operator" required maxlength="80" value="本地操作人">')+field('支付备注','<textarea name="remark" maxlength="500" placeholder="选填"></textarea>',true)+'<div class="oms-v1__form-note is-wide">此操作只确认已经发生的外部付款，不在系统内执行转账。</div>'+formStatus('')+'</form>',onSubmit:async formData=>{const at=now();for(const record of records){record.status='paid';record.payment_date=String(formData.get('payment_date')||today());record.paid_at=at;record.paid_operator=String(formData.get('operator')||'').trim();record.payment_remark=String(formData.get('remark')||'').trim();record.updated_at=at}persist();view.selectedFinance.clear();close();renderFinance()}})}
  function renderFinance(){
    const search=root.querySelector('[data-internal-finance-search]').value.trim().toLowerCase(),type=root.querySelector('[data-internal-finance-type]').value,status=root.querySelector('[data-internal-finance-status]').value;
    let items=data.finance_records.filter(item=>(!search||[item.title,item.applicant,item.payee,item.remark,item.operator].some(value=>String(value||'').toLowerCase().includes(search)))&&(!type||item.type===type)&&(!status||item.status===status));
    items=sorted(items,'finance',(item,key)=>key==='amount'?BigInt(item.amount_minor):item[key]);const range=pager('finance',items.length,renderFinance),body=root.querySelector('[data-internal-finance-body]');root.querySelector('[data-internal-finance-total]').textContent=String(items.length);
    body.innerHTML=items.length?items.slice(range.start,range.end).map(item=>'<tr><td><input type="checkbox" data-select-finance="'+item.id+'" '+(item.status!=='pending'?'disabled':'')+' '+(view.selectedFinance.has(item.id)?'checked':'')+' aria-label="选择 '+safe(item.title)+'"></td><td>'+tag(financeTypeLabel(item.type))+'</td><td><strong title="'+safe(item.title)+'">'+safe(item.title)+'</strong></td><td>'+safe(item.applicant)+'</td><td>'+safe(item.payee)+'</td><td class="is-number is-money">'+model.money(item.amount_minor)+'</td><td>'+date(item.business_date)+'</td><td>'+tag(financeStatusLabel(item.status),statusTone(item.status))+'</td><td>'+safe(item.paid_operator||item.operator||'—')+'</td><td><span class="oms-v1__row-actions"><button type="button" data-finance-detail="'+item.id+'">详情</button>'+(item.status==='paid'?'':'<button type="button" data-finance-edit="'+item.id+'">编辑</button>')+'</span></td></tr>').join(''):emptyRow(10,'暂无财务申请','新增报销或付款申请后，可保存草稿并确认支付。');
    for(const box of body.querySelectorAll('[data-select-finance]'))box.addEventListener('change',()=>{box.checked?view.selectedFinance.add(box.dataset.selectFinance):view.selectedFinance.delete(box.dataset.selectFinance);syncFinanceActions()});
    for(const button of body.querySelectorAll('[data-finance-detail]'))button.addEventListener('click',()=>openFinanceDetail(data.finance_records.find(item=>item.id===button.dataset.financeDetail)));
    for(const button of body.querySelectorAll('[data-finance-edit]'))button.addEventListener('click',()=>openFinanceEditor(data.finance_records.find(item=>item.id===button.dataset.financeEdit)));
    bindSort('finance',renderFinance);syncFinanceActions();
  }
  function syncFinanceActions(){const count=view.selectedFinance.size,bar=root.querySelector('[data-internal-finance-batch]');bar.hidden=count===0;bar.querySelector('strong').textContent=String(count)}

  root.querySelector('[data-internal-new-member]').addEventListener('click',()=>openMemberEditor());
  root.querySelector('[data-internal-new-payroll]').addEventListener('click',openPayrollCreate);
  root.querySelector('[data-internal-benefit-scheme]').addEventListener('click',openScheme);
  root.querySelector('[data-internal-payroll-performance]').addEventListener('click',openPerformanceBatch);
  root.querySelector('[data-internal-payroll-paid]').addEventListener('click',openPayrollPaidBatch);
  root.querySelector('[data-internal-new-finance]').addEventListener('click',()=>openFinanceEditor());
  root.querySelector('[data-internal-finance-paid]').addEventListener('click',openFinancePaidBatch);
  for(const input of root.querySelectorAll('[data-internal-member-search],[data-internal-payroll-search],[data-internal-finance-search]'))input.addEventListener('input',()=>{const scope=input.hasAttribute('data-internal-member-search')?'members':input.hasAttribute('data-internal-payroll-search')?'payroll':'finance';view.pages[scope]=1;({members:renderMembers,payroll:renderPayroll,finance:renderFinance}[scope])()});
  for(const select of root.querySelectorAll('[data-internal-member-status]'))select.addEventListener('change',()=>{view.pages.members=1;renderMembers()});
  for(const select of root.querySelectorAll('[data-internal-payroll-status],[data-internal-payroll-month]'))select.addEventListener('change',()=>{view.pages.payroll=1;renderPayroll()});
  for(const select of root.querySelectorAll('[data-internal-finance-type],[data-internal-finance-status]'))select.addEventListener('change',()=>{view.pages.finance=1;renderFinance()});

  renderMembers();renderPayroll();renderFinance();
})();
