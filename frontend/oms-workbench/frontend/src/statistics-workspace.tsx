import React,{useEffect,useRef,useState} from 'react'
import {createRoot} from 'react-dom/client'
import {Alert,Button,ConfigProvider,DatePicker,Drawer,Empty,Input,Pagination,Select,Space,Spin,Table,Tabs,Tag,message} from 'antd'
import zhCN from 'antd/locale/zh_CN'
import {records,filterRecords,buckets,bonusRecords,costRecords,currencyTotals,matrixRows,companyRecords,money,excelXML,feeLabels,chainLabels,costLabels} from './statistics-model.mjs'
import './statistics-workspace.css'
type Row=any
type Options={root:HTMLElement;tenantID:string;preview:boolean;applications?:string[];authorize:()=>Promise<void>;request:(path:string)=>Promise<any>;applicationRequest:(app:string,path:string,options?:any)=>Promise<any>;directory:(request:any,tenant:string,keyword:string)=>Promise<any[]>;authorizeExport?:(kind:string,rows:Row[])=>Promise<void>}
const kindLabels:Record<string,string>={contract:'合同金额',obligation:'确认义务',transaction:'实际收付',invoice_registration:'开票金额'}
const stateLabels:Record<string,string>={available:'已读取',revision_changed:'来源已更新',forbidden:'无读取权限',unavailable:'来源暂不可用',invalid:'引用失效',recorded:'已登记',confirmed:'已确认',active:'有效',open:'未结清',closed:'已关闭',draft:'草稿',withdrawn:'已撤回',receipt:'收款',payment:'付款',receivable:'应收',payable:'应付'}
const Download=(name:string,blob:Blob)=>{const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}
function Statistics({options}:{options:Options}){
 const [tab,setTab]=useState('ledger'),[projects,setProjects]=useState<Row[]>([]),[rows,setRows]=useState<Row[]>([]),[bonuses,setBonuses]=useState<Row[]>([]),[costs,setCosts]=useState<Row[]>([]),[busy,setBusy]=useState(false),[detailBusy,setDetailBusy]=useState(false),[error,setError]=useState(''),[failures,setFailures]=useState<string[]>([]),[observed,setObserved]=useState(''),[page,setPage]=useState(1),[pageSize,setPageSize]=useState(10),[selected,setSelected]=useState<Row|null>(null),[detailTab,setDetailTab]=useState('transaction'),[exporting,setExporting]=useState(false),[keyword,setKeyword]=useState(''),[fee,setFee]=useState(''),[chain,setChain]=useState(''),[status,setStatus]=useState(''),[range,setRange]=useState<any>(null),[expanded,setExpanded]=useState<React.Key[]>([])
 const cache=useRef(new Map<string,Row[]>()),bonusCache=useRef(new Map<string,Row[]>()),costCache=useRef(new Map<string,Row[]>()),generation=useRef(0),loading=useRef(false),queued=useRef(false),queuedForce=useRef(false),lastLoaded=useRef(0),currentTab=useRef(tab);currentTab.current=tab
 const keywordRef=useRef(keyword);keywordRef.current=keyword
 const costInstalled=options.applications?.includes('cost-outsourcing')??false
 const bonusInstalled=options.applications?.includes('bonus-distribution')??true
 const filters={keyword,fee,chain,status,start:range?.[0]?.format('YYYY-MM-DD')||'',end:range?.[1]?.format('YYYY-MM-DD')||''}
 const visible=filterRecords(rows,filters)
 const reset=()=>{setKeyword('');setFee('');setChain('');setStatus('');setRange(null);setPage(1);setExpanded([]);setSelected(null)}
 const readProject=async(p:Row,force=false)=>{
  if(cache.current.has(p.id)&&!force)return cache.current.get(p.id)!
  const details=await options.request('/api/v1/projects/'+p.id)
  if(details.project?.id!==p.id||details.project.tenant_id!==options.tenantID)throw Error('项目租户或 ID 不一致')
  if(!details.allowed_actions?.includes('project-workspace.commercial-summary.read'))throw Error('没有本项目经营金额读取权限')
  const result=records(await options.request('/api/v1/projects/'+p.id+'/commercial-summary'),p);cache.current.set(p.id,result);return result
 }
 const load=async(force=false)=>{
  if(loading.current){queued.current=true;queuedForce.current=queuedForce.current||force;return}loading.current=true;setBusy(true);setError('');const epoch=++generation.current,nextTab=currentTab.current,issues:string[]=[]
  try{
   if(options.preview)throw Error('当前是界面预览。登录正式工作台后读取统计数据。')
   if(nextTab==='personnel-bonus'&&!bonusInstalled){setBonuses([]);setFailures([]);return}
   await options.authorize()
   if(force||Date.now()-lastLoaded.current>30000){cache.current.clear();bonusCache.current.clear();costCache.current.clear();setRows([]);setBonuses([]);setCosts([]);if(force){setSelected(null);setExpanded([])}}
   const nested=await options.directory(options.request,options.tenantID,'');const flat=nested.flatMap((p:Row)=>p.node_type==='package'?p.children:[p]);setProjects(flat)
   const ids=new Set(flat.map(p=>p.id));for(const id of cache.current.keys())if(!ids.has(id))cache.current.delete(id)
   const values:Row[]=[]
   if(nextTab==='personnel-bonus'){
    for(const p of flat)for(const scope of ['business','production']){
     const key=p.id+':'+scope
     try{let list=bonusCache.current.get(key);if(!list||force){const v=await options.applicationRequest('bonus-distribution','/api/v1/reference-oa/bonus-allocations?project_id='+p.id+'&scope='+scope,{statisticsProject:p.id,statisticsBonusScope:scope});list=bonusRecords(v,p,scope,options.tenantID);bonusCache.current.set(key,list!)}values.push(...list!)}catch(e){bonusCache.current.delete(key);issues.push(p.name+' · '+(scope==='business'?'经营奖金':'生产奖金')+'：'+(e instanceof Error?e.message:String(e)))}
    }
    if(epoch===generation.current&&nextTab===currentTab.current)setBonuses(values)
   }else{
    // Company explicitly reads the full visible directory; ledger reads one page.
    const query=keywordRef.current.trim().toLocaleLowerCase('zh-CN'),matched=flat.filter(p=>!query||[p.name,p.project_number].join(' ').toLocaleLowerCase('zh-CN').includes(query));const batch=nextTab==='company'?flat:matched.slice((page-1)*pageSize,page*pageSize)
    for(const p of batch){try{const list=await readProject(p,force);values.push(...list);if(list.some(r=>r.amount==null||!['available','revision_changed'].includes(r.state)))issues.push(p.name+'：部分来源或金额未完整读取')}catch(e){cache.current.delete(p.id);issues.push(p.name+'：'+(e instanceof Error?e.message:String(e)))}}
    if(nextTab==='company'&&costInstalled){
     const list:Row[]=[];for(const p of flat)for(const scope of ['business','production']){const key=p.id+':'+scope;try{let items=costCache.current.get(key);if(!items||force){items=costRecords(await options.applicationRequest('cost-outsourcing','/api/v1/reference-oa/cost-entries?project_id='+p.id+'&scope='+scope,{statisticsProject:p.id,statisticsCostScope:scope}),p,scope,options.tenantID);costCache.current.set(key,items!)}list.push(...items!)}catch(e){costCache.current.delete(key);issues.push(p.name+' · '+(scope==='business'?'经营成本':'生产成本')+'：'+(e instanceof Error?e.message:String(e)))}}
     if(epoch===generation.current&&nextTab===currentTab.current)setCosts(list)
    }
    if(epoch===generation.current&&nextTab===currentTab.current)setRows([...cache.current.values()].flat())
   }
   if(epoch===generation.current&&nextTab===currentTab.current){setFailures(issues);setObserved(new Date().toLocaleString('zh-CN'));lastLoaded.current=Date.now()}
  }catch(e){if(epoch===generation.current&&nextTab===currentTab.current){setError(e instanceof Error?e.message:'统计读取未完成');setRows([]);setBonuses([]);setCosts([]);cache.current.clear();bonusCache.current.clear();costCache.current.clear();lastLoaded.current=0}}
  finally{if(epoch===generation.current)setBusy(false);loading.current=false;if(queued.current){queued.current=false;document.dispatchEvent(new Event('reference-oa-statistics-reload'))}}
 }
 useEffect(()=>{const shown=()=>{if(options.root.closest('[data-view]')?.classList.contains('is-active')&&Date.now()-lastLoaded.current>30000&&['ledger','company','personnel-bonus'].includes(currentTab.current))void load()};document.addEventListener('reference-oa-view-shown',shown);shown();return()=>document.removeEventListener('reference-oa-view-shown',shown)},[])
 useEffect(()=>{const timer=setTimeout(()=>{if(options.root.closest('[data-view]')?.classList.contains('is-active')&&['ledger','company','personnel-bonus'].includes(tab))void load()},150);return()=>clearTimeout(timer)},[tab,page,pageSize,tab==='ledger'?keyword:''])
 useEffect(()=>{const reload=()=>{const force=queuedForce.current;queuedForce.current=false;if(['ledger','company','personnel-bonus'].includes(tab))void load(force)};document.addEventListener('reference-oa-statistics-reload',reload);return()=>document.removeEventListener('reference-oa-statistics-reload',reload)},[tab,page,pageSize])
 const openLedger=async(p:Row)=>{setSelected(p);setDetailTab('transaction');setDetailBusy(true);try{await options.authorize();await readProject(p);setRows([...cache.current.values()].flat())}catch(e){cache.current.delete(p.id);setRows([...cache.current.values()].flat());message.error(e instanceof Error?e.message:'台账未读取')}finally{setDetailBusy(false)}}
 const exportRows=async(kind:string)=>{
  setExporting(true)
  try{
   const data=visible.filter((r:Row)=>r.projectID===selected?.id&&r.kind===kind)
   if(!data.length||data.some((r:Row)=>r.amount==null||!['available','revision_changed'].includes(r.state)))throw Error('含有未授权或未完整读取的记录，无法导出')
   if(!options.authorizeExport)throw Error('当前组合尚未配置该来源的统计导出端口；已显示记录不会自动取得导出权限。')
   await options.authorizeExport(kind,data)
   Download((kind==='transaction'?'支付':'开票')+'明细.xml',new Blob([excelXML(['项目','编号','往来链','费用','日期','金额','币种','状态','来源','来源 ID','revision','观测时间'],data.map((r:Row)=>[r.projectName,r.projectNumber,chainLabels[r.chain as keyof typeof chainLabels],feeLabels[r.fee as keyof typeof feeLabels],r.date,money(r.amount,r.currency),r.currency,stateLabels[r.status]||r.status,r.source,r.id,r.revision,r.observedAt]))],{type:'application/vnd.ms-excel;charset=utf-8'}));message.success('已导出当前筛选明细')
  }catch(e){message.error(e instanceof Error?e.message:'导出未完成')}finally{setExporting(false)}
 }
 const toolbar=<Space wrap className="oa-statistics-toolbar"><Input allowClear aria-label="搜索项目名称或编号" placeholder={tab==='personnel-bonus'?'搜索人员、项目或编号':'搜索项目名称或编号'} value={keyword} onChange={e=>{setKeyword(e.target.value);setPage(1)}}/>{tab!=='personnel-bonus'&&<><Select allowClear aria-label="费用类型" placeholder="费用类型" value={fee||undefined} onChange={v=>{setFee(v||'');setPage(1)}} options={Object.entries(feeLabels).map(([value,label])=>({value,label}))}/><Select allowClear aria-label="经营链" placeholder="经营链" value={chain||undefined} onChange={v=>{setChain(v||'');setPage(1)}} options={Object.entries(chainLabels).map(([value,label])=>({value,label}))}/><Select allowClear aria-label="记录状态" placeholder="记录状态" value={status||undefined} onChange={v=>{setStatus(v||'');setPage(1)}} options={[...new Set(rows.map(r=>r.status).filter(Boolean))].map(value=>({value,label:stateLabels[value]||value}))}/></>}<DatePicker.RangePicker value={range} onChange={v=>{setRange(v);setPage(1)}} placeholder={['记录日期（起）','记录日期（止）']}/><Button onClick={reset}>重置</Button><Button loading={busy} onClick={()=>void load(true)}>刷新</Button></Space>
 const columns:any[]=[{title:'往来链 / 费用',render:(_:any,r:Row)=><>{chainLabels[r.chain as keyof typeof chainLabels]}<small className="oa-statistics-meta">{feeLabels[r.fee as keyof typeof feeLabels]}</small></>},{title:'业务记录',render:(_:any,r:Row)=><>{r.title}<small className="oa-statistics-meta">{kindLabels[r.kind]} · {r.direction&&stateLabels[r.direction]}</small></>},{title:'记录日期',dataIndex:'date',render:(v:string)=>v||'—'},{title:'金额',render:(_:any,r:Row)=>money(r.amount,r.currency)},{title:'状态',render:(_:any,r:Row)=><Tag color={r.state==='available'?'green':'orange'}>{stateLabels[r.state]||r.state} · {stateLabels[r.status]||r.status}</Tag>},{title:'来源版本',render:(_:any,r:Row)=><>{r.source}<small className="oa-statistics-meta">revision {r.revision||'—'} · {r.observedAt}</small></>}]
 const summary=(items:Row[])=>items.length?buckets(items).map((b:Row)=><div key={b.key} className="oa-statistics-metric"><span>{kindLabels[b.kind]} {stateLabels[b.direction]||''}</span><strong>{money(b.amount,b.currency)}</strong><small>{chainLabels[b.chain as keyof typeof chainLabels]} · {feeLabels[b.fee as keyof typeof feeLabels]}</small><small>{b.count} 条{b.unknown?' · 含未读金额':''}</small></div>):<span>当前筛选下无可见记录</span>
 const ledgerProjects=projects.filter(p=>!keyword.trim()||[p.name,p.project_number].join(' ').toLowerCase().includes(keyword.trim().toLowerCase()))
 const company=companyRecords(visible)
 const nativeMatrixColumns:any[]=[{title:'统计口径 / 来源',render:(_:any,r:Row)=><>{r.label} {stateLabels[r.direction]||''}<small className="oa-statistics-meta">{r.source} · {r.currency||'币种未知'}</small></>},...['total',...Object.keys(feeLabels)].map(key=>({title:key==='total'?'合计':feeLabels[key as keyof typeof feeLabels],render:(_:any,r:Row)=>r[key]===undefined?'—':money(r[key],r.currency)}))]
 const keywordMatches=(r:Row)=>!keyword.trim()||[r.personName,r.projectName,r.projectNumber].join(' ').toLocaleLowerCase('zh-CN').includes(keyword.trim().toLocaleLowerCase('zh-CN'))
 const inDates=(r:Row)=>(!filters.start||(r.date&&r.date>=filters.start))&&(!filters.end||(r.date&&r.date<=filters.end))
 const costVisible=costs.filter(r=>keywordMatches(r)&&inDates(r))
 const bonusVisible=bonuses.filter(r=>keywordMatches(r)&&inDates(r))
 const people=new Map<string,Row>();for(const b of bonusVisible){let p=people.get(b.personID);if(!p){p={id:b.personID,name:b.personName,items:[]};people.set(b.personID,p)}p.items.push(b)}
 return <div className="oa-statistics-complete"><Tabs activeKey={tab} onChange={key=>{setTab(key);setRows([]);setFailures([]);setError('');cache.current.clear();setPage(1);setSelected(null);lastLoaded.current=0}} items={[
  {key:'ledger',label:'项目台账'},{key:'company',label:'公司统计'},{key:'attendance',label:'人事考勤统计'},{key:'payroll',label:'薪酬统计'},{key:'personnel-bonus',label:'人员奖金统计'}]}/>
 {['attendance','payroll'].includes(tab)?<Empty description={tab==='attendance'?'原 OA 尚未实现考勤统计':'原 OA 尚未实现薪酬统计'}/>:<>
 <header className="oa-statistics-heading"><div><h2>{tab==='ledger'?'项目财务台账':tab==='company'?'公司收支概览':'人员奖金统计'}</h2><p>{tab==='company'?'按经营链对照来源金额；未分类记录单列。':tab==='personnel-bonus'?'已登记奖金按人员展开；登记金额不代表实际支付。':'筛选项目，打开支付与开票明细。'}</p></div><small>{observed?'最近读取：'+observed:'尚未读取'}</small></header>{toolbar}
 {error&&<Alert type="error" showIcon message={error}/>}{failures.length>0&&<Alert type="warning" showIcon message={`${failures.length} 项来源未完整读取，统计范围不完整`} description={<details><summary>查看问题</summary>{failures.map((f,i)=><p key={i}>{f}</p>)}</details>}/>}
 {busy&&<div className="oa-statistics-loading"><Spin tip="正在读取权限范围内的统计…"><div/></Spin></div>}
 {tab==='ledger'&&<><div className="oa-statistics-projects">{ledgerProjects.slice((page-1)*pageSize,page*pageSize).map(p=><article key={p.id} className="oa-statistics-project"><header><div><h3>{p.name}</h3><small>{p.project_number||'未编号'} · {p.status==='active'?'有效项目':p.status}</small></div><Button onClick={()=>void openLedger(p)}>查看台账 →</Button></header><div className="oa-statistics-metrics">{cache.current.has(p.id)?summary(visible.filter((r:Row)=>r.projectID===p.id)):<span>打开台账后读取金额</span>}</div></article>)}</div>{!ledgerProjects.length&&!busy&&!error&&<Empty description="当前筛选条件下暂无可见项目"/>}<Pagination current={page} pageSize={pageSize} total={ledgerProjects.length} showSizeChanger onChange={(p,n)=>{setPage(p);setPageSize(n)}} showTotal={n=>`共 ${n} 个项目`}/></>}
 {tab==='company'&&<>
 {company.ambiguous.length>0&&<Alert type="warning" message={`${company.ambiguous.length} 条来源被多个项目引用，已排除汇总以避免重复计数`} description="请先在来源确认项目归属或分摊口径。"/>}
 {(['client','income','unclassified'] as const).map(c=><section key={c} className="oa-statistics-company"><header><h3>{chainLabels[c]}</h3><small>同来源、同币种、同含义分别汇总</small></header><Table rowKey="key" size="small" dataSource={matrixRows(company.rows.filter((r:Row)=>r.chain===c))} columns={nativeMatrixColumns} pagination={false} scroll={{x:1000}} locale={{emptyText:busy?'正在读取':'当前筛选下无可见记录'}}/></section>)}
 <section className="oa-statistics-cost"><h3>成本记录</h3><p>按项目与日期筛选经营／生产成本；成本登记不等于付款，已撤回记录不计入。</p>{costInstalled?<><div className="oa-statistics-metrics">{['business','production'].map(scope=>currencyTotals(costVisible.filter(r=>r.scope===scope)).map((b:Row)=><div className="oa-statistics-metric" key={scope+b.currency}><span>{scope==='business'?'经营成本':'生产成本'}</span><strong>{money(b.amount,b.currency)}</strong><small>{b.count} 条成本记录</small></div>))}</div><Table rowKey="id" dataSource={costVisible} columns={[{title:'项目',dataIndex:'projectName'},{title:'侧别',render:(_:any,r:Row)=>r.scope==='business'?'经营':'生产'},{title:'费用类别',render:(_:any,r:Row)=>costLabels[r.category as keyof typeof costLabels]},{title:'记录日期',dataIndex:'date'},{title:'登记金额',render:(_:any,r:Row)=>money(r.amount,r.currency)},{title:'说明',dataIndex:'description'},{title:'来源版本',dataIndex:'revision'}]} pagination={{pageSize:10,showSizeChanger:true}} scroll={{x:900}}/></>:<Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前组合未选装成本应用"/>}<Alert type="info" message="项目结余尚无已验证的来源口径；不以合同、成本、付款简单相减生成结余。"/></section>
 <section className="oa-statistics-source"><h3>来源明细</h3><Table rowKey={r=>r.key+':'+r.projectID} dataSource={visible} columns={[{title:'项目',render:(_:any,r:Row)=><Button type="link" onClick={()=>void openLedger(projects.find(p=>p.id===r.projectID)!)}>{r.projectName}</Button>},...columns]} pagination={{pageSize:10,showSizeChanger:true}} scroll={{x:1100}}/></section></>}
 {tab==='personnel-bonus'&&(bonusInstalled?<Table rowKey="id" dataSource={[...people.values()]} columns={[{title:'人员',dataIndex:'name'},{title:'奖金合计',render:(_:any,p:Row)=>currencyTotals(p.items).map((b:Row)=><div key={b.currency}>{money(b.amount,b.currency)}</div>)},{title:'相关项目',render:(_:any,p:Row)=>new Set(p.items.map((r:Row)=>r.projectID)).size+' 个'}]} expandable={{expandedRowKeys:expanded,onExpandedRowsChange:v=>setExpanded([...v]),expandRowByClick:true,expandedRowRender:p=><Table rowKey="id" dataSource={p.items} columns={[{title:'项目',dataIndex:'projectName'},{title:'类型',render:(_:any,r:Row)=>r.scope==='business'?'经营奖金':'生产奖金'},{title:'登记日期',dataIndex:'date'},{title:'登记金额',render:(_:any,r:Row)=>money(r.amount,r.currency)},{title:'来源版本',dataIndex:'revision'}]} pagination={false} scroll={{x:650}}/>}} onRow={p=>({tabIndex:0,onKeyDown:e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();setExpanded(v=>v.includes(p.id)?v.filter(id=>id!==p.id):[...v,p.id])}}})} pagination={{pageSize:10,showSizeChanger:true}} scroll={{x:550}}/>:<Empty description="当前组合未选装奖金应用"/>)}
 </>}
 <Drawer title="财务台账" width="min(1180px, 100vw)" open={!!selected} onClose={()=>setSelected(null)} className="oa-statistics-drawer"><h2>{selected?.name}</h2><p>{selected?.project_number}</p><div className="oa-statistics-metrics">{summary(visible.filter((r:Row)=>r.projectID===selected?.id))}</div><Tabs activeKey={detailTab} onChange={setDetailTab} items={[{key:'transaction',label:'支付明细'},{key:'invoice_registration',label:'开票明细'},{key:'contract',label:'合同信息'},{key:'obligation',label:'应收应付'}]}/><Space wrap style={{marginBottom:16}}><Button data-statistics-export loading={exporting} onClick={()=>void exportRows(detailTab)} disabled={detailBusy||!['transaction','invoice_registration'].includes(detailTab)||!visible.some((r:Row)=>r.projectID===selected?.id&&r.kind===detailTab)}>导出 Excel</Button><span>导出沿用当前费用、往来链和日期筛选</span></Space><Table rowKey="key" loading={detailBusy} columns={[...columns,...(detailTab==='obligation'?[{title:'已核销',render:(_:any,r:Row)=>money(r.allocated,r.currency)},{title:'未清余额',render:(_:any,r:Row)=>money(r.outstanding,r.currency)}]:detailTab==='transaction'?[{title:'未核销',render:(_:any,r:Row)=>money(r.unallocated,r.currency)}]:[])]} dataSource={visible.filter((r:Row)=>r.projectID===selected?.id&&r.kind===detailTab)} pagination={{pageSize:10,showSizeChanger:true}} scroll={{x:1200}}/></Drawer>
 </div>
}
export function createStatisticsWorkspace(options:Options){options.root.replaceChildren();createRoot(options.root).render(<ConfigProvider locale={zhCN}><Statistics options={options}/></ConfigProvider>)}
