import type { ReactNode } from 'react'

interface SectionTitleProps {
  icon: ReactNode
  children: ReactNode
}

export const SectionTitle = ({ icon, children }: SectionTitleProps) => {
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        color: '#1f1f1f',
        fontWeight: 600,
      }}
    >
      <span
        style={{
          width: 18,
          height: 18,
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderRadius: '50%',
          background: '#f3f6fb',
          color: '#1677ff',
          fontSize: 11,
          flexShrink: 0,
        }}
      >
        {icon}
      </span>
      <span>{children}</span>
    </span>
  )
}
