import { GoldOutlined } from '@ant-design/icons'
import { StageFilePanel } from './StageFilePanel'
import { PreliminaryStageEditModal } from './PreliminaryStageEditModal'

interface PreliminaryStageFileManagementProps {
  projectId: string | number
}

export const PreliminaryStageFileManagement = ({
  projectId,
}: PreliminaryStageFileManagementProps) => (
  <StageFilePanel
    projectId={projectId}
    stage="preliminary"
    title="初步设计"
    mainFileLabel="初步设计文件"
    icon={<GoldOutlined />}
    tone="preliminary"
    EditComponent={PreliminaryStageEditModal}
  />
)
