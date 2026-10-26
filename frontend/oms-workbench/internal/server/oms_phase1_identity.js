(()=>{
  'use strict';

  const root=document.querySelector('[data-oms-v1]'),page=root?.querySelector('[data-oms-v1-page="account"]');
  if(!root||!page)return;
  const route=String(root.dataset.identityRoutePrefix||'').replace(/\/$/,'');
  const administrator=root.dataset.administrator==='true';
  const safe=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));
  const show=detail=>root.dispatchEvent(new CustomEvent('oms-v1:show-dialog',{detail}));
  const close=()=>root.dispatchEvent(new CustomEvent('oms-v1:close-dialog'));
  const notice=page.querySelector('[data-identity-notice] p');
  let session=null,tenant=null,subjects=[];

  function csrfCookie(){const match=document.cookie.match(/(?:^|;\s*)oms_iam_csrf=([A-Za-z0-9_-]{16,256})(?:;|$)/);return match?match[1]:''}
  async function csrf(){let value=csrfCookie();if(value)return value;const response=await fetch(route+'/api/v1/csrf',{credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}});if(!response.ok)throw new Error('安全令牌暂时不可用');const body=await response.json();value=csrfCookie();if(!value||body.csrf_token!==value)throw new Error('安全令牌校验失败');return value}
  async function api(path,options={}){
    const headers={Accept:'application/json',...(options.headers||{})},method=String(options.method||'GET').toUpperCase();
    if(method!=='GET'&&method!=='HEAD')headers['X-CSRF-Token']=await csrf();
    const response=await fetch(route+path,{...options,method,headers,credentials:'same-origin',cache:'no-store',redirect:'error'});
    if(response.status===204)return null;
    const body=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(body.detail||body.title||'账号服务暂时不可用');
    return body;
  }
  const post=(path,value)=>api(path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(value)});
  function dateTime(value){if(!value)return '—';const parsed=new Date(value);return Number.isNaN(parsed.getTime())?String(value):new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}).format(parsed)}
  function setText(selector,value){const node=page.querySelector(selector);if(node)node.textContent=value}
  function identifierKind(value){value=String(value||'');if(/^1[3-9]\d{9}$/.test(value))return '手机号';if(/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value))return '邮箱';return '用户名'}

  function render(){
    const principal=session?.principal||{};
    setText('[data-identity-tenant-name]',tenant?.name||'—');
    setText('[data-identity-tenant-id]',tenant?.id||principal.tenant_id||'—');
    setText('[data-identity-display-name]',principal.display_name||root.dataset.displayName||'—');
    setText('[data-identity-username]',principal.username||'—');
    setText('[data-identity-account-status]','启用');
    setText('[data-identity-account-role]',principal.administrator?'租户管理员':'普通账号');
    setText('[data-identity-subject-count]',String(subjects.length));
    const body=page.querySelector('[data-identity-subject-body]');
    if(body)body.innerHTML=subjects.length?subjects.map(item=>'<tr><td><strong>'+safe(item.display_name)+'</strong></td><td><strong>'+safe(item.username)+'</strong><small class="oms-v1__cell-note">'+identifierKind(item.username)+'</small></td><td>'+safe(item.administrator?'租户管理员':'普通账号')+'</td><td><span class="oms-v1__tag '+(item.status==='active'?'':'is-muted')+'">'+safe(item.status==='active'?'已启用':item.status==='pending'?'待激活':'已停用')+'</span></td><td>'+safe(dateTime(item.updated_at))+'</td><td><span class="oms-v1__row-actions">'+(item.status==='pending'?'<button type="button" data-identity-reissue="'+safe(item.id)+'">生成新激活链接</button>':item.status==='active'?'<button type="button" data-identity-reset="'+safe(item.id)+'">重置密码</button>':'')+(item.status!=='pending'&&item.id!==principal.subject_id?'<button type="button" data-identity-status="'+safe(item.id)+'" data-next-status="'+(item.status==='active'?'disabled':'active')+'">'+(item.status==='active'?'停用':'启用')+'</button>':'')+'</span></td></tr>').join(''):'<tr><td colspan="6"><div class="oms-v1__empty"><strong>暂无其他账号</strong><span>点击“新增账号”，录入手机号或邮箱并生成激活链接。</span></div></td></tr>';
    for(const button of body?.querySelectorAll('[data-identity-status]')||[])button.onclick=()=>changeStatus(button.dataset.identityStatus,button.dataset.nextStatus);
    for(const button of body?.querySelectorAll('[data-identity-reset]')||[])button.onclick=()=>resetPassword(button.dataset.identityReset);
    for(const button of body?.querySelectorAll('[data-identity-reissue]')||[])button.onclick=()=>reissueInvitation(button.dataset.identityReissue);
  }

  async function load(){
    notice.textContent='正在读取账号与租户信息。';
    try{
      const tasks=[api('/api/v1/session'),api('/api/v1/tenant')];
      if(administrator)tasks.push(api('/api/v1/admin/subjects'));
      const values=await Promise.all(tasks);session=values[0];tenant=values[1]?.tenant||null;subjects=administrator?(values[2]?.subjects||[]):[];render();notice.textContent=administrator?'账号由 IAM 管理。新增账号后请复制一次性激活链接交给本人；系统当前不会假装发送短信或邮件。':'当前组织和本人账号来自 IAM；本页不保存浏览器假数据。';
    }catch(error){notice.textContent=error instanceof Error?error.message:'账号与租户信息读取失败'}
  }

  function tokenResult(kind,value){
    const token=value.token,fragment=kind==='activation'?'activation_token':'password_reset_token',link=location.origin+'/workbench/?view=oms-v1#'+fragment+'='+encodeURIComponent(token),activation=kind==='activation',title=activation?'账号已创建，等待本人激活':'密码重置链接已创建',expires=dateTime(value.expires_at),account=value.subject?.username||'',message=(tenant?.name||'当前组织')+(activation?' 已为你创建 OMS1 账号。':' 已为你生成 OMS1 密码重置链接。')+'\n登录账号：'+account+'\n请在 '+expires+' 前打开以下一次性链接'+(activation?'设置密码：':'重设密码：')+'\n'+link+'\n链接使用一次后立即失效。';
    show({kicker:'IAM 账号',title,dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-status"><span>'+(activation?'待激活 · 24 小时有效':'一次性重置凭证')+'</span><strong>'+safe(value.subject?.display_name||account||'账号')+'</strong></div><dl class="oms-v1__record-facts"><div><dt>登录账号</dt><dd>'+safe(account)+'</dd></div><div><dt>账号类型</dt><dd>'+safe(identifierKind(account))+'</dd></div><div><dt>有效期至</dt><dd>'+safe(expires)+'</dd></div><div><dt>发送状态</dt><dd>未自动发送</dd></div></dl><div class="oms-v1__record-remark"><span>交给本人使用的一次性链接</span><p data-identity-token-link>'+safe(link)+'</p></div><div class="oms-v1__row-actions"><button type="button" data-copy-identity-link>复制链接</button><button type="button" data-copy-identity-message>复制完整通知</button></div><div class="oms-v1__form-status" data-copy-status>请通过可信渠道交给本人。重新生成后，旧链接立即失效。</div></section>'});
    queueMicrotask(()=>{const status=root.querySelector('[data-copy-status]'),copy=async(value,success)=>{try{await navigator.clipboard.writeText(value);status.textContent=success}catch{status.textContent='复制失败，请手动复制链接'}};root.querySelector('[data-copy-identity-link]')?.addEventListener('click',()=>copy(link,'激活链接已复制'));root.querySelector('[data-copy-identity-message]')?.addEventListener('click',()=>copy(message,'完整通知已复制'))});
  }

  function invite(){show({kicker:'账号与租户',title:'新增账号',submit:'创建并生成激活链接',enabled:true,body:'<form class="oms-v1__form-grid"><label class="oms-v1__form-field"><span>姓名 *</span><input name="display_name" required maxlength="100" autocomplete="name" placeholder="请输入成员姓名"></label><label class="oms-v1__form-field"><span>登录方式 *</span><select name="identifier_kind"><option value="phone">手机号</option><option value="email">邮箱</option></select></label><label class="oms-v1__form-field is-wide"><span>手机号或邮箱 *</span><input name="login_identifier" required maxlength="254" autocomplete="off" placeholder="请输入手机号"></label><div class="oms-v1__form-note is-wide">保存后创建待激活账号并生成 24 小时一次性链接。当前不自动发送短信或邮件，请复制链接交给本人设置密码。</div><div class="oms-v1__form-status is-wide" data-oms-v1-form-status></div></form>',onSubmit:async data=>{const kind=String(data.get('identifier_kind')||'phone'),raw=String(data.get('login_identifier')||'').trim(),identifier=kind==='phone'?raw.replace(/[\s()-]/g,'').replace(/^\+86/,'').replace(/^0086/,''):raw.toLowerCase();if(kind==='phone'&&!/^1[3-9]\d{9}$/.test(identifier))throw new Error('请输入有效的中国大陆手机号');if(kind==='email'&&!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(identifier))throw new Error('请输入有效的邮箱地址');const value=await post('/api/v1/admin/invitations',{display_name:String(data.get('display_name')||'').trim(),login_identifier:identifier});close();await load();tokenResult('activation',value)}});queueMicrotask(()=>{const form=root.querySelector('.oms-v1__dialog form'),kind=form?.querySelector('[name="identifier_kind"]'),input=form?.querySelector('[name="login_identifier"]');kind?.addEventListener('change',()=>{input.placeholder=kind.value==='email'?'name@example.com':'请输入手机号';input.inputMode=kind.value==='email'?'email':'tel';input.autocomplete=kind.value==='email'?'email':'tel'})})}
  async function reissueInvitation(subjectID){if(!confirm('确定生成新的激活链接？此前尚未使用的旧链接会立即失效。'))return;try{const value=await post('/api/v1/admin/invitations/reissue',{subject_id:subjectID});tokenResult('activation',value)}catch(error){notice.textContent=error instanceof Error?error.message:'激活链接生成失败'}}
  async function changeStatus(subjectID,status){if(!confirm(status==='disabled'?'确定停用这个账号？现有会话会失效。':'确定重新启用这个账号？'))return;try{await post('/api/v1/admin/status',{subject_id:subjectID,status});await load()}catch(error){notice.textContent=error instanceof Error?error.message:'账号状态更新失败'}}
  async function resetPassword(subjectID){if(!confirm('确定创建一次性密码重置凭证？旧凭证会被后续操作取代。'))return;try{const value=await post('/api/v1/admin/password-resets',{subject_id:subjectID});tokenResult('password_reset',value)}catch(error){notice.textContent=error instanceof Error?error.message:'密码重置凭证创建失败'}}

  page.querySelector('[data-identity-invite]')?.addEventListener('click',invite);
  void load();
})();
