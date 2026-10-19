import {useState} from 'react'
import {Alert,Button,Modal,Space,message} from 'antd'
import type {UploadFile} from 'antd'
import {useQuery} from '@tanstack/react-query'
import {contractForID} from './services/registry'
import {fileService} from './services/file'
import {AttachmentUploader} from '@/components/shared/AttachmentUploader'
import {FilePreviewDrawer} from '@/components/shared/FilePreviewDrawer'
import type {File as ProjectFile} from './types'

// A small owner port inside the original uploader; no alternate visual system.
export function ContractMaterials({contractId,kind,amendmentId='',label}:{contractId:string;kind:'contract'|'amendment'|'finalization';amendmentId?:string;label:string}){
 const adapter=contractForID(contractId),[queued,setQueued]=useState<UploadFile[]>([]),[busy,setBusy]=useState(false),[error,setError]=useState(''),[preview,setPreview]=useState<ProjectFile|null>(null)
 useQuery({queryKey:['contractNative',contractId],queryFn:async()=>{await adapter.get(contractId);return adapter.native.get(contractId)}})
 const owned=adapter.materialFiles(contractId,kind,amendmentId),staged=adapter.stagedFiles(contractId,kind,amendmentId)
 const done=(f:any):UploadFile=>({uid:f.id,name:f.original_name,status:'done',size:f.file_size,response:f})
 const known=[...owned,...staged],fileList=[...known.map(done),...queued],canManage=adapter.canManageMaterials(contractId)&&!busy
 const run=async(action:()=>Promise<unknown>)=>{setBusy(true);setError('');try{await action()}catch(e){setError(e instanceof Error?e.message:'文件操作尚未确认')}finally{setBusy(false)}}
 const save=()=>run(async()=>{
  for(const file of queued){
   if(!file.originFileObj)continue
   // The adapter stores the completed version before rereading metadata.
   // Remove this local upload after either outcome if completion was recorded.
   const count=adapter.uploaded.size
   try{const uploaded=await adapter.uploadMaterial(contractId,kind,amendmentId,file.originFileObj);setQueued(q=>q.filter(v=>v.uid!==file.uid));await adapter.mutateMaterial(contractId,kind,amendmentId,uploaded.id,'append')}
   catch(e){if(adapter.uploaded.size>count)setQueued(q=>q.filter(v=>v.uid!==file.uid));throw e}
  }
  for(const file of adapter.stagedFiles(contractId,kind,amendmentId))await adapter.mutateMaterial(contractId,kind,amendmentId,file.id,'append')
  message.success('附件业务引用已保存，文件版本由 Evidence 保管')
 })
 const remove=async(file:UploadFile)=>{
  if(String(file.uid).startsWith('contract-material:')){Modal.confirm({title:'移出当前合同附件引用？',content:'保留文件版本和业务历史，不删除物理文件。',onOk:()=>run(()=>adapter.mutateMaterial(contractId,kind,amendmentId,String(file.uid),'remove'))});return false}
  if(String(file.uid).startsWith('contract-upload:')){adapter.uploaded.delete(file.uid);adapter.persistFiles();adapter.changed();return false}
  setQueued(q=>q.filter(v=>v.uid!==file.uid));return false
 }
 return <div data-contract-material-kind={kind} data-amendment-id={amendmentId}>
  <AttachmentUploader label={label} helper="上传完成后单独登记本合同引用；移出不删除文件。" fileList={fileList} disabled={!canManage} uploading={busy} beforeUpload={()=>false}
   onChange={next=>setQueued(next.filter(f=>!known.some(v=>v.id===f.uid)))} onRemove={remove}
   onPreview={file=>{const value=known.find(v=>v.id===file.uid);if(value&&adapter.files.has(value.id))setPreview(value);else message.info('文件元数据尚未读取，请先登记或继续原操作')}}
   onDownload={file=>void run(()=>fileService.downloadFile(String(file.uid),file.name))} />
  {(queued.length>0||staged.length>0)&&<Space style={{marginTop:8}}><Button type="primary" disabled={!canManage} loading={busy} onClick={()=>void save()}>上传并登记{label}</Button><span>未登记文件 {queued.length+staged.length} 份</span></Space>}
  {!adapter.native.get(contractId)?.allowed_operations.includes('reference-oa.material.manage')&&<Alert type="info" style={{marginTop:8}} message="当前只读，或外部 Evidence 未选装；不会退回旧内嵌上传。" />}
  {error&&<Alert type="error" style={{marginTop:8}} message={error} />}
  <FilePreviewDrawer open={!!preview} file={preview} onClose={()=>setPreview(null)} />
 </div>
}
