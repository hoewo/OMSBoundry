import type { ReactNode } from 'react'
import type { EmploymentStatus } from '@/types'
import './PersonnelName.css'

export interface PersonnelNameUser {
  username?: string
  real_name?: string
  phone?: string
  employment_status?: EmploymentStatus
}

interface PersonnelNameProps {
  user?: PersonnelNameUser | null
  name?: ReactNode
  fallback?: ReactNode
  employmentStatus?: EmploymentStatus
  showStatus?: boolean
  className?: string
}

export const getPersonnelName = (
  user?: PersonnelNameUser | null,
  fallback = '未命名人员'
) => user?.real_name?.trim() || user?.username || user?.phone || fallback

export const isPersonnelResigned = (
  user?: PersonnelNameUser | null,
  employmentStatus?: EmploymentStatus
) => (employmentStatus || user?.employment_status) === 'resigned'

export const PersonnelName = ({
  user,
  name,
  fallback = '-',
  employmentStatus,
  showStatus = true,
  className,
}: PersonnelNameProps) => {
  const displayName = name ?? getPersonnelName(user, String(fallback))
  const resigned = isPersonnelResigned(user, employmentStatus)

  return (
    <span
      className={[
        'personnel-name',
        resigned ? 'personnel-name--resigned' : '',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <span className="personnel-name__text">{displayName || fallback}</span>
      {resigned && showStatus && (
        <span className="personnel-name__status">已离职</span>
      )}
    </span>
  )
}

export default PersonnelName
