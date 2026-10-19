import {AuxiliaryStageUpload} from './AuxiliaryStageUpload'
export const CompletionFileUpload=(props:{projectId:string|number;visible:boolean;onCancel:()=>void;onSuccess?:()=>void})=><AuxiliaryStageUpload {...props} stage="completion" title="上传竣工验收文件" label="竣工验收文件" accept=".pdf,.doc,.docx,.xls,.xlsx,.zip,.rar,.png,.jpg,.jpeg,.dwg,.dxf" hint="支持常用文档、压缩包、图片及 CAD（DWG、DXF）"/>
