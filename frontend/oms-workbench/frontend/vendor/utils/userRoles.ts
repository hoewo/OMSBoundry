const ROLE_PRIORITY: Record<string, number> = {
  admin: 0,
  general_manager: 1,
  finance: 2,
  project_manager: 99,
  business_manager: 3,
  production_manager: 4,
  office_director: 5,
  member: 6,
}

export const USER_ROLE_LABELS: Record<string, string> = {
  admin: 'OA 管理员',
  general_manager: '总经理',
  finance: '财务室主任',
  project_manager: '已停用角色',
  business_manager: '经营经理',
  production_manager: '生产经理',
  office_director: '办公室主任',
  member: '普通职员',
}

export const sortUserRoles = <TRole extends string>(roles?: TRole[]): TRole[] => {
  if (!roles || roles.length === 0) {
    return []
  }

  return [...roles].sort((left, right) => {
    const leftPriority = ROLE_PRIORITY[left] ?? Number.MAX_SAFE_INTEGER
    const rightPriority = ROLE_PRIORITY[right] ?? Number.MAX_SAFE_INTEGER
    if (leftPriority !== rightPriority) {
      return leftPriority - rightPriority
    }
    return left.localeCompare(right, 'zh-CN')
  })
}

export const isAdminRole = (role: string): role is 'admin' => role === 'admin'

export const isDeprecatedRole = (role: string): boolean => role === 'project_manager'
