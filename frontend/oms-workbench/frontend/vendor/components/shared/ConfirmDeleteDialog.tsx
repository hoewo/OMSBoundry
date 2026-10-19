import { useEffect, useState, type ReactNode } from 'react'
import { Button } from 'antd'
import type { ButtonProps } from 'antd'
import { CloseOutlined, DeleteOutlined, ExclamationCircleOutlined } from '@ant-design/icons'
import './ConfirmDeleteDialog.css'

export type ConfirmDeleteDetail = {
  label: ReactNode
  value: ReactNode
}

export interface ConfirmDeleteDialogProps {
  open: boolean
  title?: ReactNode
  description?: ReactNode
  details?: ConfirmDeleteDetail[]
  warning?: ReactNode
  confirmText?: ReactNode
  cancelText?: ReactNode
  loading?: boolean
  confirmDisabled?: boolean
  maskClosable?: boolean
  closable?: boolean
  keyboard?: boolean
  children?: ReactNode
  onConfirm: () => void
  onCancel: () => void
}

export const ConfirmDeleteDialog = ({
  open,
  title = '确认删除',
  description = '删除后无法恢复，请确认该记录不再需要。',
  details = [],
  warning,
  confirmText = '确认删除',
  cancelText = '取消',
  loading = false,
  confirmDisabled = false,
  maskClosable = true,
  closable = true,
  keyboard = false,
  children,
  onConfirm,
  onCancel,
}: ConfirmDeleteDialogProps) => {
  useEffect(() => {
    if (!open || !keyboard || loading) return

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onCancel()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [keyboard, loading, onCancel, open])

  if (!open) return null

  return (
    <div
      className="confirm-delete"
      role="presentation"
      onClick={(event) => {
        if (event.target === event.currentTarget && maskClosable && !loading) {
          onCancel()
        }
      }}
    >
      <section
        className="confirm-delete__dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-delete-title"
        onClick={(event) => event.stopPropagation()}
      >
        {closable && (
          <button
            type="button"
            className="confirm-delete__close"
            onClick={onCancel}
            disabled={loading}
            aria-label="关闭"
          >
            <CloseOutlined />
          </button>
        )}

        <header className="confirm-delete__header">
          <span className="confirm-delete__icon">
            <ExclamationCircleOutlined />
          </span>
          <div>
            <h3 id="confirm-delete-title">{title}</h3>
            {description && <p>{description}</p>}
          </div>
        </header>

        {details.length > 0 && (
          <div className="confirm-delete__info">
            {details.map((item, index) => (
              <div key={index}>
                <span>{item.label}</span>
                <strong>{item.value}</strong>
              </div>
            ))}
          </div>
        )}

        {children && <div className="confirm-delete__content">{children}</div>}

        {warning && <div className="confirm-delete__warning">{warning}</div>}

        <footer className="confirm-delete__footer">
          <Button onClick={onCancel} disabled={loading}>
            {cancelText}
          </Button>
          <Button
            danger
            type="primary"
            onClick={onConfirm}
            loading={loading}
            disabled={confirmDisabled}
          >
            {confirmText}
          </Button>
        </footer>
      </section>
    </div>
  )
}

interface DeleteConfirmButtonProps
  extends Omit<ConfirmDeleteDialogProps, 'open' | 'onCancel'> {
  buttonProps?: ButtonProps
  buttonText?: ReactNode
}

export const DeleteConfirmButton = ({
  buttonProps,
  buttonText = '删除',
  loading,
  onConfirm,
  ...dialogProps
}: DeleteConfirmButtonProps) => {
  const [open, setOpen] = useState(false)

  return (
    <>
      <Button
        type="text"
        danger
        icon={<DeleteOutlined />}
        {...buttonProps}
        onClick={(event) => {
          buttonProps?.onClick?.(event)
          if (!event.defaultPrevented) {
            setOpen(true)
          }
        }}
      >
        {buttonText}
      </Button>
      <ConfirmDeleteDialog
        {...dialogProps}
        open={open}
        loading={loading}
        onCancel={() => setOpen(false)}
        onConfirm={() => {
          onConfirm()
          if (!loading) {
            setOpen(false)
          }
        }}
      />
    </>
  )
}
