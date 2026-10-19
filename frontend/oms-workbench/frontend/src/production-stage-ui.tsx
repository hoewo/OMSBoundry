// Original component host; the optional native workspace supplies recovery.
import {useState} from 'react'
import {createRoot} from 'react-dom/client'
import {Alert,Button,ConfigProvider,Space} from 'antd'
import zhCN from 'antd/locale/zh_CN'
import {QueryClient,QueryClientProvider,useQueryClient} from '@tanstack/react-query'
import {SchemeStageFileManagement} from '../vendor/components/production/SchemeStageFileManagement'
import {PreliminaryStageFileManagement} from '../vendor/components/production/PreliminaryStageFileManagement'
import {ConstructionStageFileManagement} from '../vendor/components/production/ConstructionStageFileManagement'
import {ChangeFileList} from '../vendor/components/production/ChangeFileList'
import {CompletionFileList} from '../vendor/components/production/CompletionFileList'
import {SurveyTestStagePanel} from '../vendor/components/production/SurveyTestStagePanel'
import {ConsultationReportPanel} from '../vendor/components/production/ConsultationReportPanel'
import {ProductionStageContext,type ProductionStageUIPort} from './production-stage-ports'
function Recovery({port}:{port:any}){
 const client=useQueryClient()
 const [error,setError]=useState(''),pending=port.pendingOperation?.(),working=port.isWorking?.()===true
 if(!pending)return null
 return <Alert type="warning" showIcon style={{marginBottom:16}} message="上次阶段保存尚未全部确认" description={<Space direction="vertical">
  <span>继续同一命令；已确认上传的文件不重传，不自动覆盖冲突。</span>
  {pending.files?.some((f:any)=>!f.reference&&!f.receipt)&&<label>重新选择原文件 <input type="file" multiple disabled={working} onChange={async event=>{try{await port.reselectPending(Array.from(event.target.files||[]));setError('')}catch(e){setError(e instanceof Error?e.message:'原文件未确认')}}}/></label>}
  <Button disabled={working} onClick={async()=>{setError('');try{await port.retryPending();await client.invalidateQueries({queryKey:['productionFilesByStage',port.projectID,port.stage],exact:true})}catch(e){setError(e instanceof Error?e.message:'续办尚未确认')}}}>继续原阶段操作</Button>
  {error&&<span role="alert">{error}</span>}
 </Space>}/>
}
export function createWorkspace(options:{root:Element;port:ProductionStageUIPort;recovery?:boolean}){
 if(!['scheme','preliminary','construction','test','consultation'].includes(options.port.stage)||!['design','consultation','survey','mapping','shared'].includes(options.port.projectTypeKey))throw Error('阶段试片必须显式提供原项目／阶段')
 const root=createRoot(options.root),client=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false}}})
 const Surface=options.port.stage==='consultation'?ConsultationReportPanel:options.port.stage==='test'?SurveyTestStagePanel:options.port.stage==='construction'?ConstructionStageFileManagement:options.port.stage==='preliminary'?PreliminaryStageFileManagement:SchemeStageFileManagement
 const render=()=>root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={client}><ProductionStageContext.Provider value={options.port}>{options.recovery&&<Recovery port={options.port}/>}<Surface projectId={options.port.projectID}/></ProductionStageContext.Provider></QueryClientProvider></ConfigProvider>)
 render()
 return {notify:render,refresh(){void client.invalidateQueries();render()},unmount(){root.unmount();client.clear()}}
}

export function createAuxiliaryWorkspace(options:{root:Element;change:ProductionStageUIPort;completion:ProductionStageUIPort;recovery?:boolean}){
 if(options.change.stage!=='change'||options.completion.stage!=='completion'||options.change.projectID!==options.completion.projectID)throw Error('变更／竣工原生端口必须属于同一 Project')
 const root=createRoot(options.root),client=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false}}})
 const render=()=>root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={client}><div className="project-workspace__stack">
  <ProductionStageContext.Provider value={options.change}>{options.recovery&&<Recovery port={options.change}/>}<ChangeFileList projectId={options.change.projectID}/></ProductionStageContext.Provider>
  <ProductionStageContext.Provider value={options.completion}>{options.recovery&&<Recovery port={options.completion}/>}<CompletionFileList projectId={options.completion.projectID}/></ProductionStageContext.Provider>
 </div></QueryClientProvider></ConfigProvider>)
 render()
 return {notify:render,refresh(){void client.invalidateQueries();render()},unmount(){root.unmount();client.clear()}}
}
