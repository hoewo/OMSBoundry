import {useState,type ComponentType,type ReactNode} from 'react'
import {Alert,Button,Empty,Spin,Upload,message} from 'antd'
import type {UploadProps} from 'antd'
import {DownloadOutlined,EyeOutlined,UploadOutlined} from '@ant-design/icons'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {useProductionStagePermissions,useProductionStagePort,useProductionStageService} from '@/production-stage-ports'
import {fileService} from '@/services/file'
import {SectionTitle} from '@/components/shared/SectionTitle'
import {DeleteConfirmButton} from '@/components/shared/ConfirmDeleteDialog'
import {CreateActionButton} from '@/components/shared/CreateActionButton'
import {FilePreviewDrawer} from '@/components/shared/FilePreviewDrawer'
import {isCadFileName,isWithinUploadLimit,MAX_UPLOAD_SIZE_LABEL} from '@/utils/upload'
import type {File as ProjectFile,ProductionFile,ProductionStage,StageFileInfo} from '@/types'
import dayjs from 'dayjs'
import '../project/ProjectWorkspace.css'
import './AuxiliaryFilePanel.css'

type UploadComponent=ComponentType<{projectId:string|number;visible:boolean;onCancel:()=>void;onSuccess?:()=>void}>
interface Props{projectId:string|number;stage:ProductionStage;title:string;description:string;icon:ReactNode;tone:'change'|'completion';UploadComponent:UploadComponent}
const formatFileSize=(bytes?:number)=>{if(!bytes)return '-';const k=1024,s=['B','KB','MB','GB'],i=Math.floor(Math.log(bytes)/Math.log(k));return `${Math.round(bytes/Math.pow(k,i)*100)/100} ${s[i]}`}
const accept:Record<'change'|'completion',string>={change:'.pdf,.doc,.docx,.xls,.xlsx,.png,.jpg,.jpeg,.dwg,.dxf',completion:'.pdf,.doc,.docx,.xls,.xlsx,.zip,.rar,.png,.jpg,.jpeg,.dwg,.dxf'}

