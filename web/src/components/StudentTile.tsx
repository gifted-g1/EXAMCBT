import { useEffect, useState } from 'react'
import { authedImage } from '../api/staff'
import type { SessionRow } from '../api/types'
import { StatusBadge } from './ui'

const STALE_MS = 90_000

/** One student in the live grid: latest webcam frame, integrity, alerts. */
export function StudentTile({
  row,
  onSelect,
  now,
  cameraOn = true,
}: {
  row: SessionRow
  onSelect: (id: number) => void
  now: number
  cameraOn?: boolean
}) {
  const [img, setImg] = useState<string | null>(null)
  const live = row.status === 'ACTIVE' && cameraOn

  useEffect(() => {
    if (!live) return
    let cancelled = false
    let current: string | null = null
    const load = async () => {
      const url = await authedImage(`/api/staff/sessions/${row.id}/image/latest`)
      if (cancelled) {
        if (url) URL.revokeObjectURL(url)
        return
      }
      if (url) {
        if (current) URL.revokeObjectURL(current)
        current = url
        setImg(url)
      }
    }
    void load()
    const id = setInterval(load, 15000)
    return () => {
      cancelled = true
      clearInterval(id)
      if (current) URL.revokeObjectURL(current)
    }
  }, [row.id, live])

  const stale = row.status === 'ACTIVE' && (!row.last_seen_at || now - new Date(row.last_seen_at).getTime() > STALE_MS)
  const integrity = Math.round(row.integrity_score)
  const bar = integrity >= 85 ? 'bg-emerald-500' : integrity >= 60 ? 'bg-amber-500' : 'bg-rose-500'

  return (
    <button
      type="button"
      onClick={() => onSelect(row.id)}
      className={`group flex w-full flex-col overflow-hidden rounded-xl bg-white text-left shadow-sm ring-1 transition hover:shadow-md ${
        row.high_count > 0 ? 'ring-rose-400' : 'ring-slate-200'
      }`}
    >
      <div className="relative aspect-video bg-slate-800">
        {img ? (
          <img src={img} alt={`Latest camera frame of ${row.full_name}`} className="h-full w-full -scale-x-100 object-cover" />
        ) : (
          <div className="grid h-full place-items-center text-xs text-slate-400">{!cameraOn ? 'Webcam monitoring off' : live ? 'Waiting for camera…' : 'Offline'}</div>
        )}
        {stale && <span className="absolute left-2 top-2 rounded bg-amber-500 px-1.5 py-0.5 text-[10px] font-bold text-white">NO HEARTBEAT</span>}
        {row.violations_count > 0 && (
          <span className="absolute right-2 top-2 rounded-full bg-rose-600 px-2 py-0.5 text-xs font-bold text-white">
            {row.violations_count}
          </span>
        )}
      </div>
      <div className="space-y-2 p-3">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="truncate font-semibold text-slate-900">{row.full_name}</p>
            <p className="font-mono text-xs text-slate-500">{row.matric_no}</p>
          </div>
          <StatusBadge status={row.status} />
        </div>
        <div>
          <div className="flex justify-between text-xs text-slate-500">
            <span>Integrity</span>
            <span className="font-semibold text-slate-700">{integrity}</span>
          </div>
          <div className="mt-1 h-1.5 rounded-full bg-slate-100">
            <div className={`h-1.5 rounded-full ${bar}`} style={{ width: `${integrity}%` }} />
          </div>
        </div>
        <p className="text-xs text-slate-500">
          {row.answered_count} answered · {row.client_ip}
        </p>
      </div>
    </button>
  )
}
