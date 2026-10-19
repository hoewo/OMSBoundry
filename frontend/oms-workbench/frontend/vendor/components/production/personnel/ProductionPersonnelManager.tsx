import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import {
  Button,
  message,
  Empty,
  Alert,
  Spin,
} from 'antd'
import {
  EditOutlined,
  SettingOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useProductionPersonnelPort, useProductionPersonnelPermissions } from '@/production-personnel-ports'
import { SectionTitle } from '@/components/shared/SectionTitle'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import { DisciplinePersonnelModal } from './DisciplinePersonnelModal'
import { ReviewerModal } from './ReviewerModal'
import PersonnelName from '@/components/shared/PersonnelName'
import type { ProjectMember, MemberRole } from '@/production-personnel-ports'
import '../../project/ProjectWorkspace.css'
import './ProductionPersonnelManager.css'

interface ProductionPersonnelManagerProps {
  projectId: string
  participantNotApplicable?: boolean
}

const DISCIPLINE_BOUND_ROLES: MemberRole[] = [
  'designer',
  'participant',
  'reviewer',
  'auditor',
]

export const ProductionPersonnelManager = ({
  projectId,
  participantNotApplicable = false,
}: ProductionPersonnelManagerProps) => {
  const [personnelModalVisible, setPersonnelModalVisible] = useState(false)
  const [reviewerModalVisible, setReviewerModalVisible] = useState(false)
  const [selectedDisciplineId, setSelectedDisciplineId] = useState<string | undefined>()
  const queryClient = useQueryClient()
  const port = useProductionPersonnelPort()
  const { canAssignProductionMembers: canManage } = useProductionPersonnelPermissions(projectId)

  const { data: projectMembers = [], isPending: membersPending, isError: membersFailed, refetch } = useQuery({
    queryKey: ['projectMembers', projectId],
    queryFn: () => port.listMembers(projectId),
    enabled: !!projectId,
  })

  useEffect(() => {
    if (!canManage) {
      setPersonnelModalVisible(false)
      setReviewerModalVisible(false)
      setSelectedDisciplineId(undefined)
    }
  }, [canManage])

  const removeMutation = useMutation({
    mutationFn: async (disciplineId: string) => {
      await port.endDiscipline(projectId, disciplineId, {participant_operation:participantNotApplicable ? 'preserve' : 'end'})
    },
    onSuccess: () => {
      message.success('已移出专业配置；历史保留')
      queryClient.invalidateQueries({
        queryKey: ['projectMembers', projectId],
      })
    },
    onError: (error: any) => {
      message.error(error.message || '删除失败')
    },
  })

  const productionMembers = projectMembers.filter(
    (m) =>
      DISCIPLINE_BOUND_ROLES.includes(m.role as MemberRole) &&
      m.discipline_id &&
      !(participantNotApplicable && m.role === 'participant')
  )

  const approver = projectMembers.find((m) => m.role === 'approver')
  const estimator = projectMembers.find((m) => m.role === 'estimator')

  const membersByDiscipline = productionMembers.reduce((acc, member) => {
    const disciplineId = member.discipline_id || 'unknown'
    if (!acc[disciplineId]) {
      acc[disciplineId] = {
        disciplineId,
        disciplineName: member.discipline?.name || '未知专业',
        members: [],
      }
    }
    acc[disciplineId].members.push(member)
    return acc
  }, {} as Record<string, { disciplineId: string; disciplineName: string; members: ProjectMember[] }>)

  const handleAddPersonnel = () => {
    setSelectedDisciplineId(undefined)
    setPersonnelModalVisible(true)
  }

  const handleEditPersonnel = (disciplineId: string) => {
    setSelectedDisciplineId(disciplineId)
    setPersonnelModalVisible(true)
  }

  // The original shared confirmation does not await a returned promise.
  // Mutation callbacks own errors; do not leak a rejected promise into the page.
  const handleDeletePersonnel = (disciplineId: string) => removeMutation.mutate(disciplineId)

  const handlePersonnelSuccess = () => {
    queryClient.invalidateQueries({
      queryKey: ['projectMembers', projectId],
    })
    setPersonnelModalVisible(false)
    setSelectedDisciplineId(undefined)
  }

  const handleReviewerSuccess = () => {
    queryClient.invalidateQueries({
      queryKey: ['projectMembers', projectId],
    })
    setReviewerModalVisible(false)
  }

  const personnelGroups = Object.values(membersByDiscipline).map((group) => {
    const designers = group.members.filter((m) => m.role === 'designer')
    const participants = group.members.filter((m) => m.role === 'participant')
    const reviewers = group.members.filter((m) => m.role === 'reviewer')
    const auditors = group.members.filter((m) => m.role === 'auditor')

    const renderNames = (members: ProjectMember[]): ReactNode =>
      members.length > 0
        ? members.map((member, index) => (
            <span key={member.id}>
              {index > 0 && '、'}
              <PersonnelName
                user={member.user}
                fallback={`用户ID: ${member.user_id}`}
              />
            </span>
          ))
        : '-'

    return {
      disciplineId: group.disciplineId,
      disciplineName: group.disciplineName,
      designer: renderNames(designers),
      participant: participantNotApplicable ? '无/不适用' : renderNames(participants),
      reviewer: renderNames(reviewers),
      auditor: renderNames(auditors),
      allMembers: group.members,
    }
  })

  const renderMemberName = (member?: ProjectMember) =>
    member ? (
      <PersonnelName user={member.user} fallback={`用户ID: ${member.user_id}`} />
    ) : (
      '未设置'
    )

  const RoleCell = ({ label, value }: { label: string; value: ReactNode }) => (
    <div className="production-personnel__role-cell">
      <span>{label}</span>
      <strong
        className={value === '-' ? 'production-personnel__role-value--empty' : ''}
      >
        {value}
      </strong>
    </div>
  )

  if (membersPending) return <Spin tip="正在读取生产人员…"><div style={{minHeight:80}}/></Spin>
  if (membersFailed) return <Alert type="error" showIcon message="生产人员读取失败，不作为空名单" action={<Button onClick={() => void refetch()}>重新读取</Button>} />

  return (
    <>
      <section className="project-workspace__card production-personnel">
        <header className="production-personnel__header">
          <div className="production-personnel__heading">
            <div className="production-personnel__heading-main">
              <SectionTitle icon={<TeamOutlined />}>生产人员配置</SectionTitle>
              <p className="production-personnel__subtitle">
                按专业维护设计、参与、复核、审核角色；参与人可按项目设为无。
              </p>
            </div>
            <div className="production-personnel__key-roles">
              <div className="production-personnel__key-role">
                <span>审定人</span>
                <strong className={!approver ? 'production-personnel__key-value--missing' : ''}>
                  {renderMemberName(approver)}
                </strong>
                {canManage && (
                  <button
                    type="button"
                    className="production-personnel__key-role-action"
                    onClick={() => setReviewerModalVisible(true)}
                    aria-label="设置审定人"
                    title="设置审定人"
                  >
                    <SettingOutlined />
                  </button>
                )}
              </div>
              <div className="production-personnel__key-role">
                <span>概算人</span>
                <strong className={!estimator ? 'production-personnel__key-value--missing' : ''}>
                  {renderMemberName(estimator)}
                </strong>
                {canManage && (
                  <button
                    type="button"
                    className="production-personnel__key-role-action"
                    onClick={() => setReviewerModalVisible(true)}
                    aria-label="设置概算人"
                    title="设置概算人"
                  >
                    <SettingOutlined />
                  </button>
                )}
              </div>
            </div>
          </div>
        </header>

        <div className="production-personnel__body">
          <div className="production-personnel__body-title">
            <span>生产人员（按专业）</span>
            <div className="production-personnel__body-tools">
              <strong>{personnelGroups.length} 个专业</strong>
              {canManage && (
                <CreateActionButton
                  className="production-personnel__add-button"
                  size="small"
                  compact
                  onClick={handleAddPersonnel}
                  aria-label="添加生产人员"
                  title="添加生产人员"
                />
              )}
            </div>
          </div>
          {personnelGroups.length > 0 ? (
            <div className="production-personnel__list">
              {personnelGroups.map((group) => (
                <article
                  className="production-personnel__item"
                  key={group.disciplineId}
                >
                  <div className="production-personnel__discipline">
                    <strong>{group.disciplineName}</strong>
                    <span>{group.allMembers.length} 人已配置</span>
                  </div>
                  <RoleCell label="设计人" value={group.designer} />
                  <RoleCell label="参与人" value={group.participant} />
                  <RoleCell label="复核人" value={group.reviewer} />
                  <RoleCell label="审核人" value={group.auditor} />
                  {canManage && (
                    <div className="production-personnel__actions">
                      <Button
                        type="text"
                        size="small"
                        icon={<EditOutlined />}
                        onClick={() => handleEditPersonnel(group.disciplineId)}
                      >
                        编辑
                      </Button>
                      <DeleteConfirmButton
                        title="确认删除生产人员"
                        description="移出当前专业配置并保留历史；不删除人员或账号。"
                        details={[
                          { label: '专业', value: group.disciplineName },
                          { label: '人数', value: `${group.allMembers.length} 人` },
                        ]}
                        buttonProps={{ size: 'small' }}
                        onConfirm={() => handleDeletePersonnel(group.disciplineId)}
                      />
                    </div>
                  )}
                </article>
              ))}
            </div>
          ) : (
            <div className="production-personnel__empty">
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="暂无生产人员"
              />
            </div>
          )}
        </div>
      </section>

      <DisciplinePersonnelModal
        visible={personnelModalVisible}
        projectId={projectId}
        participantNotApplicable={participantNotApplicable}
        disciplineId={selectedDisciplineId}
        existingMembers={
          selectedDisciplineId
            ? membersByDiscipline[selectedDisciplineId]?.members || []
            : []
        }
        onCancel={() => {
          setPersonnelModalVisible(false)
          setSelectedDisciplineId(undefined)
        }}
        onSuccess={handlePersonnelSuccess}
      />

      <ReviewerModal
        visible={reviewerModalVisible}
        projectId={projectId}
        approver={approver || null}
        estimator={estimator || null}
        onCancel={() => setReviewerModalVisible(false)}
        onSuccess={handleReviewerSuccess}
      />
    </>
  )
}
