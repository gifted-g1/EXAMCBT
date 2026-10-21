import { renderHook } from '@testing-library/react'
import { useTabSwitchDetect } from './useTabSwitchDetect'

describe('useTabSwitchDetect', () => {
  it('reports tab switches, blocks paste and devtools keys, and cleans up', () => {
    const report = vi.fn()
    const { unmount } = renderHook(() => useTabSwitchDetect(true, report))

    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(report).toHaveBeenCalledWith('TAB_SWITCH', undefined)

    const paste = new Event('paste', { cancelable: true })
    document.dispatchEvent(paste)
    expect(paste.defaultPrevented).toBe(true)
    expect(report).toHaveBeenCalledWith('COPY_PASTE', { action: 'paste' })

    const f12 = new KeyboardEvent('keydown', { key: 'F12', cancelable: true })
    window.dispatchEvent(f12)
    expect(f12.defaultPrevented).toBe(true)

    unmount()
    report.mockClear()
    document.dispatchEvent(new Event('visibilitychange'))
    expect(report).not.toHaveBeenCalled()
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
  })

  it('does nothing when inactive', () => {
    const report = vi.fn()
    renderHook(() => useTabSwitchDetect(false, report))
    document.dispatchEvent(new Event('copy', { cancelable: true }))
    expect(report).not.toHaveBeenCalled()
  })
})
