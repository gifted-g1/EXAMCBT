import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react'

/** Muted, mirrored live preview of a MediaStream. The parent captures frames
 * through the forwarded <video> ref. */
export const WebcamFeed = forwardRef<HTMLVideoElement, { stream: MediaStream | null; className?: string; label?: string }>(
  function WebcamFeed({ stream, className = '', label = 'Your camera' }, ref) {
    const video = useRef<HTMLVideoElement>(null)
    useImperativeHandle(ref, () => video.current as HTMLVideoElement)

    useEffect(() => {
      const el = video.current
      if (!el) return
      el.srcObject = stream
      if (stream) void el.play().catch(() => undefined)
      return () => {
        el.srcObject = null
      }
    }, [stream])

    return (
      <div className={`relative overflow-hidden rounded-xl bg-slate-900 ${className}`}>
        <video ref={video} aria-label={label} muted playsInline className="h-full w-full -scale-x-100 object-cover" />
        {!stream && (
          <div className="absolute inset-0 grid place-items-center text-xs text-slate-400">Camera off</div>
        )}
      </div>
    )
  },
)
