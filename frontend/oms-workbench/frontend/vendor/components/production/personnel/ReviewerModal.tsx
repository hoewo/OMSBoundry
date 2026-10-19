import { useEffect } from 'react'
import { Alert, Button, Drawer, Form, Select, Space, message } from 'antd'
import { useMutation } from '@tanstack/react-query'
import { useProductionPersonnelPort, useProductionPersonnelCandidates } from '@/production-personnel-ports'
import PersonnelSelect from '@/components/shared/PersonnelSelect'
import type { ProjectMember, MemberRole } from '@/production-personnel-ports'

interface ReviewerModalProps {
  visible: boolean
  projectId: string
  approver?: ProjectMember | null
  estimator?: ProjectMember | null
  onCancel: () => void
  onSuccess: () => void
}

export const ReviewerModal = ({
  visible,
  projectId,
  approver,
  estimator,
  onCancel,
  onSuccess,
}: ReviewerModalProps) => {
  const [form] = Form.useForm()
  const port = useProductionPersonnelPort()

  useEffect(() => {
    if (visible) {
      form.setFieldsValue({
        approver_id: approver?.user_id || undefined,
        estimator_id: estimator?.user_id || undefined,
      })
    }
  }, [visible, approver, estimator, form])

  const {
    data: usersData,
    isLoading: usersLoading,
    isError: usersLoadFailed,
    refetch: refetchUsers,
  } = useProductionPersonnelCandidates(projectId, visible)

  const users = usersData || []

  const saveMutation = useMutation({
    mutationFn: async (data: { approver_id?: string; estimator_id?: string }) => port.saveReviewers(projectId, data),
    onSuccess: () => {
      message.success('保存成功')
      onSuccess()
    },
    onError: (error: any) => {
      message.error(error.message || '保存失败')
    },
  })

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      await saveMutation.mutateAsync({
        approver_id: values.approver_id,
        estimator_id: values.estimator_id,
      })
    } catch {
      // ignore validation errors
    }
  }

  return (
    <Drawer
      title="设置审定人和概算人"
      open={visible}
      onClose={onCancel}
      width={600}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={onCancel}>取消</Button>
          <Button type="primary" onClick={handleSubmit} loading={saveMutation.isPending} disabled={usersLoading || usersLoadFailed}>
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
          approver_id: undefined,
          estimator_id: undefined,
        }}
      >
        <Form.Item
          name="approver_id"
          label="审定人"
          rules={[]}
        >
          <PersonnelSelect
            placeholder="请选择审定人（可选）"
            users={users}
            loading={usersLoading}
            disabled={usersLoadFailed}
          />
        </Form.Item>

        <Form.Item
          name="estimator_id"
          label="概算人"
          rules={[]}
        >
          <PersonnelSelect
            placeholder="请选择概算人（可选）"
            users={users}
            loading={usersLoading}
            disabled={usersLoadFailed}
          />
        </Form.Item>
      </Form>
    </Drawer>
  )
}
