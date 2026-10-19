import type { File } from '../types'
export interface BiddingInfo {
  not_applicable: boolean; engagement_method: 'comparison' | 'competitive_negotiation' | 'tender' | null
  tender_file_required: boolean; tender_file_complete: boolean; bid_file_complete: boolean; award_file_complete: boolean
  tender_files: File[]; bid_files: File[]; award_notice_files: File[]
}
// The adapter validates native wire data before exposing this view port.
const adapters = new Map<string, any>()
export const registerAdapter = (id: string, adapter: any) => adapters.set(id, adapter)
export const adapterFor = (id: string): any => {
  const adapter = adapters.get(id)
  if (!adapter) throw new Error('当前项目组件尚未装入')
  return adapter
}
export const fileAdapter = (viewID: string): any => {
  for (const adapter of adapters.values()) if (adapter.files.has(viewID)) return adapter
  for (const adapter of contracts.values()) if (adapter.files.has(viewID)) return adapter
  for (const adapter of invoices.values()) if (adapter.files.has(viewID)) return adapter
  for (const adapter of costs.values()) if (adapter.files.has(viewID)) return adapter
  for (const adapter of stages.values()) if (adapter.files.has(viewID)) return adapter
  for (const adapter of approvals.values()) if (adapter.files.has(viewID)) return adapter
  for (const adapter of commissions.values()) if (adapter.files.has(viewID)) return adapter
  throw new Error('文件版本尚未读取')
}
const contributors = new Map<string, any>()
export const registerContributor = (id: string, adapter: any) => contributors.set(id,adapter)
export const contributorFor = (id: string): any => {const value=contributors.get(id);if(!value)throw new Error('当前项目人员组件尚未装入');return value}
export const contributorForMember = (id: string): any => {for(const value of contributors.values())if(value.value?.assignments.some((a:any)=>a.id===id))return value;throw new Error('当前成员尚未读取')}
const contracts=new Map<string,any>()
export const registerContract=(id:string,adapter:any)=>contracts.set(id,adapter)
export const contractForProject=(id:string):any=>{const v=contracts.get(id);if(!v)throw new Error('当前项目合同组件尚未装入');return v}
export const contractForID=(id:string):any=>{for(const v of contracts.values())if(v.native.has(id))return v;throw new Error('当前合同尚未完整读取')}
const cash=new Map<string,any>()
export const registerCash=(id:string,adapter:any)=>cash.set(id,adapter)
export const cashForProject=(id:string):any=>{const v=cash.get(id);if(!v)throw new Error('当前项目支付组件尚未装入');return v}
const invoices=new Map<string,any>()
export const registerInvoice=(id:string,adapter:any)=>invoices.set(id,adapter)
export const invoiceForProject=(id:string):any=>{const v=invoices.get(id);if(!v)throw new Error('当前项目开票组件尚未装入');return v}
const costs=new Map<string,any>()
export const registerCost=(id:string,scope:string,adapter:any)=>costs.set(id+':'+scope,adapter)
export const costForProject=(id:string,scope:string):any=>{const v=costs.get(id+':'+scope);if(!v)throw new Error('当前项目成本组件尚未装入');return v}
const bonuses=new Map<string,any>()
const stages=new Map<string,any>()
const approvals=new Map<string,any>()
const commissions=new Map<string,any>()
export const registerApprovalAudit=(id:string,adapter:any)=>approvals.set(id,adapter)
export const registerCommission=(id:string,adapter:any)=>commissions.set(id,adapter)
export const registerStage=(id:string,adapter:any)=>stages.set(id,adapter)
export const unregisterStage=(id:string)=>stages.delete(id)
export const registerBonus=(id:string,scope:string,adapter:any)=>bonuses.set(id+':'+scope,adapter)
export const bonusForProject=(id:string,scope:string):any=>{const v=bonuses.get(id+':'+scope);if(!v)throw new Error('当前项目奖金组件尚未装入');return v}
