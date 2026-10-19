// Original D01 visual candidate, deliberately not imported by workspace.tsx.
// This closes UI/interaction checks only; native facts/IAM remain a separate gate.
import React,{useState} from 'react'
import {createRoot,type Root} from 'react-dom/client'
import {ConfigProvider,Alert,Button,Spin} from 'antd'
import zhCN from 'antd/locale/zh_CN'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import {ProductionPersonnelManager} from '../vendor/components/production/personnel/ProductionPersonnelManager'
import {ProductionPersonnelContext,useProductionPersonnelPermissions,type ProductionPersonnelUIPort} from './production-personnel-ports'
function Panel({projectID}:{projectID:string}) {
 const permission=useProductionPersonnelPermissions(projectID)
 const [initializing,setInitializing]=useState(false),[initializeError,setInitializeError]=useState('')
 if(permission.isPending)return <Spin tip="正在读取生产人员界面权限…"><div style={{minHeight:80}}/></Spin>
 if(permission.isError)return <Alert type="error" showIcon message="生产人员权限读取失败，不假定可管理" action={<Button onClick={()=>void permission.refetch()}>重新读取</Button>}/>
 if(permission.notEnrolled){
  const initialize=async()=>{setInitializing(true);setInitializeError('');try{await permission.initialize();await permission.refetch()}catch(e){setInitializeError(e instanceof Error?e.message:'生产人员初始化未确认')}finally{setInitializing(false)}}
  return <Alert type="warning" showIcon message="生产人员尚未初始化，不是无权限" description={<div><p>当前项目还没有建立生产人员事实集；初始化后才能配置专业、设计、复核、审核等人员。</p>{initializeError&&<p style={{color:'#cf1322'}}>{initializeError}</p>}</div>} action={<Button type="primary" loading={initializing} disabled={!permission.canAssignProductionMembers} onClick={()=>void initialize()}>初始化生产人员</Button>}/>
 }
 return <ProductionPersonnelManager projectId={projectID} participantNotApplicable={permission.participantNotApplicable}/>
}
class ProductionPersonnelUI {
 root:Root;query:QueryClient
 constructor(options:{root:Element;port:ProductionPersonnelUIPort}){
  this.root=createRoot(options.root)
  this.query=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false},mutations:{retry:false}}})
  this.root.render(<ConfigProvider locale={zhCN}><QueryClientProvider client={this.query}><ProductionPersonnelContext.Provider value={options.port}><Panel projectID={options.port.projectID}/></ProductionPersonnelContext.Provider></QueryClientProvider></ConfigProvider>)
 }
 refresh(){return this.query.invalidateQueries()}
 unmount(){this.root.unmount();this.query.clear()}
}
export const createWorkspace=(options:{root:Element;port:ProductionPersonnelUIPort})=>new ProductionPersonnelUI(options)
