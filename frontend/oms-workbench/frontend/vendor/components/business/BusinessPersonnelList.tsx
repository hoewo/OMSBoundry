import { useState } from 'react'
import {
  Avatar,
  Button,
  Space,
  message,
  Drawer,
  Form,
  Empty,
  Tag,
  Alert,
} from 'antd'
import { DeleteOutlined, UsergroupAddOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { projectMemberService } from '@/services/projectMember'
import { permissionService } from '@/services/permission'
import { useAvailableProjectUsers } from '@/hooks/useAvailableProjectUsers'
import { SectionTitle } from '@/components/shared/SectionTitle'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import PersonnelSelect from '@/components/shared/PersonnelSelect'
import PersonnelName from '@/components/shared/PersonnelName'
import { PROJECT_PERMISSION_KEYS } from '@/hooks/useProjectPermissions'
import type { User, ProjectMember } from '@/types'
import { USER_ROLE_LABELS, sortUserRoles } from '@/utils/userRoles'
import '../project/ProjectWorkspace.css'
import './BusinessPersonnelList.css'

const getNameInitial = (name?: string) => {
  const trimmed = name?.trim()
  return trimmed ? trimmed.slice(0, 1) : '经'
}

const getUserName = (member: ProjectMember, user?: User) =>
  user?.real_name ||
  member.user?.real_name ||
  user?.username ||
  member.user?.username ||
  `用户ID: ${member.user_id}`

const getUserAccount = (member: ProjectMember, user?: User) =>
  user?.username || member.user?.username || member.user_id

const getCompanyRoleLabels = (user?: User) => {
  const roles = sortUserRoles(user?.roles || (user?.role ? [user.role] : []))
  return roles.map((role) => USER_ROLE_LABELS[role] || role)
}

interface BusinessPersonnelListProps {
  projectId: string
}

export const BusinessPersonnelList = ({
  projectId,
}: BusinessPersonnelListProps) => {
  const [addDrawerVisible, setAddDrawerVisible] = useState(false)
  const [form] = Form.useForm()
  const queryClient = useQueryClient()

  // 检查权限：是否可以配置经营参与人
  const { data: canManage } = useQuery({
    queryKey: ['canAssignBusinessMembers', projectId],
    queryFn: async () => {
      const response = await permissionService.getEffectivePermissions(projectId)
      return response.permissions[PROJECT_PERMISSION_KEYS.businessMemberAssign] === true
    },
    enabled: !!projectId,
  })

  // 获取项目成员（经营参与人）- 所有用户都可以查看
  const { data: projectMembers, isError: membersLoadFailed } = useQuery<ProjectMember[]>({
    queryKey: ['projectMembers', projectId],
    queryFn: () => projectMemberService.list(projectId),
    enabled: !!projectId,
  })

  // 项目成员选择使用独立的候选人员接口，不依赖“人员目录管理”权限。
  const {
    data: usersData,
    isLoading: usersLoading,
    isError: usersLoadFailed,
    refetch: refetchUsers,
  } = useAvailableProjectUsers(projectId)

  // 添加参与人（只允许添加business_personnel角色）
  const addMutation = useMutation({
    mutationFn: async ({ userId }: { userId: string }) => {
      // Check if member already exists
      const existingMember = projectMembers?.find(
        (m) => m.user_id === userId && m.role === 'business_personnel' && m.is_active
      )
      if (existingMember) {
          throw new Error('该人员已在参与人列表中')
        }

      // Create new project member with business_personnel role only
      await projectMemberService.create(projectId, {
        user_id: userId,
        role: 'business_personnel', // 固定使用business_personnel角色
        join_date: new Date().toISOString().split('T')[0],
        is_active: true,
      })
    },
    onSuccess: () => {
      message.success('添加参与人成功')
      queryClient.invalidateQueries({
        queryKey: ['projectMembers', projectId],
      })
      setAddDrawerVisible(false)
      form.resetFields()
    },
    onError: (error: any) => {
      message.error(error.message || '添加失败')
    },
  })

  // 删除参与人
  const removeMutation = useMutation({
    mutationFn: async (memberId: string) => {
      await projectMemberService.remove(memberId)
    },
    onSuccess: () => {
      message.success('已移出经营参与人，历史保留')
      queryClient.invalidateQueries({
        queryKey: ['projectMembers', projectId],
      })
    },
    onError: (error: any) => {
      message.error(error.message || '删除失败')
    },
  })

  const handleAdd = async () => {
    try {
      const values = await form.validateFields()
      if (!values.user_id) {
        message.warning('请选择要添加的人员')
        return
      }
      // 不再需要选择角色，固定使用business_personnel
      addMutation.mutate({
        userId: values.user_id,
      })
    } catch (error) {
      // 表单验证错误
    }
  }

  const handleRemove = (memberId: string) => {
    removeMutation.mutate(memberId)
  }

  // 只获取经营参与人信息（不包含负责人，负责人应在基本信息中显示）
  const businessPersonnel = projectMembers?.filter(
    (m) => m.role === 'business_personnel' && m.is_active
  ) || []
  const users = (usersData || []) as User[]

  // 获取可添加的用户列表（排除已在列表中的人员）
  const existingUserIds = businessPersonnel.map((m) => m.user_id)
  const availableUsers = users.filter((user) => !existingUserIds.includes(user.id))
  const usersById = new Map(users.map((user) => [user.id, user]))

  return (
    <>
      <section className="project-workspace__card business-personnel-panel">
        <header className="business-personnel-panel__header">
          <div>
            <SectionTitle icon={<UsergroupAddOutlined />}>经营参与人</SectionTitle>
            <p className="business-personnel-panel__subtitle">
              维护本项目经营参与人；人员账号、部门和公司岗位暂未由名册提供。
            </p>
          </div>
          {canManage === true && (
            <CreateActionButton
              size="small"
              compact
              onClick={() => setAddDrawerVisible(true)}
            >
              添加参与人
            </CreateActionButton>
          )}
        </header>

        {membersLoadFailed ? <Alert type="error" message="项目名单读取失败，请刷新重试；不表示没有成员。" /> : businessPersonnel.length === 0 ? (
          <div className="business-personnel-list__empty">
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="当前还没有经营参与人"
            />
          </div>
        ) : (
          <div className="business-personnel-list">
            {businessPersonnel.map((member) => {
              const user = usersById.get(member.user_id)
              const name = getUserName(member, user)
              const account = user?.username || member.user?.username || '登录账号资料未提供'
              const companyRoleLabels = getCompanyRoleLabels(user)
              const department = user?.department || '部门资料未提供'
              const displayUser = user || member.user

              return (
                <div className="business-personnel-list__item" key={member.id}>
                  <div className="business-personnel-list__identity">
                    <Avatar className="business-personnel-list__avatar">
                      {getNameInitial(name)}
                    </Avatar>
                    <div className="business-personnel-list__identity-text">
                      <div className="business-personnel-list__name">
                        <PersonnelName user={displayUser} name={name} />
                      </div>
                      <div className="business-personnel-list__meta">
                        {account}
                        <span className="business-personnel-list__dot" />
                        {department}
                      </div>
                    </div>
                  </div>

                  <div className="business-personnel-list__role-block">
                    <span className="business-personnel-list__role-label">
                      项目角色
                    </span>
                    <Tag className="business-personnel-list__project-role">
                      经营参与人
                    </Tag>
                  </div>
                  <div className="business-personnel-list__role-block">
                    <span className="business-personnel-list__role-label">
                      公司角色
                    </span>
                    <div className="business-personnel-list__company-roles">
                      {companyRoleLabels.length > 0 ? (
                        companyRoleLabels.map((roleLabel) => (
                          <Tag
                            key={roleLabel}
                            className="business-personnel-list__company-role"
                          >
                            {roleLabel}
                          </Tag>
                        ))
                      ) : (
                        <span className="business-personnel-list__muted">
                          公司岗位资料未提供
                        </span>
                      )}
                    </div>
                  </div>

                  {canManage === true && (
                    <div className="business-personnel-list__actions">
                      <DeleteConfirmButton
                        title="确认移出经营参与人"
                        description="仅移出当前项目名单，保留分工历史；不删除人员或账号。"
                        details={[
                          {
                            label: '人员',
                            value: <PersonnelName user={displayUser} name={name} />,
                          },
                          {
                            label: '公司角色',
                            value: companyRoleLabels.join('、') || '公司岗位资料未提供',
                          },
                        ]}
                        buttonProps={{
                          type: 'text',
                          icon: <DeleteOutlined />,
                        }}
                        loading={removeMutation.isPending}
                        buttonText="移出"
                        confirmText="确认移出"
                        onConfirm={() => handleRemove(member.id)}
                      />
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </section>

      <Drawer
        title="添加经营参与人"
        open={addDrawerVisible}
        onClose={() => {
          setAddDrawerVisible(false)
          form.resetFields()
        }}
        width={520}
        destroyOnHidden
        footer={
          <Space style={{ width: '100%', justifyContent: 'flex-end' }}>
            <Button
              onClick={() => {
                setAddDrawerVisible(false)
                form.resetFields()
              }}
            >
              取消
            </Button>
            <Button type="primary" onClick={handleAdd} loading={addMutation.isPending} disabled={!canManage || usersLoading || usersLoadFailed}>
              添加
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
        <Form form={form} layout="vertical">
          <Form.Item
            label="人员"
            name="user_id"
            rules={[{ required: true, message: '请选择要添加的人员' }]}
            tooltip="只能添加经营参与人，经营负责人请在项目基本信息中配置"
          >
            <PersonnelSelect
              placeholder="请选择要添加的人员"
              users={availableUsers}
              loading={usersLoading}
              disabled={usersLoadFailed}
            />
          </Form.Item>
        </Form>
      </Drawer>
    </>
  )
}
