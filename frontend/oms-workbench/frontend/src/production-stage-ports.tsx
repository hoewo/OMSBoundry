// Explicit presentation port for the copied original stage UI. No HTTP route,
// credentials, customer-role authorization or physical storage ownership here.
import {createContext,useContext} from 'react'
import {useQuery} from '@tanstack/react-query'
import type {ProductionStage,StageFileInfo} from './types'
export interface ProductionStageUIPort {
 projectID:string
 projectTypeKey:'design'|'consultation'|'survey'|'mapping'|'shared'
 stage:'scheme'|'preliminary'|'construction'|'change'|'completion'|'test'|'consultation'
 maxUploadBytes?:number
 readStage():Promise<StageFileInfo>
 permissions():Promise<{read:boolean;upload:boolean;download:boolean;withdraw:boolean}>
 saveStage(input:{mainFiles:globalThis.File[];reviewSheets:globalThis.File[];score?:string}):Promise<void>
 replaceReference(id:string,file:globalThis.File):Promise<void>
 withdrawReference(id:string):Promise<void>
}
export const ProductionStageContext=createContext<ProductionStageUIPort|null>(null)
export function useProductionStagePort(projectID:string|number){
 const port=useContext(ProductionStageContext)
 if(!port||String(projectID)!==port.projectID)throw Error('原阶段 UI 的确切 Project／类型／阶段端口未配置')
 return port
}
export function useProductionStagePermissions(projectID:string|number){
 const port=useProductionStagePort(projectID)
 const query=useQuery({queryKey:['productionStagePermissions',port.projectID,port.projectTypeKey,port.stage],queryFn:()=>port.permissions()})
 return {query,canUploadProductionFile:query.data?.read===true&&query.data?.upload===true,canDownloadProductionFile:query.data?.download===true,canDeleteProductionFile:query.data?.read===true&&query.data?.withdraw===true}
}
export function useProductionStageService(projectID:string|number){
 const port=useProductionStagePort(projectID)
 return {
  async getProductionFilesByStage(id:string|number,stage:ProductionStage){if(String(id)!==port.projectID||stage!==port.stage)throw Error('阶段读取范围不匹配');const v=await port.readStage();if(v.stage!==port.stage||!Array.isArray(v.main_files)||!Array.isArray(v.review_sheets))throw Error('阶段回读未确认，不表示空资料');return v},
  async deleteProductionFile(id:string|number,referenceID:string){if(String(id)!==port.projectID)throw Error('阶段移出范围不匹配');return port.withdrawReference(referenceID)},
 }
}
