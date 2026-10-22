import React, { useMemo, useState } from 'react'
export {createStatisticsWorkspace} from './statistics-workspace'
export {createPermissionWorkspace} from './permission-workspace'
export {minor as statisticsMinor} from './statistics-model.mjs'
import { createRoot, type Root } from 'react-dom/client'
import { Alert, Button, Card, Checkbox, ConfigProvider, DatePicker, Descriptions, Drawer, Empty, Form, Input, List, Modal, Rate, Select, Skeleton, Space, Spin, Tag, message } from 'antd'
import type { UploadFile } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BiddingFileList } from '@/components/business/BiddingFileList'
import { BusinessPersonnelList } from '@/components/business/BusinessPersonnelList'
import { ContractAndAmendments } from '@/components/business/ContractAndAmendments'
import { PaymentRecordList } from '@/components/business/PaymentRecordList'
import { InvoiceRecordList } from '@/components/business/InvoiceRecordList'
import { BiddingMethodControl } from '@/components/oa2026/OA2026Controls'
import { registerAdapter, registerContributor, registerContract, registerCash, registerInvoice, registerCost, registerBonus, registerApprovalAudit, registerCommission } from './services/registry'
import { BonusAdapter } from './bonus-adapter.mjs'
import { BonusAllocation } from '../vendor/components/shared/BonusAllocation'
import { CostAdapter } from './cost-adapter.mjs'
import { ApprovalAuditAdapter } from './approval-audit-adapter.mjs'
import { CommissionAdapter } from './commission-adapter.mjs'
import { ProductionCostList } from '../vendor/components/production/ProductionCostList'
import { BiddingAdapter } from './bidding-adapter.mjs'
import { ContributorAdapter } from './contributor-adapter.mjs'
import { ContractAdapter } from './contract-adapter.mjs'
import { CashAdapter } from './cash-adapter.mjs'
import { InvoiceAdapter } from './invoice-adapter.mjs'
import { PageReadCache } from './page-read-cache.mjs'
import { FileTransferClient, createFetchFileTransferApi, createAliOssMultipartUploader, createDirectUploaderRouter, createLocalHttpUploader } from '../../../../packages/file-transfer-core/src/index'
import { AttachmentUploader } from '@/components/shared/AttachmentUploader'
import { FilePreviewDrawer } from '@/components/shared/FilePreviewDrawer'
import { fileService } from './services/file'
import type { File as ProjectFile } from './types'
export {createProductionLayout} from './production-layout'
export {createProductionPersonnelWorkspace} from './production-personnel-workspace'
export {createProductionStageWorkspace} from './production-stage-workspace'
dayjs.locale('zh-cn')

// Kept for the retired reference OA source and its internal tests. The current
// OMS uses evidence-transfer.ts and never loads this React workspace bundle.
export function createEvidenceTransfer(evidenceFetch:(input:RequestInfo|URL,options?:RequestInit)=>Promise<Response>) {
  const api=createFetchFileTransferApi({baseUrl:'',fetcher:evidenceFetch})
  return new FileTransferClient({
    api,
    uploader:createDirectUploaderRouter({
      aliyunOss:createAliOssMultipartUploader(),
      localHttp:createLocalHttpUploader(),
    }),
    getCacheScope:()=>undefined,
  })
}

function WorkspaceSkeleton({tip}:{tip:string}) {
  return <div className="workspace-read-skeleton" aria-busy="true" aria-live="polite"><div className="workspace-read-skeleton__label">{tip}</div><Skeleton active title={{width:'28%'}} paragraph={{rows:4,width:['92%','78%','86%','64%']}} /></div>
}

function ReadStatus({workspace}:{workspace:any}) {
  if(workspace.refreshing)return <span className="workspace-read-status" role="status"><Spin size="small" />正在后台更新</span>
  if(workspace.refreshError)return <span className="workspace-read-status workspace-read-status--warning" role="status" title={workspace.refreshError}>显示上次结果；更新失败，可手动刷新</span>
  return null
}

async function readWorkspace(workspace:any,{sequence,key,explicit,hasValue,read,isCurrent,fallback}:{sequence:number;key:string;explicit:boolean;hasValue:()=>boolean;read:()=>Promise<unknown>;isCurrent:()=>boolean;fallback:string}) {
  const cached=hasValue()
  if(!explicit&&workspace.readCache.isFresh(key,cached)){
    workspace.loading=false;workspace.refreshing=false;workspace.error='';workspace.refreshError='';workspace.render();return
  }
  workspace.loading=!cached;workspace.refreshing=cached;workspace.error='';workspace.refreshError='';workspace.render()
  try{
    await workspace.readCache.read(key,read)
    if(!isCurrent()||sequence!==workspace.sequence)return
    workspace.loading=false;workspace.refreshing=false;workspace.error='';workspace.refreshError='';workspace.render()
  }catch(error){
    if(!isCurrent()||sequence!==workspace.sequence)return
    const message=error instanceof Error?error.message:fallback
    workspace.loading=false;workspace.refreshing=false
    if(cached){workspace.error='';workspace.refreshError=message}else{workspace.error=message;workspace.refreshError=''}
    workspace.render()
  }
}

function Panel({workspace, adapter}: {workspace: ReactBiddingWorkspace; adapter: any}) {
  const [error,setError]=useState('')
  const state=adapter?.state
  const run=async (action:()=>Promise<unknown>)=>{setError('');try{await action()}catch(e){setError(e instanceof Error?e.message:'操作尚未确认')}}
  if(workspace.loading)return <WorkspaceSkeleton tip="正在读取当前项目招投标资料…" />
  if(workspace.error||!state||!adapter.dataReady)return <Alert type="error" showIcon message={workspace.error||adapter?.error||'文件元数据未完整回读'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>} />
  const pending=state.intent, disabled=!!pending||adapter.working
  return <>
    <Space wrap style={{marginBottom:16}}>
      <Button onClick={()=>void run(()=>workspace.load(true))} disabled={adapter.working}>刷新当前资料</Button>
      <Button disabled={disabled||!adapter.flow.can(state,'business-engagement.applicability.manage')} onClick={()=>void run(()=>adapter.updateApplicability(state.source?.engagement.applicability==='not_applicable'))}>{state.source?.engagement.applicability==='not_applicable'?'恢复适用':'本阶段无／不适用'}</Button>
      {pending&&!state.conflict?<Button type="primary" disabled={adapter.working} onClick={()=>void run(()=>adapter.retry())}>继续原操作</Button>:null}
      <ReadStatus workspace={workspace} />
    </Space>
    {pending?<Alert type="warning" showIcon style={{marginBottom:16}} message={state.conflict?'资料版本冲突，请先刷新；不会自动覆盖。':'上次操作尚未全部确认；继续原操作复用原命令。上传文件和历史保留。'} />:null}
    {error||adapter.error?<Alert type="error" showIcon message={error||adapter.error} style={{marginBottom:16}} />:null}
    <BiddingMethodControl projectId={adapter.projectID} canEdit={!disabled&&adapter.flow.can(state,'business-engagement.method.select')} />
    {!state.source||state.source.engagement.applicability!=='not_applicable'?<BiddingFileList projectId={adapter.projectID} />:null}
    {adapter.pending.size?<Alert type="info" showIcon style={{marginTop:16}} message="Evidence 已保存、尚未登记的文件" description={<Space direction="vertical">{[...adapter.pending.entries()].map(([id,file]:[string,any])=><Button key={id} disabled={disabled} onClick={()=>void run(()=>adapter.append(adapter.projectID,file.category,id))}>登记 {adapter.files.get(id)?.original_name||'已上传文件'}（版本 {file.version}）</Button>)}</Space>} />:null}
  </>
}
class ReactBiddingWorkspace {
  root: Root; query: QueryClient; adapters=new Map<string,any>(); current: any=null; sequence=0; loading=false; refreshing=false; error=''; refreshError=''; readCache=new PageReadCache(); options: any
  constructor(options:any) {
    this.options=options
    const mount=options.root.querySelector('[data-real-bidding-react]')
    if(!mount)throw new Error('原 OA 招投标组件挂载点未配置')
    this.root=createRoot(mount)
    this.query=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false},mutations:{retry:false}}})
    this.render()
  }
  render() {
    this.root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={this.query}><Panel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current} /></QueryClientProvider></ConfigProvider>)
  }
  invalidate() {this.sequence++;this.current=null;this.loading=false;this.refreshing=false;this.error='';this.refreshError='';this.readCache.clear();this.query.clear();for(const adapter of this.adapters.values())adapter.transfer.clearCache();this.render()}
  async load(explicit=false) {
    const sequence=++this.sequence, details=this.options.details(), id=details?.project?.id
    try {
      if(!id)throw new Error('当前项目尚未读取')
      let adapter=this.adapters.get(id)
      if(!adapter) {
        const api=createFetchFileTransferApi({baseUrl:'',fetcher:this.options.evidenceFetch})
        const transfer=new FileTransferClient({api,uploader:createDirectUploaderRouter({aliyunOss:createAliOssMultipartUploader(),localHttp:createLocalHttpUploader()}),getCacheScope:()=>undefined})
        let storage;try{storage=globalThis.sessionStorage}catch{/* memory fallback */}
        adapter=new BiddingAdapter({flow:this.options.flow,details:()=>this.options.flow.states.get(id)?.project||details,evidenceReferenceRequest:this.options.evidenceReferenceRequest,transfer,tenantID:this.options.tenantID,subjectID:this.options.subjectID,storage,record:this.options.record,changed:()=>{if(this.current===adapter){void this.query.invalidateQueries();this.render()}}})
        this.adapters.set(id,adapter);registerAdapter(id,adapter)
      }
      this.current=adapter
      await readWorkspace(this,{sequence,key:id,explicit,hasValue:()=>!!adapter.state&&adapter.dataReady,read:()=>adapter.load(explicit),isCurrent:()=>this.options.details()?.project?.id===id,fallback:'资料读取未完成'})
    }catch(error){if(sequence===this.sequence){this.loading=false;this.refreshing=false;this.error=error instanceof Error?error.message:'资料读取未完成';this.render()}}
  }
}
export const createWorkspace = (options:any) => new ReactBiddingWorkspace(options)

