import {createWorkspace as mount,createAuxiliaryWorkspace as mountAuxiliary} from './production-stage-ui'
import {createNativeStagePort,stagePendingStore,durableStageUpload,stageProfile} from './production-stage-native.mjs'
import {registerStage,unregisterStage} from './services/registry'
import {Alert,Button} from 'antd'
import {createRoot,type Root} from 'react-dom/client'
import {FileTransferClient,createFetchFileTransferApi,createAliOssMultipartUploader,createDirectUploaderRouter,createLocalHttpUploader} from '../../../../packages/file-transfer-core/src/index'

class ProductionStageWorkspace {
 options:any;current:any=null;port:any=null;ports:any[]=[];projectID:string|null=null;stage:string|null=null;loadedAt=0;epoch=0;errorRoot:Root|null=null
 constructor(options:any){if(options.profile!==stageProfile)throw Error('生产阶段资料 profile 必须显式选择');this.options=options}
 invalidate(){this.epoch++;this.current?.unmount();this.errorRoot?.unmount();this.errorRoot=null;for(const port of this.ports)port?.transfer.clearCache();if(this.projectID)unregisterStage(this.projectID);this.current=null;this.port=null;this.ports=[];this.projectID=null;this.stage=null;this.loadedAt=0}
 async load(stage='scheme',explicit=false){
  try{
   if(!['scheme','preliminary','construction','change-completion','test','consultation'].includes(stage))throw Error('生产阶段工作面尚未登记')
   const p=this.options.details()?.project
   if(!p?.id||p.tenant_id!==this.options.tenantID)throw Error('生产阶段当前 Project／Tenant 未确认')
   if(this.projectID===p.id&&this.stage===stage&&this.current){if(!explicit&&Date.now()-this.loadedAt<30000)return;this.current.refresh();this.loadedAt=Date.now();return}
   this.invalidate();const epoch=this.epoch,id=p.id
   const host=this.options.root.querySelector(`[data-real-production-stage-react="${stage}"]`);if(!host)throw Error('原阶段组件挂载点未配置')
   const assertCurrent=()=>{if(this.epoch!==epoch||this.options.details()?.project?.id!==id)throw Error('当前项目已切换；原阶段操作保留，未继续后续写入')}
   let storage:Storage;try{storage=globalThis.sessionStorage}catch{throw Error('阶段续办存储不可用，未启用写入')}
   const api=createFetchFileTransferApi({baseUrl:'',fetcher:this.options.evidenceFetch})
   const uploader=createDirectUploaderRouter({aliyunOss:createAliOssMultipartUploader(),localHttp:createLocalHttpUploader()})
   const transfer=new FileTransferClient({api,uploader,getCacheScope:()=>undefined})
   const make=(nativeStage:string)=>createNativeStagePort({projectID:id,tenantID:this.options.tenantID,subjectID:this.options.subjectID,stage:nativeStage,
    assertCurrent,projectRevision:()=>{assertCurrent();return this.options.details().project.revision},
    request:async(path:string,method='GET',body:any)=>{assertCurrent();const result=await this.options.request(path,method,body,id);assertCurrent();return result},
    pendingStore:stagePendingStore(storage,`oms.oa.stage.pending.v1:${this.options.tenantID}:${this.options.subjectID}:${id}:${nativeStage}`),
    readFile:(fileID:string,version:number)=>this.options.evidenceReferenceRequest(fileID,version),upload:durableStageUpload({api,uploader}),transfer,
    changed:()=>{if(this.epoch===epoch)this.current?.notify()}
   })
   this.projectID=id;this.stage=stage
   if(stage==='change-completion'){
    const change:any=make('change'),completion:any=make('completion');change.pendingOperation();completion.pendingOperation();this.ports=[change,completion]
    const composite={files:{has:(viewID:string)=>change.files.has(viewID)||completion.files.has(viewID)},transfer,descriptor:(viewID:string)=>change.files.has(viewID)?change.descriptor(viewID):completion.descriptor(viewID)}
    registerStage(id,composite);this.current=mountAuxiliary({root:host,change,completion,recovery:true})
   }else{
    this.port=make(stage);this.port.pendingOperation();this.ports=[this.port];registerStage(id,this.port);this.current=mount({root:host,port:this.port,recovery:true})
   }
   this.loadedAt=Date.now()
  }catch(error){
   this.invalidate();const host=this.options.root.querySelector(`[data-real-production-stage-react="${stage}"]`)||this.options.root
   this.errorRoot=createRoot(host);this.errorRoot.render(<Alert type="error" showIcon message={error instanceof Error?error.message:'生产阶段未装入'} description="未启用业务写入；读取故障不表示空资料。" action={<Button onClick={()=>void this.load(stage,true)}>重新读取</Button>}/>)
  }
 }
}
export const createProductionStageWorkspace=(options:any)=>new ProductionStageWorkspace(options)
