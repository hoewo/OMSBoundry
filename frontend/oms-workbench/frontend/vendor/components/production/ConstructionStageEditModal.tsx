import {NativeStageEditModal} from './SchemeStageEditModal'

interface ConstructionStageEditModalProps {
  projectId: string | number
  visible: boolean
  stageInfo?: import('@/types').StageFileInfo
  canDeleteExisting?: boolean
  onCancel: () => void
  onSuccess: () => void
}

// The original OA construction drawer is structurally identical to the
// preliminary drawer. Keep the copied interaction and bind only its original
// stage identity and labels; Production owns the injected native behavior.
export const ConstructionStageEditModal = (props: ConstructionStageEditModalProps) => (
  <NativeStageEditModal {...props} stage="construction" drawerTitle="编辑施工图设计阶段文件" mainFileLabel="施工图设计文件" />
)
