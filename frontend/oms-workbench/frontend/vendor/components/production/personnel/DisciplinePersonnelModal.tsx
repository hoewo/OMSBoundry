import { useEffect, useMemo } from 'react'
import { Alert, Button, Drawer, Form, Select, Space, message } from 'antd'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useProductionPersonnelPort, useProductionPersonnelCandidates } from '@/production-personnel-ports'
import { DisciplineSelector } from './DisciplineSelector'
import PersonnelSelect from '@/components/shared/PersonnelSelect'
import type { ProjectMember } from '@/production-personnel-ports'

interface DisciplinePersonnelModalProps {
  visible: boolean
  projectId: string
  participantNotApplicable?: boolean
  disciplineId?: string
  existingMembers?: ProjectMember[]
  onCancel: () => void
  onSuccess: () => void
}

export const DisciplinePersonnelModal = ({
  visible,
  projectId,
  participantNotApplicable = false,
  disciplineId: initialDisciplineId,
  existingMembers = [],
  onCancel,
  onSuccess,
}: DisciplinePersonnelModalProps) => {
  const [form] = Form.useForm()
  const port = useProductionPersonnelPort()
  const isEdit = !!initialDisciplineId

  const {
    data: usersData,
    isLoading: usersLoading,
    isError: usersLoadFailed,
    refetch: refetchUsers,
  } = useProductionPersonnelCandidates(projectId, visible)

  const users = usersData || []

  const catalog = useQuery({queryKey: ['disciplines', {includeInactive:false}], queryFn: () => port.listDisciplines(false), enabled:visible})

  const existingPersonnel = useMemo(() => {
    const designers = existingMembers.filter((m) => m.role === 'designer')
    const participants = existingMembers.filter((m) => m.role === 'participant')
    const reviewer = existingMembers.find((m) => m.role === 'reviewer')
    const auditor = existingMembers.find((m) => m.role === 'auditor')

    return {
      designer_ids: designers.map((m) => m.user_id),
      participant_ids: participants.map((m) => m.user_id),
      reviewer_id: reviewer?.user_id || undefined,
      auditor_id: auditor?.user_id || undefined,
    }
  }, [existingMembers])

  useEffect(() => {
    if (visible) {
      form.setFieldsValue({
        discipline_id: isEdit ? initialDisciplineId : undefined,
        designer_ids: isEdit ? existingPersonnel.designer_ids : [],
        participant_ids: isEdit ? existingPersonnel.participant_ids : [],
        reviewer_id: isEdit ? existingPersonnel.reviewer_id : undefined,
        auditor_id: isEdit ? existingPersonnel.auditor_id : undefined,
      })
    }
  }, [visible, isEdit, initialDisciplineId, existingPersonnel, form])

  const saveMutation = useMutation({
    mutationFn: async (data: { discipline_id: string; designer_ids?: string[]; participant_ids?: string[]; reviewer_id?: string; auditor_id?: string }) => port.saveDiscipline(projectId, { ...data, original_discipline_id: initialDisciplineId, participant_operation: participantNotApplicable ? 'preserve' : 'replace' }),
    onSuccess: () => {
      message.success(isEdit ? '更新生产人员成功' : '添加生产人员成功')
      onSuccess()
    },
    onError: (error: any) => {
      message.error(error.message || (isEdit ? '更新生产人员失败' : '添加生产人员失败'))
    },
  })

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      await saveMutation.mutateAsync({
        discipline_id: values.discipline_id,
        designer_ids: values.designer_ids || [],
        participant_ids: values.participant_ids || [],
        reviewer_id: values.reviewer_id,
        auditor_id: values.auditor_id,
      })
    } catch {
      // ignore validation errors
    }
  }

  return (
    <Drawer
      title={isEdit ? '编辑生产人员' : '添加生产人员'}
      open={visible}
      onClose={onCancel}
      width={600}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={onCancel}>取消</Button>
          <Button type="primary" onClick={handleSubmit} loading={saveMutation.isPending} disabled={usersLoading || usersLoadFailed || catalog.isPending || catalog.isError}>
            保存
          </Button>
        </Space>
      }
    >
      {usersLoadFailed && (
        <Alert
          type="error"
          showIcon
          message="人员列表加载失败"
          description="请重试；若仍然失败，请联系管理员检查网络或接口状态。"
          action={<Button size="small" onClick={() => refetchUsers()}>重试</Button>}
          style={{ marginBottom: 16 }}
        />
      )}
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          designer_ids: [],
          participant_ids: [],
          reviewer_id: undefined,
          auditor_id: undefined,
        }}
      >
        <Form.Item
          name="discipline_id"
          label="专业"
          rules={[{ required: true, message: '请选择专业' }]}
        >
          <DisciplineSelector
            placeholder="请选择专业"
            allowManage={true}
          />
        </Form.Item>

        <Form.Item
          name="designer_ids"
          label="设计人"
          rules={[]}
        >
          <PersonnelSelect
            mode="multiple"
            placeholder="请选择设计人（可多选，可选）"
            users={users}
            loading={usersLoading}
            disabled={usersLoadFailed}
          />
        </Form.Item>

        <Form.Item
          name="participant_ids"
          label={participantNotApplicable ? '参与人（无/不适用）' : '参与人'}
          rules={[]}
        >
          <PersonnelSelect
            mode="multiple"
            placeholder="请选择参与人（可多选，可选）"
            users={users}
            loading={usersLoading}
            disabled={usersLoadFailed || participantNotApplicable}
          />
        </Form.Item>

        <Form.Item
          name="reviewer_id"
          label="复核人"
          rules={[]}
        >
          <PersonnelSelect
            placeholder="请选择复核人（可选）"
            users={users}
            loading={usersLoading}
            disabled={usersLoadFailed}
          />
        </Form.Item>

        <Form.Item
          name="auditor_id"
          label="审核人"
          rules={[]}
        >
          <PersonnelSelect
            placeholder="请选择审核人（可选）"
            users={users}
            loading={usersLoading}
            disabled={usersLoadFailed}
          />
        </Form.Item>
      </Form>
    </Drawer>
  )
}
