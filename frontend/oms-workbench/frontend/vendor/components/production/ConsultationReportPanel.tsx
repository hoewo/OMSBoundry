// Copied from the user-owned reference OA ConsultationReportPanel and adapted
// only to the native Production stage port and Evidence file delivery port.
import {useMemo,useState} from 'react'
import {Button,Spin,message} from 'antd'
import {EyeOutlined,UploadOutlined} from '@ant-design/icons'
import type {UploadFile} from 'antd'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {useProductionStagePermissions,useProductionStagePort,useProductionStageService} from '@/production-stage-ports'
import {fileService} from '@/services/file'
import type {File as ProjectFile} from '@/types'
import {SectionTitle} from '@/components/shared/SectionTitle'
import {FilePreviewDrawer} from '@/components/shared/FilePreviewDrawer'
import {AttachmentUploader} from '@/components/shared/AttachmentUploader'
import {isWithinUploadLimit,MAX_UPLOAD_SIZE_LABEL} from '@/utils/upload'
import './ConsultationReportPanel.css'

export const ConsultationReportPanel=({projectId}:{projectId:string|number})=>{
 const id=String(projectId),client=useQueryClient(),port=useProductionStagePort(projectId),service=useProductionStageService(projectId),permissions=useProductionStagePermissions(projectId)
 const [previewFile,setPreviewFile]=useState<ProjectFile|null>(null),[pendingFiles,setPendingFiles]=useState<UploadFile[]>([])
 const query=useQuery({queryKey:['productionFilesByStage',id,'consultation'],queryFn:()=>service.getProductionFilesByStage(id,'consultation'),enabled:!!projectId})
 const reportFiles=query.data?.main_files||[]
 const attachmentFiles=useMemo<UploadFile[]>(()=>[...reportFiles.flatMap(record=>record.file?[{uid:`report:${record.id}`,name:record.file.original_name,size:record.file.file_size,status:'done' as const,response:record.file}]:[]),...pendingFiles],[pendingFiles,reportFiles])
 const refresh=async()=>{await client.invalidateQueries({queryKey:['productionFilesByStage',id,'consultation']});await query.refetch()}
 const upload=useMutation({mutationFn:async(files:globalThis.File[])=>port.saveStage({mainFiles:files,reviewSheets:[]}),onSuccess:async()=>{message.success('咨询报告附件上传成功');setPendingFiles([]);await refresh()},onError:(error:any)=>message.error(error?.message||'咨询报告上传失败')})
 const remove=useMutation({mutationFn:(referenceID:string)=>service.deleteProductionFile(id,referenceID),onSuccess:async()=>{message.success('咨询报告已移出，历史与 Evidence 字节保留');setPreviewFile(null);await refresh()},onError:(error:any)=>message.error(error?.message||'删除咨询报告失败')})
 const download=async(file:ProjectFile)=>{try{await fileService.downloadFile(file.id,file.original_name||file.file_name)}catch(error:any){message.error(error?.message||'文件下载失败')}}
 const save=()=>{const files=pendingFiles.flatMap(item=>item.originFileObj?[item.originFileObj]:[]);if(!files.length){message.warning('请先选择咨询报告附件');return}upload.mutate(files)}
 return <>
  <section className="consultation-report-panel">
   <header className="consultation-report-panel__header"><SectionTitle icon={<EyeOutlined/>}>咨询报告</SectionTitle><p className="consultation-report-panel__note">可集中维护多份咨询报告附件；CAD 文件保存后仅提供下载。</p></header>
   {query.isLoading?<div className="consultation-report-panel__state"><Spin/></div>:<div><AttachmentUploader label="咨询报告附件" helper={`支持多选和拖拽追加，单个文件最大 ${MAX_UPLOAD_SIZE_LABEL}`} fileList={attachmentFiles} onChange={files=>setPendingFiles(files.filter(file=>file.originFileObj))} beforeUpload={file=>isWithinUploadLimit(file)?false:false} disabled={!permissions.canUploadProductionFile||upload.isPending||remove.isPending} uploading={upload.isPending} accept=".pdf,.doc,.docx,.xls,.xlsx,.png,.jpg,.jpeg,.dwg,.dxf" maxCount={20} onPreview={file=>{const persisted=file.response as ProjectFile|undefined;if(persisted)setPreviewFile(persisted);else message.info('请先保存附件后再预览')}} onDownload={permissions.canDownloadProductionFile?file=>{const persisted=file.response as ProjectFile|undefined;if(persisted)void download(persisted)}:undefined} canRemove={file=>Boolean(file.originFileObj)||permissions.canDeleteProductionFile} onRemove={async file=>{if(file.originFileObj)return true;await remove.mutateAsync(String(file.uid).replace(/^report:/,''));return true}}/>{permissions.canUploadProductionFile&&pendingFiles.length>0&&<Button type="primary" icon={<UploadOutlined/>} loading={upload.isPending} onClick={save} style={{marginTop:12}}>上传 {pendingFiles.length} 个附件</Button>}</div>}
  </section>
  <FilePreviewDrawer open={!!previewFile} file={previewFile} onClose={()=>setPreviewFile(null)}/>
 </>
}
export default ConsultationReportPanel
