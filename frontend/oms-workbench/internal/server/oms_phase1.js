(()=>{
  const requested=new URLSearchParams(window.location.search).get('view');
  if(requested&&requested!=='oms-v1')return;
  const root=document.querySelector('[data-oms-v1]');
  if(!root)return;
  document.body.classList.add('oms-v1-active');

  const state={projects:[],parties:[],partyCache:new Map(),operatingParty:null,invoices:[],cash:[],moneyContext:null,contractContext:null,commercial:new Map(),projectPage:1,projectPageSize:20,listPages:{},listPageSizes:{},filters:{},sorts:{projects:{key:'start_date',direction:'desc'},'party-client':{key:'usage',direction:'desc'},'party-contractor':{key:'usage',direction:'desc'},'commitment-receivable':{key:'linked_at',direction:'desc'},'commitment-payable':{key:'linked_at',direction:'desc'},'invoice-outgoing':{key:'date',direction:'desc'},'invoice-incoming':{key:'date',direction:'desc'},'cash-receipt':{key:'date',direction:'desc'},'cash-payment':{key:'date',direction:'desc'}},authorized:new Set(),errors:[],projectActions:new Set(),partyActions:new Set(),invoiceActions:new Set(),contractActions:new Set(),selectedProjectID:'',projectDetailToken:0,moneyAvailable:false,contractAvailable:false,evidenceTransfer:null};
  const labels={overview:'经营总览',projects:'项目经营',receivable:'应收回款',payable:'应付支付'};
  const tabLabels={'project-list':'项目列表',clients:'甲方管理',contractors:'施工方管理',receivables:'应收管理','outgoing-invoices':'开票管理',receipts:'回款记录',payables:'应付管理',payments:'支付记录','incoming-invoices':'发票管理'};
  const defaultTabs={projects:'project-list',receivable:'receivables',payable:'payables'};
  const invoiceDateLabel=direction=>direction==='incoming'?'收票日期':'开票日期';
  const cashDateLabel=type=>type==='receipt'?'回款日期':'支付日期';
  const activeTabs={...defaultTabs};
  const contextGroup=root.querySelector('[data-oms-v1-context-group]');
  const contexts=root.querySelector('[data-oms-v1-context]');
  const sync=root.querySelector('[data-oms-v1-sync]');
  const authorizationTasks=new Map();
  let loadTask=null,evidenceRuntimeTask=null;
  let activeFilterPanel=null,activeFilterButton=null;
  // Self-contained equivalent of ReferenceOAGatewayCSRF keeps OMS first paint independent of the legacy runtime.
  class GatewayCSRF{
    constructor(route){this.route=route;this.pending=null}
    current(){const value=document.cookie.split(';').map(item=>item.trim()).find(item=>item.startsWith('oms_iam_csrf=')),token=value?value.slice('oms_iam_csrf='.length):'';return /^[A-Za-z0-9_-]{16,256}$/.test(token)?token:''}
    async token(){const current=this.current();if(current)return current;if(!this.pending)this.pending=this.refresh().finally(()=>{this.pending=null});return this.pending}
    async refresh(){if(!/^\/[A-Za-z0-9_-]+(?:\/[A-Za-z0-9_-]+)*$/.test(this.route||''))throw new Error('页面状态已过期，请刷新后重试');const response=await fetch(this.route+'/api/v1/csrf',{method:'GET',credentials:'same-origin',cache:'no-store',redirect:'error',headers:{Accept:'application/json'}});if(!response.ok||response.redirected)throw new Error('页面状态已过期，请刷新后重试');const data=await response.json(),current=this.current();if(!current||data.csrf_token!==current)throw new Error('页面状态已过期，请刷新后重试');return current}
  }
  const csrfProvider=new GatewayCSRF(root.dataset.identityRoutePrefix);
  const app=id=>{const node=root.querySelector('[data-oms-v1-application="'+id+'"]');return node?{id,route:node.dataset.omsV1Route}:null};
  const safeText=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));
  const option=(value,label,current)=>'<option value="'+safeText(value)+'" '+(value===current?'selected':'')+'>'+safeText(label)+'</option>';
  const selectMenu=document.createElement('div');selectMenu.id='oms-v1-select-menu';selectMenu.className='oms-v1__select-menu';selectMenu.hidden=true;selectMenu.setAttribute('role','listbox');document.body.append(selectMenu);
  let activeSelect=null,activeSelectTrigger=null,activeSelectIndex=-1;
  function unifiedSelectLabel(select){const own=select.getAttribute('aria-label');if(own)return own;const field=select.closest('.oms-v1__form-field'),fieldLabel=field?.querySelector(':scope > span,:scope > .oms-v1__field-head > span');if(fieldLabel)return fieldLabel.textContent.trim();const label=select.closest('label');if(label){const clone=label.cloneNode(true);for(const node of clone.querySelectorAll('select,input,textarea,button'))node.remove();const text=clone.textContent.trim();if(text)return text}return select.name||'请选择'}
  function positionUnifiedSelect(){if(!activeSelectTrigger||selectMenu.hidden)return;const rect=activeSelectTrigger.getBoundingClientRect(),padding=8,gap=6,width=Math.max(rect.width,180),left=Math.min(Math.max(padding,rect.left),Math.max(padding,window.innerWidth-width-padding));selectMenu.style.width=Math.min(width,window.innerWidth-padding*2)+'px';selectMenu.style.left=left+'px';selectMenu.style.maxHeight=Math.min(300,window.innerHeight-padding*2)+'px';const height=Math.min(selectMenu.scrollHeight,300,window.innerHeight-padding*2),below=window.innerHeight-rect.bottom-gap,above=rect.top-gap;selectMenu.style.top=(below>=height||below>=above?Math.min(window.innerHeight-height-padding,rect.bottom+gap):Math.max(padding,rect.top-height-gap))+'px'}
  function syncUnifiedSelect(select){const trigger=select?._omsV1Trigger;if(!trigger)return;const selected=select.options[select.selectedIndex];trigger.querySelector('span').textContent=selected?.textContent?.trim()||'请选择';trigger.disabled=select.disabled;trigger.classList.toggle('is-placeholder',!select.value);trigger.classList.remove('is-invalid');trigger.setAttribute('aria-disabled',String(select.disabled));if(activeSelect===select){activeSelectIndex=Math.max(0,select.selectedIndex);renderUnifiedSelectMenu()}}
  function closeUnifiedSelect(restoreFocus=false){if(!activeSelect)return;const trigger=activeSelectTrigger;selectMenu.hidden=true;selectMenu.innerHTML='';trigger?.setAttribute('aria-expanded','false');activeSelect=null;activeSelectTrigger=null;activeSelectIndex=-1;if(restoreFocus)trigger?.focus()}
  function focusUnifiedSelectOption(){const options=[...selectMenu.querySelectorAll('[role="option"]')];for(const [index,node] of options.entries())node.classList.toggle('is-current',index===activeSelectIndex);options[activeSelectIndex]?.scrollIntoView({block:'nearest'})}
  function renderUnifiedSelectMenu(){if(!activeSelect)return;const options=[...activeSelect.options];selectMenu.innerHTML=options.map((item,index)=>'<button type="button" role="option" data-oms-v1-select-index="'+index+'" aria-selected="'+String(index===activeSelect.selectedIndex)+'" '+(item.disabled?'disabled':'')+'><span>'+safeText(item.textContent.trim())+'</span>'+(index===activeSelect.selectedIndex?'<i aria-hidden="true">✓</i>':'')+'</button>').join('');selectMenu.hidden=false;focusUnifiedSelectOption();positionUnifiedSelect()}
  function openUnifiedSelect(select,trigger){if(select.disabled)return;if(activeSelect&&activeSelect!==select)closeUnifiedSelect();activeSelect=select;activeSelectTrigger=trigger;activeSelectIndex=Math.max(0,select.selectedIndex);trigger.setAttribute('aria-expanded','true');renderUnifiedSelectMenu()}
  function chooseUnifiedSelect(index){if(!activeSelect)return;const item=activeSelect.options[index];if(!item||item.disabled)return;const select=activeSelect,changed=select.selectedIndex!==index;select.selectedIndex=index;syncUnifiedSelect(select);closeUnifiedSelect(true);if(changed){select.dispatchEvent(new Event('input',{bubbles:true}));select.dispatchEvent(new Event('change',{bubbles:true}))}}
  function moveUnifiedSelect(step){if(!activeSelect)return;const options=[...activeSelect.options];let next=activeSelectIndex;do{next=Math.max(0,Math.min(options.length-1,next+step))}while(options[next]?.disabled&&next!==activeSelectIndex);activeSelectIndex=next;focusUnifiedSelectOption()}
  function enhanceUnifiedSelect(select){if(select.dataset.omsV1SelectEnhanced){syncUnifiedSelect(select);return}select.dataset.omsV1SelectEnhanced='true';select.classList.add('oms-v1__native-select');select.tabIndex=-1;select.setAttribute('aria-hidden','true');const wrap=document.createElement('span');wrap.className='oms-v1__select';select.before(wrap);wrap.append(select);const trigger=document.createElement('button');trigger.type='button';trigger.className='oms-v1__select-trigger';trigger.innerHTML='<span></span><i aria-hidden="true"></i>';trigger.setAttribute('role','combobox');trigger.setAttribute('aria-haspopup','listbox');trigger.setAttribute('aria-expanded','false');trigger.setAttribute('aria-controls',selectMenu.id);trigger.setAttribute('aria-label',unifiedSelectLabel(select));wrap.append(trigger);select._omsV1Trigger=trigger;trigger.addEventListener('click',event=>{event.preventDefault();activeSelect===select?closeUnifiedSelect(true):openUnifiedSelect(select,trigger)});trigger.addEventListener('keydown',event=>{if(!['ArrowDown','ArrowUp','Enter',' '].includes(event.key))return;event.preventDefault();if(activeSelect!==select){openUnifiedSelect(select,trigger);if(event.key==='ArrowUp'){activeSelectIndex=Math.max(0,select.options.length-1);focusUnifiedSelectOption()}return}if(event.key==='ArrowDown')moveUnifiedSelect(1);else if(event.key==='ArrowUp')moveUnifiedSelect(-1);else chooseUnifiedSelect(activeSelectIndex)});select.addEventListener('change',()=>syncUnifiedSelect(select));select.addEventListener('invalid',event=>{event.preventDefault();trigger.classList.add('is-invalid');trigger.focus()});syncUnifiedSelect(select)}
  function enhanceUnifiedSelects(scope=root){if(scope.matches?.('select'))enhanceUnifiedSelect(scope);for(const select of scope.querySelectorAll?.('select')||[])enhanceUnifiedSelect(select)}
  selectMenu.addEventListener('click',event=>{const optionButton=event.target.closest('[data-oms-v1-select-index]');if(optionButton)chooseUnifiedSelect(Number(optionButton.dataset.omsV1SelectIndex))});
  selectMenu.addEventListener('keydown',event=>{if(event.key==='ArrowDown'){event.preventDefault();moveUnifiedSelect(1)}else if(event.key==='ArrowUp'){event.preventDefault();moveUnifiedSelect(-1)}else if(event.key==='Home'){event.preventDefault();activeSelectIndex=0;focusUnifiedSelectOption()}else if(event.key==='End'){event.preventDefault();activeSelectIndex=Math.max(0,(activeSelect?.options.length||1)-1);focusUnifiedSelectOption()}else if(event.key==='Enter'||event.key===' '){event.preventDefault();chooseUnifiedSelect(activeSelectIndex)}else if(event.key==='Escape'){event.preventDefault();event.stopPropagation();closeUnifiedSelect(true)}else if(event.key==='Tab')closeUnifiedSelect()});
  document.addEventListener('pointerdown',event=>{if(activeSelect&&!selectMenu.contains(event.target)&&!activeSelectTrigger?.contains(event.target))closeUnifiedSelect()});
  document.addEventListener('keydown',event=>{if(event.key==='Escape'&&activeSelect){event.preventDefault();event.stopImmediatePropagation();closeUnifiedSelect(true)}},{capture:true});
  window.addEventListener('resize',positionUnifiedSelect);document.addEventListener('scroll',positionUnifiedSelect,true);
  new MutationObserver(mutations=>{for(const mutation of mutations){if(mutation.target.matches?.('select'))syncUnifiedSelect(mutation.target);for(const node of mutation.addedNodes){if(node.nodeType===1)enhanceUnifiedSelects(node)}}if(activeSelect&&!activeSelect.isConnected)closeUnifiedSelect()}).observe(root,{subtree:true,childList:true,attributes:true,attributeFilter:['disabled']});
  enhanceUnifiedSelects(root);
  const date=value=>{if(!value)return '—';const parsed=new Date(value);return Number.isNaN(parsed.getTime())?String(value).slice(0,10):new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit'}).format(parsed)};
  const dateTime=value=>{if(!value)return '—';const parsed=new Date(value);return Number.isNaN(parsed.getTime())?String(value):new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}).format(parsed)};
  const today=()=>{const now=new Date();return new Date(now.getTime()-now.getTimezoneOffset()*60000).toISOString().slice(0,10)};
  const money=minor=>{if(minor===null||minor===undefined)return '未授权';try{const raw=typeof minor==='bigint'?minor:BigInt(String(minor));const negative=raw<0n,absolute=negative?-raw:raw,whole=absolute/100n,fraction=String(absolute%100n).padStart(2,'0');return (negative?'-':'')+'¥'+whole.toLocaleString('zh-CN')+'.'+fraction}catch{return '—'}};
  const decimalMoney=value=>{if(value===null||value===undefined)return '未授权';const match=String(value).trim().match(/^(-?)(\d+)(?:\.(\d+))?$/);if(!match)return '—';try{const whole=BigInt(match[2]).toLocaleString('zh-CN'),fraction=(match[3]||'').padEnd(2,'0');return (match[1]||'')+'¥'+whole+'.'+fraction}catch{return '—'}};
  const decimalMinor=value=>{const match=String(value??'').trim().match(/^(-?)(\d+)(?:\.(\d{1,2}))?$/);if(!match)return null;try{return (match[1]? -1n:1n)*(BigInt(match[2])*100n+BigInt((match[3]||'').padEnd(2,'0')))}catch{return null}};
  const integerMinor=value=>{try{return value===null||value===undefined||value===''?null:BigInt(String(value))}catch{return null}};
  const minorDecimal=value=>{const negative=value<0n,absolute=negative?-value:value;return (negative?'-':'')+String(absolute/100n)+'.'+String(absolute%100n).padStart(2,'0')};
  const uuid=()=>globalThis.crypto?.randomUUID?.()||'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g,value=>{const random=Math.random()*16|0,part=value==='x'?random:(random&3|8);return part.toString(16)});
  const stableUUID=async seed=>{if(!globalThis.crypto?.subtle)return uuid();const digest=new Uint8Array(await globalThis.crypto.subtle.digest('SHA-256',new TextEncoder().encode(seed)));digest[6]=(digest[6]&15)|80;digest[8]=(digest[8]&63)|128;const hex=[...digest.slice(0,16)].map(value=>value.toString(16).padStart(2,'0')).join('');return hex.slice(0,8)+'-'+hex.slice(8,12)+'-'+hex.slice(12,16)+'-'+hex.slice(16,20)+'-'+hex.slice(20)};
  const responseProblem=async response=>{let data={};try{data=await response.json()}catch{}const code=data?.code||data?.error?.code||'',labels={INVALID_REQUEST:'提交内容不符合业务规则，请检查项目、往来主体、金额和日期',REFERENCE_INVALID:'关联的项目或主体信息已经变化，请刷新后重试',SCOPE_DENIED:'当前账号不能在所选项目中执行该操作',CAPABILITY_DENIED:'当前账号没有操作权限'};const message=labels[code]||(typeof data?.error==='string'?data.error:data?.error?.message)||data?.detail||data?.message||(response.status===403?'当前账号没有操作权限':'操作未完成，请稍后重试');const error=new Error(message);error.status=response.status;error.code=code;return error};
  const sourceIssue=(label,error)=>{const message=error instanceof Error?error.message:'';return label+'：'+(/页面状态已过期|当前账号/.test(message)?message:'数据暂不可用，请稍后重试')};
  const problem=async response=>{throw await responseProblem(response)};
  const credentialRedirect=response=>response?.type==='opaqueredirect'||response?.status===0||(response?.status>=300&&response?.status<400);
  async function authorize(id,force=false){
    if(force)state.authorized.delete(id);
    if(state.authorized.has(id))return;
    if(authorizationTasks.has(id))return authorizationTasks.get(id);
    const task=(async()=>{
      const config=app(id);if(!config)throw new Error('相关功能暂不可用，请联系管理员');
      if(!csrfProvider)throw new Error('页面状态已过期，请刷新后重试');
      const token=await csrfProvider.token();
      const body=new URLSearchParams({csrf_token:token});
      const response=await fetch('/gateway/authorize/'+encodeURIComponent(id),{method:'POST',credentials:'same-origin',body,redirect:'follow'});
      if(!response.ok)await problem(response);
      state.authorized.add(id);
    })();
    authorizationTasks.set(id,task);
    try{return await task}finally{if(authorizationTasks.get(id)===task)authorizationTasks.delete(id)}
  }
  async function api(id,path,options={}){
    const config=app(id);if(!config||!/^\/[A-Za-z0-9_-]+(?:\/[A-Za-z0-9_-]+)*$/.test(config.route||''))throw new Error('相关功能暂不可用，请联系管理员');
    const evidencePath=id==='evidence-service'&&/^\/v1\/(?:upload-sessions(?:\/[0-9a-f-]+\/(?:complete|abort))?|files\/[0-9a-f-]+\/versions\/[1-9][0-9]*\/access-grants|references\/files\/[0-9a-f-]+\/versions\/[1-9][0-9]*)$/.test(path);
    if((!evidencePath&&!/^\/api\/v1\/[A-Za-z0-9_/?=&%.-]+$/.test(path))||path.includes('..'))throw new Error('业务请求路径无效');
    const method=String(options.method||'GET').toUpperCase(),mutating=method!=='GET'&&method!=='HEAD',request=()=>fetch(config.route+path,{...options,credentials:'same-origin',redirect:'manual',headers:{Accept:'application/json',...(options.headers||{})}});
    if(mutating)await authorize(id,true);
    let response=await request();
    if(credentialRedirect(response)){state.authorized.delete(id);await authorize(id,true);response=await request()}
    if(response.status===401){const error=await responseProblem(response);if(error.code==='credential_required'){state.authorized.delete(id);await authorize(id,true);response=await request()}else throw error}
    if(credentialRedirect(response))throw new Error('页面状态已过期，请刷新后重试');
    if(!response.ok)await problem(response);
    state.authorized.add(id);
    return response.status===204?{}:response.json();
  }
  async function evidenceFetch(path,options={}){
    const config=app('evidence-service'),value=String(path);
    if(!config||!/^\/evidence(?:\/|$)/.test(config.route||''))throw new Error('附件功能暂不可用，请稍后重试');
    if(!/^\/v1\/(?:upload-sessions(?:\/[0-9a-f-]+\/(?:complete|abort))?|files\/[0-9a-f-]+\/versions\/[1-9][0-9]*\/access-grants|references\/files\/[0-9a-f-]+\/versions\/[1-9][0-9]*)$/.test(value)||value.includes('..'))throw new Error('文件请求路径无效');
    const method=String(options.method||'GET').toUpperCase(),mutating=method!=='GET'&&method!=='HEAD';
    if(mutating)await authorize('evidence-service',true);
    const headers=new Headers(options.headers||{});if(!headers.has('Accept'))headers.set('Accept','application/json');
    const request=()=>fetch(config.route+value,{...options,credentials:'same-origin',redirect:'manual',headers});let response=await request();
    if(credentialRedirect(response)){state.authorized.delete('evidence-service');await authorize('evidence-service',true);response=await request()}
    if(response.status===401){const error=await responseProblem(response.clone());if(error.code==='credential_required'){state.authorized.delete('evidence-service');await authorize('evidence-service',true);response=await request()}}
    if(credentialRedirect(response))throw new Error('页面状态已过期，请刷新后重试');
    if(response.ok)state.authorized.add('evidence-service');
    return response;
  }
  const pagination=globalThis.OMSPhase1Pagination;
  if(!pagination)throw new Error('页面加载失败，请刷新后重试');
  const loadOffsetCollection=(id,base,params={})=>pagination.loadOffsetCollection(api,id,base,params);
  const loadCursorCollection=(id,base,params={})=>pagination.loadCursorCollection(api,id,base,params);
  const operatingPartyID=()=>state.operatingParty?.id||root.dataset.operatingPartyId||'';
  const isExternalParty=item=>Boolean(item?.id)&&item.id!==operatingPartyID();
  const activeExternalParties=()=>state.parties.filter(item=>item.status==='active'&&isExternalParty(item));
  function assertExternalCounterparty(party,label){if(!party||party.status!=='active')throw new Error('请选择有效'+label);if(!isExternalParty(party))throw new Error(label+'不能是本方经营主体，请选择真实往来主体')}
  function isOMSProject(item){return Array.isArray(item?.project_type_keys)&&item.project_type_keys.includes('operations')}
  async function ensureEvidenceRuntime(){
    if(globalThis.OMSPhase1Evidence?.createEvidenceTransfer)return globalThis.OMSPhase1Evidence;
    if(!evidenceRuntimeTask)evidenceRuntimeTask=new Promise((resolve,reject)=>{const script=document.createElement('script');script.src='assets/oms-phase1-evidence.js?v=20261022.6';script.async=true;script.addEventListener('load',()=>globalThis.OMSPhase1Evidence?.createEvidenceTransfer?resolve(globalThis.OMSPhase1Evidence):reject(new Error('附件功能暂不可用，请稍后重试')),{once:true});script.addEventListener('error',()=>reject(new Error('附件功能暂不可用，请稍后重试')),{once:true});document.head.append(script)}).catch(error=>{evidenceRuntimeTask=null;throw error});
    return evidenceRuntimeTask;
  }
  function evidenceTransfer(){
    if(state.evidenceTransfer)return state.evidenceTransfer;
    return ensureEvidenceRuntime().then(runtime=>{const factory=runtime.createEvidenceTransfer;if(typeof factory!=='function'||!app('evidence-service'))throw new Error('附件功能暂不可用，请稍后重试');state.evidenceTransfer=factory(evidenceFetch);return state.evidenceTransfer});
  }
  function evidenceFiles(data,name='evidence_files'){return [...(data.getAll(name)||[])].filter(value=>value instanceof File&&value.size>0)}
  async function uploadEvidence(files,{ownerRef,purpose,status}){
    if(files.length>20)throw new Error('一次最多上传 20 份附件');
    const refs=[];
    for(let index=0;index<files.length;index++){
      const file=files[index];
      if(file.size<=0)throw new Error(file.name+' 是空文件，未上传');
      if(status)status.textContent='正在上传 '+(index+1)+' / '+files.length+'：'+file.name;
      const result=await (await evidenceTransfer()).uploadFile({file,clientContext:{ownerRef,purpose},onProgress:progress=>{if(status)status.textContent='正在上传 '+file.name+' · '+Math.round(progress.percent)+'%'}});
      refs.push({file_id:result.version.fileId,version:result.version.version,purpose,display_name:result.version.fileName});
    }
    return refs;
  }
  async function evidenceDescriptor(ref){
    const value=await api('evidence-service','/v1/references/files/'+encodeURIComponent(ref.file_id)+'/versions/'+encodeURIComponent(String(ref.version)));
    return {fileId:ref.file_id,version:Number(ref.version),fileName:value.display_name||value.file_name||value.original_name||ref.display_name||'附件',sizeBytes:Number(value.size_bytes||value.file_size||1),mediaType:value.media_type||value.mime_type||'application/octet-stream'};
  }
  async function downloadEvidence(ref,status){
    if(status)status.textContent='正在申请附件下载授权…';
    const descriptor=await evidenceDescriptor(ref);await (await evidenceTransfer()).downloadFile(descriptor,descriptor.fileName);
    if(status)status.textContent='已触发下载：'+descriptor.fileName;
  }
  function previewableEvidenceName(name){return /\.(?:pdf|png|jpe?g|webp|gif|bmp)$/i.test(String(name||''))}
  function evidenceTypeLabel(name){const match=String(name||'').match(/\.([a-z0-9]+)$/i),extension=(match?.[1]||'文件').toUpperCase();return extension==='JPEG'?'JPG':extension.slice(0,4)}
  function evidenceMarkup(refs,empty='暂无附件'){
    if(!Array.isArray(refs)||!refs.length)return '<div class="oms-v1__evidence-empty">'+safeText(empty)+'</div>';
    return '<div class="oms-v1__evidence-list">'+refs.map(ref=>{const name=ref.display_name||'附件',attributes=' data-file-id="'+safeText(ref.file_id)+'" data-version="'+safeText(ref.version)+'" data-display-name="'+safeText(name)+'"';return '<div class="oms-v1__evidence-item"><span class="oms-v1__evidence-name" title="'+safeText(name)+'"><i>'+safeText(evidenceTypeLabel(name))+'</i><b>'+safeText(name)+'</b></span><span class="oms-v1__evidence-actions">'+(previewableEvidenceName(name)?'<button type="button" data-oms-v1-evidence-preview'+attributes+'>预览</button>':'')+'<button type="button" data-oms-v1-evidence-download'+attributes+'>下载</button></span></div>'}).join('')+'</div>';
  }
  function bindEvidenceDownloads(refs,status){
    for(const button of dialogBody.querySelectorAll('[data-oms-v1-evidence-preview]'))button.addEventListener('click',async()=>{button.disabled=true;try{const ref=refs.find(value=>value.file_id===button.dataset.fileId&&String(value.version)===button.dataset.version)||{file_id:button.dataset.fileId,version:Number(button.dataset.version),display_name:button.dataset.displayName};await openEvidencePreview(ref,status,button)}catch(error){if(status)status.textContent=error instanceof Error?error.message:'附件预览未完成'}finally{button.disabled=false}});
    for(const button of dialogBody.querySelectorAll('[data-oms-v1-evidence-download]'))button.addEventListener('click',async()=>{button.disabled=true;try{const ref=refs.find(value=>value.file_id===button.dataset.fileId&&String(value.version)===button.dataset.version)||{file_id:button.dataset.fileId,version:Number(button.dataset.version),display_name:button.dataset.displayName};await downloadEvidence(ref,status)}catch(error){if(status)status.textContent=error instanceof Error?error.message:'附件下载未完成'}finally{button.disabled=false}})
  }
  function wireEvidenceManager({refs,initialPending=[],ownerRef,purpose,canManage,replace,refresh}){
    const status=dialogBody.querySelector('[data-oms-v1-evidence-status]'),input=dialogBody.querySelector('[data-oms-v1-evidence-files]'),button=dialogBody.querySelector('[data-oms-v1-evidence-save]');
    bindEvidenceDownloads(refs,status);
    if(!button||!input)return;
    button.disabled=!canManage;let pending=[...initialPending];
    button.addEventListener('click',async()=>{
      button.disabled=true;
      try{
        if(!pending.length){const files=[...(input.files||[])].filter(file=>file.size>0);if(!files.length)throw new Error('请先选择附件');pending=await uploadEvidence(files,{ownerRef,purpose,status})}
        if(status)status.textContent='文件已保存，正在登记业务引用…';
        await replace([...refs,...pending]);pending=[];if(status)status.textContent='附件已关联';await refresh();
      }catch(error){if(status)status.textContent=(pending.length?'文件已上传，引用尚未登记；再次点击可安全重试。':'')+(error instanceof Error?' '+error.message:' 操作未完成');button.disabled=!canManage}
    });
  }
  async function partyByID(id){
    if(!id)return null;
    if(state.partyCache.has(id))return state.partyCache.get(id);
    const value=await api('party-directory','/api/v1/parties/'+encodeURIComponent(id));state.partyCache.set(id,value);return value;
  }

  function setNavGroupExpanded(group,expanded){
    if(!group)return;
    group.classList.toggle('is-expanded',expanded);
    group.querySelector(':scope > [data-oms-v1-nav]')?.setAttribute('aria-expanded',expanded?'true':'false');
  }
  function closeProjectDetail(){
    const detail=root.querySelector('[data-oms-v1-project-detail]');
    if(!detail||detail.hidden)return false;
    detail.hidden=true;
    state.selectedProjectID='';
    state.projectDetailToken++;
    document.body.style.overflow='';
    return true;
  }
  function activatePage(name,tab){
    closeProjectDetail();closeInlineFilter();
    const selectedTab=tab||activeTabs[name]||defaultTabs[name]||'';
    for(const page of root.querySelectorAll('[data-oms-v1-page]'))page.classList.toggle('is-active',page.dataset.omsV1Page===name);
    for(const button of root.querySelectorAll('[data-oms-v1-nav]')){const active=button.dataset.omsV1Nav===name;button.classList.toggle('is-active',active);if(active)button.setAttribute('aria-current','page');else button.removeAttribute('aria-current')}
    for(const group of root.querySelectorAll('[data-oms-v1-nav-group]')){const active=group.dataset.omsV1NavGroup===name;group.classList.toggle('is-active',active);if(active)setNavGroupExpanded(group,true)}
    for(const group of root.querySelectorAll('[data-oms-v1-mobile-subnav]'))group.classList.toggle('is-active',group.dataset.omsV1MobileSubnav===name);
    root.querySelector('.oms-v1__mobile-subnav')?.classList.toggle('is-visible',Boolean(defaultTabs[name]));
    contextGroup.textContent=labels[name]||'OMS';
    contexts.textContent=selectedTab?(tabLabels[selectedTab]||selectedTab):(labels[name]||'OMS');
    if(selectedTab){const page=root.querySelector('[data-oms-v1-page="'+name+'"]');activateTab(page,selectedTab)}
    root.querySelector('.oms-v1__surface')?.scrollTo({top:0,behavior:'instant'});
  }
  function activateTab(scope,name){
    if(!scope)return;
    const pageName=scope.dataset.omsV1Page;if(!pageName)return;activeTabs[pageName]=name;
    for(const button of root.querySelectorAll('[data-oms-v1-tab]')){const active=button.dataset.omsV1TabPage===pageName&&button.dataset.omsV1Tab===name;button.classList.toggle('is-active',active);if(active)button.setAttribute('aria-current','page');else button.removeAttribute('aria-current')}
    for(const panel of scope.querySelectorAll(':scope > [data-oms-v1-tab-panel]'))panel.classList.toggle('is-active',panel.dataset.omsV1TabPanel===name);
    contextGroup.textContent=labels[pageName]||'OMS';
    contexts.textContent=tabLabels[name]||name;
    const title=scope.querySelector('[data-oms-v1-page-title]');if(title)title.textContent=tabLabels[name]||name;
  }
  for(const button of root.querySelectorAll('[data-oms-v1-nav]'))button.addEventListener('click',()=>{
    const group=button.closest('[data-oms-v1-nav-group]');
    if(!group){activatePage(button.dataset.omsV1Nav);return}
    if(group.classList.contains('is-active')){if(closeProjectDetail())return;setNavGroupExpanded(group,!group.classList.contains('is-expanded'));return}
    setNavGroupExpanded(group,true);activatePage(button.dataset.omsV1Nav);
  });
  for(const button of root.querySelectorAll('[data-oms-v1-tab]'))button.addEventListener('click',()=>activatePage(button.dataset.omsV1TabPage,button.dataset.omsV1Tab));
  for(const button of root.querySelectorAll('[data-oms-v1-jump]'))button.addEventListener('click',()=>{const [page,tab]=button.dataset.omsV1Jump.split(':');activatePage(page,tab)});

  function emptyRow(columns,title,copy){return '<tr><td colspan="'+columns+'"><div class="oms-v1__empty"><strong>'+safeText(title)+'</strong><span>'+safeText(copy)+'</span></div></td></tr>'}
  function compareSortValue(left,right,direction){
    const leftEmpty=left===null||left===undefined||left==='',rightEmpty=right===null||right===undefined||right==='';
    if(leftEmpty||rightEmpty)return leftEmpty===rightEmpty?0:leftEmpty?1:-1;
    let result=0;
    if(typeof left==='bigint'||typeof right==='bigint'){const a=typeof left==='bigint'?left:BigInt(left),b=typeof right==='bigint'?right:BigInt(right);result=a===b?0:a>b?1:-1}
    else if(typeof left==='number'&&typeof right==='number')result=left===right?0:left>right?1:-1;
    else result=String(left).localeCompare(String(right),'zh-CN',{numeric:true,sensitivity:'base'});
    return direction==='asc'?result:-result;
  }
  function sortedItems(items,list,accessors){const sort=state.sorts[list];if(!sort||!accessors[sort.key])return items;return [...items].sort((left,right)=>compareSortValue(accessors[sort.key](left),accessors[sort.key](right),sort.direction))}
  function syncSortHeaders(list){const sort=state.sorts[list];for(const button of root.querySelectorAll('[data-oms-v1-sort-list="'+list+'"]')){const active=button.dataset.omsV1SortKey===sort?.key,direction=active?sort.direction:'';button.classList.toggle('is-active',active);button.dataset.sortDirection=direction;button.closest('th')?.setAttribute('aria-sort',active?(direction==='asc'?'ascending':'descending'):'none');button.setAttribute('aria-label',button.textContent.trim()+(active?(direction==='asc'?'，当前升序':'，当前降序'):'，点击排序'))}}
  function pageItems(items,key){
    const size=state.listPageSizes[key]||20,pages=Math.max(1,Math.ceil(items.length/size)),current=Math.min(state.listPages[key]||1,pages),start=(current-1)*size;
    state.listPages[key]=current;
    return {items:items.slice(start,start+size),total:items.length,pages,current,size};
  }
  function syncListPager(body,key,page,render){
    const wrap=body.closest('.oms-v1__table-wrap');if(!wrap)return;
    let pager=wrap.nextElementSibling?.dataset?.omsV1ListPager===key?wrap.nextElementSibling:null;
    if(!pager){pager=document.createElement('div');pager.className='oms-v1__pagination';pager.dataset.omsV1ListPager=key;wrap.after(pager)}
    pager.hidden=page.total<=page.size;
    pager.innerHTML='<span>第 '+page.current+' / '+page.pages+' 页</span><div><button type="button" data-list-prev '+(page.current<=1?'disabled':'')+'>‹</button><button type="button" class="is-current">'+page.current+'</button><button type="button" data-list-next '+(page.current>=page.pages?'disabled':'')+'>›</button></div><select data-list-size><option value="20" '+(page.size===20?'selected':'')+'>20 条/页</option><option value="50" '+(page.size===50?'selected':'')+'>50 条/页</option><option value="100" '+(page.size===100?'selected':'')+'>100 条/页</option></select>';
    pager.querySelector('[data-list-prev]').addEventListener('click',()=>{state.listPages[key]=Math.max(1,page.current-1);render()});
    pager.querySelector('[data-list-next]').addEventListener('click',()=>{state.listPages[key]=Math.min(page.pages,page.current+1);render()});
    pager.querySelector('[data-list-size]').addEventListener('change',event=>{state.listPageSizes[key]=parseInt(event.target.value,10)||20;state.listPages[key]=1;render()});
  }
  function renderProjectBars(){
    const target=root.querySelector('[data-oms-v1-project-bars]');
    root.querySelector('[data-oms-v1-project-count]').textContent=String(state.projects.length);
    if(!state.projects.length){target.innerHTML='<div class="oms-v1__empty"><strong>暂无项目</strong><span>新建项目后将在这里显示。</span></div>';return}
    target.innerHTML=state.projects.slice(0,5).map(item=>{const {effective,allocated,outstanding}=financialFacts(item.id,'receivable');if(effective===null||allocated===null||outstanding===null)return '<button class="oms-v1__project-bar" type="button" data-project-id="'+safeText(item.id)+'"><strong title="'+safeText(item.name)+'">'+safeText(item.name)+'</strong><div><i style="width:0%"></i></div><span>'+(state.commercial.has(item.id)?'尚无应收':'资金暂不可用')+'</span></button>';const progress=moneyPercent(allocated,effective);return '<button class="oms-v1__project-bar" type="button" data-project-id="'+safeText(item.id)+'" title="已回款 '+safeText(money(allocated))+'，未回款 '+safeText(money(outstanding))+'"><strong title="'+safeText(item.name)+'">'+safeText(item.name)+'</strong><div><i style="width:'+progress+'%"></i></div><span>回款 '+progress+'% · 未回 '+safeText(money(outstanding))+'</span></button>'}).join('');
    for(const button of target.querySelectorAll('[data-project-id]'))button.addEventListener('click',()=>openProject(button.dataset.projectId));
  }
  function projectMoneyStatus(projectID,direction){const {effective,allocated,outstanding}=financialFacts(projectID,direction);if(effective===null||allocated===null||outstanding===null)return 'none';if(allocated>effective||outstanding<0n)return 'anomaly';if(outstanding===0n)return 'complete';if(allocated===0n)return 'unstarted';return 'partial'}
  function projectMoneyStatusRank(projectID,direction){const status=projectMoneyStatus(projectID,direction);return status==='anomaly'?3:status==='complete'?1:2}
  function activeFilterCount(filter,defaults={}){return Object.entries(filter||{}).filter(([key,value])=>value!==null&&value!==undefined&&value!==''&&defaults[key]!==value).length}
  function updateFilterButton(button,filter,defaults={}){if(!button)return;const count=activeFilterCount(filter,defaults);button.textContent=count?'筛选 · '+count:'筛选';button.classList.toggle('is-filtered',count>0);button.setAttribute('aria-label',count?'筛选，已设置 '+count+' 个条件':'筛选')}
  function closeInlineFilter(restoreFocus=false){
    closeUnifiedSelect();
    activeFilterPanel?.remove();
    activeFilterPanel=null;
    if(activeFilterButton){activeFilterButton.setAttribute('aria-expanded','false');if(restoreFocus)activeFilterButton.focus()}
    activeFilterButton=null;
  }
  function showInlineFilter(button,{body,onApply,onReset}){
    const toggling=activeFilterButton===button;
    closeInlineFilter();
    if(toggling)return;
    const panel=document.createElement('section');panel.className='oms-v1__filter-panel';panel.setAttribute('aria-label','列表筛选');panel.innerHTML=body;
    const form=panel.querySelector('form');if(!form)return;
    form.insertAdjacentHTML('beforeend','<div class="oms-v1__filter-actions"><button type="button" data-filter-reset>重置</button><button type="button" data-filter-close>收起</button><button class="oms-v1__primary" type="submit">应用筛选</button><span data-filter-status></span></div>');
    const anchor=button.closest('.oms-v1__list-head')||button.closest('.oms-v1__toolbar')||button;anchor.after(panel);activeFilterPanel=panel;activeFilterButton=button;button.setAttribute('aria-expanded','true');enhanceUnifiedSelects(panel);
    const status=panel.querySelector('[data-filter-status]');
    form.addEventListener('submit',async event=>{event.preventDefault();if(!form.reportValidity())return;try{status.textContent='';await onApply(new FormData(form));closeInlineFilter()}catch(error){status.textContent=error instanceof Error?error.message:'筛选未完成'}});
    panel.querySelector('[data-filter-reset]').addEventListener('click',async()=>{await onReset();closeInlineFilter();button.focus()});
    panel.querySelector('[data-filter-close]').addEventListener('click',()=>closeInlineFilter(true));
    panel.querySelector('input,.oms-v1__select-trigger')?.focus();
  }
  document.addEventListener('keydown',event=>{if(event.key==='Escape'&&activeFilterPanel){event.preventDefault();event.stopImmediatePropagation();closeInlineFilter(true)}},{capture:true});
  function filteredProjects(){
    const query=(root.querySelector('[data-oms-v1-project-search]')?.value||'').trim().toLocaleLowerCase('zh-CN'),filter=state.filters.projects||{};
    const items=state.projects.filter(item=>{const contractorIDs=(item.resource_links||[]).filter(link=>link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at).map(link=>link.resource_id);return (!filter.client_id||item.client_party?.party_id===filter.client_id)&&(!filter.contractor_id||contractorIDs.includes(filter.contractor_id))&&(!filter.receivable_status||projectMoneyStatus(item.id,'receivable')===filter.receivable_status)&&(!filter.payable_status||projectMoneyStatus(item.id,'payable')===filter.payable_status)}).filter(item=>{if(!query)return true;const contractorNames=(item.resource_links||[]).filter(link=>link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at).map(link=>state.partyCache.get(link.resource_id)?.legal_name||'').join(' ');return [item.name,item.client_party?.legal_name,contractorNames,item.summary].some(value=>String(value||'').toLocaleLowerCase('zh-CN').includes(query))});
    return sortedItems(items,'projects',{name:item=>item.name,start_date:item=>item.planned_start_date,receivable:item=>projectMoneyStatusRank(item.id,'receivable'),payable:item=>projectMoneyStatusRank(item.id,'payable')});
  }
  function commercialObligations(projectID,direction){return (state.commercial.get(projectID)?.segments||[]).filter(segment=>segment.obligation?.direction===direction&&(segment.state==='available'||segment.state==='revision_changed'))}
  function commercialContracts(projectID,relationKey=''){return (state.commercial.get(projectID)?.segments||[]).filter(segment=>segment.resource_type==='contract'&&(!relationKey||segment.relation_key===relationKey)&&segment.contract?.current_effective_amount!==null&&segment.contract?.current_effective_amount!==undefined&&(segment.state==='available'||segment.state==='revision_changed'))}
  function contractCounterparty(segment,role){return segment.contract?.parties?.find(party=>party.role===role)}
  function authoritativePartyID(party){return party?.party_reference?.party_id||party?.id||''}
  function downstreamContractByParty(projectID,partyID){return commercialContracts(projectID,'downstream-contract').find(segment=>authoritativePartyID(contractCounterparty(segment,'施工方'))===partyID)}
  function sumObligationField(values,field){if(!values.length)return null;let total=0n;for(const segment of values){const value=decimalMinor(segment.obligation?.[field]);if(value===null)return null;total+=value}return total}
  function sumContractAmount(values){if(!values.length)return null;let total=0n;for(const segment of values){const value=decimalMinor(segment.contract?.current_effective_amount);if(value===null)return null;total+=value}return total}
  function projectCashFacts(projectID,type,partyID=''){let amount=0n,count=0,latestDate='';for(const item of state.cash){if(item.type!==type||item.status!=='recorded'||cashProject(item)?.id!==projectID)continue;const counterparty=type==='receipt'?item.payer:item.payee;if(partyID&&authoritativePartyID(counterparty)!==partyID)continue;const value=decimalMinor(item.effective_amount??item.amount);if(value===null)return {amount:null,count,latestDate};amount+=value;count++;if(String(item.occurred_on||'')>latestDate)latestDate=String(item.occurred_on||'')}return {amount,count,latestDate}}
  function projectCashAmount(projectID,type,partyID=''){return projectCashFacts(projectID,type,partyID).amount}
  function financialFacts(projectID,direction){const obligations=commercialObligations(projectID,direction);if(obligations.length)return {effective:sumObligationField(obligations,'effective_amount'),allocated:sumObligationField(obligations,'allocated_amount'),outstanding:sumObligationField(obligations,'outstanding_amount'),source:'obligation'};const contracts=commercialContracts(projectID,direction==='receivable'?'primary-contract':'downstream-contract'),effective=sumContractAmount(contracts);if(effective===null)return {effective:null,allocated:null,outstanding:null,source:''};const allocated=projectCashAmount(projectID,direction==='receivable'?'receipt':'payment');if(allocated===null)return {effective,allocated:null,outstanding:null,source:'contract'};return {effective,allocated,outstanding:effective-allocated,source:'contract'}}
  function moneyPercent(value,total){if(total<=0n)return 0;const raw=Number((value*100n)/total);return Math.max(0,Math.min(100,raw))}
  function obligationSum(projectID,direction,field){const facts=financialFacts(projectID,direction),value=field==='effective_amount'?facts.effective:field==='allocated_amount'?facts.allocated:field==='outstanding_amount'?facts.outstanding:null;return value===null?null:decimalMoney(minorDecimal(value))}
  function obligationDisplay(projectID,direction,field){return obligationSum(projectID,direction,field)||(state.commercial.has(projectID)?'尚无':'暂不可用')}
  function invoiceProject(item){const typed=item.source_refs?.find(source=>source.kind==='project'&&source.resource_id),typedProject=typed&&state.projects.find(project=>project.id===typed.resource_id);if(typedProject)return typedProject;for(const source of item.source_refs||[]){if(source.kind!=='manual_confirmation')continue;const match=String(source.display_summary||'').match(/^OMS项目 ([0-9a-f-]{36}) · /);if(match){const project=state.projects.find(value=>value.id===match[1]);if(project)return project}}return null}
  function invoiceProjectSource(project){if(project.status==='active'||project.status==='pending_completion')return {kind:'project',resource_id:project.id,source_revision:project.revision};return {kind:'manual_confirmation',manual_summary:'OMS项目 '+project.id+' · '+project.name}}
  function projectInvoiceFacts(projectID,direction,partyID=''){
    const configured=root.dataset.operatingPartyId||'';if(!configured)return {amount:null,count:null};
    let amount=0n,count=0,hidden=false,latestDate='';
    for(const item of state.invoices){
      const matchesDirection=direction==='outgoing'?authoritativePartyID(item.issuer_party)===configured:authoritativePartyID(item.recipient_party)===configured;
      if(item.status!=='recorded'||!matchesDirection||invoiceProject(item)?.id!==projectID)continue;
      const counterparty=direction==='outgoing'?item.recipient_party:item.issuer_party;
      if(partyID&&authoritativePartyID(counterparty)!==partyID)continue;
      count++;
      if(String(item.issued_on||'')>latestDate)latestDate=String(item.issued_on||'');
      if(!item.amount_visible||item.amount_minor_decimal===null||item.amount_minor_decimal===undefined){hidden=true;continue}
      try{amount+=BigInt(String(item.amount_minor_decimal))}catch{hidden=true}
    }
    return {amount:hidden?null:amount,count,latestDate};
  }
  function projectInvoiceCount(projectID,direction){return projectInvoiceFacts(projectID,direction).count}
  function projectInvoiceItems(projectID,direction){
    const configured=root.dataset.operatingPartyId||'';if(!configured)return[];
    return state.invoices.filter(item=>item.status==='recorded'&&invoiceProject(item)?.id===projectID&&(direction==='outgoing'?authoritativePartyID(item.issuer_party)===configured:authoritativePartyID(item.recipient_party)===configured)).sort((left,right)=>String(right.issued_on||'').localeCompare(String(left.issued_on||''))||String(right.created_at||'').localeCompare(String(left.created_at||'')));
  }
  function renderProjectInvoiceDetails(projectID){
    for(const direction of ['outgoing','incoming']){
      const items=projectInvoiceItems(projectID,direction),count=root.querySelector('[data-oms-v1-project-invoice-count="'+direction+'"]'),target=root.querySelector('[data-oms-v1-project-invoice-list="'+direction+'"]');
      count.textContent=items.length+' 笔';
      if(!items.length){target.innerHTML='<div class="oms-v1__detail-empty">'+(direction==='outgoing'?'暂无开票记录':'暂无收票记录')+'</div>';continue}
      target.innerHTML='<div class="oms-v1__invoice-row is-head '+(direction==='incoming'?'has-party':'')+'"><span>日期</span>'+(direction==='incoming'?'<span>施工方</span>':'')+'<span>金额</span><span>备注</span></div>'+items.map(item=>{const counterparty=item.issuer_party?.legal_name||'施工方待核对',amount=item.amount_visible?money(item.amount_minor_decimal):'未授权',note=item.note||'—';return '<div class="oms-v1__invoice-row '+(direction==='incoming'?'has-party':'')+'"><span>'+safeText(date(item.issued_on))+'</span>'+(direction==='incoming'?'<strong title="'+safeText(counterparty)+'">'+safeText(counterparty)+'</strong>':'')+'<b>'+safeText(amount)+'</b><span title="'+safeText(note)+'">'+safeText(note)+'</span></div>'}).join('');
    }
  }
  function projectMoneyStatusCell(projectID,direction){
    const status=projectMoneyStatus(projectID,direction),tone=status==='complete'?'complete':status==='anomaly'?'anomaly':'incomplete',label=status==='complete'?'已完成':status==='anomaly'?'金额异常':'未完成';
    return '<span class="oms-v1__project-status is-'+tone+'">'+label+'</span>';
  }
  function renderProjects(){
    const body=root.querySelector('[data-oms-v1-project-body]'),items=filteredProjects(),pages=Math.max(1,Math.ceil(items.length/state.projectPageSize));state.projectPage=Math.min(state.projectPage,pages);const start=(state.projectPage-1)*state.projectPageSize,page=items.slice(start,start+state.projectPageSize);
    updateFilterButton(root.querySelector('[data-oms-v1-filter]'),state.filters.projects||{});
    syncSortHeaders('projects');
    root.querySelector('[data-oms-v1-project-total]').textContent=String(items.length);root.querySelector('[data-oms-v1-project-page-copy]').textContent='第 '+state.projectPage+' / '+pages+' 页';root.querySelector('[data-oms-v1-project-prev]').disabled=state.projectPage<=1;root.querySelector('[data-oms-v1-project-next]').disabled=state.projectPage>=pages;
    const metrics={total:items.length,'receivable-complete':items.filter(item=>projectMoneyStatus(item.id,'receivable')==='complete').length,'payable-complete':items.filter(item=>projectMoneyStatus(item.id,'payable')==='complete').length,anomaly:items.filter(item=>projectMoneyStatus(item.id,'receivable')==='anomaly'||projectMoneyStatus(item.id,'payable')==='anomaly').length};
    for(const [key,value] of Object.entries(metrics)){const target=root.querySelector('[data-oms-v1-project-metric="'+key+'"]');if(target)target.textContent=String(value)}
    body.innerHTML=page.length?page.map(item=>'<tr data-oms-v1-row data-project-id="'+safeText(item.id)+'"><td><strong title="'+safeText(item.name)+'">'+safeText(item.name)+'</strong></td><td><strong title="'+safeText(item.client_party?.legal_name||'待补充')+'">'+safeText(item.client_party?.legal_name||'待补充')+'</strong></td><td class="is-date">'+safeText(date(item.planned_start_date))+'</td><td class="is-number">'+projectMoneyStatusCell(item.id,'receivable')+'</td><td class="is-number">'+projectMoneyStatusCell(item.id,'payable')+'</td><td><button class="oms-v1__row-action" type="button">详情 ›</button></td></tr>').join(''):emptyRow(6,'暂无匹配项目','请调整搜索或筛选条件。');
    for(const row of body.querySelectorAll('[data-project-id]'))row.addEventListener('click',()=>openProject(row.dataset.projectId));
    renderProjectBars();
  }
  function commitmentCounterparty(segment,direction){const role=direction==='receivable'?'debtor':'creditor';return segment.obligation?.parties?.find(party=>party.role===role)}
  function commitmentItems(direction){const items=[];for(const project of state.projects){const obligations=commercialObligations(project.id,direction);for(const segment of obligations)items.push({project,segment,counterparty:commitmentCounterparty(segment,direction),source:'obligation',facts:{effective:decimalMinor(segment.obligation.effective_amount),allocated:decimalMinor(segment.obligation.allocated_amount),outstanding:decimalMinor(segment.obligation.outstanding_amount)}});if(obligations.length)continue;const contracts=commercialContracts(project.id,direction==='receivable'?'primary-contract':'downstream-contract');if(direction==='receivable'){const facts=financialFacts(project.id,direction);if(contracts.length&&facts.effective!==null)items.push({project,segment:contracts[0],counterparty:project.client_party,source:'contract',facts,summary:contracts.length>1?'甲方合同（'+contracts.length+' 份）':contracts[0].display_summary});continue}for(const segment of contracts){const counterparty=contractCounterparty(segment,'施工方'),effective=decimalMinor(segment.contract.current_effective_amount),allocated=counterparty?projectCashAmount(project.id,'payment',authoritativePartyID(counterparty)):null;if(effective!==null)items.push({project,segment,counterparty,source:'contract',facts:{effective,allocated,outstanding:allocated===null?null:effective-allocated},summary:segment.display_summary})}}return items.sort((left,right)=>String(right.segment.linked_at||'').localeCompare(String(left.segment.linked_at||'')))}
  function commitmentStatus(item){const {effective,allocated,outstanding}=item.facts;if(effective===null||allocated===null||outstanding===null)return '';if(allocated>effective||outstanding<0n)return 'anomaly';if(outstanding===0n)return 'complete';if(allocated===0n)return 'none';return 'partial'}
  function filteredCommitmentItems(direction){
    const query=(root.querySelector('[data-oms-v1-commitment-search="'+direction+'"]')?.value||'').trim().toLocaleLowerCase('zh-CN');
    const filter=state.filters['commitment-'+direction]||{};
    const items=commitmentItems(direction).filter(item=>(!filter.project_id||item.project.id===filter.project_id)&&(!filter.counterparty_id||authoritativePartyID(item.counterparty)===filter.counterparty_id)&&(!filter.status||commitmentStatus(item)===filter.status)).filter(item=>!query||(item.project.name||'').toLocaleLowerCase('zh-CN').includes(query)||(item.counterparty?.legal_name||'').toLocaleLowerCase('zh-CN').includes(query)||(item.segment.display_summary||'').toLocaleLowerCase('zh-CN').includes(query));
    return sortedItems(items,'commitment-'+direction,{linked_at:item=>item.segment.linked_at,project:item=>item.project.name,counterparty:item=>item.counterparty?.legal_name,effective:item=>item.facts.effective,allocated:item=>item.facts.allocated,outstanding:item=>item.facts.outstanding});
  }
  function renderCommitments(direction){
    const items=filteredCommitmentItems(direction);
    const key='commitment-'+direction,page=pageItems(items,key),body=root.querySelector('[data-oms-v1-commitment-body="'+direction+'"]'),paidLabel=direction==='receivable'?'已回款':'已支付',emptyTitle=direction==='receivable'?'暂无应收记录':'暂无应付记录';
    updateFilterButton(root.querySelector('[data-oms-v1-commitment-filter="'+direction+'"]'),state.filters[key]||{});
    syncSortHeaders(key);
    root.querySelector('[data-oms-v1-commitment-total="'+direction+'"]').textContent=items.length+' 条';
    body.innerHTML=page.items.length?page.items.map(item=>{const counterparty=item.counterparty?.legal_name||item.summary||item.segment.display_summary||'往来主体待核对',subjectCells=direction==='receivable'?'<td><strong title="'+safeText(item.project.name)+'">'+safeText(item.project.name)+'</strong></td><td><span class="oms-v1__cell-text" title="'+safeText(counterparty)+'">'+safeText(counterparty)+'</span></td>':'<td><strong title="'+safeText(item.project.name)+'">'+safeText(item.project.name)+'</strong><small title="'+safeText(counterparty)+'">'+safeText(counterparty)+'</small></td>',paidCopy=direction==='receivable'?'':'<small>'+paidLabel+'</small>';return '<tr data-oms-v1-row data-project-id="'+safeText(item.project.id)+'" data-commitment-id="'+safeText(item.segment.link_id)+'" data-commitment-direction="'+direction+'" data-commitment-source="'+safeText(item.source)+'">'+subjectCells+'<td class="is-number"><strong>'+safeText(money(item.facts.effective))+'</strong></td><td class="is-number"><strong>'+safeText(money(item.facts.allocated))+'</strong>'+paidCopy+'</td><td class="is-number"><strong>'+safeText(money(item.facts.outstanding))+'</strong></td><td><button class="oms-v1__row-action" type="button">详情 ›</button></td></tr>'}).join(''):emptyRow(direction==='receivable'?6:5,emptyTitle,direction==='receivable'?'建立项目及应收后将在这里显示。':'关联施工方及应付后将在这里显示。');
    for(const row of body.querySelectorAll('[data-commitment-id]'))row.addEventListener('click',()=>openCommitment(row.dataset.projectId,row.dataset.commitmentId,row.dataset.commitmentDirection,row.dataset.commitmentSource));
    syncListPager(body,key,page,()=>renderCommitments(direction));
  }
  function partyListData(kind){
    const query=(root.querySelector('[data-oms-v1-party-search="'+kind+'"]')?.value||'').trim().toLocaleLowerCase('zh-CN');
    const clientCount=new Map(),contractorCount=new Map();for(const project of state.projects){const clientID=project.client_party?.party_id;if(clientID)clientCount.set(clientID,(clientCount.get(clientID)||0)+1);const contractorIDs=(project.resource_links||[]).filter(link=>link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at).map(link=>link.resource_id);for(const id of new Set(contractorIDs))contractorCount.set(id,(contractorCount.get(id)||0)+1)}
    const clientIDs=new Set(clientCount.keys()),contractorIDs=new Set(contractorCount.keys()),roleCount=kind==='client'?clientCount:contractorCount,key='party-'+kind,filtered=state.parties.filter(isExternalParty).filter(item=>partyVisibleForRole(kind,item.id,clientIDs,contractorIDs)).filter(item=>!query||(item.legal_name||'').toLocaleLowerCase('zh-CN').includes(query)||(item.registration_identifier||'').toLocaleLowerCase('zh-CN').includes(query)),items=sortedItems(filtered,key,{usage:item=>roleCount.get(item.id)||0,name:item=>item.legal_name,registration:item=>item.registration_identifier,status:item=>item.status});return {key,items,roleCount};
  }
  function partyVisibleForRole(kind,id,clientIDs,contractorIDs){return kind==='client'?clientIDs.has(id)||!contractorIDs.has(id):contractorIDs.has(id)}
  function renderParties(kind){
    const {key,items,roleCount}=partyListData(kind);
    syncSortHeaders(key);
    root.querySelector('[data-oms-v1-party-total="'+kind+'"]').textContent=String(items.length);
    const page=pageItems(items,key),body=root.querySelector('[data-oms-v1-party-body="'+kind+'"]');body.innerHTML=page.items.length?page.items.map(item=>{const count=roleCount.get(item.id)||0,usage=count?((kind==='client'?'已作为甲方关联 ':'已作为施工方关联 ')+count+' 个项目'):'尚未关联项目';return '<tr data-oms-v1-row data-party-id="'+safeText(item.id)+'"><td><strong title="'+safeText(item.legal_name)+'">'+safeText(item.legal_name)+'</strong><small>'+safeText((item.kind==='organization'?'企业 / 组织':item.kind||'主体')+' · '+usage)+'</small></td><td>'+safeText(item.registration_identifier||'—')+'</td><td><span class="oms-v1__tag '+(item.status==='active'?'':'is-muted')+'">'+safeText(item.status==='active'?'启用':'停用')+'</span></td><td><button class="oms-v1__row-action" type="button">详情 ›</button></td></tr>'}).join(''):emptyRow(4,kind==='client'?'暂无甲方档案':'暂无已关联项目的施工方',kind==='client'?'点击“新增甲方”录入第一条甲方档案。':'施工方与项目形成关联后将在这里显示。');
    for(const row of body.querySelectorAll('[data-party-id]'))row.addEventListener('click',()=>openParty(row.dataset.partyId,kind));
    syncListPager(body,key,page,()=>renderParties(kind));
  }
  function renderInvoices(){
    const configured=root.dataset.operatingPartyId||'';
    for(const direction of ['outgoing','incoming']){
      const body=root.querySelector('[data-oms-v1-invoice-body="'+direction+'"]');
      const total=root.querySelector('[data-oms-v1-invoice-total="'+direction+'"]');
      const key='invoice-'+direction;
      updateFilterButton(root.querySelector('[data-oms-v1-invoice-filter="'+direction+'"]'),state.filters[key]||{},{status:'recorded'});
      syncSortHeaders(key);
      if(!configured){const page=pageItems([],key);total.textContent='—';body.innerHTML=emptyRow(5,'票据信息暂不可用','请联系管理员检查系统配置。');syncListPager(body,key,page,renderInvoices);continue}
      const items=filteredInvoiceItems(direction);
      const page=pageItems(items,key);
      total.textContent=items.length+' 条';
      body.innerHTML=page.items.length?page.items.map(item=>{const project=invoiceProject(item),flow=(item.issuer_party?.legal_name||'开票方待核对')+' → '+(item.recipient_party?.legal_name||'收票方待核对');return '<tr data-oms-v1-row data-invoice-id="'+safeText(item.id)+'" data-invoice-direction="'+direction+'"><td>'+safeText(item.issued_on)+'</td><td><strong title="'+safeText(project?.name||'项目来源待核对')+'">'+safeText(project?.name||'项目来源待核对')+'</strong><small>'+safeText(flow)+'</small></td><td class="is-number"><strong>'+safeText(item.amount_visible?money(item.amount_minor_decimal):'未授权')+'</strong></td><td><span class="oms-v1__clamp" title="'+safeText(item.note||'')+'">'+safeText(item.note||'—')+'</span></td><td><button class="oms-v1__row-action" type="button">详情 ›</button></td></tr>'}).join(''):emptyRow(5,'暂无记录','新增记录后将在这里显示。');
      for(const row of body.querySelectorAll('[data-invoice-id]'))row.addEventListener('click',()=>openInvoice(row.dataset.invoiceId,row.dataset.invoiceDirection));
      syncListPager(body,key,page,renderInvoices);
    }
  }
  function filteredInvoiceItems(direction){
    const configured=root.dataset.operatingPartyId||'';if(!configured)return[];
    const key='invoice-'+direction,query=(root.querySelector('[data-oms-v1-invoice-search="'+direction+'"]')?.value||'').trim().toLocaleLowerCase('zh-CN'),filter=state.filters[key]||{};
    const items=state.invoices.filter(item=>direction==='outgoing'?authoritativePartyID(item.issuer_party)===configured:authoritativePartyID(item.recipient_party)===configured).filter(item=>{const project=invoiceProject(item);return (!filter.project_id||project?.id===filter.project_id)&&(!filter.date_from||item.issued_on>=filter.date_from)&&(!filter.date_to||item.issued_on<=filter.date_to)&&(filter.status==='all'||item.status==='recorded')}).filter(item=>{const project=invoiceProject(item),flow=(item.issuer_party?.legal_name||'')+' '+(item.recipient_party?.legal_name||'');return !query||(project?.name||'').toLocaleLowerCase('zh-CN').includes(query)||flow.toLocaleLowerCase('zh-CN').includes(query)||(item.note||'').toLocaleLowerCase('zh-CN').includes(query)});
    return sortedItems(items,key,{date:item=>item.issued_on,project:item=>invoiceProject(item)?.name,amount:item=>item.amount_visible?integerMinor(item.amount_minor_decimal):null});
  }
  function cashProject(item){const raw=String(item.manual_source_reference||''),id=raw.startsWith('project:')?raw.slice(8):'';return state.projects.find(project=>project.id===id)}
  function filteredCashItems(type){
    const key='cash-'+type,query=(root.querySelector('[data-oms-v1-cash-search="'+type+'"]')?.value||'').trim().toLocaleLowerCase('zh-CN'),filter=state.filters[key]||{};
    const items=state.cash.filter(item=>item.type===type).filter(item=>{const project=cashProject(item);return (!filter.project_id||project?.id===filter.project_id)&&(!filter.date_from||item.occurred_on>=filter.date_from)&&(!filter.date_to||item.occurred_on<=filter.date_to)&&(filter.status==='all'||item.status==='recorded')}).filter(item=>{const project=cashProject(item),counterparty=type==='receipt'?item.payer:item.payee;return !query||(project?.name||'').toLocaleLowerCase('zh-CN').includes(query)||(counterparty?.legal_name||'').toLocaleLowerCase('zh-CN').includes(query)||(item.description||'').toLocaleLowerCase('zh-CN').includes(query)});
    return sortedItems(items,key,{date:item=>item.occurred_on,project:item=>cashProject(item)?.name,amount:item=>item.amount_visible?decimalMinor(item.effective_amount??item.amount):null});
  }
  function renderCash(){
    for(const type of ['receipt','payment']){
      const key='cash-'+type,items=filteredCashItems(type);
      updateFilterButton(root.querySelector('[data-oms-v1-cash-filter="'+type+'"]'),state.filters[key]||{},{status:'recorded'});
      syncSortHeaders(key);
      const page=pageItems(items,key);
      root.querySelector('[data-oms-v1-cash-total="'+type+'"]').textContent=items.length+' 条';
      const body=root.querySelector('[data-oms-v1-cash-body="'+type+'"]');
      body.innerHTML=page.items.length?page.items.map(item=>{const project=cashProject(item),counterparty=type==='receipt'?item.payer:item.payee;return '<tr data-oms-v1-row data-cash-id="'+safeText(item.id)+'"><td>'+safeText(item.occurred_on)+'</td><td><strong title="'+safeText(project?.name||'项目来源待核对')+'">'+safeText(project?.name||'项目来源待核对')+'</strong><small>'+safeText(counterparty?.legal_name||'往来主体待核对')+'</small></td><td class="is-number"><strong>'+safeText(decimalMoney(item.amount_visible?item.amount:null))+'</strong></td><td><span class="oms-v1__clamp" title="'+safeText(item.description||'')+'">'+safeText(item.description||'—')+'</span></td><td><button class="oms-v1__row-action" type="button">详情 ›</button></td></tr>'}).join(''):emptyRow(5,type==='receipt'?'暂无回款记录':'暂无支付记录','有效记录会按'+cashDateLabel(type)+'倒序显示。');
      for(const row of body.querySelectorAll('[data-cash-id]'))row.addEventListener('click',()=>openCash(row.dataset.cashId));
      syncListPager(body,key,page,renderCash);
    }
  }
  function renderOverview(){
    const note=root.querySelector('[data-oms-v1-flow-note]'),period=root.querySelector('[data-oms-v1-period]')?.value||'',year=(period.match(/^(\d{4})/)||[])[1]||'';
    const bars=[...root.querySelectorAll('[data-oms-v1-month]')];
    const clearBars=()=>{for(const bar of bars){bar.querySelector('i').style.height='0%';bar.querySelector('b').style.height='0%';bar.removeAttribute('title')}};
    const applyProgress=(direction,paidSelector,outstandingSelector,rateSelector,donutSelector)=>{const values=state.projects.map(project=>financialFacts(project.id,direction)).filter(value=>value.effective!==null&&value.allocated!==null&&value.outstanding!==null),paidNode=root.querySelector(paidSelector),outstandingNode=root.querySelector(outstandingSelector),rateNode=root.querySelector(rateSelector),donut=root.querySelector(donutSelector);if(!values.length){paidNode.textContent='—';outstandingNode.textContent='—';rateNode.textContent='—';donut.style.setProperty('--value','0%');return}const effective=values.reduce((sum,value)=>sum+value.effective,0n),allocated=values.reduce((sum,value)=>sum+value.allocated,0n),outstanding=values.reduce((sum,value)=>sum+value.outstanding,0n),progress=moneyPercent(allocated,effective);paidNode.textContent=money(allocated);outstandingNode.textContent=money(outstanding);rateNode.textContent=progress+'%';donut.style.setProperty('--value',progress+'%')};
    applyProgress('receivable','[data-oms-v1-received]','[data-oms-v1-unreceived]','[data-oms-v1-receipt-rate]','.oms-v1__donut:not(.is-amber)');
    applyProgress('payable','[data-oms-v1-paid]','[data-oms-v1-unpaid]','[data-oms-v1-payment-rate]','.oms-v1__donut.is-amber');
    if(!state.moneyAvailable){note.textContent='实际资金来源暂不可用';clearBars();return}
    const relevant=state.cash.filter(item=>!year||String(item.occurred_on||'').startsWith(year+'-'));
    if(relevant.some(item=>!item.amount_visible)){note.textContent='当前账号无金额查看权限';clearBars();return}
    const receipts=Array(12).fill(0n),payments=Array(12).fill(0n);
    for(const item of relevant){const match=String(item.occurred_on||'').match(/^\d{4}-(\d{2})-\d{2}$/),value=decimalMinor(item.effective_amount??item.amount);if(!match||value===null)continue;const month=Number(match[1])-1;if(month<0||month>11)continue;if(item.type==='receipt')receipts[month]+=value;else if(item.type==='payment')payments[month]+=value}
    const maximum=[...receipts,...payments].reduce((largest,value)=>value>largest?value:largest,0n);
    bars.forEach((bar,index)=>{const receipt=receipts[index],payment=payments[index],height=value=>maximum===0n?0:Number((value*100n)/maximum);bar.querySelector('i').style.height=height(receipt)+'%';bar.querySelector('b').style.height=height(payment)+'%';bar.title=(index+1)+'月 · 回款 '+decimalMoney(minorDecimal(receipt))+' · 支付 '+decimalMoney(minorDecimal(payment))});
    note.textContent=(year?year+' 年':'全部时间')+' · '+relevant.length+' 笔有效流水';
  }
  function projectContractorItems(project){
    const parties=new Map(),links=(project.resource_links||[]).filter(link=>link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at),segments=commercialContracts(project.id,'downstream-contract');
    for(const link of links)parties.set(link.resource_id,{id:link.resource_id,party:state.partyCache.get(link.resource_id)||null,link,segment:null});
    for(const segment of segments){const party=contractCounterparty(segment,'施工方'),partyID=authoritativePartyID(party);if(!partyID)continue;const current=parties.get(partyID)||{id:partyID,party:state.partyCache.get(partyID)||party,link:null,segment:null};current.segment=segment;if(!current.party)current.party=party;parties.set(partyID,current)}
    return [...parties.values()].map(value=>{const payable=value.segment?decimalMinor(value.segment.contract?.current_effective_amount):null,paid=projectCashAmount(project.id,'payment',value.id),invoice=projectInvoiceFacts(project.id,'incoming',value.id);return {...value,payable,paid,invoice,outstanding:payable===null||paid===null?null:payable-paid}}).sort((left,right)=>String(right.link?.linked_at||right.segment?.linked_at||'').localeCompare(String(left.link?.linked_at||left.segment?.linked_at||'')));
  }
  function detailAmount(value,empty='尚无记录'){return value===null?empty:money(value)}
  function detailTone(total,actual){if(total===null||actual===null)return '';if(actual>total)return ' is-anomaly';if(total>0n&&actual===total)return ' is-complete';return ''}
  function dialogRecordDetail({status='',metrics=[],facts=[],remark=''}){const tones=new Set(['primary','positive','warning','danger']);return '<section class="oms-v1__record-detail">'+(status?'<div class="oms-v1__record-status"><span>当前状态</span><strong>'+safeText(status)+'</strong></div>':'')+(metrics.length?'<div class="oms-v1__record-metrics">'+metrics.map(item=>'<article class="'+(tones.has(item.tone)?'is-'+item.tone:'')+'"><span>'+safeText(item.label)+'</span><strong>'+safeText(item.value)+'</strong>'+(item.note?'<small>'+safeText(item.note)+'</small>':'')+'</article>').join('')+'</div>':'')+(facts.length?'<dl class="oms-v1__record-facts">'+facts.map(item=>'<div><dt>'+safeText(item.label)+'</dt><dd title="'+safeText(item.value)+'">'+safeText(item.value)+'</dd></div>').join('')+'</dl>':'')+(remark?'<div class="oms-v1__record-remark"><span>备注</span><p>'+safeText(remark)+'</p></div>':'')+'</section>'}
  function dialogFormState(status){return '<div class="oms-v1__record-status oms-v1__record-status--form"><span>当前状态</span><strong>'+safeText(status||'—')+'</strong></div>'}
  function renderProjectContractors(project,items,metadata=new Map()){
    const target=root.querySelector('[data-oms-v1-contractor-list]');
    if(!items.length){target.innerHTML='<div class="oms-v1__detail-empty">尚未关联施工方</div>';return}
    target.innerHTML=items.map((item,index)=>{const name=item.party?.legal_name||'施工方档案待核对',meta=item.segment?metadata.get(item.segment.resource_id):null,contract=item.segment?.contract,status=contractStatusLabel(meta?.workspace?.contract?.status||contract?.status),visibleStatus=status==='草稿'?'':status,attachments=meta?.evidence?.evidence_refs,invoiceCopy=item.invoice.count===null?'票据方向待配置':item.invoice.count+' 笔',contractCopy=item.segment?[visibleStatus,Array.isArray(attachments)?attachments.length+' 份附件':'附件读取中'].filter(Boolean).join(' · '):'尚未建立合同';return '<details class="oms-v1__contractor-item" '+(index===0?'open':'')+'><summary><span><i>施工方 '+String(index+1).padStart(2,'0')+'</i><strong title="'+safeText(name)+'">'+safeText(name)+'</strong><small>'+safeText(contractCopy)+'</small></span><b class="'+detailTone(item.payable,item.paid).trim()+'"><em>未支付</em>'+safeText(detailAmount(item.outstanding))+'</b></summary><div><dl><div><dt>应付总额</dt><dd>'+safeText(detailAmount(item.payable))+'</dd></div><div><dt>累计收票</dt><dd>'+safeText(detailAmount(item.invoice.amount))+'<small>'+safeText(invoiceCopy)+'</small></dd></div><div><dt>实际支付</dt><dd>'+safeText(detailAmount(item.paid))+'</dd></div><div><dt>未支付</dt><dd>'+safeText(detailAmount(item.outstanding))+'</dd></div></dl>'+(item.segment?'<button type="button" data-oms-v1-detail-contract="'+safeText(item.segment.resource_id)+'">查看合同与附件</button>':'<span class="oms-v1__detail-missing">可从“施工方应付”补充合同与金额</span>')+'</div></details>'}).join('');
    for(const button of target.querySelectorAll('[data-oms-v1-detail-contract]'))button.addEventListener('click',()=>openProjectContractEvidence(project,button.dataset.omsV1DetailContract));
  }
  function projectHistoryLabel(type){return ({project_created:'建立项目',project_updated:'更新项目信息',resource_linked:'关联业务资料',resource_unlinked:'解除业务资料',assignment_added:'新增项目成员',assignment_ended:'结束项目成员关系'})[type]||String(type||'项目记录').replaceAll('_',' ')}
  function renderProjectHistory(items){
    const target=root.querySelector('[data-oms-v1-project-history]');
    target.innerHTML=items.length?'<ol class="oms-v1__detail-events">'+items.slice(0,8).map(item=>'<li><i></i><div><strong>'+safeText(projectHistoryLabel(item.event_type))+'</strong><span>'+safeText(dateTime(item.occurred_at))+'</span></div></li>').join('')+'</ol>':'<div class="oms-v1__detail-empty">暂无项目记录</div>';
  }
  function renderContractChanges(records){
    const target=root.querySelector('[data-oms-v1-contract-changes]'),changes=[];
    for(const record of records){for(const instrument of record.workspace?.instruments||[]){if(instrument.instrument_type==='original_contract')continue;changes.push({label:record.label,instrument})}}
    changes.sort((left,right)=>String(right.instrument.effective_on||right.instrument.signed_on||'').localeCompare(String(left.instrument.effective_on||left.instrument.signed_on||'')));
    target.innerHTML=changes.length?'<ol class="oms-v1__detail-events oms-v1__detail-events--changes">'+changes.map(({label,instrument})=>{const effect=instrument.amount_effect==='adjustment'?'调整 '+decimalMoney(instrument.amount_value):instrument.amount_effect==='replacement'?'变更为 '+decimalMoney(instrument.amount_value):'金额不变';return '<li><i></i><div><strong>'+safeText(instrument.title||'合同变更')+'</strong><span>'+safeText(label)+' · '+safeText(effect)+' · '+safeText(date(instrument.effective_on||instrument.signed_on))+'</span></div></li>'}).join('')+'</ol>':'<div class="oms-v1__detail-empty">暂无已生效金额变更</div>';
  }
  async function readProjectContractRecord(segment){
    const [workspaceResult,evidenceResult]=await Promise.allSettled([api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(segment.resource_id)+'/workspace'),api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(segment.resource_id)+'/evidence')]);
    return {segment,label:contractEvidenceLabel(segment),workspace:workspaceResult.status==='fulfilled'?workspaceResult.value:null,evidence:evidenceResult.status==='fulfilled'?(evidenceResult.value.evidence||{revision:0,evidence_refs:[]}):null};
  }
  async function hydrateProjectDetail(project,contractors,token){
    const segments=commercialContracts(project.id),[contractRecords,historyResult]=await Promise.all([Promise.all(segments.map(readProjectContractRecord)),api('project-workspace','/api/v1/projects/'+encodeURIComponent(project.id)+'/history?limit=8').catch(()=>({items:[]}))]);
    if(token!==state.projectDetailToken||state.selectedProjectID!==project.id)return;
    const metadata=new Map(contractRecords.map(record=>[record.segment.resource_id,record])),primary=contractRecords.find(record=>record.segment.relation_key==='primary-contract'),primaryNode=root.querySelector('[data-oms-v1-upstream-contract]');
    if(primary){const status=contractStatusLabel(primary.workspace?.contract?.status||primary.segment.contract?.status),refs=primary.evidence?.evidence_refs,parts=[status==='草稿'?'':status,Array.isArray(refs)?refs.length+' 份附件':'附件暂不可用'];primaryNode.textContent=parts.filter(Boolean).join(' · ')}else primaryNode.textContent='尚未关联甲方合同';
    renderProjectContractors(project,contractors,metadata);renderContractChanges(contractRecords);renderProjectHistory(historyResult.items||[]);
  }
  function openProject(id){
    const item=state.projects.find(project=>project.id===id);if(!item)return;
    state.selectedProjectID=id;const token=++state.projectDetailToken,detail=root.querySelector('[data-oms-v1-project-detail]'),contractors=projectContractorItems(item),receivable=financialFacts(id,'receivable'),payable=financialFacts(id,'payable'),outgoing=projectInvoiceFacts(id,'outgoing'),incoming=projectInvoiceFacts(id,'incoming'),receipts=projectCashFacts(id,'receipt'),payments=projectCashFacts(id,'payment'),latestBusiness=[outgoing.latestDate,incoming.latestDate,receipts.latestDate,payments.latestDate].filter(Boolean).sort().at(-1)||'',values={'receivable-total':detailAmount(receivable.effective,'尚无应收'),'outgoing-invoice':detailAmount(outgoing.amount),'received':detailAmount(receivable.allocated),'unreceived':detailAmount(receivable.outstanding),'payable-total':detailAmount(payable.effective,'尚无应付'),'incoming-invoice':detailAmount(incoming.amount),'paid':detailAmount(payable.allocated),'unpaid':detailAmount(payable.outstanding)};
    detail.querySelector('[data-oms-v1-detail-name]').textContent=item.name||'—';detail.querySelector('[data-oms-v1-detail-client]').textContent='经营数据实时汇总'+(latestBusiness?' · 最近票款 '+date(latestBusiness):' · 暂无票款记录');detail.querySelector('[data-oms-v1-detail-date]').textContent=date(item.planned_start_date);detail.querySelector('[data-oms-v1-detail-client-name]').textContent=item.client_party?.legal_name||'待补充';detail.querySelector('[data-oms-v1-detail-contractors]').textContent=contractors.length+' 个';detail.querySelector('[data-oms-v1-detail-updated]').textContent=dateTime(item.updated_at||item.created_at);detail.querySelector('[data-oms-v1-detail-contractor-count]').textContent=contractors.length+' 个施工方';detail.querySelector('[data-oms-v1-detail-created]').textContent='创建于 '+dateTime(item.created_at);detail.querySelector('[data-oms-v1-detail-revision]').textContent='更新于 '+dateTime(item.updated_at||item.created_at);detail.querySelector('[data-oms-v1-detail-summary]').textContent=item.summary||'暂无项目说明';for(const node of detail.querySelectorAll('[data-oms-v1-flow]'))node.textContent=values[node.dataset.omsV1Flow]||'—';detail.querySelector('[data-oms-v1-flow-count="outgoing"]').textContent=outgoing.count===null?'暂不可用':outgoing.count+' 笔'+(outgoing.latestDate?' · 最近 '+date(outgoing.latestDate):'');detail.querySelector('[data-oms-v1-flow-count="incoming"]').textContent=incoming.count===null?'暂不可用':incoming.count+' 笔'+(incoming.latestDate?' · 最近 '+date(incoming.latestDate):'');detail.querySelector('[data-oms-v1-flow-count="receipt"]').textContent=receipts.count+' 笔'+(receipts.latestDate?' · 最近 '+date(receipts.latestDate):'');detail.querySelector('[data-oms-v1-flow-count="payment"]').textContent=payments.count+' 笔'+(payments.latestDate?' · 最近 '+date(payments.latestDate):'');detail.querySelector('[data-oms-v1-upstream-contract]').textContent='正在读取合同…';detail.querySelector('[data-oms-v1-contract-changes]').innerHTML='<div class="oms-v1__detail-empty">正在读取…</div>';detail.querySelector('[data-oms-v1-project-history]').innerHTML='<div class="oms-v1__detail-empty">正在读取…</div>';renderProjectInvoiceDetails(id);renderProjectContractors(item,contractors);detail.hidden=false;detail.scrollTop=0;document.body.style.overflow='hidden';hydrateProjectDetail(item,contractors,token).catch(()=>{if(token!==state.projectDetailToken)return;detail.querySelector('[data-oms-v1-upstream-contract]').textContent='合同信息暂不可用';detail.querySelector('[data-oms-v1-contract-changes]').innerHTML='<div class="oms-v1__detail-empty">请稍后重试</div>'});
  }
  root.querySelector('[data-oms-v1-project-back]').addEventListener('click',()=>closeProjectDetail());

  const dialog=root.querySelector('[data-oms-v1-dialog]'),dialogTitle=root.querySelector('[data-oms-v1-dialog-title]'),dialogKicker=root.querySelector('[data-oms-v1-dialog-kicker]'),dialogBody=root.querySelector('[data-oms-v1-dialog-body]'),dialogSubmit=root.querySelector('[data-oms-v1-dialog-submit]');
  const preview=root.querySelector('[data-oms-v1-preview]'),previewTitle=root.querySelector('[data-oms-v1-preview-title]'),previewMeta=root.querySelector('[data-oms-v1-preview-meta]'),previewBody=root.querySelector('[data-oms-v1-preview-body]'),previewDownload=root.querySelector('[data-oms-v1-preview-download]'),previewZoom=root.querySelector('[data-oms-v1-preview-zoom]'),previewZoomReset=root.querySelector('[data-oms-v1-preview-zoom-reset]');
  let dialogAction=null,dialogReturnFocus=null,currentPreview=null,previewScale=1,previewReturnFocus=null;
  function formatFileSize(value){const size=Number(value)||0;if(size<1024)return size+' B';if(size<1024*1024)return (size/1024).toFixed(size<10240?1:0)+' KB';return (size/1024/1024).toFixed(size<10*1024*1024?1:0)+' MB'}
  function setPreviewScale(next){previewScale=Math.max(.5,Math.min(3,next));const image=previewBody.querySelector('img');if(image)image.style.transform='scale('+previewScale+')';previewZoomReset.textContent=Math.round(previewScale*100)+'%'}
  function closeEvidencePreview(){if(currentPreview&&state.evidenceTransfer){try{state.evidenceTransfer.revokeFile(currentPreview)}catch{}}currentPreview=null;preview.hidden=true;previewBody.innerHTML='';previewDownload.disabled=true;previewZoom.hidden=true;document.body.classList.remove('oms-v1-preview-open');previewReturnFocus?.focus?.();previewReturnFocus=null}
  async function openEvidencePreview(ref,status,returnFocus){
    previewReturnFocus=returnFocus||document.activeElement;preview.hidden=false;document.body.classList.add('oms-v1-preview-open');previewTitle.textContent=ref.display_name||'附件';previewMeta.textContent='正在读取文件信息…';previewBody.innerHTML='<div class="oms-v1__preview-state">正在准备安全预览…</div>';previewDownload.disabled=true;previewZoom.hidden=true;setPreviewScale(1);
    try{
      const descriptor=await evidenceDescriptor(ref),transfer=await evidenceTransfer(),policy=transfer.getPreviewPolicy(descriptor);currentPreview=descriptor;previewTitle.textContent=descriptor.fileName;previewMeta.textContent=evidenceTypeLabel(descriptor.fileName)+' · '+formatFileSize(descriptor.sizeBytes);previewDownload.disabled=false;
      if(policy.mode==='download'){const reason=policy.reason==='size-limit'||policy.reason==='large-image'?'文件较大，当前安全策略仅允许下载后查看。':'当前文件格式不支持浏览器内安全预览。';previewBody.innerHTML='<div class="oms-v1__preview-state"><strong>暂不支持在线预览</strong><span>'+safeText(reason)+'</span><button type="button" data-oms-v1-preview-fallback-download>下载文件</button></div>';previewBody.querySelector('[data-oms-v1-preview-fallback-download]')?.addEventListener('click',()=>previewDownload.click());return}
      const url=await transfer.resolvePreviewUrl(descriptor,policy.mode),pdf=descriptor.mediaType.includes('pdf')||/\.pdf$/i.test(descriptor.fileName);
      if(pdf){previewBody.innerHTML='<iframe title="'+safeText(descriptor.fileName)+'" src="'+safeText(url)+'"></iframe>'}else{previewBody.innerHTML='<div class="oms-v1__preview-image"><img src="'+safeText(url)+'" alt="'+safeText(descriptor.fileName)+'"></div>';previewZoom.hidden=false;setPreviewScale(1)}
      if(status)status.textContent='正在预览：'+descriptor.fileName;
    }catch(error){previewBody.innerHTML='<div class="oms-v1__preview-state"><strong>预览未能打开</strong><span>'+safeText(error instanceof Error?error.message:'请下载文件后查看')+'</span></div>';previewMeta.textContent='预览失败';if(status)status.textContent=error instanceof Error?error.message:'附件预览未完成';throw error}
  }
  function showDialog({kicker='业务操作',title,body,submit='保存',enabled=false,onSubmit=null,dismissOnly=false}){closeUnifiedSelect();if(dialog.hidden)dialogReturnFocus=document.activeElement;dialogKicker.textContent=kicker;dialogTitle.textContent=title;dialogBody.innerHTML=body;dialogBody.scrollTop=0;dialogSubmit.textContent=submit;dialogSubmit.disabled=!enabled;dialogSubmit.hidden=dismissOnly;dialog.classList.toggle('is-dismiss-only',dismissOnly);const footerClose=dialog.querySelector('footer [data-oms-v1-dialog-close]');if(footerClose)footerClose.textContent=dismissOnly?'关闭':'取消';dialogAction=!dismissOnly&&enabled?onSubmit:null;dialog.hidden=false;document.body.classList.add('oms-v1-dialog-open');enhanceUnifiedSelects(dialogBody);const form=dialogBody.querySelector('form');if(form)form.addEventListener('submit',event=>{event.preventDefault();dialogSubmit.click()});const initialFocus=dialogBody.querySelector('input:not([disabled]),textarea:not([disabled]),.oms-v1__select-trigger:not([disabled])');if(initialFocus)initialFocus.focus();else dialog.querySelector('[data-oms-v1-dialog-close]')?.focus()}
  function closeDialog(){closeUnifiedSelect();dialog.hidden=true;dialogAction=null;document.body.classList.remove('oms-v1-dialog-open');dialogReturnFocus?.focus?.();dialogReturnFocus=null}
  dialogSubmit.addEventListener('click',async()=>{if(!dialogAction)return;const form=dialogBody.querySelector('form');if(form&&!form.reportValidity())return;const status=dialogBody.querySelector('[data-oms-v1-form-status]');dialogSubmit.disabled=true;const original=dialogSubmit.textContent;dialogSubmit.textContent=original.includes('下载')?'正在生成…':'正在保存…';if(status)status.textContent='';try{await dialogAction(form?new FormData(form):new FormData(),form)}catch(error){if(status)status.textContent=error instanceof Error?error.message:'操作未完成';else sync.textContent=error instanceof Error?error.message:'操作未完成';dialogSubmit.disabled=false;dialogSubmit.textContent=original}});
  for(const close of root.querySelectorAll('[data-oms-v1-dialog-close]'))close.addEventListener('click',closeDialog);dialog.addEventListener('click',event=>{if(event.target===dialog)closeDialog()});
  for(const close of root.querySelectorAll('[data-oms-v1-preview-close]'))close.addEventListener('click',closeEvidencePreview);preview.addEventListener('click',event=>{if(event.target===preview)closeEvidencePreview()});previewDownload.addEventListener('click',async()=>{if(!currentPreview)return;previewDownload.disabled=true;const original=previewDownload.textContent;previewDownload.textContent='准备中…';try{await (await evidenceTransfer()).downloadFile(currentPreview,currentPreview.fileName)}catch(error){previewMeta.textContent=error instanceof Error?error.message:'下载未完成'}finally{previewDownload.disabled=false;previewDownload.textContent=original}});root.querySelector('[data-oms-v1-preview-zoom-out]').addEventListener('click',()=>setPreviewScale(previewScale-.25));root.querySelector('[data-oms-v1-preview-zoom-in]').addEventListener('click',()=>setPreviewScale(previewScale+.25));previewZoomReset.addEventListener('click',()=>setPreviewScale(1));document.addEventListener('keydown',event=>{if(event.key==='Escape'&&!preview.hidden){event.preventDefault();event.stopImmediatePropagation();closeEvidencePreview()}},{capture:true});
  function exportDecimal(value){const minor=decimalMinor(value);return minor===null?'':minorDecimal(minor)}
  function exportMinor(value){try{return minorDecimal(BigInt(String(value)))}catch{return ''}}
  function csvCell(value){let text=String(value??'');if(/^\s*[=+\-@]/.test(text))text="'"+text;return '"'+text.replace(/"/g,'""')+'"'}
  function downloadCSV(filename,headers,rows){const content='\uFEFF'+[headers,...rows].map(row=>row.map(csvCell).join(',')).join('\r\n');const url=URL.createObjectURL(new Blob([content],{type:'text/csv;charset=utf-8'})),anchor=document.createElement('a');anchor.href=url;anchor.download=filename;anchor.hidden=true;document.body.append(anchor);anchor.click();anchor.remove();globalThis.setTimeout(()=>URL.revokeObjectURL(url),1000)}
  function exportDefinition(kind){
    if(kind==='projects'){const statuses={none:'尚未设置',unstarted:'未完成',partial:'未完成',complete:'已完成',anomaly:'金额异常'};return {title:'项目列表',headers:['项目','甲方','项目开启日期','应收状态','应付状态'],rows:filteredProjects().map(item=>[item.name,item.client_party?.legal_name||'待补充',item.planned_start_date||'',statuses[projectMoneyStatus(item.id,'receivable')]||'待核对',statuses[projectMoneyStatus(item.id,'payable')]||'待核对'])}}
    if(kind==='clients'||kind==='contractors'){const partyKind=kind==='clients'?'client':'contractor',{items,roleCount}=partyListData(partyKind);return {title:partyKind==='client'?'甲方列表':'施工方列表',headers:['名称','登记标识','状态','关联项目数'],rows:items.map(item=>[item.legal_name,item.registration_identifier||'',item.status==='active'?'启用':'停用',String(roleCount.get(item.id)||0)])}}
    const definitions={receivable:{title:'应收台账',direction:'receivable'},payable:{title:'应付台账',direction:'payable'}};
    if(definitions[kind]){const value=definitions[kind],received=value.direction==='receivable'?'已回款（元）':'已支付（元）',outstanding=value.direction==='receivable'?'未回款（元）':'未支付（元）',party=value.direction==='receivable'?'甲方':'施工方',statuses={none:'未发生',partial:'部分完成',complete:'全部完成',anomaly:'金额异常'};return {title:value.title,headers:['项目',party,value.direction==='receivable'?'应收总额（元）':'应付总额（元）',received,outstanding,'资金状态','备注'],rows:filteredCommitmentItems(value.direction).map(item=>[item.project.name,item.counterparty?.legal_name||'待核对',item.facts.effective===null?'':minorDecimal(item.facts.effective),item.facts.allocated===null?'':minorDecimal(item.facts.allocated),item.facts.outstanding===null?'':minorDecimal(item.facts.outstanding),statuses[commitmentStatus(item)]||'待核对',item.summary||item.segment.display_summary||''])}}
    if(kind==='outgoing'||kind==='incoming'){const outgoing=kind==='outgoing',title=outgoing?'开票记录':'施工方发票',rows=filteredInvoiceItems(kind).map(item=>{const project=invoiceProject(item),amount=item.amount_visible&&item.amount_minor_decimal!==null&&item.amount_minor_decimal!==undefined?exportMinor(item.amount_minor_decimal):'未授权',status=item.status==='recorded'?'有效':item.status==='registration_invalidated'||item.status==='invalidated'?'已作废':item.status||'待核对';return [item.issued_on||'',project?.name||'项目来源待核对',item.issuer_party?.legal_name||'开票方待核对',item.recipient_party?.legal_name||'收票方待核对',amount,status,item.note||'']});return {title,headers:[invoiceDateLabel(kind),'项目','开票方','收票方','金额（元）','状态','备注'],rows}}
    if(kind==='receipt'||kind==='payment'){const receipt=kind==='receipt',title=receipt?'回款记录':'支付记录',rows=filteredCashItems(kind).map(item=>{const project=cashProject(item),counterparty=receipt?item.payer:item.payee;return [item.occurred_on||'',project?.name||'项目来源待核对',counterparty?.legal_name||'往来主体待核对',item.amount_visible?exportDecimal(item.amount):'未授权',item.status==='recorded'?'有效':'已冲销',item.description||'']});return {title,headers:[cashDateLabel(kind),'项目',receipt?'付款方':'施工方','金额（元）','状态','备注'],rows}}
    return null;
  }
  function exportFilterKey(kind){if(kind==='projects')return 'projects';if(kind==='clients'||kind==='contractors')return 'export-'+kind;return kind==='receivable'||kind==='payable'?'commitment-'+kind:kind==='outgoing'||kind==='incoming'?'invoice-'+kind:'cash-'+kind}
  function exportWithFilter(kind,filter){const key=exportFilterKey(kind),previous=state.filters[key],hadPrevious=Object.prototype.hasOwnProperty.call(state.filters,key);state.filters[key]=filter;try{return exportDefinition(kind)}finally{if(hadPrevious)state.filters[key]=previous;else delete state.filters[key]}}
  function exportFilterFromData(kind,data){
    if(kind==='projects')return {client_id:String(data.get('client_id')||''),contractor_id:String(data.get('contractor_id')||''),receivable_status:String(data.get('receivable_status')||''),payable_status:String(data.get('payable_status')||'')};
    if(kind==='clients'||kind==='contractors')return {};
    if(kind==='receivable'||kind==='payable')return {project_id:String(data.get('project_id')||''),counterparty_id:String(data.get('counterparty_id')||''),status:String(data.get('status')||'')};
    return {project_id:String(data.get('project_id')||''),date_from:String(data.get('date_from')||''),date_to:String(data.get('date_to')||''),status:String(data.get('status')||'recorded')};
  }
  function exportFilterFields(kind,current){
    if(kind==='clients'||kind==='contractors')return '<div class="oms-v1__export-current">沿用当前搜索词与列表排序</div>';
    if(kind==='projects'){
      const clients=new Map(),contractors=new Map();for(const project of state.projects){if(project.client_party?.party_id)clients.set(project.client_party.party_id,project.client_party.legal_name||'甲方档案待核对');for(const link of project.resource_links||[]){if(link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at){const party=state.partyCache.get(link.resource_id);contractors.set(link.resource_id,party?.legal_name||'施工方档案待核对')}}}
      return '<label>甲方<select name="client_id">'+option('','全部甲方',current.client_id)+[...clients].map(([id,name])=>option(id,name,current.client_id)).join('')+'</select></label><label>施工方<select name="contractor_id">'+option('','全部施工方',current.contractor_id)+[...contractors].map(([id,name])=>option(id,name,current.contractor_id)).join('')+'</select></label><label>应收状态<select name="receivable_status">'+option('','全部状态',current.receivable_status)+option('none','尚未设置',current.receivable_status)+option('unstarted','未完成',current.receivable_status)+option('partial','未完成',current.receivable_status)+option('complete','已完成',current.receivable_status)+option('anomaly','金额异常',current.receivable_status)+'</select></label><label>应付状态<select name="payable_status">'+option('','全部状态',current.payable_status)+option('none','尚未设置',current.payable_status)+option('unstarted','未完成',current.payable_status)+option('partial','未完成',current.payable_status)+option('complete','已完成',current.payable_status)+option('anomaly','金额异常',current.payable_status)+'</select></label>';
    }
    const projectField='<label>项目<select name="project_id">'+option('','全部项目',current.project_id)+state.projects.map(item=>option(item.id,item.name,current.project_id)).join('')+'</select></label>';
    if(kind==='receivable'||kind==='payable'){
      const partyLabel=kind==='receivable'?'甲方':'施工方',parties=new Map();for(const item of commitmentItems(kind)){const id=authoritativePartyID(item.counterparty);if(id)parties.set(id,item.counterparty?.legal_name||partyLabel+'待核对')}
      return projectField+'<label>'+partyLabel+'<select name="counterparty_id">'+option('','全部'+partyLabel,current.counterparty_id)+[...parties].map(([id,name])=>option(id,name,current.counterparty_id)).join('')+'</select></label><label>资金状态<select name="status">'+option('','全部状态',current.status)+option('none','未发生',current.status)+option('partial','部分完成',current.status)+option('complete','全部完成',current.status)+option('anomaly','金额异常',current.status)+'</select></label>';
    }
    const dateLabel=kind==='outgoing'||kind==='incoming'?invoiceDateLabel(kind):cashDateLabel(kind),invalidLabel=kind==='outgoing'||kind==='incoming'?'包含已作废':'包含已冲销';
    return projectField+'<label>'+dateLabel+'从<input name="date_from" type="date" value="'+safeText(current.date_from||'')+'"></label><label>'+dateLabel+'至<input name="date_to" type="date" value="'+safeText(current.date_to||'')+'"></label><label>记录状态<select name="status">'+option('recorded','有效',current.status||'recorded')+option('all',invalidLabel,current.status)+'</select></label>';
  }
  function openExport(kind){
    const key=exportFilterKey(kind),current={...(state.filters[key]||{})},containsMoney=!['projects','clients','contractors'].includes(kind),defaultWarning=containsMoney?'金额以两位小数文本写入，可直接用 Excel 打开。':'CSV 文件沿用当前列表筛选与排序，可直接用 Excel 打开。';if(['outgoing','incoming','receipt','payment'].includes(kind)&&!current.status)current.status='recorded';const initial=exportWithFilter(kind,current);if(!initial)return;
    showDialog({kicker:'数据导出',title:'导出'+initial.title,submit:'下载 CSV',enabled:true,onSubmit:async data=>{const filter=exportFilterFromData(kind,data);if(filter.date_from&&filter.date_to&&filter.date_from>filter.date_to)throw new Error('开始日期不能晚于结束日期');const definition=exportWithFilter(kind,filter);if(!definition?.rows.length)throw new Error('当前筛选条件下没有可导出记录');downloadCSV('OMS-'+definition.title+'-'+today()+'.csv',definition.headers,definition.rows);closeDialog();sync.textContent=definition.title+'已按导出条件生成'},body:'<form class="oms-v1__export-form"><div class="oms-v1__export-filters">'+exportFilterFields(kind,current)+'</div><div class="oms-v1__export-summary"><strong data-oms-v1-export-count>正在计算…</strong><p>导出符合上述条件及当前搜索词的全部记录，不受列表分页限制。</p><div class="oms-v1__form-note" data-oms-v1-export-warning>'+defaultWarning+'</div><div class="oms-v1__form-status" data-oms-v1-form-status></div></div></form>'});
    const form=dialogBody.querySelector('form'),count=form.querySelector('[data-oms-v1-export-count]'),warning=form.querySelector('[data-oms-v1-export-warning]'),status=form.querySelector('[data-oms-v1-form-status]');
    const update=()=>{const filter=exportFilterFromData(kind,new FormData(form)),invalid=filter.date_from&&filter.date_to&&filter.date_from>filter.date_to,definition=invalid?null:exportWithFilter(kind,filter),total=definition?.rows.length||0;count.textContent=invalid?'日期范围有误':total+' 条记录';status.textContent=invalid?'开始日期不能晚于结束日期':total?'':'当前筛选条件下没有可导出记录';warning.textContent=total>500?'记录较多，生成文件可能需要一些时间；建议缩小筛选范围。':defaultWarning;dialogSubmit.disabled=invalid||total===0};
    form.addEventListener('input',update);form.addEventListener('change',update);update();
  }
  for(const button of root.querySelectorAll('[data-oms-v1-export]'))button.addEventListener('click',()=>openExport(button.dataset.omsV1Export));
  root.querySelector('[data-oms-v1-new-project]').addEventListener('click',()=>{
    const projectCommand=uuid(),contractCommand=uuid(),clientContractPartyCommand=uuid(),ownerContractPartyCommand=uuid(),linkCommand=uuid(),suffix=projectCommand.slice(0,8).toUpperCase(),contractRuntimeReady=state.contractContext?.party_runtime_mode==='directory'&&state.contractContext?.directory_authority_established===true,externalParties=activeExternalParties(),enabled=state.projectActions.has('project-workspace.create')&&state.projectActions.has('project-workspace.resource-link.manage')&&state.contractActions.has('draft.create')&&contractRuntimeReady&&state.operatingParty?.status==='active'&&externalParties.length>0;
    const disabledReason=!state.projectActions.has('project-workspace.create')?'当前账号没有项目创建权限':!state.projectActions.has('project-workspace.resource-link.manage')?'当前账号没有项目维护权限':!state.contractAvailable?'合同功能暂不可用，请稍后重试':!state.contractActions.has('draft.create')?'当前账号没有合同创建权限':!contractRuntimeReady?'合同功能尚未准备完成，请联系管理员':!state.operatingParty?'本公司信息暂不可用':state.operatingParty.status!=='active'?'本公司档案未启用':!externalParties.length?'请先创建甲方档案':'';
    showDialog({kicker:'项目经营',title:'新建项目',submit:'保存项目',enabled,onSubmit:async(data,form)=>{
      const status=form.querySelector('[data-oms-v1-form-status]'),name=String(data.get('name')||'').trim(),duplicate=state.projects.some(item=>(item.name||'').trim()===name);if(duplicate&&!globalThis.confirm('已有同名项目，仍要继续创建吗？建议使用可区分的名称。'))throw new Error('已取消同名项目创建');
      const selected=form.elements.client_party.selectedOptions[0],client=state.partyCache.get(String(selected?.value||''));assertExternalCounterparty(client,'甲方');
      const amount=decimalMinor(data.get('receivable_amount'));if(amount===null||amount<=0n)throw new Error('应收总额必须是大于 0 且最多两位小数的数字');
      const openedOn=String(data.get('opened_on')||today()),summary=String(data.get('summary')||'').trim(),projectNumber='OMS-'+openedOn.replaceAll('-','')+'-'+suffix,contractNumber='OMS-'+openedOn.replaceAll('-','')+'-'+suffix;
      status.textContent='1/5 正在建立项目…';
      const project=await api('project-workspace','/api/v1/projects',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:projectCommand,project_number:projectNumber,name,summary,planned_start_date:openedOn+'T00:00:00Z',planned_end_date:null,project_type_keys:['operations'],client_party:{party_id:client.id,source_revision:client.revision}})});
      status.textContent='2/5 项目已建立，正在保存甲方合同金额…';
      const contractResult=await api('contract-ledger','/api/v1/contracts',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:contractCommand,number_namespace:'oms-v1',contract_number:contractNumber,title:name+'甲方合同',category:'upstream-service',currency:'CNY',total_amount:minorDecimal(amount),signed_on:openedOn,effective_on:openedOn})}),contractID=contractResult.receipt?.contract_id;if(!contractID)throw new Error('合同保存未完成，请保留页面并重试');
      status.textContent='3/5 正在确认甲方信息…';
      const clientLink=await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(contractID)+'/parties',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:clientContractPartyCommand,expected_draft_version:contractResult.receipt?.draft_version||1,party_reference:{party_id:client.id,revision:client.revision},role:'甲方'})});
      status.textContent='4/5 正在确认本公司信息…';
      const ownerLink=await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(contractID)+'/parties',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:ownerContractPartyCommand,expected_draft_version:clientLink.receipt?.draft_version||2,party_reference:{party_id:state.operatingParty.id,revision:state.operatingParty.revision},role:'本方主体'})});
      status.textContent='5/5 项目和合同已建立，正在确认关联…';
      await api('project-workspace','/api/v1/projects/'+encodeURIComponent(project.id)+'/resource-links',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:linkCommand,expected_revision:project.revision,operation:'link',target_service:'contract-ledger',resource_type:'contract',resource_id:contractID,source_revision:ownerLink.receipt?.draft_version||3,relation_key:'primary-contract',note:'OMS 一期甲方合同'})});
      closeDialog();await loadAll();activatePage('projects','project-list');sync.textContent='项目、甲方合同和经营关联已保存';
    },body:'<form><label>项目名称 *<input name="name" required maxlength="200" placeholder="请输入项目名称"></label><div class="oms-v1__form-field"><div class="oms-v1__field-head"><span>甲方 *</span><button type="button" data-oms-v1-create-client>＋ 创建甲方</button></div><select name="client_party" required><option value="">请选择甲方</option>'+externalParties.map(item=>'<option value="'+safeText(item.id)+'" data-revision="'+safeText(item.revision)+'">'+safeText(item.legal_name)+'</option>').join('')+'</select></div><label>应收总额（元） *<input name="receivable_amount" required inputmode="decimal" placeholder="0.00"></label><label>项目开启日期<input name="opened_on" type="date" value="'+today()+'"></label><label>备注<textarea name="summary" maxlength="2000" placeholder="选填"></textarea></label><div class="oms-v1__form-status" data-oms-v1-form-status>'+safeText(disabledReason)+'</div></form>'});
    dialogBody.querySelector('[data-oms-v1-create-client]')?.addEventListener('click',()=>{
      closeDialog();
      activatePage('projects','clients');
      sync.textContent='请先创建甲方档案，完成后再返回项目列表新建项目';
      root.querySelector('[data-oms-v1-new-party="client"]')?.focus();
    });
  });
  for(const button of root.querySelectorAll('[data-oms-v1-new-party]'))button.addEventListener('click',()=>{const role=button.dataset.omsV1NewParty==='client'?'甲方':'施工方',enabled=state.partyActions.has('party-directory.create');showDialog({kicker:'主体档案',title:'新增'+role+'档案',submit:'保存主体',enabled,onSubmit:async data=>{const name=String(data.get('legal_name')||'').trim(),duplicate=state.parties.some(item=>(item.legal_name||'').trim()===name);if(duplicate&&!globalThis.confirm('已有同名主体档案，仍要继续创建吗？建议使用可区分的名称。'))throw new Error('已取消同名主体创建');const identifier=String(data.get('registration_identifier')||'').trim();await api('party-directory','/api/v1/parties',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),kind:String(data.get('kind')||'organization'),legal_name:name,registration_identifier:identifier||null})});closeDialog();await loadAll();activatePage('projects',role==='甲方'?'clients':'contractors');sync.textContent=role==='甲方'?'甲方档案“'+name+'”已保存，可在新建项目时直接选择':'主体档案“'+name+'”已保存；请在项目中选择为施工方，关联后将在当前列表显示';},body:'<form><label>名称 *<input name="legal_name" required maxlength="200" placeholder="请输入主体名称"></label><label>主体类型<select name="kind"><option value="organization">企业 / 组织</option><option value="individual">个人</option></select></label><label>统一社会信用代码或登记标识<input name="registration_identifier" maxlength="100" placeholder="选填"></label><div class="oms-v1__form-note">'+(role==='甲方'?'甲方档案保存后立即进入当前列表；项目关系建立后补充关联项目数。':'主体档案保持中性；在项目中选择为施工方后进入当前列表。')+'</div><div class="oms-v1__form-status" data-oms-v1-form-status>'+(enabled?'':'当前账号没有新增主体权限')+'</div></form>'})});
  function openInvoiceCreate(incoming){
    const operatingParty=state.operatingParty,enabled=!!operatingParty&&operatingParty.status==='active'&&state.invoiceActions.has('record')&&state.projects.length>0;
    const status=!root.dataset.operatingPartyId?'本公司信息尚未配置，请联系管理员':!operatingParty?'本公司信息暂不可用':operatingParty.status!=='active'?'本公司信息尚未启用':!state.invoiceActions.has('record')?'当前账号没有票据登记权限':!state.projects.length?'没有可选项目':'';
    showDialog({kicker:incoming?'应付支付':'应收回款',title:incoming?'新增施工方发票':'新增开票',submit:'保存票据',enabled,onSubmit:async(data,form)=>{
      const project=state.projects.find(item=>item.id===data.get('project_id')),counterparty=state.partyCache.get(String(data.get('counterparty_party_id')||''));if(!project||!operatingParty)throw new Error('请选择有效项目');assertExternalCounterparty(counterparty,incoming?'施工方':'甲方');
      const amount=decimalMinor(data.get('amount'));if(amount===null||amount<=0n)throw new Error('金额必须是大于 0 且最多两位小数的数字');if(amount>BigInt(Number.MAX_SAFE_INTEGER))throw new Error('金额超过浏览器安全提交范围，请联系管理员登记');
      const body={command_id:uuid(),issued_on:String(data.get('issued_on')||today()),amount_minor:Number(amount),currency:'CNY',issuer_party_ref:{id:(incoming?counterparty:operatingParty).id,revision:(incoming?counterparty:operatingParty).revision},recipient_party_ref:{id:(incoming?operatingParty:counterparty).id,revision:(incoming?operatingParty:counterparty).revision},source_refs:[invoiceProjectSource(project)],note:String(data.get('note')||'').trim()};
      const created=await api('billing-invoice','/api/v1/invoice-registrations',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}),files=evidenceFiles(data);let attachmentError='',pendingEvidence=[];
      if(files.length){const formStatus=form.querySelector('[data-oms-v1-form-status]');try{pendingEvidence=await uploadEvidence(files,{ownerRef:'billing-invoice:'+created.id,purpose:'invoice_document',status:formStatus});await api('billing-invoice','/api/v1/invoice-registrations/'+encodeURIComponent(created.id)+'/evidence',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_revision:created.revision,reason:'OMS 票据附件登记',evidence_refs:pendingEvidence})});pendingEvidence=[]}catch(error){attachmentError=error instanceof Error?error.message:'附件关联未完成'}}
      closeDialog();await loadAll();activatePage(incoming?'payable':'receivable',incoming?'incoming-invoices':'outgoing-invoices');const summary=projectInvoiceFacts(project.id,incoming?'incoming':'outgoing'),summaryText='；项目累计'+(incoming?'收票 ':'开票 ')+detailAmount(summary.amount)+'（'+summary.count+' 笔）';sync.textContent=(attachmentError?'票据已保存，附件未关联；请从详情重试：'+attachmentError:(files.length?'票据与附件已保存':'票据已保存'))+summaryText;if(attachmentError)await openInvoice(created.id,incoming?'incoming':'outgoing',attachmentError,pendingEvidence);
    },body:'<form><label>项目 *<select name="project_id" required><option value="">请选择项目</option>'+state.projects.map(item=>option(item.id,item.name,'')).join('')+'</select></label><label>'+(incoming?'施工方':'甲方')+' *<select name="counterparty_party_id" required disabled><option value="">选择项目后加载</option></select></label><label>金额（元） *<input name="amount" required inputmode="decimal" placeholder="0.00"></label><label>'+invoiceDateLabel(incoming?'incoming':'outgoing')+'<input name="issued_on" type="date" value="'+today()+'"></label><label>票据附件<input name="evidence_files" type="file" multiple accept=".pdf,.png,.jpg,.jpeg,.doc,.docx,.xls,.xlsx"></label><label>备注<textarea name="note" maxlength="1000" placeholder="选填；列表最多展示两行"></textarea></label><div class="oms-v1__form-status" data-oms-v1-form-status>'+safeText(status)+'</div></form>'});
    const form=dialogBody.querySelector('form'),projectSelect=form?.elements.project_id,counterpartySelect=form?.elements.counterparty_party_id;if(!projectSelect||!counterpartySelect)return;
    projectSelect.addEventListener('change',async()=>{const project=state.projects.find(item=>item.id===projectSelect.value),ids=incoming?(project?.resource_links||[]).filter(link=>link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at).map(link=>link.resource_id):[project?.client_party?.party_id].filter(Boolean),formStatus=dialogBody.querySelector('[data-oms-v1-form-status]');counterpartySelect.disabled=true;counterpartySelect.innerHTML='<option value="">正在读取'+(incoming?'施工方':'甲方')+'…</option>';try{const parties=(await Promise.all(ids.map(partyByID))).filter(item=>item?.status==='active'&&isExternalParty(item));counterpartySelect.innerHTML='<option value="">请选择'+(incoming?'施工方':'甲方')+'</option>'+parties.map(item=>option(item.id,item.legal_name,'')).join('');counterpartySelect.disabled=!enabled||!parties.length;if(parties.length===1)counterpartySelect.value=parties[0].id;if(!parties.length)formStatus.textContent=ids.includes(operatingPartyID())?'该项目甲方与本方经营主体相同，不能登记自开自收票据；请重新建立项目并选择真实甲方':incoming?'该项目尚未关联可用施工方':'该项目甲方档案不可用';else formStatus.textContent=''}catch(error){counterpartySelect.innerHTML='<option value="">往来主体读取失败</option>';formStatus.textContent=error.message}});
  }
  for(const button of root.querySelectorAll('[data-oms-v1-new-record]'))button.addEventListener('click',()=>openInvoiceCreate(button.dataset.omsV1NewRecord==='incoming-invoice'));
  function openCashCreate(receipt){
    const operatingParty=state.operatingParty,context=state.moneyContext||{},runtimeReady=context.party_runtime_mode==='directory'&&context.directory_authority_established===true,canRecord=Array.isArray(context.allowed_operations)&&context.allowed_operations.includes('cash.record'),enabled=!!operatingParty&&operatingParty.status==='active'&&runtimeReady&&canRecord&&state.projects.length>0;
    const status=!root.dataset.operatingPartyId?'本公司信息尚未配置，请联系管理员':!operatingParty?'本公司信息暂不可用':operatingParty.status!=='active'?'本公司信息尚未启用':!runtimeReady?'收付款功能尚未准备完成，请联系管理员':!canRecord?'当前账号没有实际资金登记权限':!state.projects.length?'没有可选项目':'';
    showDialog({kicker:receipt?'应收回款':'应付支付',title:receipt?'新增回款':'新增支付',submit:receipt?'保存回款':'保存支付',enabled,onSubmit:async(data,form)=>{
      const project=state.projects.find(item=>item.id===data.get('project_id')),counterparty=state.partyCache.get(String(data.get('counterparty_party_id')||''));if(!project||!operatingParty)throw new Error('请选择有效项目');assertExternalCounterparty(counterparty,receipt?'付款方':'施工方');
      const amount=decimalMinor(data.get('amount'));if(amount===null||amount<=0n)throw new Error('金额必须是大于 0 且最多两位小数的数字');
      const note=String(data.get('note')||'').trim(),account=String(data.get('account_reference')||'').trim();if(!account)throw new Error('请填写资金账户或流水依据');
      const created=await api('money-flow-ledger','/api/v1/cash-transactions',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),type:receipt?'receipt':'payment',ledger_owner_party_id:'',counterparty_party_id:'',ledger_owner_party_reference:{party_id:operatingParty.id,revision:operatingParty.revision},counterparty_party_reference:{party_id:counterparty.id,revision:counterparty.revision},description:note||(receipt?'项目回款':'项目支付'),amount:minorDecimal(amount),currency:'CNY',occurred_on:String(data.get('occurred_on')||today()),account_reference:account,manual_source_reference:'project:'+project.id,external_transaction_reference:''})}),transaction=created.transaction,files=evidenceFiles(data);let attachmentError='',pendingEvidence=[];
      if(files.length&&transaction?.id){const formStatus=form.querySelector('[data-oms-v1-form-status]');try{pendingEvidence=await uploadEvidence(files,{ownerRef:'money-flow-ledger:'+transaction.id,purpose:receipt?'receipt_proof':'payment_proof',status:formStatus});await api('money-flow-ledger','/api/v1/cash-transactions/'+encodeURIComponent(transaction.id)+'/evidence',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_revision:0,reason:'OMS '+(receipt?'回款':'支付')+'凭证登记',evidence_refs:pendingEvidence})});pendingEvidence=[]}catch(error){attachmentError=error instanceof Error?error.message:'附件关联未完成'}}
      closeDialog();await loadAll();activatePage(receipt?'receivable':'payable',receipt?'receipts':'payments');const financial=financialFacts(project.id,receipt?'receivable':'payable'),records=projectCashFacts(project.id,receipt?'receipt':'payment'),summaryText='；项目累计'+(receipt?'回款 ':'支付 ')+detailAmount(financial.allocated)+'（'+records.count+' 笔）';sync.textContent=(attachmentError?(receipt?'回款':'支付')+'已保存，凭证未关联；请从详情重试：'+attachmentError:(files.length?(receipt?'回款':'支付')+'与凭证已保存':(receipt?'回款':'支付')+'已保存'))+summaryText;if(attachmentError&&transaction?.id)await openCash(transaction.id,attachmentError,pendingEvidence);
    },body:'<form><label>项目 *<select name="project_id" required><option value="">请选择项目</option>'+state.projects.map(item=>option(item.id,item.name,'')).join('')+'</select></label><label>'+(receipt?'付款方':'施工方')+' *<select name="counterparty_party_id" required disabled><option value="">选择项目后加载</option></select></label><label>金额（元） *<input name="amount" required inputmode="decimal" placeholder="0.00"></label><label>'+cashDateLabel(receipt?'receipt':'payment')+'<input name="occurred_on" type="date" value="'+today()+'"></label><label>资金账户或流水依据 *<input name="account_reference" required maxlength="200" placeholder="例如：基本户 / 银行回单"></label><label>回单或凭证<input name="evidence_files" type="file" multiple accept=".pdf,.png,.jpg,.jpeg,.doc,.docx,.xls,.xlsx"></label><label>备注<textarea name="note" maxlength="200" placeholder="选填；列表最多展示两行"></textarea></label><div class="oms-v1__form-status" data-oms-v1-form-status>'+safeText(status)+'</div></form>'});
    const form=dialogBody.querySelector('form'),projectSelect=form?.elements.project_id,counterpartySelect=form?.elements.counterparty_party_id;if(!projectSelect||!counterpartySelect)return;
    projectSelect.addEventListener('change',async()=>{const project=state.projects.find(item=>item.id===projectSelect.value),ids=receipt?[project?.client_party?.party_id].filter(Boolean):(project?.resource_links||[]).filter(link=>link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at).map(link=>link.resource_id),formStatus=dialogBody.querySelector('[data-oms-v1-form-status]');counterpartySelect.disabled=true;counterpartySelect.innerHTML='<option value="">正在读取'+(receipt?'付款方':'施工方')+'…</option>';try{const parties=(await Promise.all(ids.map(partyByID))).filter(item=>item?.status==='active'&&isExternalParty(item));counterpartySelect.innerHTML='<option value="">请选择'+(receipt?'付款方':'施工方')+'</option>'+parties.map(item=>option(item.id,item.legal_name,'')).join('');counterpartySelect.disabled=!enabled||!parties.length;if(parties.length===1)counterpartySelect.value=parties[0].id;if(!parties.length)formStatus.textContent=ids.includes(operatingPartyID())?'该项目往来主体与本方经营主体相同，不能登记自己向自己收付款；请重新建立项目并选择真实甲方':receipt?'该项目甲方档案不可用':'该项目尚未关联可用施工方';else formStatus.textContent=''}catch(error){counterpartySelect.innerHTML='<option value="">往来主体读取失败</option>';formStatus.textContent=error.message}});
  }
  for(const button of root.querySelectorAll('[data-oms-v1-new-cash]'))button.addEventListener('click',()=>openCashCreate(button.dataset.omsV1NewCash==='receipt'));
  async function openCash(id,notice='',initialPending=[]){
    const item=state.cash.find(value=>value.id===id);if(!item)return;
    const receipt=item.type==='receipt',counterparty=receipt?item.payer:item.payee,project=cashProject(item),canManage=(item.allowed_operations||[]).includes('cash.evidence.manage')&&!!app('evidence-service');let snapshot={revision:0,evidence_refs:[]},evidenceError='';
    try{const result=await api('money-flow-ledger','/api/v1/cash-transactions/'+encodeURIComponent(id)+'/evidence');snapshot=result.evidence||snapshot}catch(error){evidenceError=error instanceof Error?error.message:'凭证读取未完成'}
    const refs=Array.isArray(snapshot.evidence_refs)?snapshot.evidence_refs:[];
    const record=dialogRecordDetail({status:item.status==='recorded'?'有效':'已冲销',metrics:[{label:receipt?'本次回款':'本次支付',value:decimalMoney(item.amount_visible?item.amount:null),tone:'primary'}],facts:[{label:cashDateLabel(item.type),value:date(item.occurred_on)},{label:'往来主体',value:counterparty?.legal_name||'待核对'}],remark:item.description||'暂无备注'});
    showDialog({kicker:receipt?'回款记录':'支付记录',title:project?.name||'项目来源待核对',dismissOnly:true,body:record+'<section class="oms-v1__evidence"><header><strong>'+(receipt?'回款凭证':'支付凭证')+'</strong><span>'+refs.length+' 份</span></header>'+evidenceMarkup(refs)+'<label>添加凭证<input type="file" multiple data-oms-v1-evidence-files accept=".pdf,.png,.jpg,.jpeg,.doc,.docx,.xls,.xlsx" '+(canManage?'':'disabled')+'></label><button type="button" data-oms-v1-evidence-save '+(canManage?'':'disabled')+'>保存附件</button><div class="oms-v1__form-status" data-oms-v1-evidence-status>'+safeText(notice||evidenceError||(!canManage?'当前账号没有凭证维护权限':''))+'</div></section>'});
    wireEvidenceManager({refs,initialPending,ownerRef:'money-flow-ledger:'+id,purpose:receipt?'receipt_proof':'payment_proof',canManage,replace:all=>api('money-flow-ledger','/api/v1/cash-transactions/'+encodeURIComponent(id)+'/evidence',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_revision:Number(snapshot.revision||0),reason:'OMS '+(receipt?'回款':'支付')+'凭证更新',evidence_refs:all})}),refresh:async()=>{await loadAll();await openCash(id)}});
  }
  async function openInvoice(id,direction,notice='',initialPending=[]){
    const item=state.invoices.find(value=>value.id===id);if(!item)return;
    const project=invoiceProject(item),status=item.status==='recorded'?'有效':item.status==='registration_invalidated'||item.status==='invalidated'?'已作废':item.status,refs=Array.isArray(item.evidence_refs)?item.evidence_refs:[],enabled=(item.allowed_actions||[]).includes('correct'),canManage=(item.allowed_actions||[]).includes('evidence_manage')&&!!app('evidence-service'),amountInput=item.amount_visible&&item.amount_minor_decimal!==null&&item.amount_minor_decimal!==undefined?minorDecimal(BigInt(String(item.amount_minor_decimal))):'';
    showDialog({kicker:direction==='incoming'?'施工方发票':'甲方开票',title:project?.name||'项目来源待核对',submit:'保存更正',enabled,onSubmit:async data=>{const patch={issued_on:String(data.get('issued_on')||today()),note:String(data.get('note')||'').trim()};if(item.amount_visible){const amount=decimalMinor(data.get('amount'));if(amount===null||amount<=0n)throw new Error('金额必须是大于 0 且最多两位小数的数字');if(amount>BigInt(Number.MAX_SAFE_INTEGER))throw new Error('金额超过浏览器安全提交范围，请联系管理员更正');patch.amount_minor=Number(amount)}await api('billing-invoice','/api/v1/invoice-registrations/'+encodeURIComponent(item.id)+'/corrections',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_revision:item.revision,reason:String(data.get('reason')||'').trim()||'OMS 票据登记更正',patch})});closeDialog();await loadAll();activatePage(direction==='incoming'?'payable':'receivable',direction==='incoming'?'incoming-invoices':'outgoing-invoices');sync.textContent='票据更正已保存，历史记录保留';},body:'<form>'+dialogFormState(status)+'<label>项目<input value="'+safeText(project?.name||'项目来源待核对')+'" disabled></label><label>开票方向<input value="'+safeText(item.issuer_party?.legal_name||'—')+' → '+safeText(item.recipient_party?.legal_name||'—')+'" disabled></label><label>'+invoiceDateLabel(direction)+'<input name="issued_on" type="date" value="'+safeText(item.issued_on||'')+'"></label>'+(item.amount_visible?'<label>金额（元）<input name="amount" required inputmode="decimal" value="'+safeText(amountInput)+'"></label>':'<label>金额<input value="当前账号无金额查看权限" disabled></label>')+'<label>备注<textarea name="note" maxlength="2000" placeholder="选填">'+safeText(item.note||'')+'</textarea></label><label>修改说明<textarea name="reason" maxlength="1000" placeholder="选填"></textarea></label><section class="oms-v1__evidence"><header><strong>票据附件</strong><span>'+refs.length+' 份</span></header>'+evidenceMarkup(refs)+'<label>添加附件<input type="file" multiple data-oms-v1-evidence-files accept=".pdf,.png,.jpg,.jpeg,.doc,.docx,.xls,.xlsx" '+(canManage?'':'disabled')+'></label><button type="button" data-oms-v1-evidence-save '+(canManage?'':'disabled')+'>保存附件</button><div class="oms-v1__form-status" data-oms-v1-evidence-status>'+safeText(notice||(!canManage?'当前账号没有票据附件维护权限':''))+'</div></section><div class="oms-v1__form-status" data-oms-v1-form-status>'+(enabled?'':'当前账号或记录状态不允许更正')+'</div></form>'});
    wireEvidenceManager({refs,initialPending,ownerRef:'billing-invoice:'+id,purpose:'invoice_document',canManage,replace:all=>api('billing-invoice','/api/v1/invoice-registrations/'+encodeURIComponent(id)+'/evidence',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_revision:item.revision,reason:'OMS 票据附件更新',evidence_refs:all})}),refresh:async()=>{await loadAll();await openInvoice(id,direction)}});
  }
  function openCommitment(projectID,linkID,direction,source){const item=commitmentItems(direction).find(value=>value.project.id===projectID&&value.segment.link_id===linkID&&value.source===source);if(!item)return;const {project,segment,counterparty,facts}=item,label=direction==='receivable'?'应收':'应付',actualLabel=direction==='receivable'?'已回款':'已支付',outstandingLabel=direction==='receivable'?'未回款':'未支付',stateLabel=facts.outstanding===null?'待核对':facts.outstanding<0n?'金额异常':facts.outstanding===0n?'已完成':facts.allocated===0n?'未开始':'进行中',detail=dialogRecordDetail({status:stateLabel,metrics:[{label:label+'总额',value:money(facts.effective),tone:'primary'},{label:actualLabel,value:money(facts.allocated),tone:'positive'},{label:outstandingLabel,value:money(facts.outstanding),tone:facts.outstanding!==null&&facts.outstanding<0n?'danger':'warning'}],facts:[{label:'往来主体',value:counterparty?.legal_name||'待核对'},{label:'计算依据',value:source==='contract'?'合同金额与实际收付款':'已确认应收应付记录'}],remark:item.summary||segment.display_summary||'暂无备注'});showDialog({kicker:label+'管理',title:project.name,dismissOnly:true,body:detail})}
  function openParty(id,kind){const item=state.parties.find(party=>party.id===id);if(!item)return;const enabled=(item.allowed_actions||[]).includes('update')&&item.status==='active',status=item.status!=='active'?'该主体已停用，启用后才能修改':enabled?'':'当前账号没有更正主体档案权限';showDialog({kicker:kind==='client'?'甲方档案':'施工方档案',title:item.legal_name,submit:'保存档案',enabled,onSubmit:async data=>{const name=String(data.get('legal_name')||'').trim(),duplicate=state.parties.some(party=>party.id!==item.id&&(party.legal_name||'').trim()===name);if(duplicate&&!globalThis.confirm('已有同名主体档案，仍要继续保存吗？建议使用可区分的名称。'))throw new Error('已取消同名主体保存');const identifier=String(data.get('registration_identifier')||'').trim();await api('party-directory','/api/v1/parties/'+encodeURIComponent(item.id),{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_revision:item.revision,kind:String(data.get('kind')||item.kind),legal_name:name,registration_identifier:identifier||null,reason:String(data.get('reason')||'').trim()||'OMS 主体档案更新'})});closeDialog();await loadAll();activatePage('projects',kind==='client'?'clients':'contractors');sync.textContent='主体档案已更新，历史业务记录保持不变';},body:'<form><label>名称 *<input name="legal_name" required maxlength="200" value="'+safeText(item.legal_name||'')+'"></label><label>主体类型<select name="kind">'+option('organization','企业 / 组织',item.kind)+option('individual','个人',item.kind)+'</select></label><label>统一社会信用代码或登记标识<input name="registration_identifier" maxlength="100" value="'+safeText(item.registration_identifier||'')+'" placeholder="选填"></label><label>修改说明<textarea name="reason" maxlength="1000" placeholder="选填"></textarea></label><div class="oms-v1__form-note">修改只影响当前档案，历史业务记录保持不变。</div><div class="oms-v1__form-status" data-oms-v1-form-status>'+safeText(status)+'</div></form>'})}
  root.querySelector('[data-oms-v1-project-edit]').addEventListener('click',()=>{
    const project=state.projects.find(item=>item.id===state.selectedProjectID);if(!project)return;
    const sourceComplete=!!project.project_number&&Array.isArray(project.project_type_keys)&&project.project_type_keys.length>0;
    const enabled=state.projectActions.has('project-workspace.update')&&sourceComplete;
    const status=!state.projectActions.has('project-workspace.update')?'当前账号没有编辑项目权限':!sourceComplete?'项目信息不完整，暂时无法编辑；请联系管理员':'';
    showDialog({kicker:'项目经营',title:'编辑项目',submit:'保存项目',enabled,onSubmit:async data=>{
      const name=String(data.get('name')||'').trim();
      const duplicate=state.projects.some(item=>item.id!==project.id&&(item.name||'').trim()===name);
      if(duplicate&&!globalThis.confirm('已有同名项目，仍要继续保存吗？建议使用可区分的名称。'))throw new Error('已取消同名项目保存');
      const plannedStart=String(data.get('planned_start_date')||'');
      await api('project-workspace','/api/v1/projects/'+encodeURIComponent(project.id),{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_revision:project.revision,project_number:project.project_number,name,summary:String(data.get('summary')||'').trim(),planned_start_date:plannedStart?plannedStart+'T00:00:00Z':null,planned_end_date:project.planned_end_date||null,project_type_keys:project.project_type_keys})});
      closeDialog();await loadAll();openProject(project.id);sync.textContent='项目基本信息已更新';
    },body:'<form><label>项目名称 *<input name="name" required maxlength="200" value="'+safeText(project.name||'')+'"></label><label>甲方<input value="'+safeText(project.client_party?.legal_name||'待补充')+'" disabled></label><label>项目开启日期<input name="planned_start_date" type="date" value="'+safeText(String(project.planned_start_date||'').slice(0,10))+'"></label><label>项目说明<textarea name="summary" maxlength="2000" placeholder="选填">'+safeText(project.summary||'')+'</textarea></label><div class="oms-v1__form-note">仅修改项目基本信息；甲方和金额请在对应业务板块中维护。</div><div class="oms-v1__form-status" data-oms-v1-form-status>'+safeText(status)+'</div></form>'});
  });
  function contractEvidenceLabel(segment){if(segment.relation_key==='primary-contract')return '甲方合同';const party=contractCounterparty(segment,'施工方');return party?.legal_name?'施工方 · '+party.legal_name:'施工方合同'}
  function contractStatusLabel(status){return ({draft:'草稿',active:'已生效',terminated:'已终止',closed:'已关闭',cancelled:'已取消'})[status]||status||'待核对'}
  function contractCounterpartyName(project,segment){return segment.relation_key==='primary-contract'?(project.client_party?.legal_name||'甲方待核对'):(contractCounterparty(segment,'施工方')?.legal_name||'施工方待核对')}
  function contractActualCash(project,segment){
    if(segment.relation_key==='primary-contract')return commercialContracts(project.id,'primary-contract').length===1?{label:'已回款',amount:projectCashAmount(project.id,'receipt')}:null;
    const partyID=authoritativePartyID(contractCounterparty(segment,'施工方'));
    if(!partyID)return null;
    const matches=commercialContracts(project.id,'downstream-contract').filter(value=>authoritativePartyID(contractCounterparty(value,'施工方'))===partyID);
    return matches.length===1?{label:'已支付',amount:projectCashAmount(project.id,'payment',partyID)}:null;
  }
  async function openProjectContractEditor(project,segment,workspace){
    const contract=workspace?.contract,canEdit=contract?.status==='draft'&&workspace?.allowed_operations?.includes('draft.update');
    if(!contract||!canEdit){await openProjectContractEvidence(project,segment.resource_id,'该合同当前不能在一期工作台直接编辑');return}
    const actual=contractActualCash(project,segment),amount=decimalMinor(contract.total_amount),actualAmount=actual?.amount??null;
    showDialog({kicker:'合同详情',title:'编辑'+contractEvidenceLabel(segment),submit:'保存合同',enabled:true,onSubmit:async(data,form)=>{
      const nextAmount=decimalMinor(data.get('total_amount'));if(nextAmount===null||nextAmount<=0n)throw new Error('合同金额必须是大于 0 且最多两位小数的数字');
      if(actualAmount!==null&&nextAmount<actualAmount&&!globalThis.confirm('新合同金额 '+money(nextAmount)+' 低于'+actual.label+' '+money(actualAmount)+'，保存后会出现金额异常。仍要继续吗？'))throw new Error('已取消金额异常的合同修改');
      form.querySelector('[data-oms-v1-form-status]').textContent='正在更新合同…';
      await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(contract.id),{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_draft_version:contract.draft_version,number_namespace:contract.number_namespace,contract_number:contract.contract_number,title:String(data.get('title')||'').trim(),category:contract.category,currency:contract.currency,total_amount:minorDecimal(nextAmount),signed_on:String(data.get('signed_on')||''),effective_on:String(data.get('effective_on')||'')})});
      closeDialog();await loadAll();const current=state.projects.find(value=>value.id===project.id)||project;await openProjectContractEvidence(current,segment.resource_id,'合同信息已更新，历史版本已保留');sync.textContent='合同信息已更新';
    },body:'<form><div class="oms-v1__contract-edit-context"><span>'+safeText(contractEvidenceLabel(segment))+'</span><strong>'+safeText(contractCounterpartyName(project,segment))+'</strong></div><label>合同名称 *<input name="title" required maxlength="300" value="'+safeText(contract.title||'')+'"></label><label>合同金额（元） *<input name="total_amount" required inputmode="decimal" value="'+safeText(amount===null?'':minorDecimal(amount))+'"></label><div class="oms-v1__contract-dates"><label>签订日期<input name="signed_on" type="date" value="'+safeText(contract.signed_on||'')+'"></label><label>生效日期<input name="effective_on" type="date" value="'+safeText(contract.effective_on||'')+'"></label></div><div class="oms-v1__amount-warning" data-oms-v1-contract-amount-warning hidden></div><div class="oms-v1__form-status" data-oms-v1-form-status></div></form>'});
    const input=dialogBody.querySelector('[name="total_amount"]'),warning=dialogBody.querySelector('[data-oms-v1-contract-amount-warning]');
    const updateWarning=()=>{if(!warning||actualAmount===null)return;const value=decimalMinor(input?.value);const visible=value!==null&&value<actualAmount;warning.hidden=!visible;warning.textContent=visible?'当前输入低于'+actual.label+' '+money(actualAmount)+'，保存时必须再次确认。':''};
    input?.addEventListener('input',updateWarning);updateWarning();
  }
  async function openProjectContractEvidence(project,selectedID='',notice='',initialPending=[]){
    const contracts=commercialContracts(project.id),segment=contracts.find(value=>value.resource_id===selectedID)||contracts[0];
    if(!segment){showDialog({kicker:'合同详情',title:project.name,dismissOnly:true,body:'<div class="oms-v1__evidence-empty">当前项目尚未关联可用合同。</div>'});return}
    let snapshot={revision:0,evidence_refs:[]},workspace=null,readErrors=[];
    try{workspace=await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(segment.resource_id)+'/workspace')}catch(error){readErrors.push(error instanceof Error?error.message:'合同信息读取未完成')}
    try{const result=await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(segment.resource_id)+'/evidence');snapshot=result.evidence||snapshot}catch(error){readErrors.push(error instanceof Error?error.message:'合同附件读取未完成')}
    const contract=workspace?.contract,amount=decimalMinor(contract?.total_amount??segment.contract?.current_effective_amount),refs=Array.isArray(snapshot.evidence_refs)?snapshot.evidence_refs:[],canManage=state.contractActions.has('evidence.manage')&&!!app('evidence-service'),canEdit=contract?.status==='draft'&&workspace?.allowed_operations?.includes('draft.update'),purpose=segment.relation_key==='primary-contract'?'contract_document':'contractor_agreement',status=contractStatusLabel(contract?.status||segment.contract?.status),title=contract?.title||contractEvidenceLabel(segment),signedOn=contract?.signed_on||'',effectiveOn=contract?.effective_on||'';
    showDialog({kicker:'合同详情',title:project.name,dismissOnly:true,body:'<div class="oms-v1__form-field"><span>查看合同</span><select data-oms-v1-contract-evidence-select>'+contracts.map(value=>option(value.resource_id,contractEvidenceLabel(value),segment.resource_id)).join('')+'</select></div><section class="oms-v1__contract-summary"><header><div><span>'+safeText(contractEvidenceLabel(segment))+'</span><strong>'+safeText(title)+'</strong></div>'+(status==='草稿'?'':'<span class="oms-v1__tag is-muted">'+safeText(status)+'</span>')+'</header><div class="oms-v1__contract-amount"><span>合同金额</span><strong>'+safeText(money(amount))+'</strong></div><dl><div><dt>往来主体</dt><dd>'+safeText(contractCounterpartyName(project,segment))+'</dd></div><div><dt>签订日期</dt><dd>'+safeText(date(signedOn))+'</dd></div><div><dt>生效日期</dt><dd>'+safeText(date(effectiveOn))+'</dd></div></dl>'+(canEdit?'<button type="button" data-oms-v1-contract-edit>编辑合同</button>':'')+'</section><section class="oms-v1__evidence"><header><strong>合同附件</strong><span>'+refs.length+' 份</span></header>'+evidenceMarkup(refs)+'<label>添加合同文件<input type="file" multiple data-oms-v1-evidence-files accept=".pdf,.png,.jpg,.jpeg,.doc,.docx,.xls,.xlsx" '+(canManage?'':'disabled')+'></label><button type="button" data-oms-v1-evidence-save '+(canManage?'':'disabled')+'>保存附件</button><div class="oms-v1__form-status" data-oms-v1-evidence-status>'+safeText(notice||readErrors.join('；')||(!canManage?'当前账号没有合同附件维护权限':''))+'</div></section>'});
    dialogBody.querySelector('[data-oms-v1-contract-evidence-select]')?.addEventListener('change',event=>openProjectContractEvidence(project,event.target.value));
    dialogBody.querySelector('[data-oms-v1-contract-edit]')?.addEventListener('click',()=>openProjectContractEditor(project,segment,workspace));
    wireEvidenceManager({refs,initialPending,ownerRef:'contract-ledger:'+segment.resource_id,purpose,canManage,replace:all=>api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(segment.resource_id)+'/evidence',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_revision:Number(snapshot.revision||0),reason:'OMS 合同附件更新',evidence_refs:all})}),refresh:()=>openProjectContractEvidence(project,segment.resource_id)});
  }
  root.querySelector('[data-oms-v1-project-evidence]').addEventListener('click',()=>{const project=state.projects.find(item=>item.id===state.selectedProjectID);if(project)openProjectContractEvidence(project)});
  function contractDirectoryReady(){return state.contractContext?.party_runtime_mode==='directory'&&state.contractContext?.directory_authority_established===true}
  function projectContractorLinked(project,partyID){return (project.resource_links||[]).some(link=>link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at&&link.resource_id===partyID)}
  async function createDownstreamContract(project,party,amount,note,status){
    if(!state.operatingParty?.id||state.operatingParty.status!=='active')throw new Error('本公司信息暂不可用');
    assertExternalCounterparty(party,'施工方');
    const seed='oms-v1:downstream:'+project.id+':'+party.id,commands=await Promise.all(['contract','contractor-party','owner-party','project-party','project-contract'].map(kind=>stableUUID(seed+':'+kind))),openedOn=String(project.planned_start_date||'').slice(0,10)||today(),contractNumber='OMS-D-'+project.id+'-'+party.id;
    status.textContent='1/5 正在建立施工方合同…';
    const contractResult=await api('contract-ledger','/api/v1/contracts',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:commands[0],number_namespace:'oms-v1-downstream',contract_number:contractNumber,title:project.name+' · '+party.legal_name+'施工协议',category:'downstream-service',currency:'CNY',total_amount:minorDecimal(amount),signed_on:openedOn,effective_on:openedOn})}),contractID=contractResult.receipt?.contract_id;if(!contractID)throw new Error('施工方合同保存未完成，请保留页面并重试');
    const workspace=await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(contractID)+'/workspace'),activeParties=Array.isArray(workspace.parties)?workspace.parties.filter(value=>value.active):[];
    let draftVersion=Number(workspace.contract?.draft_version||contractResult.receipt?.draft_version||1);
    if(!activeParties.some(value=>(value.party_reference?.party_id||value.party_id)===party.id&&value.role==='施工方')){
      status.textContent='2/5 正在确认施工方信息…';
      const contractorLink=await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(contractID)+'/parties',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:commands[1],expected_draft_version:draftVersion,party_reference:{party_id:party.id,revision:party.revision},role:'施工方'})});
      draftVersion=Number(contractorLink.receipt?.draft_version||draftVersion+1);
    }
    if(!activeParties.some(value=>(value.party_reference?.party_id||value.party_id)===state.operatingParty.id&&['本方主体','资质单位'].includes(value.role))){
      status.textContent='3/5 正在确认本公司信息…';
      const ownerLink=await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(contractID)+'/parties',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:commands[2],expected_draft_version:draftVersion,party_reference:{party_id:state.operatingParty.id,revision:state.operatingParty.revision},role:'本方主体'})});
      draftVersion=Number(ownerLink.receipt?.draft_version||draftVersion+1);
    }
    let expectedRevision=project.revision;
    if(!projectContractorLinked(project,party.id)){
      status.textContent='4/5 正在关联项目施工方…';
      const partyResult=await api('project-workspace','/api/v1/projects/'+encodeURIComponent(project.id)+'/resource-links',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:commands[3],expected_revision:expectedRevision,operation:'link',link_id:'',target_service:'party-directory',resource_type:'party',resource_id:party.id,source_revision:party.revision,relation_key:'contractor-party',note})});
      expectedRevision=partyResult.project?.revision||expectedRevision+1;
    }
    status.textContent='5/5 正在关联施工方合同…';
    await api('project-workspace','/api/v1/projects/'+encodeURIComponent(project.id)+'/resource-links',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:commands[4],expected_revision:expectedRevision,operation:'link',link_id:'',target_service:'contract-ledger',resource_type:'contract',resource_id:contractID,source_revision:draftVersion,relation_key:'downstream-contract',note:'OMS 一期施工方合同'})});
  }
  root.querySelector('[data-oms-v1-add-contractor]').addEventListener('click',()=>{
    const project=state.projects.find(item=>item.id===state.selectedProjectID);if(!project)return;
    const candidates=activeExternalParties().filter(item=>!downstreamContractByParty(project.id,item.id)),runtimeReady=contractDirectoryReady(),enabled=state.projectActions.has('project-workspace.resource-link.manage')&&state.contractActions.has('draft.create')&&runtimeReady&&state.operatingParty?.status==='active'&&candidates.length>0,status=!state.projectActions.has('project-workspace.resource-link.manage')?'当前账号没有项目关联权限':!state.contractActions.has('draft.create')?'当前账号没有合同创建权限':!runtimeReady?'合同功能尚未准备完成，请联系管理员':!state.operatingParty?'本公司信息暂不可用':!candidates.length?'没有待关联或待补合同的施工方主体':'';
    showDialog({kicker:'项目经营',title:'关联施工方',submit:'保存施工方与应付',enabled,onSubmit:async(data,form)=>{const party=state.parties.find(item=>item.id===data.get('party_id'));if(!party)throw new Error('请选择有效施工方主体');const amount=decimalMinor(data.get('payable_amount'));if(amount===null||amount<=0n)throw new Error('应付总额必须是大于 0 且最多两位小数的数字');await createDownstreamContract(project,party,amount,String(data.get('note')||'').trim(),form.querySelector('[data-oms-v1-form-status]'));closeDialog();await loadAll();openProject(project.id);sync.textContent='施工方、合同和应付总额已保存';},body:'<form><label>项目<input value="'+safeText(project.name)+'" disabled></label><label>施工方主体 *<select name="party_id" required><option value="">请选择施工方</option>'+candidates.map(item=>'<option value="'+safeText(item.id)+'">'+safeText(item.legal_name)+(projectContractorLinked(project,item.id)?'（已关联，待补合同）':'')+'</option>').join('')+'</select></label><label>应付总额（元） *<input name="payable_amount" required inputmode="decimal" placeholder="0.00"></label><label>备注<textarea name="note" maxlength="1000" placeholder="选填"></textarea></label><div class="oms-v1__form-status" data-oms-v1-form-status>'+safeText(status)+'</div></form>'});
  });
  root.querySelector('[data-oms-v1-project-payable]').addEventListener('click',()=>{
    const project=state.projects.find(item=>item.id===state.selectedProjectID);if(!project)return;
    const contractorIDs=(project.resource_links||[]).filter(link=>link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at).map(link=>link.resource_id),contractors=contractorIDs.map(id=>state.partyCache.get(id)).filter(Boolean),enabled=state.contractActions.has('draft.create')&&contractDirectoryReady()&&contractors.length>0,status=!contractors.length?'请先关联施工方':!state.contractActions.has('draft.create')?'当前账号没有合同维护权限':!contractDirectoryReady()?'合同功能尚未准备完成，请联系管理员':'';
    showDialog({kicker:'项目经营',title:'施工方应付',submit:'保存应付总额',enabled,onSubmit:async(data,form)=>{const party=state.partyCache.get(String(data.get('party_id')||''));if(!party)throw new Error('请选择施工方');const amount=decimalMinor(data.get('payable_amount'));if(amount===null||amount<=0n)throw new Error('应付总额必须是大于 0 且最多两位小数的数字');const segment=downstreamContractByParty(project.id,party.id),formStatus=form.querySelector('[data-oms-v1-form-status]');if(!segment){await createDownstreamContract(project,party,amount,'项目详情补录施工方应付',formStatus)}else{formStatus.textContent='正在更新施工方合同…';const workspace=await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(segment.resource_id)+'/workspace'),contract=workspace.contract;if(!contract||contract.status!=='draft'||!workspace.allowed_operations?.includes('draft.update'))throw new Error('该施工方合同当前不能直接调整，请在合同详情中处理');await api('contract-ledger','/api/v1/contracts/'+encodeURIComponent(contract.id),{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({command_id:uuid(),expected_draft_version:contract.draft_version,number_namespace:contract.number_namespace,contract_number:contract.contract_number,title:contract.title,category:contract.category,currency:contract.currency,total_amount:minorDecimal(amount),signed_on:contract.signed_on,effective_on:contract.effective_on})})}closeDialog();await loadAll();openProject(project.id);sync.textContent='施工方应付总额已更新，历史调整记录已保留';},body:'<form><label>施工方 *<select name="party_id" required><option value="">请选择施工方</option>'+contractors.map(item=>'<option value="'+safeText(item.id)+'">'+safeText(item.legal_name)+'</option>').join('')+'</select></label><label>应付总额（元） *<input name="payable_amount" required inputmode="decimal" placeholder="0.00"></label><div class="oms-v1__form-note">保存后更新该施工方的应付总额，历史调整记录保留。</div><div class="oms-v1__form-status" data-oms-v1-form-status>'+safeText(status)+'</div></form>'});
    const form=dialogBody.querySelector('form'),partySelect=form?.elements.party_id,amountInput=form?.elements.payable_amount;if(partySelect&&amountInput)partySelect.addEventListener('change',()=>{const segment=downstreamContractByParty(project.id,partySelect.value),amount=segment?decimalMinor(segment.contract?.current_effective_amount):null;amountInput.value=amount===null?'':minorDecimal(amount)});
  });
  root.querySelector('[data-oms-v1-filter]')?.addEventListener('click',event=>{
    const button=event.currentTarget,current=state.filters.projects||{},clients=new Map(),contractors=new Map();
    for(const project of state.projects){if(project.client_party?.party_id)clients.set(project.client_party.party_id,project.client_party.legal_name||'甲方档案待核对');for(const link of project.resource_links||[]){if(link.target_service==='party-directory'&&link.resource_type==='party'&&link.relation_key==='contractor-party'&&!link.unlinked_at){const party=state.partyCache.get(link.resource_id);contractors.set(link.resource_id,party?.legal_name||'施工方档案待核对')}}}
    showInlineFilter(button,{onApply:async data=>{state.filters.projects={client_id:String(data.get('client_id')||''),contractor_id:String(data.get('contractor_id')||''),receivable_status:String(data.get('receivable_status')||''),payable_status:String(data.get('payable_status')||'')};state.projectPage=1;renderProjects()},onReset:async()=>{state.filters.projects={};state.projectPage=1;renderProjects()},body:'<form><label>甲方<select name="client_id">'+option('','全部甲方',current.client_id)+[...clients].map(([id,name])=>option(id,name,current.client_id)).join('')+'</select></label><label>施工方<select name="contractor_id">'+option('','全部施工方',current.contractor_id)+[...contractors].map(([id,name])=>option(id,name,current.contractor_id)).join('')+'</select></label><label>应收状态<select name="receivable_status">'+option('','全部状态',current.receivable_status)+option('none','尚未设置',current.receivable_status)+option('unstarted','未完成',current.receivable_status)+option('partial','未完成',current.receivable_status)+option('complete','已完成',current.receivable_status)+option('anomaly','金额异常',current.receivable_status)+'</select></label><label>应付状态<select name="payable_status">'+option('','全部状态',current.payable_status)+option('none','尚未设置',current.payable_status)+option('unstarted','未完成',current.payable_status)+option('partial','未完成',current.payable_status)+option('complete','已完成',current.payable_status)+option('anomaly','金额异常',current.payable_status)+'</select></label></form>'});
  });

  root.querySelector('[data-oms-v1-project-search]').addEventListener('input',()=>{state.projectPage=1;renderProjects()});
  root.querySelector('[data-oms-v1-period]').addEventListener('change',renderOverview);
  for(const kind of ['client','contractor'])root.querySelector('[data-oms-v1-party-search="'+kind+'"]').addEventListener('input',()=>{state.listPages['party-'+kind]=1;renderParties(kind)});
  for(const direction of ['receivable','payable']){
    const key='commitment-'+direction;
    root.querySelector('[data-oms-v1-commitment-search="'+direction+'"]').addEventListener('input',()=>{state.listPages[key]=1;renderCommitments(direction)});
    root.querySelector('[data-oms-v1-commitment-filter="'+direction+'"]').addEventListener('click',event=>{
      const current=state.filters[key]||{};
      const partyLabel=direction==='receivable'?'甲方':'施工方',parties=new Map();
      for(const item of commitmentItems(direction)){const id=authoritativePartyID(item.counterparty);if(id)parties.set(id,item.counterparty?.legal_name||partyLabel+'档案待核对')}
      showInlineFilter(event.currentTarget,{onApply:async data=>{state.filters[key]={project_id:String(data.get('project_id')||''),counterparty_id:String(data.get('counterparty_id')||''),status:String(data.get('status')||'')};state.listPages[key]=1;renderCommitments(direction)},onReset:async()=>{state.filters[key]={};state.listPages[key]=1;renderCommitments(direction)},body:'<form><label>项目<select name="project_id">'+option('','全部项目',current.project_id)+state.projects.map(item=>option(item.id,item.name,current.project_id)).join('')+'</select></label><label>'+partyLabel+'<select name="counterparty_id">'+option('','全部'+partyLabel,current.counterparty_id)+[...parties].map(([id,name])=>option(id,name,current.counterparty_id)).join('')+'</select></label><label>资金状态<select name="status">'+option('','全部状态',current.status)+option('none','未发生',current.status)+option('partial','部分完成',current.status)+option('complete','全部完成',current.status)+option('anomaly','金额异常',current.status)+'</select></label></form>'});
    });
  }
  for(const direction of ['outgoing','incoming']){
    const key='invoice-'+direction;
    root.querySelector('[data-oms-v1-invoice-search="'+direction+'"]').addEventListener('input',()=>{state.listPages[key]=1;renderInvoices()});
    root.querySelector('[data-oms-v1-invoice-filter="'+direction+'"]').addEventListener('click',event=>{
      const current=state.filters[key]||{};
      showInlineFilter(event.currentTarget,{onApply:async data=>{const dateFrom=String(data.get('date_from')||''),dateTo=String(data.get('date_to')||'');if(dateFrom&&dateTo&&dateFrom>dateTo)throw new Error('开始日期不能晚于结束日期');state.filters[key]={project_id:String(data.get('project_id')||''),date_from:dateFrom,date_to:dateTo,status:String(data.get('status')||'recorded')};state.listPages[key]=1;renderInvoices()},onReset:async()=>{state.filters[key]={status:'recorded'};state.listPages[key]=1;renderInvoices()},body:'<form><label>项目<select name="project_id">'+option('','全部项目',current.project_id)+state.projects.map(item=>option(item.id,item.name,current.project_id)).join('')+'</select></label><label>'+invoiceDateLabel(direction)+'从<input name="date_from" type="date" value="'+safeText(current.date_from||'')+'"></label><label>'+invoiceDateLabel(direction)+'至<input name="date_to" type="date" value="'+safeText(current.date_to||'')+'"></label><label>记录状态<select name="status">'+option('recorded','有效',current.status||'recorded')+option('all','包含已作废',current.status)+'</select></label></form>'});
    });
  }
  for(const type of ['receipt','payment']){
    const key='cash-'+type;
    root.querySelector('[data-oms-v1-cash-search="'+type+'"]').addEventListener('input',()=>{state.listPages[key]=1;renderCash()});
    root.querySelector('[data-oms-v1-cash-filter="'+type+'"]').addEventListener('click',event=>{
      const current=state.filters[key]||{};
      showInlineFilter(event.currentTarget,{onApply:async data=>{const dateFrom=String(data.get('date_from')||''),dateTo=String(data.get('date_to')||'');if(dateFrom&&dateTo&&dateFrom>dateTo)throw new Error('开始日期不能晚于结束日期');state.filters[key]={project_id:String(data.get('project_id')||''),date_from:dateFrom,date_to:dateTo,status:String(data.get('status')||'recorded')};state.listPages[key]=1;renderCash()},onReset:async()=>{state.filters[key]={status:'recorded'};state.listPages[key]=1;renderCash()},body:'<form><label>项目<select name="project_id">'+option('','全部项目',current.project_id)+state.projects.map(item=>option(item.id,item.name,current.project_id)).join('')+'</select></label><label>'+cashDateLabel(type)+'从<input name="date_from" type="date" value="'+safeText(current.date_from||'')+'"></label><label>'+cashDateLabel(type)+'至<input name="date_to" type="date" value="'+safeText(current.date_to||'')+'"></label><label>记录状态<select name="status">'+option('recorded','有效',current.status||'recorded')+option('all','包含已冲销',current.status)+'</select></label></form>'});
    });
  }
  const sortRenderers={projects:renderProjects,'party-client':()=>renderParties('client'),'party-contractor':()=>renderParties('contractor'),'commitment-receivable':()=>renderCommitments('receivable'),'commitment-payable':()=>renderCommitments('payable'),'invoice-outgoing':renderInvoices,'invoice-incoming':renderInvoices,'cash-receipt':renderCash,'cash-payment':renderCash};
  for(const button of root.querySelectorAll('[data-oms-v1-sort-list]'))button.addEventListener('click',()=>{const list=button.dataset.omsV1SortList,key=button.dataset.omsV1SortKey,current=state.sorts[list];state.sorts[list]=current?.key===key?{key,direction:current.direction==='asc'?'desc':'asc'}:{key,direction:button.dataset.omsV1SortDefault||'asc'};if(list==='projects')state.projectPage=1;else state.listPages[list]=1;sortRenderers[list]?.()});
  root.querySelector('[data-oms-v1-project-prev]').addEventListener('click',()=>{state.projectPage=Math.max(1,state.projectPage-1);renderProjects()});root.querySelector('[data-oms-v1-project-next]').addEventListener('click',()=>{state.projectPage+=1;renderProjects()});root.querySelector('[data-oms-v1-project-page-size]').addEventListener('change',event=>{state.projectPageSize=parseInt(event.target.value,10)||20;state.projectPage=1;renderProjects()});

  async function loadCommercialSummaries(){state.commercial.clear();if(!state.projects.length)return;let cursor=0,failed=0;const worker=async()=>{while(cursor<state.projects.length){const project=state.projects[cursor++];try{state.commercial.set(project.id,await api('project-workspace','/api/v1/projects/'+encodeURIComponent(project.id)+'/commercial-summary'))}catch{failed++}}};await Promise.all(Array.from({length:Math.min(6,state.projects.length)},worker));if(failed)state.errors.push('经营汇总：'+failed+' 个项目暂不可用')}
  async function loadOperatingParty(){const id=root.dataset.operatingPartyId||'';state.operatingParty=null;if(!id)return;try{state.operatingParty=await partyByID(id)}catch(error){state.errors.push(sourceIssue('本公司信息',error))}}

  async function performLoadAll(){
    const refresh=root.querySelector('[data-oms-v1-refresh]'),notice=root.querySelector('[data-oms-v1-overview-notice]');
    refresh.disabled=true;sync.textContent='正在更新数据…';sync.classList.remove('is-error');state.errors=[];notice.classList.remove('is-warning');notice.querySelector('p').textContent='正在读取项目与合作主体。';
    const projectTask=loadOffsetCollection('project-workspace','/api/v1/project-directory',{status:'all'}),partyTask=loadCursorCollection('party-directory','/api/v1/parties',{status:'all'}),invoiceTask=loadCursorCollection('billing-invoice','/api/v1/invoice-registrations'),cashTask=loadOffsetCollection('money-flow-ledger','/api/v1/cash-transactions',{status:'all'}),moneyContextTask=api('money-flow-ledger','/api/v1/cash-workspace'),contractContextTask=api('contract-ledger','/api/v1/contracts/workspace-context');
    const primary=await Promise.allSettled([projectTask,partyTask]);
    if(primary[0].status==='fulfilled'){state.projects=(primary[0].value.items||[]).filter(isOMSProject);state.projectActions=new Set(primary[0].value.allowed_actions||[])}else{state.projects=[];state.projectActions.clear();state.errors.push(sourceIssue('项目',primary[0].reason))}
    state.partyCache.clear();
    if(primary[1].status==='fulfilled'){state.parties=primary[1].value.items||[];for(const party of state.parties)state.partyCache.set(party.id,party);state.partyActions=new Set(primary[1].value.allowed_actions||[])}else{state.parties=[];state.partyActions.clear();state.errors.push(sourceIssue('合作主体',primary[1].reason))}
    await loadOperatingParty();renderProjects();renderParties('client');renderParties('contractor');
    sync.textContent='项目与主体已就绪 · 正在汇总票款';notice.querySelector('p').textContent='项目与合作主体已可使用，票据与资金正在后台补齐。';
    const commercialTask=loadCommercialSummaries(),secondary=await Promise.allSettled([invoiceTask,cashTask,moneyContextTask,contractContextTask]);
    if(secondary[0].status==='fulfilled'){state.invoices=(secondary[0].value.items||[]).filter(item=>!!invoiceProject(item));state.invoiceActions=new Set(secondary[0].value.allowed_actions||[])}else{state.invoices=[];state.invoiceActions.clear();state.errors.push(sourceIssue('票据',secondary[0].reason))}
    if(secondary[1].status==='fulfilled'){state.cash=(secondary[1].value.items||[]).filter(item=>!!cashProject(item));state.moneyAvailable=true}else{state.cash=[];state.moneyAvailable=false;state.errors.push(sourceIssue('收付款',secondary[1].reason))}
    if(secondary[2].status==='fulfilled'){state.moneyContext=secondary[2].value}else{state.moneyContext=null;state.errors.push(sourceIssue('收付款登记',secondary[2].reason))}
    if(secondary[3].status==='fulfilled'){state.contractContext=secondary[3].value;state.contractActions=new Set(secondary[3].value.allowed_operations||[]);state.contractAvailable=true}else{state.contractContext=null;state.contractActions.clear();state.contractAvailable=false;state.errors.push(sourceIssue('合同',secondary[3].reason))}
    renderInvoices();renderCash();renderOverview();
    await commercialTask;renderProjects();renderCommitments('receivable');renderCommitments('payable');renderOverview();
    if(state.errors.length){sync.textContent='部分数据暂不可用';sync.classList.add('is-error');notice.classList.add('is-warning');notice.querySelector('p').textContent=state.errors.join('；')}else{sync.textContent='数据已更新';notice.classList.remove('is-warning');notice.querySelector('p').textContent='经营数据已更新至最新状态。'}
    refresh.disabled=false;
  }
  function loadAll(){
    if(loadTask)return loadTask;
    loadTask=performLoadAll().finally(()=>{root.querySelector('[data-oms-v1-refresh]').disabled=false;loadTask=null});
    return loadTask;
  }
  root.querySelector('[data-oms-v1-refresh]').addEventListener('click',loadAll);
  document.addEventListener('keydown',event=>{if(event.key==='Escape'){if(!dialog.hidden)closeDialog();else if(!root.querySelector('[data-oms-v1-project-detail]').hidden)root.querySelector('[data-oms-v1-project-back]').click()}});
  loadAll();
})();