function PersonnelPanel({workspace, adapter}: {workspace: ReactPersonnelWorkspace; adapter: any}) {
  const [error,setError]=useState('')
  const run=async(action:()=>Promise<unknown>)=>{setError('');try{await action()}catch(e){setError(e instanceof Error?e.message:'人员操作尚未确认')}}
  if(workspace.loading)return <WorkspaceSkeleton tip="正在读取当前项目经营参与人…" />
  if(workspace.error||!adapter?.value)return <Alert type="error" showIcon message={workspace.error||'人员分工未完整回读'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>} />
  return <>
    <Space wrap style={{marginBottom:16}}>
      <Button disabled={adapter.working} onClick={()=>void run(()=>adapter.reconcile())}>刷新当前人员</Button>
      {adapter.pending&&!adapter.conflict?<Button type="primary" disabled={adapter.working} onClick={()=>void run(()=>adapter.retry())}>继续原人员操作</Button>:null}
      <ReadStatus workspace={workspace} />
    </Space>
    {adapter.pending?<Alert type="warning" showIcon style={{marginBottom:16}} message={adapter.conflict?'项目版本冲突，请先刷新；不会自动覆盖。':'上次人员操作尚未确认；继续操作复用原命令，不重复添加。'} />:null}
    {error||adapter.error?<Alert type="error" showIcon message={error||adapter.error} style={{marginBottom:16}} />:null}
    <BusinessPersonnelList projectId={adapter.projectID} />
  </>
}
class ReactPersonnelWorkspace {
  root: Root; query: QueryClient; adapters=new Map<string,any>(); current: any=null; sequence=0; loading=false; refreshing=false; error=''; refreshError=''; readCache=new PageReadCache(); options:any
  constructor(options:any) {
    this.options=options
    const mount=options.root.querySelector('[data-real-personnel-react]')
    if(!mount)throw new Error('原 OA 经营参与人组件挂载点未配置')
    this.root=createRoot(mount)
    this.query=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false},mutations:{retry:false}}})
    this.render()
  }
  render() {
    if(this.current?.value){
      this.query.setQueryData(['projectMembers',this.current.projectID],this.current.members())
      this.query.setQueryData(['canAssignBusinessMembers',this.current.projectID],this.current.canManage())
    }
    this.root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={this.query}><PersonnelPanel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current} /></QueryClientProvider></ConfigProvider>)
  }
  invalidate(){this.sequence++;this.current=null;this.loading=false;this.refreshing=false;this.error='';this.refreshError='';this.readCache.clear();this.query.clear();this.render()}
  async load(explicit=false) {
    const sequence=++this.sequence, id=this.options.details()?.project?.id
    try {
      if(!id)throw new Error('当前项目尚未读取')
      let adapter=this.adapters.get(id)
      if(!adapter){
        let storage;try{storage=globalThis.sessionStorage}catch{/* memory fallback */}
        adapter=new ContributorAdapter({...this.options,storage,changed:()=>{if(this.current===adapter)this.render()}})
        this.adapters.set(id,adapter);registerContributor(id,adapter)
      }
      this.current=adapter
      await readWorkspace(this,{sequence,key:id,explicit,hasValue:()=>!!adapter.value,read:()=>adapter.load(),isCurrent:()=>this.options.details()?.project?.id===id,fallback:'人员读取未完成'})
    }catch(error){if(sequence===this.sequence){this.loading=false;this.refreshing=false;this.error=error instanceof Error?error.message:'人员读取未完成';this.render()}}
  }
}
export const createPersonnelWorkspace = (options:any) => new ReactPersonnelWorkspace(options)

function ContractPanel({workspace,adapter}:{workspace:ReactContractWorkspace;adapter:any}){
  const [error,setError]=useState('')
  const run=async(action:()=>Promise<unknown>)=>{setError('');try{await action()}catch(e){setError(e instanceof Error?e.message:'合同操作尚未确认')}}
  if(workspace.loading)return <WorkspaceSkeleton tip="正在读取当前项目两链合同…" />
  if(workspace.error||!adapter?.context)return <Alert type="error" showIcon message={workspace.error||'合同原生权限上下文未读取'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>} />
  return <>
    <Space wrap style={{marginBottom:16}}><Button disabled={adapter.working} onClick={()=>void run(()=>workspace.load(true))}>刷新当前合同</Button>{adapter.pending&&!adapter.conflict?<Button type="primary" disabled={adapter.working} onClick={()=>void run(()=>adapter.hasInvalidLink()?adapter.repairInvalidLink():adapter.retry())}>{adapter.hasInvalidLink()?'修复无效关联并续办':'继续原合同操作'}</Button>:null}<ReadStatus workspace={workspace} /></Space>
    <Alert type="info" showIcon style={{marginBottom:16}} message="合同草稿与显式确认" description="基础保存仍是草稿；抽屉内维护签约主体并确认原始合同。协议和结算须显式确认，正式历史不可覆盖。三类附件分别上传并登记引用，移出不删除文件；最终结算后更正尚未实现。" />
    {adapter.pending?<Alert type="warning" showIcon style={{marginBottom:16}} message={adapter.conflict?'版本冲突，需要核对；不会自动更新版本覆盖。':`上次操作尚未全部确认（${adapter.pending.stage}）；继续操作复用原命令，不重建合同。`} />:null}
    {error||adapter.error?<Alert type="error" showIcon message={error||adapter.error} style={{marginBottom:16}} />:null}
    <ContractAndAmendments projectId={adapter.projectID} />
  </>
}
class ReactContractWorkspace {
  root:Root;query:QueryClient;adapters=new Map<string,any>();current:any=null;sequence=0;loading=false;refreshing=false;error='';refreshError='';readCache=new PageReadCache();options:any
  seen=new Map<string,string>()
  constructor(options:any){this.options=options;const mount=options.root.querySelector('[data-real-contract-react]');if(!mount)throw new Error('合同组件挂载点未配置');this.root=createRoot(mount);this.query=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false},mutations:{retry:false}}});this.render()}
  render(){
    if(this.current?.context){const a=this.current;this.query.setQueryData(['contracts',a.projectID],a.list());this.query.setQueryData(['canManageBusinessInfo',a.projectID],a.canManage());for(const v of a.native.values()){const signature=JSON.stringify(v);if(signature!==this.seen.get(v.contract.id)){this.query.setQueryData(['contractNative',v.contract.id],v);this.query.setQueryData(['contract',v.contract.id],a.view(v));this.query.setQueryData(['contractAmendments',a.projectID,v.contract.id],a.amendments(v.contract.id));this.seen.set(v.contract.id,signature)}this.query.setQueryData(['canManageContract',a.projectID,v.contract.id],a.canEdit(v.contract.id));this.query.setQueryData(['canManageContractAmendments',a.projectID,v.contract.id],a.canManageAmendments(v.contract.id));}}
    this.root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={this.query}><ContractPanel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current} /></QueryClientProvider></ConfigProvider>)
  }
  invalidate(){this.sequence++;this.current=null;this.loading=false;this.refreshing=false;this.error='';this.refreshError='';this.readCache.clear();this.query.clear();this.seen.clear();for(const adapter of this.adapters.values())adapter.transfer.clearCache();this.render()}
  async load(explicit=false){const sequence=++this.sequence,id=this.options.details()?.project?.id;try{
    if(!id)throw new Error('当前项目尚未读取');let adapter=this.adapters.get(id);if(!adapter){let storage;try{storage=globalThis.sessionStorage}catch{}const api=createFetchFileTransferApi({baseUrl:'',fetcher:this.options.evidenceFetch});const transfer=new FileTransferClient({api,uploader:createDirectUploaderRouter({aliyunOss:createAliOssMultipartUploader(),localHttp:createLocalHttpUploader()}),getCacheScope:()=>undefined});adapter=new ContractAdapter({...this.options,transfer,storage,changed:()=>{if(this.current===adapter)this.render()}});this.adapters.set(id,adapter);registerContract(id,adapter)}
    this.current=adapter;await readWorkspace(this,{sequence,key:id,explicit,hasValue:()=>!!adapter.context,read:()=>adapter.load(),isCurrent:()=>this.options.details()?.project?.id===id,fallback:'合同读取未完成'});
  }catch(e){if(sequence===this.sequence){this.loading=false;this.refreshing=false;this.error=e instanceof Error?e.message:'合同读取未完成';this.render()}}}
}
export const createContractWorkspace=(options:any)=>new ReactContractWorkspace(options)

