import {AuxiliaryStageUpload} from './AuxiliaryStageUpload'
export const ChangeFileUpload=(props:{projectId:string|number;visible:boolean;onCancel:()=>void;onSuccess?:()=>void})=><AuxiliaryStageUpload {...props} stage="change" title="上传变更洽商文件" label="变更洽商文件" accept=".pdf,.doc,.docx,.xls,.xlsx,.png,.jpg,.jpeg,.dwg,.dxf" hint="支持常用文档、图片及 CAD（DWG、DXF）"/>
