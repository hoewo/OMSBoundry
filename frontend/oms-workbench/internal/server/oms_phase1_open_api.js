(()=>{
  'use strict';

  const root=document.querySelector('[data-oms-v1]');
  if(!root)return;

  const route=String(root.dataset.identityRoutePrefix||'').replace(/\/$/,'');
  const keyBody=root.querySelector('[data-open-api-key-body]');
  const usageBody=root.querySelector('[data-open-api-usage-body]');
  const notice=root.querySelector('[data-open-api-notice] p');
  const createButton=root.querySelector('[data-open-api-new-key]');
  if(!route||!keyBody||!usageBody||!notice||!createButton)return;

  const safe=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));
  const show=detail=>root.dispatchEvent(new CustomEvent('oms-v1:show-dialog',{detail}));
  const close=()=>root.dispatchEvent(new CustomEvent('oms-v1:close-dialog'));
  const dateTime=value=>{if(!value)return '—';const parsed=new Date(value);return Number.isNaN(parsed.getTime())?String(value):new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}).format(parsed)};
  const empty=(columns,title,copy)=>'<tr><td colspan="'+columns+'"><div class="oms-v1__empty"><strong>'+safe(title)+'</strong><span>'+safe(copy)+'</span></div></td></tr>';
  const currentCSRF=()=>{const item=document.cookie.split(';').map(value=>value.trim()).find(value=>value.startsWith('oms_iam_csrf=')),token=item?item.slice('oms_iam_csrf='.length):'';return /^[A-Za-z0-9_-]{16,256}$/.test(token)?token:''};
  let loading=null,loaded=false;

  async function csrf(){
    const current=currentCSRF();
    if(current)return current;
    const response=await fetch(route+'/api/v1/csrf',{credentials:'same-origin',cache:'no-store',redirect:'manual',headers:{Accept:'application/json'}});
    if(!response.ok||response.type==='opaqueredirect'||response.status===0)throw new Error('登录状态已过期，请刷新页面后重试');
    const value=await response.json(),fresh=currentCSRF();
    if(!fresh||value.csrf_token!==fresh)throw new Error('安全校验未能建立，请刷新页面后重试');
    return fresh;
  }

  async function problem(response){
    let value={};try{value=await response.json()}catch{}
    const labels={open_api_capability_denied:'当前账号没有开放 API 权限',person_mapping_required:'当前账号尚未关联有效成员档案',person_reference_unavailable:'暂时无法确认账号与成员的关联',active_api_key_limit:'最多可同时持有 5 个有效 API Key',csrf_failed:'安全校验失败，请刷新页面后重试'};
    const code=value.code||value.error?.code;
    return new Error(labels[code]||value.message||value.error?.message||(response.status===401?'登录状态已过期，请刷新页面后重试':'操作未完成，请稍后重试'));
  }

  async function request(path,options={}){
    const method=String(options.method||'GET').toUpperCase(),headers=new Headers(options.headers||{});
    headers.set('Accept','application/json');
    if(method!=='GET'&&method!=='HEAD')headers.set('X-CSRF-Token',await csrf());
    const response=await fetch(route+path,{...options,method,headers,credentials:'same-origin',redirect:'manual',cache:'no-store'});
    if(response.type==='opaqueredirect'||response.status===0)throw new Error('登录状态已过期，请刷新页面后重试');
    if(!response.ok)throw await problem(response);
    return response.status===204?{}:response.json();
  }

  function keyStatus(item){
    if(item.status==='revoked')return ['已撤销','muted'];
    if(new Date(item.expires_at).getTime()<=Date.now())return ['已到期','muted'];
    const remaining=new Date(item.expires_at).getTime()-Date.now();
    if(remaining<=14*86400000)return ['即将到期','warning'];
    return ['有效','positive'];
  }

  function renderKeys(items){
    if(!items.length){keyBody.innerHTML=empty(7,'尚未创建 API Key','创建后完整 secret 只显示一次，请及时复制并安全保存。');return}
    keyBody.innerHTML=items.map(item=>{const [label,tone]=keyStatus(item),canRevoke=item.status==='active'&&new Date(item.expires_at).getTime()>Date.now();return '<tr><td><strong>'+safe(item.label)+'</strong><small>成员引用 r'+safe(item.person_revision)+'</small></td><td><code>'+safe(item.prefix)+'</code></td><td>'+safe(dateTime(item.created_at))+'</td><td>'+safe(dateTime(item.expires_at))+'</td><td>'+safe(dateTime(item.last_used_at))+'</td><td><span class="oms-v1__tag '+(tone==='muted'?'is-muted':'')+'">'+safe(label)+'</span></td><td>'+(canRevoke?'<button class="oms-v1__row-action" type="button" data-open-api-revoke="'+safe(item.id)+'" data-label="'+safe(item.label)+'">撤销</button>':'')+'</td></tr>'}).join('');
    for(const button of keyBody.querySelectorAll('[data-open-api-revoke]'))button.addEventListener('click',()=>openRevoke(button.dataset.openApiRevoke,button.dataset.label));
  }

  function renderUsage(items){
    if(!items.length){usageBody.innerHTML=empty(6,'暂无调用记录','首次使用 API Key 后，这里会显示脱敏调用元数据。');return}
    usageBody.innerHTML=items.map(item=>{const successful=item.http_status>=200&&item.http_status<400,label=item.completed_at?(successful?'成功':'失败'):'处理中';return '<tr><td>'+safe(dateTime(item.occurred_at))+'</td><td><code>'+safe(item.key_prefix)+'</code></td><td><strong>'+safe(item.action)+'</strong></td><td><span class="oms-v1__tag '+(successful?'':'is-muted')+'">'+safe(label)+' · '+safe(item.http_status||'—')+'</span><small>'+safe(item.result_code)+'</small></td><td><code title="'+safe(item.request_id)+'">'+safe(String(item.request_id||'').slice(0,16))+(String(item.request_id||'').length>16?'…':'')+'</code></td><td>'+safe(item.source_ip_masked||'—')+'</td></tr>'}).join('');
  }

  async function load(force=false){
    if(loading)return loading;
    if(loaded&&!force)return;
    notice.textContent='正在读取 API Key 与本人调用记录。';
    loading=(async()=>{
      const values=await Promise.allSettled([request('/api/v1/api-keys'),request('/api/v1/open-api-usage?limit=20')]);
      if(values[0].status==='fulfilled')renderKeys(Array.isArray(values[0].value?.items)?values[0].value.items:[]);else keyBody.innerHTML=empty(7,'API Key 暂不可用',values[0].reason instanceof Error?values[0].reason.message:'请稍后重试');
      if(values[1].status==='fulfilled')renderUsage(Array.isArray(values[1].value?.items)?values[1].value.items:[]);else usageBody.innerHTML=empty(6,'调用记录暂不可用',values[1].reason instanceof Error?values[1].reason.message:'请稍后重试');
      const failures=values.filter(value=>value.status==='rejected');notice.textContent=failures.length?'部分开放平台数据暂不可用，请按页面提示处理。':'API Key 绑定当前账号、租户与成员；撤销、账号停用或权限回收会立即阻止后续调用。';
      loaded=failures.length===0;
    })().finally(()=>{loading=null});
    return loading;
  }

  function revealSecret(value){
    show({kicker:'开放平台',title:'API Key 已创建',dismissOnly:true,body:'<section class="oms-v1__secret-reveal"><p>这是唯一一次显示完整 secret。关闭后系统无法再次查看或恢复，请现在复制并保存到安全的凭证存储中。</p><label>完整 API Key<input readonly spellcheck="false" autocomplete="off" value="'+safe(value.secret)+'" data-open-api-secret></label><div class="oms-v1__record-inline-actions"><button type="button" data-open-api-copy>复制 API Key</button></div><div class="oms-v1__form-status" data-oms-v1-form-status></div></section>'});
    const input=root.querySelector('[data-open-api-secret]'),copy=root.querySelector('[data-open-api-copy]'),status=root.querySelector('[data-oms-v1-dialog-body] [data-oms-v1-form-status]');
    input?.select();
    if(copy)copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(value.secret);status.textContent='已复制。请妥善保管，后续页面不会再次展示。'}catch{input?.select();status.textContent='浏览器未允许自动复制，请使用系统复制快捷键。'}});
  }

  function openCreate(){
    show({kicker:'开放平台',title:'创建 API Key',submit:'创建并显示一次',enabled:true,onSubmit:async(data,form)=>{
      if(data.get('confirm')!=='yes')throw new Error('请确认已了解 secret 只显示一次');
      const value=await request('/api/v1/api-keys',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({label:String(data.get('label')||'').trim(),ttl_days:Number(data.get('ttl_days')||180)})});
      // The secret is unrecoverable after this response. Reveal it before any
      // non-essential refresh so a transient list failure can never hide it.
      loaded=false;revealSecret(value);void load(true);
    },body:'<form><label>名称 *<input name="label" required maxlength="80" placeholder="例如：个人报销 CLI"></label><label>有效期<select name="ttl_days"><option value="30">30 天</option><option value="90">90 天</option><option value="180" selected>180 天（默认）</option><option value="365">365 天</option></select></label><label class="oms-v1__inline-check"><input name="confirm" type="checkbox" value="yes" required>我知道完整 secret 只显示一次，创建后会立即保存</label><div class="oms-v1__form-note">每人最多同时持有 5 个有效 Key。Key 只继承本人已获授权的发票报销能力，不能绕过账号、租户或成员状态校验。</div><div class="oms-v1__form-status" data-oms-v1-form-status></div></form>'});
  }

  function openRevoke(id,label){
    show({kicker:'开放平台',title:'撤销 API Key',submit:'确认撤销',enabled:true,onSubmit:async data=>{
      if(data.get('confirm')!=='yes')throw new Error('请确认撤销操作');
      await request('/api/v1/api-keys/'+encodeURIComponent(id)+'/revoke',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'});close();await load(true);
    },body:'<form><p>撤销“'+safe(label)+'”后，所有使用该 Key 的 CLI、Skill 和其他工具会立即停止工作；该操作不可恢复。</p><label class="oms-v1__inline-check"><input name="confirm" type="checkbox" value="yes" required>确认立即撤销此 API Key</label><div class="oms-v1__form-status" data-oms-v1-form-status></div></form>'});
  }

  createButton.addEventListener('click',openCreate);
  for(const button of root.querySelectorAll('[data-oms-v1-nav="open-api"]'))button.addEventListener('click',()=>load());
  root.querySelector('[data-oms-v1-refresh]')?.addEventListener('click',()=>{if(root.querySelector('[data-oms-v1-page="open-api"]')?.classList.contains('is-active'))load(true)});
})();
