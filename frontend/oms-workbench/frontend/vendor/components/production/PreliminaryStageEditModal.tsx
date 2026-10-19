import {NativeStageEditModal} from './SchemeStageEditModal'

interface PreliminaryStageEditModalProps {
  projectId: string | number
  visible: boolean
  stageInfo?: import('@/types').StageFileInfo
  canDeleteExisting?: boolean
  onCancel: () => void
  onSuccess: () => void
}

// The original OA preliminary drawer has the same interaction structure as
// the scheme drawer. Reuse the copied component and only bind its original
// stage identity and labels; the injected native port owns all behavior.
export const PreliminaryStageEditModal = (props: PreliminaryStageEditModalProps) => (
  <NativeStageEditModal {...props} stage="preliminary" drawerTitle="编辑初步设计阶段文件" mainFileLabel="初步设计文件" />
)
