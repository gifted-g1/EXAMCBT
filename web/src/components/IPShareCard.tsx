import type { AccessCard } from '../api/types'
import { Button } from './ui'

/** The card an invigilator projects: IP to type, PIN, and a QR code. */
export function IPShareCard({
  card,
  canManage,
  onRotatePin,
  onClearPin,
}: {
  card: AccessCard
  canManage: boolean
  onRotatePin: () => void
  onClearPin: () => void
}) {
  const shownUrl = card.url.replace(/^https?:\/\//, '')
  return (
    <div className="grid gap-6 rounded-2xl bg-slate-900 p-6 text-white sm:grid-cols-[1fr_auto]">
      <div>
        <p className="text-sm uppercase tracking-widest text-slate-400">Type this in your browser</p>
        <p className="mt-2 break-all font-mono text-4xl font-bold sm:text-5xl" data-testid="access-ip">
          {shownUrl}
        </p>
        <p className="mt-1 text-xs text-slate-400">
          Only devices on {card.lan_cidr} can open it{card.url.startsWith('https') ? ' · secure connection' : ''}
        </p>

        <div className="mt-6">
          <p className="text-sm uppercase tracking-widest text-slate-400">Exam PIN</p>
          {card.pin ? (
            <p className="mt-1 font-mono text-5xl font-bold tracking-[0.3em] text-amber-300" data-testid="access-pin">
              {card.pin}
            </p>
          ) : (
            <p className="mt-1 text-slate-300">No PIN — anyone on the hall network can open the exam.</p>
          )}
        </div>

        {canManage && card.status !== 'CLOSED' && (
          <div className="mt-6 flex flex-wrap gap-2">
            <Button variant="secondary" onClick={onRotatePin}>
              {card.pin ? 'Rotate PIN' : 'Require a PIN'}
            </Button>
            {card.pin && (
              <Button variant="ghost" className="text-slate-300 hover:bg-slate-800" onClick={onClearPin}>
                Remove PIN
              </Button>
            )}
          </div>
        )}
      </div>
      <figure className="justify-self-center rounded-xl bg-white p-3">
        <img
          src={`data:image/png;base64,${card.qr_png_base64}`}
          alt={`QR code for ${card.qr_url}`}
          className="h-48 w-48 [image-rendering:pixelated]"
        />
        <figcaption className="mt-1 text-center text-xs text-slate-500">Scan to open</figcaption>
      </figure>
    </div>
  )
}
