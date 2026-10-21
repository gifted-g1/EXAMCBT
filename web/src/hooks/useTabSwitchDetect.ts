import { useEffect, useRef } from 'react'

export type ClientEvent =
  | 'TAB_SWITCH'
  | 'WINDOW_BLUR'
  | 'COPY_PASTE'
  | 'CONTEXT_MENU'
  | 'DEVTOOLS_SHORTCUT'
  | 'PRINT_SCREEN'
  | 'MULTIPLE_DISPLAYS'
  | 'NETWORK_OFFLINE'

type Reporter = (type: ClientEvent, meta?: Record<string, string | number | boolean>) => void

export interface DetectOptions {
  /** Tab switches, leaving the window, extra displays. */
  tabs?: boolean
  /** Block + report clipboard, right-click, devtools keys, PrintScreen. */
  clipboard?: boolean
}

const devtoolsCombo = (e: KeyboardEvent) => {
  const k = e.key.toUpperCase()
  if (e.key === 'F12') return true
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && ['I', 'J', 'C'].includes(k)) return true
  if ((e.ctrlKey || e.metaKey) && k === 'U') return true
  return false
}

/**
 * Detects the browser-observable integrity signals while `active`, limited to
 * the groups the exam's proctoring settings enable, and blocks clipboard,
 * context menu and devtools shortcuts when clipboard blocking is on. These are
 * deterrents and signals — a determined student can bypass any of them, which
 * is why the server treats them as evidence for a human, never as a verdict.
 */
export function useTabSwitchDetect(active: boolean, report: Reporter, { tabs = true, clipboard = true }: DetectOptions = {}) {
  const reportRef = useRef(report)
  reportRef.current = report

  useEffect(() => {
    if (!active) return
    const r = (t: ClientEvent, m?: Record<string, string | number | boolean>) => reportRef.current(t, m)
    const cleanups: (() => void)[] = []
    const on = <K extends keyof WindowEventMap>(target: Window | Document, type: K | string, fn: (e: never) => void, capture = false) => {
      target.addEventListener(type, fn as EventListener, capture)
      cleanups.push(() => target.removeEventListener(type, fn as EventListener, capture))
    }

    if (tabs) {
      on(document, 'visibilitychange', () => {
        if (document.visibilityState === 'hidden') r('TAB_SWITCH')
      })
      on(window, 'blur', () => {
        // A blur that also hides the page is reported as TAB_SWITCH instead.
        setTimeout(() => {
          if (document.visibilityState === 'visible' && !document.hasFocus()) r('WINDOW_BLUR')
        }, 50)
      })
      const extended = (screen as Screen & { isExtended?: boolean }).isExtended
      if (extended) r('MULTIPLE_DISPLAYS')
    }

    if (clipboard) {
      const onClipboard = (e: ClipboardEvent) => {
        e.preventDefault()
        r('COPY_PASTE', { action: e.type })
      }
      on(document, 'copy', onClipboard)
      on(document, 'cut', onClipboard)
      on(document, 'paste', onClipboard)
      on(document, 'contextmenu', (e: MouseEvent) => {
        e.preventDefault()
        r('CONTEXT_MENU')
      })
      on(
        window,
        'keydown',
        (e: KeyboardEvent) => {
          if (devtoolsCombo(e)) {
            e.preventDefault()
            r('DEVTOOLS_SHORTCUT', { key: e.key })
          }
        },
        true,
      )
      on(
        window,
        'keyup',
        (e: KeyboardEvent) => {
          if (e.key === 'PrintScreen') r('PRINT_SCREEN')
        },
        true,
      )
    }

    on(window, 'offline', () => r('NETWORK_OFFLINE'))

    return () => cleanups.forEach((c) => c())
  }, [active, tabs, clipboard])
}
