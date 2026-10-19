import {useState} from 'react'
import {Drawer,Form,Button,message,Space} from 'antd'
import type {UploadFile} from 'antd'
import {InlineFilePreviewLayout} from '@/components/shared/InlineFilePreviewLayout'
import {ProductionStageFileField} from './ProductionStageFileField'
import {useProductionStagePort} from '@/production-stage-ports'
import {MAX_UPLOAD_SIZE_LABEL} from '@/utils/upload'
import type {ProductionStage} from '@/types'
import './ProductionStageEditModal.css'

export function AuxiliaryStageUpload({projectId,visible,onCancel,onSuccess,stage,title,label,accept,hint}:{projectId:string|number;visible:boolean;onCancel:()=>void;onSuccess?:()=>void;stage:ProductionStage;title:string;label:string;accept:string;hint:string}){
 const port=useProductionStagePort(projectId),[form]=Form.useForm(),[files,setFiles]=useState<UploadFile[]>([]),[preview,setPreview]=useState<UploadFile|null>(null),[uploading,setUploading]=useState(false)
 const cancel=()=>{form.resetFields();setFiles([]);setPreview(null);onCancel()}
 const submit=async()=>{if(!files.length){message.warning('请选择要上传的文件');return}setUploading(true);try{const blobs=files.map(file=>{if(!file.originFileObj)throw Error(`文件 ${file.name} 无效`);return file.originFileObj});if(port.stage!==stage)throw Error('辅助阶段端口不匹配');await port.saveStage({mainFiles:blobs,reviewSheets:[]});message.success(`成功上传 ${files.length} 个文件`);form.resetFields();setFiles([]);setPreview(null);onSuccess?.()}catch(e:any){message.error(e?.message||'文件上传失败')}finally{setUploading(false)}}
 const remove=(uid:string)=>{setFiles(rows=>rows.filter(row=>row.uid!==uid));if(preview?.uid===uid)setPreview(null)}
 return <Drawer title={title} open={visible} onClose={cancel} width={preview?'calc(100vw - 24px)':620} destroyOnHidden footer={<Space style={{width:'100%',justifyContent:'flex-end'}}><Button onClick={cancel} disabled={uploading}>取消</Button><Button type="primary" onClick={submit} loading={uploading}>确认上传</Button></Space>}>
  <InlineFilePreviewLayout className="production-stage-edit__layout" contentClassName="production-stage-edit__form-column" uploadFile={preview} onClosePreview={()=>setPreview(null)} defaultPreviewWidth={820} minPreviewWidth={780} minContentWidth={460} contentWidth={620} pinContentToRight minHeight="calc(100vh - 170px)" previewHeight="calc(100vh - 170px)">
   <Form form={form} layout="vertical" className="production-stage-edit__form"><Form.Item><ProductionStageFileField label={label} required fileList={files} onFileListChange={setFiles} onRemovePendingFile={remove} multiple accept={accept} uploadHint={`${hint}，最大 ${MAX_UPLOAD_SIZE_LABEL}，可上传多个文件`} deleteTitle="确认删除文件" deleteDescription="仅从本次待上传列表移除。" disabled={uploading} onPreviewFile={()=>undefined} onPreviewUploadFile={setPreview} onDownloadFile={()=>undefined} onDeleteExistingFile={()=>undefined}/></Form.Item></Form>
  </InlineFilePreviewLayout>
 </Drawer>
}
