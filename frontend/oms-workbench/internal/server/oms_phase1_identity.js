(()=>{
  'use strict';

  const root=document.querySelector('[data-oms-v1]'),page=root?.querySelector('[data-oms-v1-page="account"]');
  if(!root||!page)return;
  const route=String(root.dataset.identityRoutePrefix||'').replace(/\/$/,'');
  const administrator=root.dataset.administrator==='true';
  const ownerAPI=globalThis.OMSPhase1OwnerAPI;
  const ownerPeople=Boolean(ownerAPI?.app?.('people-groups'));
  const safe=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));
  const show=detail=>root.dispatchEvent(new CustomEvent('oms-v1:show-dialog',{detail}));
  const close=()=>root.dispatchEvent(new CustomEvent('oms-v1:close-dialog'));
  const notice=page.querySelector('[data-identity-notice] p');
  const railOrganization=root.querySelector('[data-identity-rail-organization]');
  let session=null,tenant=null,organizations=[],subjects=[],people=[],peopleAvailable=false,loadPromise=null,organizationRecoveryShown=false;
  const filters={search:'',role:'all',status:'all',link:'all'};
  const uuid=()=>globalThis.crypto?.randomUUID?.()||'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g,char=>{const value=Math.random()*16|0;return (char==='x'?value:(value&3|8)).toString(16)});
  const wait=milliseconds=>new Promise(resolve=>globalThis.setTimeout(resolve,milliseconds));

  function csrfCookie(){const match=document.cookie.match(/(?:^|;\s*)oms_iam_csrf=([A-Za-z0-9_-]{16,256})(?:;|$)/);return match?match[1]:''}
  async function csrf(){let value=csrfCookie();if(value)return value;const response=await fetch(route+'/api/v1/csrf',{credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}});if(!response.ok)throw new Error('安全令牌暂时不可用');const body=await response.json();value=csrfCookie();if(!value||body.csrf_token!==value)throw new Error('安全令牌校验失败');return value}
  async function api(path,options={}){
    const headers={Accept:'application/json',...(options.headers||{})},method=String(options.method||'GET').toUpperCase();
    if(method!=='GET'&&method!=='HEAD')headers['X-CSRF-Token']=await csrf();
    const response=await fetch(route+path,{...options,method,headers,credentials:'same-origin',cache:'no-store',redirect:'error'});
    if(response.status===204)return null;
    const body=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(body?.error?.message||body.detail||body.title||'账号服务暂时不可用');
    return body;
  }
  const post=(path,value)=>api(path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(value)});
  function dateTime(value){if(!value)return '—';const parsed=new Date(value);return Number.isNaN(parsed.getTime())?String(value):new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}).format(parsed)}
  function setText(selector,value){const node=page.querySelector(selector);if(node)node.textContent=value}
  function identifierKind(value){value=String(value||'');if(/^1[3-9]\d{9}$/.test(value))return '手机号';if(/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value))return '邮箱';return '用户名'}

  function closeRailOrganization(){
    const trigger=railOrganization?.querySelector('[data-identity-rail-organization-trigger]'),menu=railOrganization?.querySelector('[data-identity-rail-organization-menu]');
    if(trigger)trigger.setAttribute('aria-expanded','false');
    if(menu)menu.hidden=true;
  }
  function renderRailOrganization(activeOrganizations){
    const current=activeOrganizations.find(item=>item.current)||null,label=railOrganization?.querySelector('[data-identity-rail-tenant-name]'),trigger=railOrganization?.querySelector('[data-identity-rail-organization-trigger]'),menu=railOrganization?.querySelector('[data-identity-rail-organization-menu]'),options=railOrganization?.querySelector('[data-identity-rail-organization-options]'),chevron=railOrganization?.querySelector('[data-identity-rail-organization-chevron]'),ready=Boolean(current&&tenant?.id),switchable=ready&&activeOrganizations.length>1;
    if(label)label.textContent=current?.tenant_name||tenant?.name||'组织信息未就绪';
    if(trigger){trigger.disabled=!switchable;trigger.title=!ready?'组织信息未就绪，请重新加载':switchable?'切换组织':'当前账号仅加入一个组织';trigger.setAttribute('aria-expanded','false')}
    if(chevron)chevron.hidden=!switchable;
    if(menu)menu.hidden=true;
    if(options)options.innerHTML=activeOrganizations.map(item=>'<button class="oms-v1__rail-organization-option" type="button" role="option" aria-selected="'+String(Boolean(item.current))+'" data-identity-rail-organization-option="'+safe(item.tenant_id)+'"><span><strong>'+safe(item.tenant_name)+'</strong><small>'+(item.administrator?'管理员':'普通用户')+'</small></span><i aria-hidden="true">'+(item.current?'✓':'→')+'</i></button>').join('');
  }

  function render(){
    const principal=session?.principal||{};
    setText('[data-identity-tenant-name]',tenant?.name||'—');
    const switcher=page.querySelector('[data-identity-organization-switch]'),organizationSelect=page.querySelector('[data-identity-organization-select]'),activeOrganizations=organizations.filter(item=>item.status==='active');
    if(switcher&&organizationSelect){switcher.hidden=activeOrganizations.length<2;organizationSelect.innerHTML=activeOrganizations.map(item=>'<option value="'+safe(item.tenant_id)+'" '+(item.current?'selected':'')+'>'+safe(item.tenant_name)+(item.administrator?' · 管理员':'')+'</option>').join('')}
    renderRailOrganization(activeOrganizations);
    setText('[data-identity-display-name]',principal.display_name||root.dataset.displayName||'—');
    setText('[data-identity-username]',principal.username||'—');
    setText('[data-identity-account-status]','启用');
    setText('[data-identity-account-role]',principal.administrator?'管理员':'普通用户');
    setText('[data-identity-subject-count]',String(subjects.length));
    setText('[data-identity-administrator-count]',String(subjects.filter(item=>item.administrator).length));
    setText('[data-identity-active-count]',String(subjects.filter(item=>item.status==='active').length));
    const body=page.querySelector('[data-identity-subject-body]');
    if(!body)return;
    const personBySubject=new Map(people.filter(person=>person.iam_subject_id).map(person=>[String(person.iam_subject_id),person]));
    const visible=subjects.filter(item=>{
      const person=personBySubject.get(String(item.id));
      const text=(String(item.display_name||'')+' '+String(item.username||'')+' '+String(person?.display_name||'')).toLowerCase();
      return (!filters.search||text.includes(filters.search))
        &&(filters.role==='all'||(filters.role==='administrator')===Boolean(item.administrator))
        &&(filters.status==='all'||item.status===filters.status)
        &&(filters.link==='all'||(filters.link==='linked')===Boolean(person));
    });
    setText('[data-identity-visible-count]',String(visible.length));
    const registrationInvite=item=>item.status==='pending'&&/^invite-[0-9a-f]{32}$/.test(String(item.username||''));
    const statusLabel=item=>item.status==='active'?'已启用':registrationInvite(item)?'等待注册':item.status==='pending'?'待激活':'已停用';
    const actions=(item,person)=>{
      const buttons=[];
      if(item.status!=='pending')buttons.push('<button type="button" data-identity-edit-account="'+safe(item.id)+'">编辑账号姓名</button>');
      if(person&&peopleAvailable)buttons.push('<button type="button" data-identity-edit-member="'+safe(person.id)+'">编辑成员资料</button>');
      if(item.status==='pending')buttons.push('<button type="button" data-identity-reissue="'+safe(item.id)+'">'+(registrationInvite(item)?'生成新邀请链接':'生成新激活链接')+'</button>');
      else if(item.status==='active')buttons.push('<button type="button" data-identity-reset="'+safe(item.id)+'">重置密码</button>');
      if(item.status!=='pending'&&item.id!==principal.subject_id)buttons.push('<button type="button" data-identity-administrator="'+safe(item.id)+'" data-next-administrator="'+(!item.administrator)+'">'+(item.administrator?'改为普通用户':'设为管理员')+'</button>');
      if(item.status!=='pending'&&item.id!==principal.subject_id)buttons.push('<button type="button" class="'+(item.status==='active'?'is-danger':'')+'" data-identity-status="'+safe(item.id)+'" data-next-status="'+(item.status==='active'?'disabled':'active')+'">'+(item.status==='active'?'停用账号':'启用账号')+'</button>');
      return buttons.length?'<details class="oms-v1__identity-actions"><summary aria-label="管理 '+safe(item.display_name)+'">•••</summary><div>'+buttons.join('')+'</div></details>':'<span class="oms-v1__identity-no-action">—</span>';
    };
    body.innerHTML=visible.length?visible.map(item=>{
      const person=personBySubject.get(String(item.id)),current=item.id===principal.subject_id,isInvitation=registrationInvite(item);
      const personMeta=person?[person.department,person.position_title].filter(Boolean).join(' · '):'';
      const association=isInvitation?'<span class="oms-v1__identity-unlinked">注册后关联</span><small>成员关系不随邀请预设</small>':person?'<strong>'+safe(person.display_name)+'</strong>'+(personMeta?'<small>'+safe(personMeta)+'</small>':'<small>已关联成员</small>'):peopleAvailable?'<span class="oms-v1__identity-unlinked">未关联</span><small>不影响登录</small>':'<span class="oms-v1__identity-unlinked">暂不可用</span><small>成员服务未连接</small>';
      const userName=isInvitation?'手机号 / 邮箱由本人填写':safe(item.username)+' · '+safe(identifierKind(item.username));
      return '<tr class="'+(current?'is-current':'')+'"><td><div class="oms-v1__identity-user"><strong>'+safe(isInvitation?'待受邀人注册':item.display_name)+'</strong>'+(current?'<span>当前账号</span>':'')+'<small>'+userName+'</small></div></td><td><div class="oms-v1__identity-member">'+association+'</div></td><td><div class="oms-v1__identity-access"><strong>'+safe(item.administrator?'管理员':'普通用户')+'</strong><span class="oms-v1__tag '+(item.status==='active'?'':'is-muted')+'">'+safe(statusLabel(item))+'</span></div></td><td><time>'+safe(dateTime(item.updated_at))+'</time></td><td>'+actions(item,person)+'</td></tr>';
    }).join(''):'<tr><td colspan="5"><div class="oms-v1__empty"><strong>'+(subjects.length?'没有匹配的账号':'暂无组织账号')+'</strong><span>'+(subjects.length?'调整搜索或筛选条件后重试。':'点击“创建邀请链接”，由受邀人自行注册手机号或邮箱。')+'</span></div></td></tr>';
    for(const button of body?.querySelectorAll('[data-identity-status]')||[])button.onclick=()=>changeStatus(button.dataset.identityStatus,button.dataset.nextStatus);
    for(const button of body?.querySelectorAll('[data-identity-reset]')||[])button.onclick=()=>resetPassword(button.dataset.identityReset);
    for(const button of body?.querySelectorAll('[data-identity-reissue]')||[])button.onclick=()=>reissueInvitation(button.dataset.identityReissue);
    for(const button of body?.querySelectorAll('[data-identity-administrator]')||[])button.onclick=()=>changeAdministrator(button.dataset.identityAdministrator,button.dataset.nextAdministrator==='true');
    for(const button of body?.querySelectorAll('[data-identity-edit-account]')||[])button.onclick=()=>editAccount(button.dataset.identityEditAccount);
    for(const button of body?.querySelectorAll('[data-identity-edit-member]')||[])button.onclick=()=>editMember(button.dataset.identityEditMember);
  }

  function setReloadVisible(visible){const button=page.querySelector('[data-identity-reload]');if(button)button.hidden=!visible}
  function requireOrganizationReady(values){
    const principal=values[0]?.principal,currentTenant=values[1]?.tenant,memberships=values[2]?.organizations;
    if(!principal?.tenant_id||!currentTenant?.id||principal.tenant_id!==currentTenant.id||!Array.isArray(memberships)||!memberships.some(item=>item.current&&item.status==='active'&&item.tenant_id===principal.tenant_id))throw new Error('组织信息尚未准备完成');
  }
  function showOrganizationRecovery(error){
    if(organizationRecoveryShown)return;
    organizationRecoveryShown=true;
    show({kicker:'组织信息',title:'组织信息未就绪',dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-status is-warning"><span>当前页面暂不可继续</span><strong>请重新加载组织信息</strong></div><p>登录或切换组织后的信息没有完整返回，可能是网络波动或会话尚未同步。系统已自动重试，但仍未完成。</p><div class="oms-v1__form-note">'+safe(error instanceof Error?error.message:'组织信息读取失败')+'。请先重新加载；仍失败时刷新整个页面并重新登录。</div><div class="oms-v1__row-actions"><button type="button" data-identity-retry-load>重新加载组织信息</button><button type="button" data-identity-refresh-page>刷新页面</button></div></section>'});
    queueMicrotask(()=>{root.querySelector('[data-identity-retry-load]')?.addEventListener('click',()=>{organizationRecoveryShown=false;close();void load({interactive:true})});root.querySelector('[data-identity-refresh-page]')?.addEventListener('click',()=>location.reload())});
  }
  async function performLoad({interactive=false}={}){
    organizationRecoveryShown=interactive?false:organizationRecoveryShown;
    setReloadVisible(false);
    let lastError=null;
    for(let attempt=0;attempt<3;attempt+=1){
      if(attempt)await wait(attempt===1?500:1200);
      notice.textContent=attempt?'正在重试组织信息（'+(attempt+1)+'/3）…':'正在读取账号与租户信息。';
      try{
        const tasks=[api('/api/v1/session'),api('/api/v1/tenant'),api('/api/v1/organizations')];
        if(administrator)tasks.push(api('/api/v1/admin/subjects'));
        const values=await Promise.all(tasks);requireOrganizationReady(values);session=values[0];tenant=values[1].tenant;organizations=values[2].organizations;subjects=administrator?(values[3]?.subjects||[]):[];
        const peopleResult=await Promise.allSettled([administrator&&ownerPeople?ownerAPI.api('people-groups','/api/v1/persons'):Promise.resolve(null)]);peopleAvailable=Boolean(administrator&&ownerPeople&&peopleResult[0].status==='fulfilled');people=peopleAvailable&&Array.isArray(peopleResult[0].value)?peopleResult[0].value:[];render();organizationRecoveryShown=false;setReloadVisible(false);notice.textContent=administrator?(peopleAvailable?'账号由 IAM 管理；账号姓名与成员档案分别维护，每个组织独立保存权限和关联。':'账号已就绪；成员服务暂不可用，可重新加载后再编辑成员资料。'):'当前组织和本人账号来自 IAM；可加入多个组织并在此切换。';return;
      }catch(error){lastError=error}
    }
    session=null;tenant=null;organizations=[];subjects=[];people=[];peopleAvailable=false;render();setReloadVisible(true);notice.textContent='组织信息未就绪，请点击“重新加载”；仍失败时刷新页面重试。';showOrganizationRecovery(lastError);
  }
  function load(options={}){
    if(loadPromise)return loadPromise;
    loadPromise=performLoad(options).finally(()=>{loadPromise=null});
    return loadPromise;
  }

  function tokenResult(kind,value,association={status:'not_applicable'}){
    const registration=kind==='registration',activation=kind==='activation',token=value.token,fragment=registration?'registration_invitation':activation?'activation_token':'password_reset_token',link=location.origin+'/workbench/?view=oms-v1#'+fragment+'='+encodeURIComponent(token),title=registration?'邀请链接已创建':activation?'账号已创建，等待本人激活':'密码重置链接已创建',expires=dateTime(value.expires_at),account=value.subject?.username||'',role=value.subject?.administrator?'管理员':'普通用户',message=registration?(tenant?.name||'当前组织')+' 邀请你以“'+role+'”身份加入 OMS1。\n请在 '+expires+' 前打开以下一次性链接，选择手机号或邮箱完成注册：\n'+link+'\n链接使用一次后立即失效。':(tenant?.name||'当前组织')+(activation?' 已为你创建 OMS1 账号。':' 已为你生成 OMS1 密码重置链接。')+'\n登录账号：'+account+'\n请在 '+expires+' 前打开以下一次性链接'+(activation?'设置密码：':'重设密码：')+'\n'+link+'\n链接使用一次后立即失效。';
    if(registration){show({kicker:'账号与租户',title,dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-status"><span>一次性邀请 · 24 小时有效</span><strong>'+safe(tenant?.name||'当前组织')+'</strong></div><dl class="oms-v1__record-facts"><div><dt>加入身份</dt><dd>'+safe(role)+'</dd></div><div><dt>登录账号</dt><dd>由受邀人填写</dd></div><div><dt>有效期至</dt><dd>'+safe(expires)+'</dd></div><div><dt>发送状态</dt><dd>未自动发送</dd></div></dl><div class="oms-v1__form-status">受邀人打开链接后自行选择手机号或邮箱、填写姓名和密码。注册完成后，可再到成员管理中显式关联成员。</div><div class="oms-v1__record-remark"><span>交给受邀人使用的一次性链接</span><p data-identity-token-link>'+safe(link)+'</p></div><div class="oms-v1__row-actions"><button type="button" data-copy-identity-link>复制链接</button><button type="button" data-copy-identity-message>复制完整通知</button></div><div class="oms-v1__form-status" data-copy-status>请通过可信渠道交给受邀人。重新生成后，旧链接立即失效。</div></section>'});queueMicrotask(()=>{const status=root.querySelector('[data-copy-status]'),copy=async(value,success)=>{try{await navigator.clipboard.writeText(value);status.textContent=success}catch{status.textContent='复制失败，请手动复制链接'}};root.querySelector('[data-copy-identity-link]')?.addEventListener('click',()=>copy(link,'邀请链接已复制'));root.querySelector('[data-copy-identity-message]')?.addEventListener('click',()=>copy(message,'完整邀请已复制'))});return}
    const associationBlock=!activation||association.status==='not_applicable'?'':association.status==='linked'?'<div class="oms-v1__form-status">已关联成员“'+safe(association.person_name)+'”；账号激活后即可按成员身份使用员工业务。</div>':association.status==='failed'?'<div class="oms-v1__form-status is-error">账号已创建，但成员“'+safe(association.person_name)+'”关联未完成：'+safe(association.message||'请稍后续办')+' <button type="button" data-identity-open-member-link>前往成员管理</button></div>':'<div class="oms-v1__form-status">本次未关联成员；不影响登录或使用组织级功能，可稍后在成员管理中关联。</div>';
    show({kicker:'IAM 账号',title,dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-status"><span>'+(activation?'待激活 · 24 小时有效':'一次性重置凭证')+'</span><strong>'+safe(value.subject?.display_name||account||'账号')+'</strong></div><dl class="oms-v1__record-facts"><div><dt>登录账号</dt><dd>'+safe(account)+'</dd></div><div><dt>账号类型</dt><dd>'+safe(identifierKind(account))+'</dd></div><div><dt>有效期至</dt><dd>'+safe(expires)+'</dd></div><div><dt>发送状态</dt><dd>未自动发送</dd></div></dl>'+associationBlock+'<div class="oms-v1__record-remark"><span>交给本人使用的一次性链接</span><p data-identity-token-link>'+safe(link)+'</p></div><div class="oms-v1__row-actions"><button type="button" data-copy-identity-link>复制链接</button><button type="button" data-copy-identity-message>复制完整通知</button></div><div class="oms-v1__form-status" data-copy-status>请通过可信渠道交给本人。重新生成后，旧链接立即失效。</div></section>'});
    queueMicrotask(()=>{const status=root.querySelector('[data-copy-status]'),copy=async(value,success)=>{try{await navigator.clipboard.writeText(value);status.textContent=success}catch{status.textContent='复制失败，请手动复制链接'}};root.querySelector('[data-copy-identity-link]')?.addEventListener('click',()=>copy(link,'激活链接已复制'));root.querySelector('[data-copy-identity-message]')?.addEventListener('click',()=>copy(message,'完整通知已复制'));root.querySelector('[data-identity-open-member-link]')?.addEventListener('click',()=>{close();location.hash='member-account-link'})});
  }

  function joinedResult(value,association={status:'none'}){const subject=value?.subject||{},associationBlock=association.status==='linked'?'<div class="oms-v1__form-status">已关联本组织成员“'+safe(association.person_name)+'”。</div>':association.status==='failed'?'<div class="oms-v1__form-status is-error">账号已加入组织，但成员“'+safe(association.person_name)+'”关联未完成：'+safe(association.message||'请稍后续办')+'</div>':'<div class="oms-v1__form-status">本次未关联成员；同一账号在本组织最多关联一名成员，在其他组织可关联各自成员。</div>';show({kicker:'IAM 账号',title:'账号已加入当前组织',dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-status"><span>无需再次激活</span><strong>'+safe(subject.display_name||subject.username||'账号')+'</strong></div><dl class="oms-v1__record-facts"><div><dt>登录账号</dt><dd>'+safe(subject.username||'—')+'</dd></div><div><dt>当前组织</dt><dd>'+safe(tenant?.name||'—')+'</dd></div><div><dt>账号状态</dt><dd>已启用</dd></div><div><dt>登录密码</dt><dd>沿用原密码</dd></div></dl>'+associationBlock+'<div class="oms-v1__record-remark"><span>后续使用</span><p>该账号登录后可在“账号与租户”中切换组织；各组织权限、管理员级别和成员关联相互隔离。</p></div></section>'})}

  function editAccount(subjectID){
    const item=subjects.find(value=>String(value.id)===String(subjectID));
    if(!item)return;
    show({kicker:'IAM 账号',title:'编辑账号姓名',submit:'保存账号姓名',enabled:true,body:'<form class="oms-v1__form-grid"><label class="oms-v1__form-field is-wide"><span>账号姓名 *</span><input name="display_name" required maxlength="100" value="'+safe(item.display_name||'')+'" autocomplete="name"></label><label class="oms-v1__form-field is-wide"><span>登录账号</span><input value="'+safe(item.username||'')+'" readonly aria-readonly="true"></label><div class="oms-v1__form-note is-wide">账号姓名属于 IAM，只用于登录身份、操作人和账号目录显示。若该账号已关联员工，员工姓名、部门和岗位请使用“编辑成员资料”单独维护。</div><div class="oms-v1__form-status is-wide" data-oms-v1-form-status></div></form>',onSubmit:async formData=>{const displayName=String(formData.get('display_name')||'').trim();await post('/api/v1/admin/subjects/'+encodeURIComponent(item.id)+'/profile',{expected_revision:Number(item.revision),display_name:displayName});close();await load();notice.textContent='账号姓名已更新；历史审计仍保留原操作人快照。'}})
  }

  function editMember(personID){
    const person=people.find(value=>String(value.id)===String(personID));
    if(!person)return;
    show({kicker:'People 成员',title:'编辑成员资料',submit:'保存成员资料',enabled:true,body:'<form class="oms-v1__form-grid"><label class="oms-v1__form-field is-wide"><span>员工姓名 *</span><input name="display_name" required maxlength="100" value="'+safe(person.display_name||'')+'" autocomplete="name"></label><label class="oms-v1__form-field"><span>部门</span><input name="department" maxlength="100" value="'+safe(person.department||'')+'" placeholder="选填"></label><label class="oms-v1__form-field"><span>岗位</span><input name="position_title" maxlength="100" value="'+safe(person.position_title||'')+'" placeholder="选填"></label><div class="oms-v1__form-note is-wide">这里修改 People 员工档案，不会改动登录账号、密码、管理员身份或历史工资快照。账号目录会在保存后重新读取最新成员资料。</div><div class="oms-v1__form-status is-wide" data-oms-v1-form-status></div></form>',onSubmit:async formData=>{await ownerAPI.api('people-groups','/api/v1/persons/'+encodeURIComponent(person.id)+'/profile',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_revision:Number(person.revision),reference_code:String(person.reference_code||''),display_name:String(formData.get('display_name')||'').trim(),department:String(formData.get('department')||'').trim(),position_title:String(formData.get('position_title')||'').trim()})});close();await load();notice.textContent='成员资料已更新；账号姓名保持不变。'}})
  }

  function invite(){show({kicker:'账号与租户',title:'创建邀请链接',submit:'生成邀请链接',enabled:true,body:'<ol class="oms-v1__identity-dialog-flow" aria-label="账号邀请流程"><li><b>1</b><span>设置加入身份</span></li><li><b>2</b><span>发送邀请链接</span></li><li><b>3</b><span>本人完成注册</span></li></ol><form class="oms-v1__form-grid"><label class="oms-v1__form-field is-wide"><span>加入后的用户级别 *</span><select name="access_level"><option value="user">普通用户</option><option value="administrator">管理员</option></select></label><div class="oms-v1__form-note is-wide">无需代替对方填写姓名或登录账号。链接 24 小时内一次有效；受邀人打开后可自行选择手机号或邮箱并设置密码。若账号已存在，需用原密码确认后加入本组织，不会覆盖原密码。</div><div class="oms-v1__form-status is-wide" data-oms-v1-form-status></div></form>',onSubmit:async data=>{const value=await post('/api/v1/admin/invitations',{administrator:String(data.get('access_level')||'user')==='administrator'});close();tokenResult('registration',value);void load()}})}
  function editTenant(){show({kicker:'组织设置',title:'编辑组织信息',submit:'保存设置',enabled:true,body:'<form class="oms-v1__form-grid"><label class="oms-v1__form-field is-wide"><span>组织显示名称 *</span><input name="name" required maxlength="100" value="'+safe(tenant?.name||'')+'" autocomplete="organization" placeholder="请输入组织名称"></label><label class="oms-v1__form-field is-wide"><span>组织稳定 ID</span><input value="'+safe(tenant?.id||'')+'" readonly aria-readonly="true"></label><div class="oms-v1__form-note is-wide">这里维护登录后显示的组织名称。稳定 ID 不可修改；营业执照名称、统一社会信用代码等企业主体资料应在主体档案中维护。</div><div class="oms-v1__form-status is-wide" data-oms-v1-form-status></div></form>',onSubmit:async data=>{await post('/api/v1/admin/tenant',{name:String(data.get('name')||'').trim()});close();await load();notice.textContent='组织名称已更新。'}})}
  async function reissueInvitation(subjectID){if(!confirm('确定生成新链接？此前尚未使用的旧链接会立即失效。'))return;try{const value=await post('/api/v1/admin/invitations/reissue',{subject_id:subjectID});tokenResult(value.action==='registration_invited'?'registration':'activation',value)}catch(error){notice.textContent=error instanceof Error?error.message:'邀请链接生成失败'}}
  async function changeStatus(subjectID,status){if(!confirm(status==='disabled'?'确定停用这个账号？现有会话会失效。':'确定重新启用这个账号？'))return;try{await post('/api/v1/admin/status',{subject_id:subjectID,status});await load()}catch(error){notice.textContent=error instanceof Error?error.message:'账号状态更新失败'}}
  async function changeAdministrator(subjectID,administrator){if(!confirm(administrator?'确定将该用户设为管理员？该用户将可以管理本组织用户。':'确定撤销该用户的管理员身份？其现有会话会失效。'))return;try{await post('/api/v1/admin/administrator',{subject_id:subjectID,administrator});await load()}catch(error){notice.textContent=error instanceof Error?error.message:'管理员身份更新失败'}}
  async function resetPassword(subjectID){if(!confirm('确定创建一次性密码重置凭证？旧凭证会被后续操作取代。'))return;try{const value=await post('/api/v1/admin/password-resets',{subject_id:subjectID});tokenResult('password_reset',value)}catch(error){notice.textContent=error instanceof Error?error.message:'密码重置凭证创建失败'}}
  async function switchOrganization(tenantID,source){
    if(!tenantID||tenantID===tenant?.id){closeRailOrganization();return}
    for(const button of root.querySelectorAll('[data-identity-rail-organization-option]'))button.disabled=true;
    if(source)source.disabled=true;
    const label=railOrganization?.querySelector('[data-identity-rail-tenant-name]');if(label)label.textContent='正在切换…';
    notice.textContent='正在切换组织并刷新权限…';
    try{await post('/api/v1/session/organization',{tenant_id:tenantID});location.reload()}
    catch(error){render();notice.textContent=error instanceof Error?error.message:'组织切换失败'}
  }

  page.querySelector('[data-identity-invite]')?.addEventListener('click',invite);
  page.querySelector('[data-identity-tenant-settings]')?.addEventListener('click',editTenant);
  page.querySelector('[data-identity-search]')?.addEventListener('input',event=>{filters.search=String(event.currentTarget.value||'').trim().toLowerCase();render()});
  page.querySelector('[data-identity-role-filter]')?.addEventListener('change',event=>{filters.role=String(event.currentTarget.value||'all');render()});
  page.querySelector('[data-identity-status-filter]')?.addEventListener('change',event=>{filters.status=String(event.currentTarget.value||'all');render()});
  page.querySelector('[data-identity-link-filter]')?.addEventListener('change',event=>{filters.link=String(event.currentTarget.value||'all');render()});
  page.querySelector('[data-identity-organization-submit]')?.addEventListener('click',()=>{const select=page.querySelector('[data-identity-organization-select]'),button=page.querySelector('[data-identity-organization-submit]');void switchOrganization(String(select?.value||''),button)});
  page.querySelector('[data-identity-reload]')?.addEventListener('click',()=>{organizationRecoveryShown=false;void load({interactive:true})});
  railOrganization?.querySelector('[data-identity-rail-organization-trigger]')?.addEventListener('click',()=>{const trigger=railOrganization.querySelector('[data-identity-rail-organization-trigger]'),menu=railOrganization.querySelector('[data-identity-rail-organization-menu]');if(trigger.disabled)return;const open=menu.hidden;closeRailOrganization();if(open){menu.hidden=false;trigger.setAttribute('aria-expanded','true')}});
  railOrganization?.querySelector('[data-identity-rail-organization-options]')?.addEventListener('click',event=>{const option=event.target.closest('[data-identity-rail-organization-option]');if(option)void switchOrganization(String(option.dataset.identityRailOrganizationOption||''),option)});
  document.addEventListener('click',event=>{if(railOrganization&&!railOrganization.contains(event.target))closeRailOrganization()});
  document.addEventListener('keydown',event=>{if(event.key==='Escape')closeRailOrganization()});
  void load();
})();