function CashPanel({workspace,adapter}:{workspace:ReactCashWorkspace;adapter:any}){
  const [error,setError]=useState('')
  const run=async(action:()=>Promise<unknown>)=>{setError('');try{await action()}catch(e){setError(e instanceof Error?e.message:'支付操作未确认')}}
  if(workspace.loading)return <WorkspaceSkeleton tip="正在读取两链实际资金…" />
  if(workspace.error||!adapter?.context)return <Alert type="error" showIcon message={workspace.error||'资金权限未读取'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>} />
  return <>
    <Space wrap style={{marginBottom:16}}><Button disabled={adapter.working} onClick={()=>void run(()=>workspace.load(true))}>刷新当前支付</Button>{adapter.pending&&!adapter.conflict?<Button type="primary" disabled={adapter.working} onClick={()=>void run(()=>adapter.retry())}>继续原支付操作</Button>:null}<ReadStatus workspace={workspace} /></Space>
    {adapter.pending?<Alert type="warning" showIcon style={{marginBottom:16}} message={adapter.conflict?'资金／项目版本冲突，需要核对，不自动覆盖。':`支付操作尚未全部确认（${adapter.pending.stage}）；不会重复新建资金。`} />:null}
    {error||adapter.error?<Alert type="error" showIcon message={error||adapter.error} style={{marginBottom:16}} />:null}
    <Alert type="info" showIcon style={{marginBottom:16}} message="甲方实付与公司实收分别登记" description="每条是 Money 独立的实际资金，项目只保存两链／费用关系。已确认记录不可覆盖；开票关联与核销操作尚未接通。" />
    <div className="business-panel-surface__groups transaction-panel__groups"><PaymentRecordList projectId={adapter.projectID} chainType="client_to_qualification" hideCard groupTitle="甲方实付" groupDescription="甲方向资质单位的实际支付" groupKicker="甲方 → 资质单位" /><PaymentRecordList projectId={adapter.projectID} chainType="qualification_to_our_company" hideCard groupTitle="公司实收" groupDescription="资质单位向公司的实际支付" groupKicker="资质单位 → 我方公司" groupTone="income" /></div>
  </>
}
class ReactCashWorkspace {
  root:Root;query:QueryClient;adapters=new Map<string,any>();current:any=null;sequence=0;loading=false;refreshing=false;error='';refreshError='';readCache=new PageReadCache();options:any
  constructor(options:any){this.options=options;const mount=options.root.querySelector('[data-real-cash-react]');if(!mount)throw new Error('支付组件挂载点未配置');this.root=createRoot(mount);this.query=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false},mutations:{retry:false}}});this.render()}
  render(){if(this.current?.context){const a=this.current;this.query.setQueryData(['projectCash',a.projectID],a.list());this.query.setQueryData(['canManageCash',a.projectID],a.canManage());for(const v of a.list().payment_records)this.query.setQueryData(['cashPayment',a.projectID,v.id],v)}this.root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={this.query}><CashPanel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current} /></QueryClientProvider></ConfigProvider>)}
  invalidate(){this.sequence++;this.current=null;this.loading=false;this.refreshing=false;this.error='';this.refreshError='';this.readCache.clear();this.query.clear();this.render()}
  async load(explicit=false){const sequence=++this.sequence,id=this.options.details()?.project?.id;try{if(!id)throw new Error('当前项目尚未读取');let adapter=this.adapters.get(id);if(!adapter){let storage;try{storage=globalThis.sessionStorage}catch{}adapter=new CashAdapter({...this.options,storage,changed:()=>{if(this.current===adapter)this.render()}});this.adapters.set(id,adapter);registerCash(id,adapter)}this.current=adapter;await readWorkspace(this,{sequence,key:id,explicit,hasValue:()=>!!adapter.context,read:()=>adapter.load(),isCurrent:()=>this.options.details()?.project?.id===id,fallback:'资金读取未完成'})}catch(e){if(sequence===this.sequence){this.loading=false;this.refreshing=false;this.error=e instanceof Error?e.message:'资金读取未完成';this.render()}}}
}
export const createCashWorkspace=(options:any)=>new ReactCashWorkspace(options)

function InvoicePanel({workspace,adapter}:{workspace:ReactInvoiceWorkspace;adapter:any}){
 const [error,setError]=useState('')
 const run=async(fn:()=>Promise<unknown>)=>{setError('');try{await fn()}catch(e){setError(e instanceof Error?e.message:'开票操作未确认')}}
 if(workspace.loading)return <WorkspaceSkeleton tip="正在读取当前项目开票登记…" />
 if(workspace.error||!adapter?.context)return <Alert type="error" showIcon message={workspace.error||'开票权限未读取'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>} />
 return <>
  <Space wrap style={{marginBottom:16}}><Button disabled={adapter.working} onClick={()=>void run(()=>workspace.load(true))}>刷新当前开票</Button>{adapter.pending&&!adapter.conflict&&<Button type="primary" disabled={adapter.working} onClick={()=>void run(()=>adapter.retry())}>继续原开票操作</Button>}<ReadStatus workspace={workspace} /></Space>
  {adapter.pending&&<Alert type="warning" showIcon style={{marginBottom:16}} message={adapter.conflict?'开票／项目版本冲突，须核对，不自动覆盖。':`开票操作尚未全部确认（${adapter.pending.stage}），复用原命令、不重复登记。`} />}
  {(error||adapter.error)&&<Alert type="error" showIcon style={{marginBottom:16}} message={error||adapter.error} />}
  {(adapter.contextError||adapter.partyError)&&<Alert type="warning" showIcon style={{marginBottom:16}} message={adapter.contextError||adapter.partyError} action={adapter.partyError?<Button disabled={adapter.working} onClick={()=>void run(()=>adapter.loadParties())}>重新读取主体</Button>:undefined} />}
  <Alert type="info" showIcon style={{marginBottom:16}} message="两条开票往来链分别登记" description="Billing 保存已开具票据及追加历史；与实际资金分别确认。支付关联尚未接通，登记无效不是税务作废。" />
  <div className="business-panel-surface__groups transaction-panel__groups"><InvoiceRecordList projectId={adapter.projectID} chainType="client_to_qualification" hideCard groupTitle="甲方开票" groupDescription="对应甲方与资质单位的开票登记" groupKicker="甲方 ↔ 资质单位" /><InvoiceRecordList projectId={adapter.projectID} chainType="qualification_to_our_company" hideCard groupTitle="公司开票" groupDescription="对应资质单位与我方公司的开票登记" groupKicker="资质单位 ↔ 我方公司" groupTone="income" /></div>
 </>
}
class ReactInvoiceWorkspace {
 root:Root;query:QueryClient;adapters=new Map<string,any>();current:any=null;sequence=0;loading=false;refreshing=false;error='';refreshError='';readCache=new PageReadCache();options:any;seen=new Map<string,string>()
 constructor(options:any){this.options=options;const mount=options.root.querySelector('[data-real-invoice-react]');if(!mount)throw new Error('开票组件挂载点未配置');this.root=createRoot(mount);this.query=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false},mutations:{retry:false}}});this.render()}
 render(){if(this.current?.context){const a=this.current;this.query.setQueryData(['projectInvoices',a.projectID],a.list());this.query.setQueryData(['canManageInvoice',a.projectID],a.canManage());for(const v of a.list().invoice_records){const signature=JSON.stringify(v);if(signature!==this.seen.get(v.id)){this.seen.set(v.id,signature);this.query.setQueryData(['invoice',a.projectID,v.id],v)}}}this.root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={this.query}><InvoicePanel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current} /></QueryClientProvider></ConfigProvider>)}
 invalidate(){this.sequence++;this.current=null;this.loading=false;this.refreshing=false;this.error='';this.refreshError='';this.readCache.clear();this.query.clear();this.seen.clear();for(const a of this.adapters.values())a.transfer.clearCache();this.render()}
 async load(explicit=false){const sequence=++this.sequence,id=this.options.details()?.project?.id;try{if(!id)throw new Error('当前项目尚未读取');let a=this.adapters.get(id);if(!a){let storage;try{storage=globalThis.sessionStorage}catch{}const api=createFetchFileTransferApi({baseUrl:'',fetcher:this.options.evidenceFetch});const transfer=new FileTransferClient({api,uploader:createDirectUploaderRouter({aliyunOss:createAliOssMultipartUploader(),localHttp:createLocalHttpUploader()}),getCacheScope:()=>undefined});a=new InvoiceAdapter({...this.options,storage,transfer,changed:()=>{if(this.current===a)this.render()}});this.adapters.set(id,a);registerInvoice(id,a)}this.current=a;await readWorkspace(this,{sequence,key:id,explicit,hasValue:()=>!!a.context,read:()=>a.load(),isCurrent:()=>this.options.details()?.project?.id===id,fallback:'开票读取未完成'})}catch(e){if(sequence===this.sequence){this.loading=false;this.refreshing=false;this.error=e instanceof Error?e.message:'开票读取未完成';this.render()}}}
}
export const createInvoiceWorkspace=(options:any)=>new ReactInvoiceWorkspace(options)