export const AuxiliaryFilePanel=({projectId,stage,title,description,icon,tone,UploadComponent}:Props)=>{
 const port=useProductionStagePort(projectId),service=useProductionStageService(projectId),permissions=useProductionStagePermissions(projectId),client=useQueryClient()
 const [uploadOpen,setUploadOpen]=useState(false),[previewFile,setPreviewFile]=useState<ProjectFile|null>(null),[replacing,setReplacing]=useState<string|null>(null)
 const canUpload=permissions.canUploadProductionFile,canDownload=permissions.canDownloadProductionFile,canDelete=permissions.canDeleteProductionFile
 const query=useQuery<StageFileInfo>({queryKey:['productionFilesByStage',projectId,stage],queryFn:()=>service.getProductionFilesByStage(projectId,stage),enabled:!!projectId})
 const refresh=async()=>{await client.invalidateQueries({queryKey:['productionFilesByStage',projectId]});await query.refetch()}
 const remove=useMutation({mutationFn:(id:string)=>service.deleteProductionFile(projectId,id),onSuccess:async()=>{message.success('已移出阶段资料，历史与文件字节保留');await refresh()},onError:(e:any)=>message.error(e?.message||'移出失败')})
 const download=async(id:string,name?:string)=>{try{await fileService.downloadFile(id,name)}catch(e:any){message.error(e?.message||'下载失败')}}
 const replace=(item:ProductionFile):UploadProps['beforeUpload']=>async file=>{if(!isWithinUploadLimit(file)){message.error(`文件大小不能超过 ${MAX_UPLOAD_SIZE_LABEL}`);return Upload.LIST_IGNORE}setReplacing(item.id);try{await port.replaceReference(item.id,file);if(previewFile?.id===item.file_id)setPreviewFile(null);message.success('文件替换成功，旧引用历史已保留');await refresh()}catch(e:any){message.error(e?.message||'文件替换失败')}finally{setReplacing(null)}return Upload.LIST_IGNORE}
 const files=query.data?.main_files||[]
 if(query.isLoading)return <section className={`project-workspace__card auxiliary-file-panel auxiliary-file-panel--${tone}`}><div className="auxiliary-file-panel__loading"><Spin/><span>正在加载{title}文件...</span></div></section>
 if(query.error)return <section className={`project-workspace__card auxiliary-file-panel auxiliary-file-panel--${tone}`}><Alert message="加载失败" description={query.error instanceof Error?query.error.message:'加载文件失败'} type="error" showIcon action={<Button onClick={()=>query.refetch()}>重试</Button>}/></section>
 return <>
  <section className={`project-workspace__card auxiliary-file-panel auxiliary-file-panel--${tone}`}>
   <header className="auxiliary-file-panel__header"><div><SectionTitle icon={icon}>{title}</SectionTitle><p className="auxiliary-file-panel__subtitle">{description}</p></div>{canUpload&&<CreateActionButton size="small" compact onClick={()=>setUploadOpen(true)}>上传文件</CreateActionButton>}</header>
   <div className="auxiliary-file-panel__summary"><span>当前文件</span><strong>{files.length} 份</strong></div>
   {files.length===0?<div className="auxiliary-file-panel__empty"><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={canUpload?`暂无${title}文件，请上传`:`暂无${title}文件`}/></div>:<div className="auxiliary-file-panel__list">{files.map(item=>{const file=item.file;if(!file)return null;const busy=replacing===item.id||remove.isPending,cad=isCadFileName(file.original_name);return <article className="auxiliary-file-panel__item" key={item.id}>
    <div className="auxiliary-file-panel__file-main"><span className="auxiliary-file-panel__file-icon">{file.original_name.split('.').pop()?.slice(0,4).toUpperCase()||'FILE'}</span><div>{cad?<strong className="auxiliary-file-panel__file-name">{file.original_name}</strong>:<button type="button" className="auxiliary-file-panel__file-name" onClick={()=>setPreviewFile(file)} disabled={busy}>{file.original_name}</button>}<div className="auxiliary-file-panel__file-meta">{formatFileSize(file.file_size)} · {dayjs(item.created_at).format('YYYY-MM-DD HH:mm')}</div></div></div>
    <div className="auxiliary-file-panel__actions">{!cad&&<Button type="text" size="small" icon={<EyeOutlined/>} disabled={busy} onClick={()=>setPreviewFile(file)}>预览</Button>}{canDownload&&<Button type="text" size="small" icon={<DownloadOutlined/>} disabled={busy} onClick={()=>download(file.id,file.original_name)}>下载</Button>}{canUpload&&<Upload accept={accept[tone]} beforeUpload={replace(item)} disabled={busy} maxCount={1} showUploadList={false}><Button type="text" size="small" icon={<UploadOutlined/>} loading={replacing===item.id} disabled={busy}>替换</Button></Upload>}{canDelete&&<DeleteConfirmButton title="确认移出文件" description="移出当前阶段并保留引用历史和 Evidence 文件字节。" details={[{label:'文件',value:file.original_name},{label:'上传时间',value:dayjs(item.created_at).format('YYYY-MM-DD')}]} buttonProps={{size:'small',disabled:busy}} loading={remove.isPending} onConfirm={()=>{if(previewFile?.id===file.id)setPreviewFile(null);remove.mutate(item.id)}}/>}</div>
   </article>})}</div>}
  </section>
  {canUpload&&<UploadComponent projectId={projectId} visible={uploadOpen} onCancel={()=>setUploadOpen(false)} onSuccess={async()=>{setUploadOpen(false);await refresh()}}/>}
  <FilePreviewDrawer open={!!previewFile} file={previewFile} onClose={()=>setPreviewFile(null)}/>
 </>
}
