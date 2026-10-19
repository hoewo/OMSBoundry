import { BulbOutlined } from '@ant-design/icons'
import { StageFilePanel } from './StageFilePanel'
import { SchemeStageEditModal } from './SchemeStageEditModal'

interface SchemeStageFileManagementProps {
  projectId: string | number
}

export const SchemeStageFileManagement = ({
  projectId,
}: SchemeStageFileManagementProps) => (
  <StageFilePanel
    projectId={projectId}
    stage="scheme"
    title="方案阶段"
    mainFileLabel="方案文件"
    icon={<BulbOutlined />}
    tone="scheme"
    EditComponent={SchemeStageEditModal}
  />
)
