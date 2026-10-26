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
    if(body)body.innerHTML=subjects.length?subjects.map(item=>'<tr><td><strong>'+safe(item.display_name)+'</strong></td><td>'+safe(item.username)+'</td><td>'+safe(item.administrator?'租户管理员':'普通账号')+'</td><td><span class="oms-v1__tag '+(item.status==='active'?'':'is-muted')+'">'+safe(item.status==='active'?'启用':item.status==='pending'?'待激活':'停用')+'</span></td><td>'+safe(dateTime(item.updated_at))+'</td><td><span class="oms-v1__row-actions">'+(item.status==='pending'?'':'<button type="button" data-identity-reset="'+safe(item.id)+'">重置密码</button>')+(item.id!==principal.subject_id?'<button type="button" data-identity-status="'+safe(item.id)+'" data-next-status="'+(item.status==='active'?'disabled':'active')+'">'+(item.status==='active'?'停用':'启用')+'</button>':'')+'</span></td></tr>').join(''):'<tr><td colspan="6"><div class="oms-v1__empty"><strong>暂无租户账号</strong><span>邀请账号后将在这里显示。</span></div></td></tr>';
    for(const button of body?.querySelectorAll('[data-identity-status]')||[])button.onclick=()=>changeStatus(button.dataset.identityStatus,button.dataset.nextStatus);
    for(const button of body?.querySelectorAll('[data-identity-reset]')||[])button.onclick=()=>resetPassword(button.dataset.identityReset);
  }

  async function load(){
    notice.textContent='正在读取账号与租户信息。';
    try{
      const tasks=[api('/api/v1/session'),api('/api/v1/tenant')];
      if(administrator)tasks.push(api('/api/v1/admin/subjects'));
      const values=await Promise.all(tasks);session=values[0];tenant=values[1]?.tenant||null;subjects=administrator?(values[2]?.subjects||[]):[];render();notice.textContent=administrator?'账号管理已连接 IAM；邀请、停用和密码重置都会写入真实身份服务。':'当前组织和本人账号来自 IAM；本页不保存浏览器假数据。';
    }catch(error){notice.textContent=error instanceof Error?error.message:'账号与租户信息读取失败'}
  }

  function tokenResult(kind,value){
    const token=value.token,fragment=kind==='activation'?'activation_token':'password_reset_token',link=location.origin+'/workbench/?view=oms-v1#'+fragment+'='+encodeURIComponent(token),title=kind==='activation'?'邀请已创建':'密码重置凭证已创建';
    show({kicker:'IAM 账号',title,dismissOnly:true,body:'<section class="oms-v1__record-detail"><div class="oms-v1__record-status"><span>仅显示一次</span><strong>'+safe(value.subject?.display_name||value.subject?.username||'账号')+'</strong></div><div class="oms-v1__record-remark"><span>交给对应用户的安全链接</span><p data-identity-token-link>'+safe(link)+'</p></div><div class="oms-v1__row-actions"><button type="button" data-copy-identity-link>复制链接</button></div><div class="oms-v1__form-status" data-copy-status>链接包含一次性凭证，请通过可信渠道传递。</div></section>'});
    queueMicrotask(()=>root.querySelector('[data-copy-identity-link]')?.addEventListener('click',async()=>{const status=root.querySelector('[data-copy-status]');try{await navigator.clipboard.writeText(link);status.textContent='已复制到剪贴板'}catch{status.textContent='复制失败，请手动复制链接'}}));
  }

  function invite(){show({kicker:'账号与租户',title:'邀请账号',submit:'创建邀请',enabled:true,body:'<form class="oms-v1__form-grid"><label class="oms-v1__form-field"><span>显示名称 *</span><input name="display_name" required maxlength="100"></label><label class="oms-v1__form-field"><span>用户名 *</span><input name="username" required pattern="[a-z0-9][a-z0-9._-]{2,63}" autocomplete="off"></label><div class="oms-v1__form-note is-wide">账号只属于当前租户；邀请凭证仅返回一次，由用户设置自己的密码后激活。</div><div class="oms-v1__form-status is-wide" data-oms-v1-form-status></div></form>',onSubmit:async data=>{const value=await post('/api/v1/admin/invitations',{display_name:String(data.get('display_name')||'').trim(),username:String(data.get('username')||'').trim().toLowerCase()});close();await load();tokenResult('activation',value)}})}
  async function changeStatus(subjectID,status){if(!confirm(status==='disabled'?'确定停用这个账号？现有会话会失效。':'确定重新启用这个账号？'))return;try{await post('/api/v1/admin/status',{subject_id:subjectID,status});await load()}catch(error){notice.textContent=error instanceof Error?error.message:'账号状态更新失败'}}
  async function resetPassword(subjectID){if(!confirm('确定创建一次性密码重置凭证？旧凭证会被后续操作取代。'))return;try{const value=await post('/api/v1/admin/password-resets',{subject_id:subjectID});tokenResult('password_reset',value)}catch(error){notice.textContent=error instanceof Error?error.message:'密码重置凭证创建失败'}}

  page.querySelector('[data-identity-invite]')?.addEventListener('click',invite);
  void load();
})();
