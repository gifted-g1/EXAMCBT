import { useEffect, useRef, useState } from 'react'
import { staffApi } from '../api/staff'
import type { HubEvent } from '../api/types'

export type SocketStatus = 'connecting' | 'open' | 'closed'

/**
 * Live proctor feed for one exam over a native WebSocket (the Go hub speaks
 * plain WebSocket, not the Socket.IO protocol). Each connection first trades
 * the access token for a one-time 30 s ticket, then reconnects with capped
 * exponential backoff. Everything is torn down on unmount or exam change.
 */
export function useSocket(examId: number | null, onEvent: (e: HubEvent) => void): SocketStatus {
  const [status, setStatus] = useState<SocketStatus>('closed')
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  useEffect(() => {
    if (!examId) return
    let ws: WebSocket | null = null
    let timer: ReturnType<typeof setTimeout> | null = null
    let attempt = 0
    let disposed = false

    const connect = async () => {
      if (disposed) return
      setStatus('connecting')
      try {
        const { ticket } = await staffApi.wsTicket(examId)
        if (disposed) return
        const proto = location.protocol === 'https:' ? 'wss' : 'ws'
        ws = new WebSocket(`${proto}://${location.host}/api/staff/ws?ticket=${ticket}`)
        ws.onopen = () => {
          attempt = 0
          setStatus('open')
        }
        ws.onmessage = (m) => {
          try {
            onEventRef.current(JSON.parse(m.data as string) as HubEvent)
          } catch {
            /* ignore malformed frames */
          }
        }
        ws.onclose = () => {
          ws = null
          if (!disposed) {
            setStatus('closed')
            retry()
          }
        }
      } catch {
        retry()
      }
    }
    const retry = () => {
      if (disposed || timer) return
      const delay = Math.min(30000, 1000 * 2 ** attempt++)
      timer = setTimeout(() => {
        timer = null
        void connect()
      }, delay)
    }

    void connect()
    return () => {
      disposed = true
      if (timer) clearTimeout(timer)
      ws?.close()
      setStatus('closed')
    }
  }, [examId])

  return status
}