function CostPanel({workspace,adapter}:{workspace:ReactCostWorkspace;adapter:any}){
 const [error,setError]=useState('')
 const run=async(fn:()=>Promise<unknown>)=>{setError('');try{await fn()}catch(e){setError(e instanceof Error?e.message:'成本操作尚未确认')}}
 if(workspace.loading)return <WorkspaceSkeleton tip="正在读取当前项目成本…" />
 if(workspace.error||!adapter?.context)return <Alert type="error" showIcon message={workspace.error||'成本原生权限未完整读取'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>}/>
 return <><Space wrap style={{marginBottom:16}}><Button disabled={adapter.working} onClick={()=>void run(()=>workspace.load(true))}>刷新当前成本</Button>{adapter.pending&&!adapter.conflict&&<Button type="primary" disabled={adapter.working} onClick={()=>void run(()=>adapter.retry())}>继续原成本操作</Button>}<ReadStatus workspace={workspace} /></Space>
 {adapter.pending&&<Alert type="warning" style={{marginBottom:16}} message={adapter.conflict?'成本版本冲突，须核对，不自动覆盖。':'成本操作尚未全部确认；继续复用原命令，已上传文件保留。'}/>} {(error||adapter.error)&&<Alert type="error" style={{marginBottom:16}} message={error||adapter.error}/>}
 <ProductionCostList projectId={adapter.projectID} scope={adapter.scope}/></>
}
class ReactCostWorkspace {
 root:Root;query:QueryClient;adapters=new Map<string,any>();current:any=null;sequence=0;loading=false;refreshing=false;error='';refreshError='';readCache=new PageReadCache();options:any
 constructor(options:any){this.options=options;const mount=options.root.querySelector('[data-real-cost-react]');if(!mount)throw new Error('成本组件挂载点未配置');this.root=createRoot(mount);this.query=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false},mutations:{retry:false}}});this.render()}
 render(){if(this.current?.context){const a=this.current;this.query.setQueryData(['costs',a.projectID,a.scope],a.list());this.query.setQueryData(['costStatistics',a.projectID,a.scope],a.statistics());}this.root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={this.query}><CostPanel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current}/></QueryClientProvider></ConfigProvider>)}
 invalidate(){this.sequence++;this.current=null;this.loading=false;this.refreshing=false;this.error='';this.refreshError='';this.readCache.clear();this.query.clear();for(const a of this.adapters.values())a.transfer.clearCache();this.render()}
 async load(explicit=false){const sequence=++this.sequence,id=this.options.details()?.project?.id;try{if(!id)throw new Error('当前项目尚未读取');let a=this.adapters.get(id);if(!a){let storage;try{storage=globalThis.sessionStorage}catch{}const api=createFetchFileTransferApi({baseUrl:'',fetcher:this.options.evidenceFetch});const transfer=new FileTransferClient({api,uploader:createDirectUploaderRouter({aliyunOss:createAliOssMultipartUploader(),localHttp:createLocalHttpUploader()}),getCacheScope:()=>undefined});a=new CostAdapter({...this.options,storage,transfer,changed:()=>{if(this.current===a)this.render()}});this.adapters.set(id,a);registerCost(id,this.options.scope,a)}this.current=a;await readWorkspace(this,{sequence,key:`${id}:${this.options.scope}`,explicit,hasValue:()=>!!a.context,read:()=>a.load(),isCurrent:()=>this.options.details()?.project?.id===id,fallback:'成本读取未完成'})}catch(e){if(sequence===this.sequence){this.loading=false;this.refreshing=false;this.error=e instanceof Error?e.message:'成本读取未完成';this.render()}}}
}
export const createCostWorkspace=(options:any)=>new ReactCostWorkspace(options)

const resultLabels:Record<string,string>={approval:'批复',audit:'审计',recorded:'有效',withdrawn:'已撤回',design_fee:'设计费',survey_fee:'勘察费',consultation_fee:'咨询费'}
function LegacyApprovalAuditPanel({workspace,adapter}:{workspace:LegacyReactApprovalAuditWorkspace;adapter:any}){
 const [form]=Form.useForm(),[open,setOpen]=useState(false),[editing,setEditing]=useState<any>(null),[kind,setKind]=useState<'approval'|'audit'>('approval'),[history,setHistory]=useState<any[]|null>(null),[busy,setBusy]=useState(false)
 const run=async(fn:()=>Promise<unknown>)=>{setBusy(true);try{await fn();message.success('操作已由批复审计系统确认');setOpen(false);setEditing(null);await workspace.load(true)}catch(e){message.error(e instanceof Error?e.message:'操作未确认')}finally{setBusy(false)}}
 if(workspace.loading)return <WorkspaceSkeleton tip="正在读取当前项目批复与审计…"/>
 if(workspace.error||!adapter?.context)return <Alert type="error" showIcon message={workspace.error||'批复审计原生权限未完整读取'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>}/>
 const rows=adapter.list(),openForm=(nextKind:'approval'|'audit',row?:any)=>{setKind(nextKind);setEditing(row||null);form.resetFields();form.setFieldsValue({source_key:adapter.sources()[0]&&`${adapter.sources()[0].application}:${adapter.sources()[0].resource_type}:${adapter.sources()[0].resource_id}:${adapter.sources()[0].revision}`,confirmed_by:row?.confirmed_by||'',confirmed_at:row?.confirmed_at?dayjs(row.confirmed_at):dayjs(),reason:''});setOpen(true)}
 return <>
  <section className="project-workspace__card approval-audit"><header className="project-workspace__card-header"><div><h3 className="project-workspace__card-title">批复审计信息</h3><p className="project-workspace__card-subtitle">复用原 OA 的批复／审计双列语义；事实、金额和历史由 Approval &amp; Audit Results 保存。</p></div><Space><Button onClick={()=>void workspace.load(true)}>刷新</Button><Button disabled={!adapter.canCreate('approval')} onClick={()=>openForm('approval')}>登记批复</Button><Button type="primary" disabled={!adapter.canCreate('audit')} onClick={()=>openForm('audit')}>登记审计</Button></Space></header>
  {!rows.length?<Empty description="当前项目尚无批复或审计事实；按钮不可用时请先补齐已连接合同来源，而不是扩大权限。"/>:<div className="approval-audit__grid">{rows.map((row:any)=><Card key={row.id} title={`${resultLabels[row.kind]} · revision ${row.revision}`} extra={<Tag color={row.state==='recorded'?'green':'default'}>{resultLabels[row.state]}</Tag>}><Descriptions size="small" column={1} items={[{key:'amount',label:'合计',children:row.total_amount===null?'金额已脱敏':`CNY ${row.total_amount}`},...row.components.map((item:any)=>({key:item.kind,label:resultLabels[item.kind]||item.kind,children:`CNY ${item.amount}`})),{key:'source',label:'冻结来源',children:`${row.source?.resource_type||'—'} / revision ${row.source?.revision||'—'}`},{key:'confirmed',label:'外部确认',children:`${row.confirmed_by||'—'} · ${row.confirmed_at?dayjs(row.confirmed_at).format('YYYY-MM-DD HH:mm'):'—'}`} ]}/><Space wrap><Button disabled={!row.can_correct} onClick={()=>openForm(row.kind,row)}>追加纠正</Button><Button disabled={!row.can_history} onClick={()=>void run(async()=>setHistory(await adapter.history(row.id)))}>查看历史</Button><Button danger disabled={!row.can_withdraw} onClick={()=>{const reason=globalThis.prompt?.('请输入撤回原因')||'';if(reason)void run(()=>adapter.withdraw(row.id,reason))}}>撤回</Button></Space></Card>)}</div>}
  </section>
  <Drawer width={560} title={editing?`纠正${resultLabels[kind]}`:`登记${resultLabels[kind]}`} open={open} onClose={()=>setOpen(false)} destroyOnClose extra={<Button type="primary" loading={busy} onClick={()=>form.submit()}>保存</Button>}><Form form={form} layout="vertical" onFinish={values=>void run(()=>editing?adapter.correct(editing.id,{...values,confirmed_at:values.confirmed_at.toISOString()}):adapter.create(kind,{...values,confirmed_at:values.confirmed_at.toISOString()}))}>{kind==='approval'&&!editing&&<Form.Item name="source_key" label="合同／结算来源" rules={[{required:true}]}><Select options={adapter.sources().map((source:any)=>({value:`${source.application}:${source.resource_type}:${source.resource_id}:${source.revision}`,label:`${source.label} · revision ${source.revision}`}))}/></Form.Item>}<Form.Item name="confirmed_by" label="外部确认人／单位" rules={[{required:true}]}><Input maxLength={200}/></Form.Item><Form.Item name="confirmed_at" label="确认时间" rules={[{required:true}]}><DatePicker showTime style={{width:'100%'}}/></Form.Item>{editing&&<Form.Item name="reason" label="纠正原因" rules={[{required:true}]}><Input.TextArea rows={3} maxLength={500}/></Form.Item>}<Alert type="info" showIcon message="金额沿用冻结来源" description="本表单不在页面重算金额；批复沿用精确合同／结算 revision，审计沿用有效批复 revision。"/></Form></Drawer>
  <Modal open={history!==null} title="批复审计历史" footer={null} onCancel={()=>setHistory(null)}><List dataSource={history||[]} renderItem={(item:any)=><List.Item><List.Item.Meta title={`${item.event_kind} · revision ${item.revision}`} description={`${item.reason||'—'} · ${dayjs(item.occurred_at).format('YYYY-MM-DD HH:mm')}`}/></List.Item>}/></Modal>
 </>
}
class LegacyReactApprovalAuditWorkspace{
 root:Root;current:any=null;sequence=0;loading=false;error='';options:any
 constructor(options:any){this.options=options;const mount=options.root.querySelector('[data-real-approval-audit-react]');if(!mount)throw new Error('批复审计组件挂载点未配置');this.root=createRoot(mount);this.render()}
 render(){this.root.render(<ConfigProvider locale={zhCN}><LegacyApprovalAuditPanel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current}/></ConfigProvider>)}
 invalidate(){this.sequence++;this.current=null;this.loading=false;this.error='';this.render()}
 async load(_explicit=false){const sequence=++this.sequence,id=this.options.details()?.project?.id;this.loading=true;this.error='';this.render();try{if(!id)throw new Error('当前项目尚未读取');if(!this.current||this.current.projectID!==id)this.current=new ApprovalAuditAdapter({...this.options,changed:()=>this.render()});await this.current.load();if(sequence===this.sequence){this.loading=false;this.render()}}catch(e){if(sequence===this.sequence){this.loading=false;this.error=e instanceof Error?e.message:'批复审计读取未完成';this.render()}}}
}
const createLegacyApprovalAuditWorkspace=(options:any)=>new LegacyReactApprovalAuditWorkspace(options)

