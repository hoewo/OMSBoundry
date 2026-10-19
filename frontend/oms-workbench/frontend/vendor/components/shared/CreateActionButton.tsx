import type { ReactNode } from 'react'
import { Button } from 'antd'
import type { ButtonProps } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import './CreateActionButton.css'

interface CreateActionButtonProps extends Omit<ButtonProps, 'type' | 'danger'> {
  compact?: boolean
  children?: ReactNode
}

export const CreateActionButton = ({
  className,
  compact = false,
  icon = <PlusOutlined />,
  children,
  ...buttonProps
}: CreateActionButtonProps) => {
  const mergedClassName = [
    'create-action-button',
    compact ? 'create-action-button--compact' : '',
    children ? '' : 'create-action-button--icon-only',
    className,
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <Button
      {...buttonProps}
      type="default"
      icon={icon}
      className={mergedClassName}
    >
      {children}
    </Button>
  )
}
