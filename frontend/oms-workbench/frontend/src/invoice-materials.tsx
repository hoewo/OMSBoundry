import {useState} from 'react'
import {Alert,Button,Modal,message} from 'antd'
import type {UploadFile} from 'antd'
import {invoiceForProject} from './services/registry'
import {fileService} from './services/file'
import {AttachmentUploader} from '@/components/shared/AttachmentUploader'
import {FilePreviewDrawer} from '@/components/shared/FilePreviewDrawer'
import type {File as ProjectFile} from './types'
export function InvoiceMaterials({projectId,invoiceId}:{projectId:string;invoiceId:string}){
 const a=invoiceForProject(projectId),[queued,setQueued]=useState<UploadFile[]>([]),[busy,setBusy]=useState(false),[error,setError]=useState(''),[preview,setPreview]=useState<ProjectFile|null>(null)
 const known=[...a.get(invoiceId).invoice_files,...a.stagedFiles(invoiceId)],fileList=[...known.map((f:any)=>({uid:f.id,name:f.original_name,size:f.file_size,status:'done' as const,response:f})),...queued],can=a.canEvidence(invoiceId)&&!busy
 const run=async(fn:()=>Promise<unknown>)=>{setBusy(true);setError('');try{await fn()}catch(e){setError(e instanceof Error?e.message:'附件操作未确认')}finally{setBusy(false)}}
 const save=()=>run(async()=>{for(const f of queued){if(!f.originFileObj)continue;const count=a.uploaded.size;try{await a.uploadFile(invoiceId,f.originFileObj);setQueued(q=>q.filter(v=>v.uid!==f.uid))}catch(e){if(a.uploaded.size>count)setQueued(q=>q.filter(v=>v.uid!==f.uid));throw e}}await a.evidence(invoiceId);message.success('文件已完成并登记 Billing 业务引用')})
 return <div data-invoice-materials>
  <AttachmentUploader label="发票文件" helper="先完成 Evidence 上传，再登记开票引用；移出不删除文件。" fileList={fileList} disabled={!can} uploading={busy} beforeUpload={()=>false} onChange={v=>setQueued(v.filter(f=>!known.some((x:any)=>x.id===f.uid)))}
   onRemove={async f=>{if(String(f.uid).startsWith('invoice-file:'))Modal.confirm({title:'移出开票文件引用？',content:'只追加业务引用更正，保留文件版本及历史。',onOk:()=>run(()=>a.evidence(invoiceId,String(f.uid)))});else if(String(f.uid).startsWith('invoice-upload:')){a.uploaded.delete(f.uid);a.persistFiles();a.changed()}else setQueued(q=>q.filter(v=>v.uid!==f.uid));return false}}
   onPreview={f=>{const v=a.files.get(f.uid);if(v)setPreview(v);else message.info('元数据待读取，请先登记或继续原操作')}} onDownload={f=>void run(()=>fileService.downloadFile(String(f.uid),f.name))} />
  {(queued.length>0||a.stagedFiles(invoiceId).length>0)&&<Button style={{margin:'8px 0 16px'}} type="primary" disabled={!can} loading={busy} onClick={()=>void save()}>上传并登记发票文件</Button>}
  {error&&<Alert type="error" style={{marginBottom:16}} message={error} />}
  <FilePreviewDrawer open={!!preview} file={preview} onClose={()=>setPreview(null)} />
 </div>
}