function NativeApprovalAuditPanel({workspace,adapter}:{workspace:ReactApprovalAuditWorkspace;adapter:any}){
 const [form]=Form.useForm(),[open,setOpen]=useState(false),[editing,setEditing]=useState<any>(null),[kind,setKind]=useState<'approval'|'audit'>('approval'),[history,setHistory]=useState<any[]|null>(null),[busy,setBusy]=useState(false),[fileList,setFileList]=useState<UploadFile[]>([]),[preview,setPreview]=useState<ProjectFile|null>(null)
 const useReference=Form.useWatch('use_reference',form),amountValues=Form.useWatch([],form)
 const total=useMemo(()=>['design_fee','survey_fee','consultation_fee'].reduce((sum,key)=>{const raw=String(amountValues?.[key]??'');return /^(0|[1-9]\d*)(\.\d{1,2})?$/.test(raw)?sum+Number(raw):sum},0),[amountValues])
 const run=async(fn:()=>Promise<unknown>)=>{setBusy(true);try{await fn();message.success('操作已由批复审计系统确认');setOpen(false);setEditing(null);setFileList([]);await workspace.load(true)}catch(e){message.error(e instanceof Error?e.message:'操作未确认')}finally{setBusy(false)}}
 if(workspace.loading)return <WorkspaceSkeleton tip="正在读取当前项目批复与审计…"/>
 if(workspace.error||!adapter?.context)return <Alert type="error" showIcon message={workspace.error||'批复审计原生权限未完整读取'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>}/>
 const rows=adapter.list(),byKind=new Map(rows.map((row:any)=>[row.kind,row])),done=(file:any):UploadFile=>({uid:file.id,name:file.original_name,size:file.file_size,status:'done',response:file})
 const openForm=(nextKind:'approval'|'audit',row?:any)=>{const known=[...(row?.report_files||[]),...adapter.stagedFiles(nextKind)];setKind(nextKind);setEditing(row||null);setFileList(known.map(done));form.resetFields();const amounts=Object.fromEntries((row?.components||[]).map((item:any)=>[item.kind,item.amount]));form.setFieldsValue({source_key:adapter.sources()[0]&&`${adapter.sources()[0].application}:${adapter.sources()[0].resource_type}:${adapter.sources()[0].resource_id}:${adapter.sources()[0].revision}`,confirmed_by:row?.confirmed_by||'',confirmed_at:row?.confirmed_at?dayjs(row.confirmed_at):dayjs(),use_reference:row?row.uses_source_amount:true,...amounts,override_reason:row?.override_reason||'',reason:''});setOpen(true)}
 const submit=async(values:any)=>{const ids:string[]=[];for(const file of fileList){if(file.originFileObj){const uploaded=await adapter.upload(kind,file.originFileObj);ids.push(uploaded.id);setFileList(current=>current.map(item=>item.uid===file.uid?done(uploaded):item));}else ids.push(String(file.uid));}const input={...values,report_file_ids:ids,confirmed_at:values.confirmed_at.toISOString()};return editing?adapter.correct(editing.id,input):adapter.create(kind,input)}
 const renderFiles=(row:any)=>!row?.report_files?.length?<div className="approval-audit__empty-block approval-audit__empty-block--file"><Empty description="暂无文件" image={Empty.PRESENTED_IMAGE_SIMPLE}/></div>:<div>{row.report_files.map((file:any)=><div className="approval-audit__file" key={file.id}><div className="approval-audit__file-main"><span className="approval-audit__file-icon">▤</span><div><button className="approval-audit__file-name" type="button" onClick={()=>setPreview(file)}>{file.original_name}</button><div className="approval-audit__file-meta">{(file.file_size/1024/1024).toFixed(2)}MB · {dayjs(file.created_at).format('YYYY-MM-DD')}</div></div></div><Space wrap><Button type="text" size="small" onClick={()=>setPreview(file)}>预览</Button><Button type="text" size="small" onClick={()=>void fileService.downloadFile(file.id,file.original_name)}>下载</Button></Space></div>)}</div>
 const renderAmounts=(row:any,label:string)=>!row?<div className="approval-audit__empty-block"><Empty description="暂无数据" image={Empty.PRESENTED_IMAGE_SIMPLE}/></div>:row.total_amount===null?<Alert type="info" showIcon message={`${label}金额已按权限脱敏`}/>:<div className="approval-audit__amounts">{row.components.map((item:any)=><div key={item.kind}><span>{resultLabels[item.kind]||item.kind}</span><strong>¥{item.amount}</strong></div>)}<div className="approval-audit__amount-total"><span>合计</span><strong>¥{row.total_amount}</strong></div></div>
 const renderColumn=(nextKind:'approval'|'audit')=>{const row:any=byKind.get(nextKind),label=resultLabels[nextKind];return <section className={`approval-audit__column approval-audit__column--${nextKind}`}><header className="approval-audit__column-header"><div><span>{label}</span><strong>{row?.total_amount===null?'金额已脱敏':row?`¥${row.total_amount}`:'—'}</strong></div>{row?.state==='recorded'?<Button className="approval-audit__edit-button" type="text" disabled={!row.can_correct} onClick={()=>openForm(nextKind,row)}>✎ 编辑{label}</Button>:<Button className="approval-audit__edit-button" type="text" disabled={!adapter.canCreate(nextKind)} onClick={()=>openForm(nextKind)}>＋ 新增{label}</Button>}</header><div className="approval-audit__column-body">{row&&<Space wrap style={{marginBottom:12}}><Tag color={row.state==='recorded'?'green':'default'}>{resultLabels[row.state]}</Tag><span>revision {row.revision}</span><span>{row.confirmed_by} · {dayjs(row.confirmed_at).format('YYYY-MM-DD HH:mm')}</span></Space>}<div className="approval-audit__section-title">{label}报告</div>{renderFiles(row)}<div className="approval-audit__section-title approval-audit__section-title--amount">{label}金额</div>{renderAmounts(row,label)}{row&&<Space wrap style={{marginTop:16}}><Button disabled={!row.can_history} onClick={()=>void adapter.history(row.id).then(setHistory).catch((error:Error)=>message.error(error.message))}>查看历史</Button><Button danger disabled={!row.can_withdraw} onClick={()=>{const reason=globalThis.prompt?.(`请输入撤回${label}原因`)||'';if(reason)void run(()=>adapter.withdraw(row.id,reason))}}>撤回</Button></Space>}</div></section>}
 return <>
  <section className="project-workspace__card approval-audit"><header className="approval-audit__header"><div><h3 className="project-workspace__card-title">批复审计信息</h3><p className="approval-audit__subtitle">汇总批复、审计报告文件与三项精确金额；每次修改追加 Results revision。</p></div><Space><Button disabled={adapter.working} onClick={()=>void workspace.load(true)}>刷新</Button>{adapter.pending&&!adapter.conflict&&<Button type="primary" disabled={adapter.working} onClick={()=>void run(()=>adapter.retry())}>继续原操作</Button>}<ReadStatus workspace={workspace}/></Space></header>
  {adapter.pending&&<Alert type="warning" showIcon style={{marginBottom:16}} message={adapter.conflict?'版本冲突，须重新读取并核对；不会自动覆盖。':'原批复审计命令尚未确认；继续会复用同一命令，不重复登记。'}/>} {(adapter.error||workspace.refreshError)&&<Alert type="error" showIcon style={{marginBottom:16}} message={adapter.error||workspace.refreshError}/>}<div className="approval-audit__grid">{renderColumn('approval')}{renderColumn('audit')}</div><p className="reference-projection-note"><strong>边界说明：</strong>Results 拥有结果、金额快照和历史；Contract 只提供精确来源，Evidence 只保管文件，Workbench 不复制业务事实。</p>
  </section>
  <Drawer width={620} title={editing?`编辑${resultLabels[kind]}（追加纠正）`:`新增${resultLabels[kind]}`} open={open} onClose={()=>setOpen(false)} destroyOnHidden extra={<Button type="primary" loading={busy||adapter.working} onClick={()=>form.submit()}>保存</Button>}><Form form={form} layout="vertical" onFinish={values=>void run(()=>submit(values))}>{kind==='approval'&&!editing&&<Form.Item name="source_key" label="合同／结算来源" rules={[{required:true,message:'请选择精确来源 revision'}]}><Select options={adapter.sources().map((source:any)=>({value:`${source.application}:${source.resource_type}:${source.resource_id}:${source.revision}`,label:`${source.label} · revision ${source.revision}`}))}/></Form.Item>}<Form.Item label={`${resultLabels[kind]}报告`}><AttachmentUploader helper="支持多个 PDF、Word、图片；先由 Evidence 完成上传，再随本次 Results 命令冻结引用。" fileList={fileList} onChange={setFileList} beforeUpload={()=>false} accept=".pdf,.png,.jpg,.jpeg,.doc,.docx" disabled={!adapter.canEvidence(kind)&&!busy} uploading={busy} onRemove={async file=>{if(String(file.uid).startsWith('approval-audit-upload:')){adapter.staged.delete(String(file.uid));adapter.persist();adapter.changed()}setFileList(current=>current.filter(item=>item.uid!==file.uid));return false}} onPreview={file=>{if(file.originFileObj)message.info('本地文件将在上传完成后预览');else{const value=adapter.files.get(String(file.uid));if(value)setPreview(value)}}} onDownload={file=>void fileService.downloadFile(String(file.uid),file.name)}/></Form.Item><div className="approval-audit-drawer__amount-head"><span>{resultLabels[kind]}金额</span><Form.Item name="use_reference" valuePropName="checked" noStyle><Checkbox>引用{kind==='approval'?'合同／结算':'当前批复'}精确金额</Checkbox></Form.Item></div>{useReference===false&&<><div className="approval-audit-drawer__amount-grid">{(['design_fee','survey_fee','consultation_fee'] as const).map(key=><Form.Item key={key} name={key} label={`${resultLabels[key]}（元）`} rules={[{required:true},{pattern:/^(0|[1-9]\d*)(\.\d{1,2})?$/,message:'请输入非负金额，最多两位小数'}]}><Input inputMode="decimal" prefix="¥"/></Form.Item>)}</div><div className="approval-audit-drawer__total"><span>合计</span><strong>¥{total.toFixed(2)}</strong></div><Form.Item name="override_reason" label="金额说明" rules={[{required:true,message:'手工金额必须说明与冻结来源的差异'}]}><Input.TextArea rows={3} maxLength={1000}/></Form.Item></>}<Form.Item name="confirmed_by" label="外部确认人／单位" rules={[{required:true}]}><Input maxLength={200}/></Form.Item><Form.Item name="confirmed_at" label="确认时间" rules={[{required:true}]}><DatePicker showTime style={{width:'100%'}}/></Form.Item>{editing&&<Form.Item name="reason" label="纠正原因" rules={[{required:true}]}><Input.TextArea rows={3} maxLength={1000}/></Form.Item>}<Alert type="info" showIcon message="修改会追加 revision" description="移出文件只改变本结果的引用，不删除 Evidence 文件；原金额、文件和来源仍保留在历史 revision。"/></Form></Drawer>
  <Modal open={history!==null} title="批复审计历史" footer={null} onCancel={()=>setHistory(null)}><List dataSource={history||[]} renderItem={(item:any)=><List.Item><List.Item.Meta title={`${item.event_kind} · revision ${item.revision}`} description={`${item.snapshot?.state==='withdrawn'?'已撤回':'有效'} · ${item.snapshot?.total_amount===null?'金额脱敏':`¥${item.snapshot?.total_amount}`} · ${item.reason||'—'} · ${dayjs(item.occurred_at).format('YYYY-MM-DD HH:mm')}`}/></List.Item>}/></Modal><FilePreviewDrawer open={!!preview} file={preview} onClose={()=>setPreview(null)}/>
 </>
}
class ReactApprovalAuditWorkspace{
 root:Root;adapters=new Map<string,any>();current:any=null;sequence=0;loading=false;refreshing=false;error='';refreshError='';readCache=new PageReadCache();options:any
 constructor(options:any){this.options=options;const mount=options.root.querySelector('[data-real-approval-audit-react]');if(!mount)throw new Error('批复审计组件挂载点未配置');this.root=createRoot(mount);this.render()}
 render(){this.root.render(<ConfigProvider locale={zhCN}><NativeApprovalAuditPanel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current}/></ConfigProvider>)}
 invalidate(){this.sequence++;this.current=null;this.loading=false;this.refreshing=false;this.error='';this.refreshError='';this.readCache.clear();for(const adapter of this.adapters.values())adapter.transfer.clearCache();this.render()}
 async load(explicit=false){const sequence=++this.sequence,id=this.options.details()?.project?.id;try{if(!id)throw new Error('当前项目尚未读取');let adapter=this.adapters.get(id);if(!adapter){let storage;try{storage=globalThis.sessionStorage}catch{}const api=createFetchFileTransferApi({baseUrl:'',fetcher:this.options.evidenceFetch});const transfer=new FileTransferClient({api,uploader:createDirectUploaderRouter({aliyunOss:createAliOssMultipartUploader(),localHttp:createLocalHttpUploader()}),getCacheScope:()=>undefined});adapter=new ApprovalAuditAdapter({...this.options,storage,transfer,changed:()=>{if(this.current===adapter)this.render()}});this.adapters.set(id,adapter);registerApprovalAudit(id,adapter)}this.current=adapter;await readWorkspace(this,{sequence,key:id,explicit,hasValue:()=>!!adapter.context,read:()=>adapter.load(),isCurrent:()=>this.options.details()?.project?.id===id,fallback:'批复审计读取未完成'})}catch(e){if(sequence===this.sequence){this.loading=false;this.refreshing=false;this.error=e instanceof Error?e.message:'批复审计读取未完成';this.render()}}}
}
export const createApprovalAuditWorkspace=(options:any)=>new ReactApprovalAuditWorkspace(options)

