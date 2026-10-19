import {CheckCircleOutlined} from '@ant-design/icons'
import {AuxiliaryFilePanel} from './AuxiliaryFilePanel'
import {CompletionFileUpload} from './CompletionFileUpload'
export const CompletionFileList=({projectId}:{projectId:string|number})=><AuxiliaryFilePanel projectId={projectId} stage="completion" title="竣工验收" description="集中维护竣工验收文件，作为生产资料的收口记录。" icon={<CheckCircleOutlined/>} tone="completion" UploadComponent={CompletionFileUpload}/>
