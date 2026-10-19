import { useState } from 'react'
import { Button, Drawer } from 'antd'
import { TrophyOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { canManageBonus } from '@/services/permission'
import { SectionTitle } from '@/components/shared/SectionTitle'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import { BonusAllocationForm } from './BonusAllocationForm'
import { BonusAllocationList } from './BonusAllocationList'
import { BonusAllocationStatistics } from './BonusAllocationStatistics'
import '../project/ProjectWorkspace.css'
import './BonusAllocation.css'

interface BonusAllocationProps {
  projectId: string
  bonusType: 'business' | 'production'
  showStatistics?: boolean
  onRefresh?: () => void
}

/**
 * 可复用的奖金分配主组件
 * 整合表单、统计、列表，支持bonusType参数：business/production
 * T499: 创建可复用的奖金分配主组件
 */
export const BonusAllocation = ({
  projectId,
  bonusType,
  showStatistics = false,
  onRefresh,
}: BonusAllocationProps) => {
  const [modalVisible, setModalVisible] = useState(false)

  const { data: canManage } = useQuery({
    queryKey: ['canManageBonus', projectId, bonusType],
    queryFn: () => canManageBonus(projectId, bonusType),
    enabled: !!projectId,
  })

  const handleCreate = () => {
    setModalVisible(true)
  }

  const handleModalClose = () => {
    setModalVisible(false)
  }

  const handleSuccess = () => {
    setModalVisible(false)
    onRefresh?.()
  }

  const title = bonusType === 'business' ? '经营奖金分配' : '生产奖金分配'

  return (
    <>
      <section
        className={`project-workspace__card bonus-allocation bonus-allocation--${bonusType}`}
      >
        <header className="bonus-allocation__header">
          <div>
            <SectionTitle icon={<TrophyOutlined />}>{title}</SectionTitle>
            <p className="bonus-allocation__subtitle">
              支持按比例和固定金额混合分配登记，固定金额不会占用比例池。
            </p>
          </div>
          {canManage && (
            <CreateActionButton
              size="small"
              compact
              onClick={handleCreate}
            >
              分配奖金
            </CreateActionButton>
          )}
        </header>

        {showStatistics && (
          <BonusAllocationStatistics projectId={projectId} bonusType={bonusType} />
        )}

        <BonusAllocationList
          projectId={projectId}
          bonusType={bonusType}
          canManage={canManage || false}
        />
      </section>

      <Drawer
        title={`分配${bonusType === 'business' ? '经营' : '生产'}奖金`}
        open={modalVisible}
        onClose={handleModalClose}
        footer={null}
        width={860}
        zIndex={1000}
        destroyOnHidden
      >
        <BonusAllocationForm
          projectId={projectId}
          bonusType={bonusType}
          onSuccess={handleSuccess}
          onCancel={handleModalClose}
        />
      </Drawer>
    </>
  )
}
