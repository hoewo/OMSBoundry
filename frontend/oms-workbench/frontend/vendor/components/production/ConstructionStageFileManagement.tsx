import { FileTextOutlined } from '@ant-design/icons'
import { StageFilePanel } from './StageFilePanel'
import { ConstructionStageEditModal } from './ConstructionStageEditModal'

interface ConstructionStageFileManagementProps {
  projectId: string | number
}

export const ConstructionStageFileManagement = ({
  projectId,
}: ConstructionStageFileManagementProps) => (
  <StageFilePanel
    projectId={projectId}
    stage="construction"
    title="施工图设计"
    mainFileLabel="施工图设计文件"
    icon={<FileTextOutlined />}
    tone="construction"
    EditComponent={ConstructionStageEditModal}
  />
)