function CommissionPanel({workspace,adapter}:{workspace:ReactCommissionWorkspace;adapter:any}){
 const [form]=Form.useForm(),[assessmentForm]=Form.useForm(),[suggestionForm]=Form.useForm(),[open,setOpen]=useState(false),[editing,setEditing]=useState<any>(null),[history,setHistory]=useState<any[]|null>(null),[assessment,setAssessment]=useState<any>(null),[suggestion,setSuggestion]=useState<any>(null),[fileList,setFileList]=useState<UploadFile[]>([]),[preview,setPreview]=useState<ProjectFile|null>(null),[busy,setBusy]=useState(false)
 const run=async(fn:()=>Promise<unknown>,done?:()=>void)=>{setBusy(true);try{await fn();message.success('对外委托操作已确认');done?.()}catch(e){message.error(e instanceof Error?e.message:'委托操作未确认')}finally{setBusy(false)}}
 if(workspace.loading)return <WorkspaceSkeleton tip="正在读取当前项目对外委托…"/>
 if(workspace.error||!adapter?.context)return <Alert type="error" showIcon message={workspace.error||'对外委托原生权限未完整读取'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>}/>
 const rows=adapter.list().filter((row:any)=>row.state==='recorded'),summary=adapter.summary()
 const openForm=(row?:any)=>{setEditing(row||null);form.resetFields();const files=row?.contract_files||adapter.stagedFiles();setFileList(files.map((file:any)=>({uid:file.id,name:file.original_name,size:file.file_size,status:'done'})));form.setFieldsValue({provider_id:row?.provider?.id,contact_name:row?.contact_name,contact_phone:row?.contact_phone,scope:row?.scope,amount:row?.amount,contract_key:adapter.contractKey(row?.contract),note:row?.note,reason:''});setOpen(true)}
 const showFile=async(file:any)=>{try{setPreview(await adapter.readFile(file))}catch(e){message.error(e instanceof Error?e.message:'文件元数据未读取')}}
 const submit=async(values:any)=>{const ids:string[]=[];for(const file of fileList){if(file.originFileObj){const uploaded=await adapter.upload(file.originFileObj);ids.push(uploaded.id)}else ids.push(String(file.uid))}const payload={...values,contract_file_ids:ids};return editing?adapter.correct(editing.id,payload):adapter.create(payload)}
 const paymentLabel=(row:any)=>row.suggestion?row.suggestion.state==='accepted'?'义务已确认':row.suggestion.state==='proposed'?'建议待 Money 决定':row.suggestion.state==='target_unavailable'?'Money 暂不可用':row.suggestion.state==='rejected'?'建议已拒绝':'建议已撤回':'尚未提出应付建议'
 return <>
  <section className="project-workspace__card external-commission">
   <header className="external-commission__header"><div><h3 className="project-workspace__card-title">对外委托</h3><p className="external-commission__subtitle">维护受托方、发生时联系人、委托内容、合同文件、评分和应付建议。</p></div><div className="external-commission__header-actions"><ReadStatus workspace={workspace}/><Button size="small" onClick={()=>void workspace.load(true)}>刷新</Button><Button size="small" type="primary" disabled={!adapter.canCreate()} onClick={()=>openForm()}>＋ 添加委托</Button></div></header>
   {adapter.pending&&<Alert type="warning" showIcon style={{margin:'12px 16px'}} message={adapter.conflict?'委托版本冲突，须刷新核对，不能自动覆盖。':'上次操作尚未全部确认；继续会复用原命令，不重复登记。'} action={!adapter.conflict?<Button size="small" loading={adapter.working} onClick={()=>void run(()=>adapter.retry())}>继续原操作</Button>:undefined}/>} {(adapter.error||adapter.partyError||adapter.suggestionError)&&<Alert type="error" showIcon style={{margin:'12px 16px'}} message={adapter.error||adapter.partyError||adapter.suggestionError}/>} 
   <div className="external-commission__summary" aria-label="对外委托统计"><div><span>委托总数</span><strong>{summary.total_count} 条</strong></div><div><span>总金额</span><strong>{summary.total_amount===null?'未授权':`¥${summary.total_amount}`}</strong></div><div><span>平均评分</span><strong>{summary.average_score===null?'—':`${summary.average_score.toFixed(1)} 分`}</strong></div></div>
   {!rows.length?<div className="external-commission__list"><Empty description={adapter.canCreate()?'暂无对外委托记录，请点击右上角添加':'暂无对外委托记录'}/></div>:<div className="external-commission__list">{rows.map((row:any)=><article className="external-commission__item" key={row.id}>
    <div className="external-commission__vendor"><strong>{row.provider.legal_name}</strong><span>{row.contact_name||'—'} · {row.contact_phone||'—'} · {row.score===null?'未评分':`${row.score} 分`}</span></div>
    <div className="external-commission__content"><span>委托内容</span><strong title={row.scope}>{row.scope}</strong></div>
    <div className="external-commission__amount"><span>委托金额</span><strong>{row.amount===null?'未授权':`¥${row.amount}`}</strong></div>
    <div className="external-commission__contract"><span>委托合同</span>{row.contract_files.length?<div className="external-commission__contract-actions">{row.contract_files.map((file:any)=><div key={file.id}><button type="button" onClick={()=>void showFile(file)}>▣ {file.original_name}</button><span className="external-commission__file-actions"><Button type="text" size="small" onClick={()=>void showFile(file)}>预览</Button><Button type="text" size="small" onClick={async()=>{const loaded=await adapter.readFile(file);await fileService.downloadFile(loaded.id,loaded.original_name)}}>下载</Button></span></div>)}</div>:<strong>{row.contract?'已连接正式合同，无附件':'—'}</strong>}</div>
    <div className="external-commission__payments"><span>支付信息</span><div><strong>{paymentLabel(row)}</strong>{row.suggestion&&<em>¥{row.suggestion_amount}</em>}</div></div>
    <div className="external-commission__actions"><Button type="text" size="small" disabled={!row.can_correct} onClick={()=>openForm(row)}>编辑</Button><Button type="text" size="small" disabled={!row.can_assess} onClick={()=>{setAssessment(row);assessmentForm.setFieldsValue({score:row.score||5,reason:''})}}>评分</Button>{row.can_suggest?<Button type="text" size="small" onClick={()=>{setSuggestion(row);suggestionForm.setFieldsValue({purpose:`对外委托：${row.scope}`,due_on:dayjs().add(30,'day')})}}>应付</Button>:row.suggestion&&['proposed','target_unavailable'].includes(row.suggestion.state)?<Button type="text" size="small" onClick={()=>void run(()=>adapter.synchronize(row.id))}>同步</Button>:null}<Button type="text" size="small" disabled={!row.can_history} onClick={()=>void run(async()=>setHistory(await adapter.history(row.id)))}>历史</Button><Button danger type="text" size="small" disabled={!row.can_withdraw} onClick={()=>{const reason=globalThis.prompt?.('请输入撤回原因')||'';if(reason)void run(()=>adapter.withdraw(row.id,reason))}}>撤回</Button></div>
   </article>)}</div>}
  </section>
  <Drawer rootClassName="external-commission-drawer" width={880} title={editing?'编辑对外委托（追加纠正）':'添加对外委托'} open={open} onClose={()=>setOpen(false)} destroyOnHidden extra={<Button type="primary" loading={busy||adapter.working} onClick={()=>form.submit()}>保存</Button>}>
   <Form form={form} layout="vertical" onFinish={values=>void run(()=>submit(values),()=>{setOpen(false);setEditing(null);setFileList([])})}><div style={{display:'grid',gridTemplateColumns:'1fr 1fr',gap:16}}><Form.Item name="provider_id" label="受托方" rules={[{required:true}]}><Select showSearch optionFilterProp="label" options={adapter.parties.map((party:any)=>({value:party.id,label:party.legal_name}))}/></Form.Item><Form.Item name="contract_key" label="正式合同（可选）"><Select allowClear options={adapter.contractSources().map((source:any)=>({value:adapter.contractKey(source),label:`${source.label} · revision ${source.revision}`}))}/></Form.Item><Form.Item name="contact_name" label="联系人"><Input maxLength={100}/></Form.Item><Form.Item name="contact_phone" label="联系电话"><Input maxLength={50}/></Form.Item><Form.Item name="scope" label="委托内容" rules={[{required:true}]}><Input maxLength={2000}/></Form.Item><Form.Item name="amount" label="委托金额（元）" rules={[{required:true},{pattern:/^(0|[1-9]\d*)(\.\d{1,2})?$/,message:'请输入非负金额，最多两位小数'}]}><Input inputMode="decimal" prefix="¥"/></Form.Item></div>
    <Form.Item label="委托合同附件（可选）"><AttachmentUploader helper="文件先由 Evidence 保存；移出只改变本委托引用，不删除字节。" fileList={fileList} onChange={setFileList} beforeUpload={()=>false} accept=".pdf,.png,.jpg,.jpeg,.doc,.docx" disabled={!adapter.canEvidence(editing?adapter.native.get(editing.id):undefined)} uploading={busy} onPreview={file=>{if(file.originFileObj)message.info('本地文件将在上传完成后预览');else{const value=adapter.files.get(String(file.uid));if(value)void showFile(value)}}} onDownload={file=>void fileService.downloadFile(String(file.uid),file.name)} onRemove={async file=>{if(adapter.staged.has(String(file.uid))){adapter.staged.delete(String(file.uid));adapter.persist()}setFileList(current=>current.filter(item=>item.uid!==file.uid));return false}}/></Form.Item>
    <Form.Item name="note" label="备注"><Input.TextArea rows={3} maxLength={2000}/></Form.Item>{editing&&<Form.Item name="reason" label="纠正原因" rules={[{required:true}]}><Input.TextArea rows={3} maxLength={1000}/></Form.Item>}<Alert type="info" showIcon message="合同附件、法律合同和支付是三种事实" description="附件归 Evidence，法律合同归 Contract；本页只提出应付建议，Money 独立确认义务与实付。"/>
   </Form>
  </Drawer>
  <Modal open={!!assessment} title="委托评分" confirmLoading={busy} onCancel={()=>setAssessment(null)} onOk={()=>assessmentForm.submit()}><Form form={assessmentForm} layout="vertical" onFinish={values=>void run(()=>adapter.assess(assessment.id,values.score,values.reason),()=>setAssessment(null))}><Form.Item name="score" label="评分" rules={[{required:true}]}><Rate/></Form.Item><Form.Item name="reason" label="评分说明" rules={[{required:true}]}><Input.TextArea rows={3} maxLength={1000}/></Form.Item></Form></Modal>
  <Modal open={!!suggestion} title="提出应付建议" confirmLoading={busy} onCancel={()=>setSuggestion(null)} onOk={()=>suggestionForm.submit()}><Form form={suggestionForm} layout="vertical" onFinish={values=>void run(()=>adapter.propose(suggestion.id,{...values,due_on:values.due_on?.format('YYYY-MM-DD')}),()=>setSuggestion(null))}><Form.Item name="ledger_owner_id" label="本方账套主体" rules={[{required:true}]}><Select showSearch optionFilterProp="label" options={adapter.parties.filter((party:any)=>party.id!==suggestion?.provider?.id).map((party:any)=>({value:party.id,label:party.legal_name}))}/></Form.Item><Form.Item name="due_on" label="建议到期日" rules={[{required:true}]}><DatePicker style={{width:'100%'}}/></Form.Item><Form.Item name="purpose" label="用途" rules={[{required:true}]}><Input.TextArea rows={3} maxLength={1000}/></Form.Item><Alert type="warning" showIcon message="这里只提出建议" description="Money 接受后才形成应付义务；实际支付和核销仍须在 Money 中完成。"/></Form></Modal>
  <Modal open={history!==null} title="对外委托历史" footer={null} onCancel={()=>setHistory(null)}><List dataSource={history||[]} renderItem={(item:any)=><List.Item><List.Item.Meta title={`${item.event_kind} · revision ${item.revision}`} description={`${item.snapshot?.contact_name||'—'} · ${item.snapshot?.state==='withdrawn'?'已撤回':'有效'} · ${item.reason||'—'} · ${dayjs(item.occurred_at).format('YYYY-MM-DD HH:mm')}`}/></List.Item>}/></Modal>
  <FilePreviewDrawer open={!!preview} file={preview} onClose={()=>setPreview(null)}/>
 </>
}
class ReactCommissionWorkspace{
 root:Root;adapters=new Map<string,any>();current:any=null;sequence=0;loading=false;refreshing=false;error='';refreshError='';readCache=new PageReadCache();options:any
 constructor(options:any){this.options=options;const mount=options.root.querySelector('[data-real-commission-react]');if(!mount)throw new Error('对外委托组件挂载点未配置');this.root=createRoot(mount);this.render()}
 render(){this.root.render(<ConfigProvider locale={zhCN}><CommissionPanel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current}/></ConfigProvider>)}
 invalidate(){this.sequence++;this.current=null;this.loading=false;this.refreshing=false;this.error='';this.refreshError='';this.readCache.clear();for(const adapter of this.adapters.values())adapter.transfer?.clearCache();this.render()}
 async load(explicit=false){const sequence=++this.sequence,id=this.options.details()?.project?.id;try{if(!id)throw new Error('当前项目尚未读取');let adapter=this.adapters.get(id);if(!adapter){let storage;try{storage=globalThis.sessionStorage}catch{}const api=createFetchFileTransferApi({baseUrl:'',fetcher:this.options.evidenceFetch});const transfer=new FileTransferClient({api,uploader:createDirectUploaderRouter({aliyunOss:createAliOssMultipartUploader(),localHttp:createLocalHttpUploader()}),getCacheScope:()=>undefined});adapter=new CommissionAdapter({...this.options,storage,transfer,loaded:()=>this.readCache.markFresh(id),changed:()=>{if(this.current===adapter)this.render()}});this.adapters.set(id,adapter);registerCommission(id,adapter)}this.current=adapter;await readWorkspace(this,{sequence,key:id,explicit,hasValue:()=>!!adapter.context,read:()=>adapter.load(),isCurrent:()=>this.options.details()?.project?.id===id,fallback:'对外委托读取未完成'})}catch(e){if(sequence===this.sequence){this.loading=false;this.refreshing=false;this.error=e instanceof Error?e.message:'对外委托读取未完成';this.render()}}}
}
export const createCommissionWorkspace=(options:any)=>new ReactCommissionWorkspace(options)

