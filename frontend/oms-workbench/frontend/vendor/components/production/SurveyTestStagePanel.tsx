// Copied from the user-owned reference OA SurveyTestStagePanel and adapted
// only at the service boundary: the visual and interaction structure stay in
// the component while the port writes versioned Production/Evidence facts.
import {useState} from 'react'
import {Button,Empty,Space,message} from 'antd'
import type {UploadFile} from 'antd'
import {DownloadOutlined,UploadOutlined} from '@ant-design/icons'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {AttachmentUploader} from '@/components/shared/AttachmentUploader'
import {useProductionStagePermissions,useProductionStagePort,useProductionStageService} from '@/production-stage-ports'
import {fileService} from '@/services/file'

export const SurveyTestStagePanel=({projectId}:{projectId:string|number})=>{
 const id=String(projectId),client=useQueryClient(),port=useProductionStagePort(projectId),service=useProductionStageService(projectId)
 const permissions=useProductionStagePermissions(projectId),[fileList,setFileList]=useState<UploadFile[]>([])
 const query=useQuery({queryKey:['productionFilesByStage',id,'test'],queryFn:()=>service.getProductionFilesByStage(id,'test')})
 const files=query.data?.main_files||[]
 const upload=useMutation({mutationFn:async()=>{const pending=fileList.flatMap(file=>file.originFileObj?[file.originFileObj]:[]);if(!pending.length)throw Error('请选择试验阶段文件');await port.saveStage({mainFiles:pending,reviewSheets:[]})},onSuccess:async()=>{setFileList([]);await client.invalidateQueries({queryKey:['productionFilesByStage',id,'test']});message.success('试验阶段资料已上传，进度已自动更新')},onError:(error:any)=>message.error(error?.message||'试验阶段资料上传失败')})
 const download=async(viewID:string,name:string)=>{try{await fileService.downloadFile(viewID,name)}catch(error:any){message.error(error?.message||'文件下载失败')}}
 return <section className="project-workspace__card">
  <header className="project-workspace__card-header"><div><h3 className="project-workspace__card-title">试验阶段资料</h3><p className="project-workspace__card-subtitle">勘察项目专用；上传至少一份有效资料后，系统自动完成本阶段。支持普通文档及 CAD 文件，CAD 不提供在线预览。</p></div></header>
  <AttachmentUploader fileList={fileList} onChange={setFileList} disabled={!permissions.canUploadProductionFile||upload.isPending} helper="文件保存后自动计入勘察试验阶段，不需要手动调整进度。" accept=".pdf,.doc,.docx,.xls,.xlsx,.png,.jpg,.jpeg,.dwg,.dxf"/>
  {permissions.canUploadProductionFile&&<div style={{marginTop:12}}><Button type="primary" icon={<UploadOutlined/>} loading={upload.isPending} onClick={()=>upload.mutate()}>上传并保存</Button></div>}
  <div style={{marginTop:20}}>{query.isLoading?<Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="正在读取试验阶段资料"/>:files.length===0?<Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无试验阶段资料"/>:<div className="attachment-uploader__list">{files.map(record=>record.file&&<div className="attachment-uploader__row" key={record.id}><div><strong>{record.file.original_name}</strong></div><Space>{permissions.canDownloadProductionFile&&<Button type="link" icon={<DownloadOutlined/>} onClick={()=>download(record.file!.id,record.file!.original_name)}>下载</Button>}</Space></div>)}</div>}</div>
 </section>
}
