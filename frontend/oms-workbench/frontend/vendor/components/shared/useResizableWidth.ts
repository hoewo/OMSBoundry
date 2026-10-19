import { useCallback, useEffect, useRef, useState } from 'react'
import type {
  KeyboardEvent as ReactKeyboardEvent,
  MouseEvent as ReactMouseEvent,
  RefObject,
} from 'react'

interface UseResizableWidthOptions {
  defaultWidth: number
  minWidth: number
  maxWidth?: number
  minRemainingWidth?: number
  gutter?: number
  enabled?: boolean
  invertDelta?: boolean
  resizeStep?: number
  containerRef?: RefObject<HTMLElement | null>
  getMaxWidth?: () => number
}

interface DragState {
  startX: number
  startWidth: number
}

export const useResizableWidth = ({
  defaultWidth,
  minWidth,
  maxWidth,
  minRemainingWidth,
  gutter = 18,
  enabled = true,
  invertDelta = false,
  resizeStep = 40,
  containerRef,
  getMaxWidth,
}: UseResizableWidthOptions) => {
  const [width, setWidth] = useState(defaultWidth)
  const [isResizing, setIsResizing] = useState(false)
  const dragStateRef = useRef<DragState>({
    startX: 0,
    startWidth: defaultWidth,
  })
  const defaultWidthRef = useRef(defaultWidth)

  const getEffectiveMaxWidth = useCallback(() => {
    const candidates: number[] = []

    if (typeof maxWidth === 'number' && Number.isFinite(maxWidth)) {
      candidates.push(maxWidth)
    }

    if (getMaxWidth) {
      const dynamicMaxWidth = getMaxWidth()
      if (Number.isFinite(dynamicMaxWidth)) {
        candidates.push(dynamicMaxWidth)
      }
    }

    if (containerRef?.current && typeof minRemainingWidth === 'number') {
      const containerWidth = containerRef.current.getBoundingClientRect().width
      if (Number.isFinite(containerWidth)) {
        candidates.push(containerWidth - minRemainingWidth - gutter)
      }
    }

    if (candidates.length === 0) {
      return Number.POSITIVE_INFINITY
    }

    return Math.max(minWidth, Math.min(...candidates))
  }, [containerRef, getMaxWidth, gutter, maxWidth, minRemainingWidth, minWidth])

  const clampWidth = useCallback(
    (nextWidth: number) => {
      const boundedWidth = Math.max(minWidth, nextWidth)
      const effectiveMaxWidth = getEffectiveMaxWidth()
      if (!Number.isFinite(effectiveMaxWidth)) {
        return boundedWidth
      }
      return Math.min(boundedWidth, effectiveMaxWidth)
    },
    [getEffectiveMaxWidth, minWidth]
  )

  useEffect(() => {
    if (defaultWidthRef.current === defaultWidth) {
      return
    }

    defaultWidthRef.current = defaultWidth
    setWidth(clampWidth(defaultWidth))
  }, [clampWidth, defaultWidth])

  useEffect(() => {
    if (!enabled) {
      setIsResizing(false)
      return
    }

    const syncWidth = () => {
      setWidth((currentWidth) => clampWidth(currentWidth))
    }

    syncWidth()
    window.addEventListener('resize', syncWidth)
    return () => {
      window.removeEventListener('resize', syncWidth)
    }
  }, [clampWidth, enabled])

  useEffect(() => {
    if (!enabled || !isResizing) {
      return
    }

    const overlay = document.createElement('div')
    overlay.setAttribute('data-resize-overlay', 'true')
    Object.assign(overlay.style, {
      position: 'fixed',
      inset: '0',
      zIndex: '2147483647',
      background: 'transparent',
      cursor: 'col-resize',
    })
    document.body.appendChild(overlay)

    const stopResizing = () => {
      setIsResizing(false)
    }

    const handleMouseMove = (event: MouseEvent) => {
      if (event.buttons === 0) {
        stopResizing()
        return
      }

      const dragState = dragStateRef.current
      const deltaX = invertDelta
        ? dragState.startX - event.clientX
        : event.clientX - dragState.startX
      setWidth(clampWidth(dragState.startWidth + deltaX))
    }

    const handleMouseUp = (event: MouseEvent) => {
      handleMouseMove(event)
      stopResizing()
    }

    const handleWindowBlur = () => {
      stopResizing()
    }

    overlay.addEventListener('mousemove', handleMouseMove)
    overlay.addEventListener('mouseup', handleMouseUp)
    window.addEventListener('mousemove', handleMouseMove)
    window.addEventListener('mouseup', handleMouseUp)
    window.addEventListener('blur', handleWindowBlur)
    document.body.style.userSelect = 'none'
    document.body.style.cursor = 'col-resize'

    return () => {
      overlay.removeEventListener('mousemove', handleMouseMove)
      overlay.removeEventListener('mouseup', handleMouseUp)
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseup', handleMouseUp)
      window.removeEventListener('blur', handleWindowBlur)
      overlay.remove()
      document.body.style.userSelect = ''
      document.body.style.cursor = ''
    }
  }, [clampWidth, enabled, invertDelta, isResizing])

  const handleResizeStart = useCallback(
    (event: ReactMouseEvent<HTMLElement>) => {
      if (!enabled) {
        return
      }

      event.preventDefault()
      event.stopPropagation()
      dragStateRef.current = {
        startX: event.clientX,
        startWidth: width,
      }
      setIsResizing(true)
    },
    [enabled, width]
  )

  const handleResizeKeyDown = useCallback(
    (event: ReactKeyboardEvent<HTMLElement>) => {
      if (!enabled) {
        return
      }

      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') {
        return
      }

      event.preventDefault()
      const direction = event.key === 'ArrowRight' ? 1 : -1
      const delta = (invertDelta ? -direction : direction) * resizeStep
      setWidth((currentWidth) => clampWidth(currentWidth + delta))
    },
    [clampWidth, enabled, invertDelta, resizeStep]
  )

  return {
    width,
    isResizing,
    setWidth,
    handleResizeStart,
    handleResizeKeyDown,
  }
}

export default useResizableWidth