function BonusPanel({workspace,adapter}:{workspace:ReactBonusWorkspace;adapter:any}) {
 const [error,setError]=useState('')
 const run=async(fn:()=>Promise<unknown>)=>{setError('');try{await fn()}catch(e){setError(e instanceof Error?e.message:'奖金操作未确认')}}
 if(workspace.loading)return <WorkspaceSkeleton tip="正在读取当前项目奖金分配…" />
 if(workspace.error||!adapter?.context)return <Alert type="error" showIcon message={workspace.error||'奖金原生权限未完整读取'} action={<Button onClick={()=>void workspace.load(true)}>重新读取</Button>}/>
 return <><Space wrap style={{marginBottom:16}}><Button disabled={adapter.working} onClick={()=>void run(()=>workspace.load(true))}>刷新当前奖金</Button>{adapter.pending&&!adapter.conflict&&<Button type="primary" disabled={adapter.working} onClick={()=>void run(()=>adapter.retry())}>继续原奖金操作</Button>}<ReadStatus workspace={workspace} /></Space>
 {adapter.pending&&<Alert type="warning" style={{marginBottom:16}} message={adapter.conflict?'奖金版本冲突，须核对，不自动覆盖。':'奖金操作尚未全部确认；继续复用原命令，不重复登记。'}/>} {(error||adapter.error)&&<Alert type="error" style={{marginBottom:16}} message={error||adapter.error}/>}
 <Alert type="info" showIcon style={{marginBottom:16}} message="手工分配登记不代表付款" description="本次比例池基数由你填写；不等于已确认结算或收款奖池。固定额不占比例池，原分配与撤回均保留历史。来源奖池计算另行接通。"/>
 <BonusAllocation projectId={adapter.projectID} bonusType={adapter.scope} showStatistics/></>
}
class ReactBonusWorkspace {
 root:Root;query:QueryClient;adapters=new Map<string,any>();current:any=null;sequence=0;loading=false;refreshing=false;error='';refreshError='';readCache=new PageReadCache();options:any
 constructor(options:any){this.options=options;const mount=options.root.querySelector('[data-real-bonus-react]');if(!mount)throw new Error('奖金组件挂载点未配置');this.root=createRoot(mount);this.query=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false},mutations:{retry:false}}});this.render()}
 render(){if(this.current?.context){const a=this.current;this.query.setQueryData(['bonusRecords',a.projectID,a.scope],a.list());this.query.setQueryData(['canManageBonus',a.projectID,a.scope],a.canManage());this.query.setQueryData([a.scope==='production'?'productionBonusStatistics':'businessBonusStatistics',a.projectID],a.statistics());}this.root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={this.query}><BonusPanel key={this.current?.projectID||'loading'} workspace={this} adapter={this.current}/></QueryClientProvider></ConfigProvider>)}
 invalidate(){this.sequence++;this.current=null;this.loading=false;this.refreshing=false;this.error='';this.refreshError='';this.readCache.clear();this.query.clear();this.render()}
async load(explicit=false){const sequence=++this.sequence,id=this.options.details()?.project?.id;try{if(!id)throw new Error('当前项目尚未读取');let a=this.adapters.get(id);if(!a){let storage;try{storage=globalThis.sessionStorage}catch{}a=new BonusAdapter({...this.options,storage,changed:()=>{if(this.current===a)this.render()}});this.adapters.set(id,a);registerBonus(id,this.options.scope,a)}this.current=a;await readWorkspace(this,{sequence,key:`${id}:${this.options.scope}`,explicit,hasValue:()=>!!a.context,read:async()=>{await this.options.beforeLoad?.();if(this.options.details()?.project?.id!==id)throw new Error('当前项目已切换');return a.load()},isCurrent:()=>this.options.details()?.project?.id===id,fallback:'奖金读取未完成'})}catch(e){if(sequence===this.sequence){this.loading=false;this.refreshing=false;this.error=e instanceof Error?e.message:'奖金读取未完成';this.render()}}}
}
export const createBonusWorkspace=(options:any)=>new ReactBonusWorkspace(options)
