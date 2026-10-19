// reference-oa-statistics-v1: ephemeral, source-separated read model.
export const feeLabels={design:'设计费',survey:'勘察费',consultation:'咨询费',unclassified:'未分类'};
export const chainLabels={client:'甲方 → 资质单位',income:'资质单位 → 我司',unclassified:'未分类往来'};
export const costLabels={vehicle:'用车费',accommodation:'住宿费',transport:'交通费',expert_fee:'专家费',bid_signup_fee:'投标报名费',bid_service_fee:'投标服务费',other:'其他费用'};
export function minor(value){
 if(typeof value!=='string'||! /^-?(?:0|[1-9]\d*)(?:\.\d{1,2})?$/.test(value))return null;
 const negative=value.startsWith('-'),[a,b='']=value.replace(/^-/,'').split('.');return (BigInt(a)*100n+BigInt(b.padEnd(2,'0')))*(negative?-1n:1n);
}
export function money(value,currency='CNY'){
 if(value==null)return '金额未授权／未知';const n=BigInt(value),a=n<0n?-n:n;
 return `${currency} ${n<0n?'-':''}${String(a/100n).replace(/\B(?=(\d{3})+(?!\d))/g,',')}.${String(a%100n).padStart(2,'0')}`;
}
export function classification(key){const m=/^reference-oa-(contract|cash|invoice)-v1-(client|income)(?:-(design|survey|consultation))?$/.exec(key||'');return {chain:m?.[2]||'unclassified',fee:m?.[3]||'unclassified'};}
export function records(summary,project){
 if(summary?.project_id!==project.id||!Array.isArray(summary.segments))throw Error('台账项目响应不一致');
 const seen=new Map(),rows=[];
 for(const s of summary.segments){
  const owner={contract:'contract-ledger',obligation:'money-flow-ledger',transaction:'money-flow-ledger',invoice_registration:'billing-invoice'}[s.resource_type];
  if(!owner||s.source_application!==owner||!s.resource_id||!['available','revision_changed','unavailable','forbidden','invalid'].includes(s.state))throw Error('台账来源契约无效');
  const key=`${owner}:${s.resource_type}:${s.resource_id}`,c=classification(s.relation_key);
  if(seen.has(key)){if(seen.get(key)!==JSON.stringify(s))throw Error('重复引用的分类或事实不一致');continue;}seen.set(key,JSON.stringify(s));
  const f=s.contract||s.obligation||s.transaction||s.invoice_registration;
  let amount=null;
  if(['available','revision_changed'].includes(s.state)&&f){
   if(s.resource_type==='invoice_registration')amount=Number.isSafeInteger(f.amount_minor)?BigInt(f.amount_minor):null;
   else amount=minor(f.current_effective_amount??f.effective_amount);
  }
  rows.push({key,id:s.resource_id,projectID:project.id,projectName:project.name,projectNumber:project.project_number,source:owner,kind:s.resource_type,...c,
   currency:f?.currency||'',amount,date:f?.occurred_on||f?.issued_on||'',direction:f?.direction||f?.transaction_type||'',
   allocated:f?.allocated_amount==null?null:minor(f.allocated_amount),outstanding:f?.outstanding_amount==null?null:minor(f.outstanding_amount),unallocated:f?.unallocated_amount==null?null:minor(f.unallocated_amount),
   status:s.business_status||'',state:s.state,revision:s.source_revision,observedAt:s.observed_at||summary.observed_at,title:s.display_summary||s.resource_id,problem:s.error_code||''});
 }
 return rows;
}
export function filterRecords(rows,{keyword='',fee='',chain='',start='',end='',status=''}={}){
 const q=keyword.trim().toLocaleLowerCase('zh-CN');
 return rows.filter(r=>(!q||[r.projectName,r.projectNumber,r.title].join(' ').toLocaleLowerCase('zh-CN').includes(q))&&(!fee||r.fee===fee)&&(!chain||r.chain===chain)&&(!status||r.status===status)&&(!start||(r.date&&r.date>=start))&&(!end||(r.date&&r.date<=end)));
}
export function buckets(rows){
 const map=new Map();for(const r of rows){const key=[r.source,r.kind,r.direction,r.chain,r.fee,r.currency].join(':');let b=map.get(key);if(!b){b={...r,key,amount:0n,count:0,unknown:0};map.set(key,b)}b.count++;if(r.amount==null||!['available','revision_changed'].includes(r.state))b.unknown++;else b.amount+=r.amount;}
 return [...map.values()].map(b=>({...b,amount:b.unknown?null:b.amount}));
}
// No inferred receivable or cross-owner net: each cell is one native meaning.
export function matrixRows(rows){
 const definitions=[['contract','合同金额','amount'],['obligation','确认义务','amount'],['obligation','已核销','allocated'],['obligation','未清余额','outstanding'],['transaction','实际收付','amount'],['invoice_registration','开票登记','amount']];
 const result=[];
 for(const [kind,label,field] of definitions){
  const groups=new Map();for(const r of rows.filter(r=>r.kind===kind)){const key=[r.source,r.direction,r.currency].join(':');const list=groups.get(key)||[];list.push(r);groups.set(key,list)}
  for(const [key,items] of groups){const sum=list=>!list.length?undefined:list.some(r=>r[field]==null)?null:list.reduce((a,r)=>a+r[field],0n);result.push({key:kind+':'+field+':'+key,label,direction:items[0].direction,currency:items[0].currency,source:items[0].source,total:sum(items),...Object.fromEntries(Object.keys(feeLabels).map(fee=>[fee,sum(items.filter(r=>r.fee===fee))]))})}
 }
 return result;
}
export function companyRecords(rows){
 const projects=new Map();for(const r of rows){const ids=projects.get(r.key)||new Set();ids.add(r.projectID);projects.set(r.key,ids)}
 const ambiguous=[...projects].filter(([,ids])=>ids.size>1).map(([key])=>key),excluded=new Set(ambiguous);
 return {rows:rows.filter(r=>!excluded.has(r.key)),ambiguous};
}
export function bonusRecords(page,project,scope,tenant){
 if(page?.project_id!==project.id||page.scope!==scope||!['reference-oa-bonus-v2','reference-oa-bonus-v3'].includes(page.profile)||!Array.isArray(page.items))throw Error('奖金统计范围或版本不一致');
 const seen=new Set();return page.items.filter(v=>{
  if(!v.id||seen.has(v.id)||v.tenant_id!==tenant||v.project?.id!==project.id||v.scope!==scope||!v.person?.id||!v.person.display_name||!['recorded','withdrawn'].includes(v.state)||!Array.isArray(v.allowed_actions))throw Error('奖金人员／来源记录不一致');seen.add(v.id);return v.state==='recorded';
 }).map(v=>({id:v.id,personID:v.person.id,personName:v.person.display_name,projectID:project.id,projectName:project.name,projectNumber:project.project_number,scope,currency:v.currency||'',date:v.occurred_on,revision:v.revision,amount:v.currency&&v.allowed_actions.includes('bonus-manual-allocation.amount.read')&&typeof v.amount_minor==='string'&&/^\d+$/.test(v.amount_minor)?BigInt(v.amount_minor):null}));
}
export function costRecords(page,project,scope,tenant){
 if(page?.project_id!==project.id||page.scope!==scope||page.profile!=='reference-oa-cost-v1'||!Array.isArray(page.items))throw Error('成本统计范围或版本不一致');
 const seen=new Set();return page.items.filter(v=>{
  if(!v.id||seen.has(v.id)||v.tenant_id!==tenant||v.project?.id!==project.id||v.scope!==scope||v.profile!==page.profile||!costLabels[v.category]||!['recorded','withdrawn'].includes(v.state)||!Array.isArray(v.allowed_actions))throw Error('成本来源记录不一致');seen.add(v.id);return v.state==='recorded';
 }).map(v=>({id:v.id,projectID:project.id,projectName:project.name,projectNumber:project.project_number,scope,category:v.category,currency:v.currency||'',date:v.occurred_on||'',description:v.description||'',revision:v.revision,amount:v.currency&&v.allowed_actions.includes('cost-entry.amount.read')&&typeof v.amount_minor==='string'&&/^\d+$/.test(v.amount_minor)?BigInt(v.amount_minor):null}));
}
export function currencyTotals(rows){
 const groups=new Map();for(const r of rows){const list=groups.get(r.currency)||[];list.push(r);groups.set(r.currency,list)}
 return [...groups].map(([currency,list])=>({currency,count:list.length,amount:list.some(r=>r.amount==null)?null:list.reduce((a,r)=>a+r.amount,0n)}));
}
export function excelXML(columns,rows){
 const escape=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&apos;'}[c]));
 // All values use String cells: no formula execution or precision loss in Excel.
 const line=r=>'<Row>'+r.map(v=>`<Cell><Data ss:Type="String">${escape(v)}</Data></Cell>`).join('')+'</Row>';
 return '<?xml version="1.0" encoding="UTF-8"?><?mso-application progid="Excel.Sheet"?><Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet"><Worksheet ss:Name="统计明细"><Table>'+line(columns)+rows.map(line).join('')+'</Table></Worksheet></Workbook>';
}
