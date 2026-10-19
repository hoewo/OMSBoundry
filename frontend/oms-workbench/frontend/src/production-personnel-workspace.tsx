// Selected D01 work surface: reuse the original UI; load only the visible Project.
import {createWorkspace as mount} from './production-personnel-native-ui'
import {Alert,Button} from 'antd'
import {createRoot,type Root} from 'react-dom/client'
const profile='reference-oa-production-personnel-v1'
class ProductionPersonnelWorkspace {
 options:any; current:any=null; projectID:string|null=null; loadedAt=0; errorRoot:Root|null=null
 constructor(options:any){if(options.profile!==profile)throw Error('生产人员 profile 未显式选择');this.options=options}
 invalidate(){this.current?.unmount();this.errorRoot?.unmount();this.current=null;this.errorRoot=null;this.projectID=null;this.loadedAt=0}
 async load(explicit=false){
  try{
  const p=this.options.details()?.project
  if(!p?.id||p.tenant_id!==this.options.tenantID)throw Error('生产人员当前 Project／Tenant 未确认')
  if(this.projectID===p.id&&this.current){if(!explicit&&Date.now()-this.loadedAt<30000)return;this.current.refresh();this.loadedAt=Date.now();return}
  this.invalidate()
  const host=this.options.root.querySelector('[data-real-production-personnel-react]')
  if(!host)throw Error('原生产人员组件挂载点未配置')
  let storage:Storage
  try{storage=globalThis.sessionStorage}catch{throw Error('生产人员命令续办存储不可用，未启用写入')}
  // Scope is enforced by the authenticated transport and each native owner.
  // UI profile selection never grants capabilities or enrolls a Project.
  const request=this.options.request
  this.current=mount({root:host,native:{projectID:p.id,request,
   candidates:async()=>{const rows=await this.options.applicationRequest('people-groups','/api/v1/persons');if(!Array.isArray(rows)||rows.some((v:any)=>v.tenant_id!==this.options.tenantID))throw Error('人员目录 Tenant 不一致');return rows.filter((v:any)=>v.status==='active')},
   pendingStore:{load:()=>{const v=storage.getItem(this.key(p.id));return v?JSON.parse(v):null},save:v=>storage.setItem(this.key(p.id),JSON.stringify(v)),clear:()=>storage.removeItem(this.key(p.id))},
   enroll:(revision:number)=>this.enroll(revision),
  }})
  this.projectID=p.id;this.loadedAt=Date.now()
  }catch(error){
   this.invalidate()
   const host=this.options.root.querySelector('[data-real-production-personnel-react]')||this.options.root
   this.errorRoot=createRoot(host)
   this.errorRoot.render(<Alert type="error" showIcon message={error instanceof Error?error.message:'生产人员读取未完成'} description="读取失败不表示空名单；未启用业务写入。" action={<Button onClick={()=>void this.load(true)}>重新读取</Button>}/> )
  }
 }
 async enroll(expectedRevision:number){
  const details=this.options.details(),p=details?.project
  if(!p?.id||p.tenant_id!==this.options.tenantID||!Number.isSafeInteger(expectedRevision)||expectedRevision<1)throw Error('生产人员初始化上下文无效')
  let storage:Storage
  try{storage=globalThis.sessionStorage}catch{throw Error('初始化命令续办存储不可用')}
  const key=`oms.oa.personnel.enroll.v1:${this.options.tenantID}:${this.options.subjectID}:${p.id}`
  let command=storage.getItem(key)
  if(!command){command=crypto.randomUUID();storage.setItem(key,command)}
  try{
   await this.options.request(`/api/v1/projects/${p.id}/reference-oa/production-profile`,'POST',{command_id:command,expected_revision:expectedRevision})
   storage.removeItem(key)
  }catch(error){
   const status=(error as any)?.status
   if(Number.isInteger(status)&&status>=400&&status<500)storage.removeItem(key)
   throw error
  }
  await this.load(true)
 }
 key(id:string){return `oms.oa.personnel.pending.v1:${this.options.tenantID}:${this.options.subjectID}:${id}`}
}
export const createProductionPersonnelWorkspace=(options:any)=>new ProductionPersonnelWorkspace(options)
