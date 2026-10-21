import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * Tracks fullscreen and reports exits while `active`. Browsers only allow
 * entering fullscreen from a user gesture, so `enter` must be called from a
 * click handler; the exam UI blocks itself behind a button until it is.
 */
export function useFullscreenLock(active: boolean, onExit: () => void) {
  const [isFullscreen, setIsFullscreen] = useState<boolean>(() => typeof document !== 'undefined' && !!document.fullscreenElement)
  const onExitRef = useRef(onExit)
  onExitRef.current = onExit

  useEffect(() => {
    const handler = () => {
      const fs = !!document.fullscreenElement
      setIsFullscreen(fs)
      if (!fs && active) onExitRef.current()
    }
    document.addEventListener('fullscreenchange', handler)
    return () => document.removeEventListener('fullscreenchange', handler)
  }, [active])

  const enter = useCallback(async () => {
    try {
      await document.documentElement.requestFullscreen({ navigationUI: 'hide' })
      return true
    } catch {
      return false
    }
  }, [])

  const exit = useCallback(async () => {
    if (document.fullscreenElement) await document.exitFullscreen().catch(() => undefined)
  }, [])

  return { isFullscreen, enter, exit, supported: typeof document !== 'undefined' && !!document.fullscreenEnabled }
}
