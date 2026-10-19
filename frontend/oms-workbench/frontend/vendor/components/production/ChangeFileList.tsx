import {SwapOutlined} from '@ant-design/icons'
import {AuxiliaryFilePanel} from './AuxiliaryFilePanel'
import {ChangeFileUpload} from './ChangeFileUpload'
export const ChangeFileList=({projectId}:{projectId:string|number})=><AuxiliaryFilePanel projectId={projectId} stage="change" title="变更洽商" description="集中维护项目变更洽商文件，避免和主阶段资料混放。" icon={<SwapOutlined/>} tone="change" UploadComponent={ChangeFileUpload}/>
