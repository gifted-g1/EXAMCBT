import { useState } from 'react'
import { staffApi } from '../../api/staff'
import type { Exam, Proctoring } from '../../api/types'
import { Card, ErrorText, errorMessage } from '../ui'

const SWITCHES: { key: keyof Proctoring; label: string; on: string; off: string }[] = [
  {
    key: 'face_verification',
    label: 'Face verification at entry',
    on: 'Students take a selfie that is matched against their roster photo before starting.',
    off: 'Students enter with their matric number and consent only. No photo is taken.',
  },
  {
    key: 'camera_monitoring',
    label: 'Webcam monitoring',
    on: 'The webcam stays on; a frame every 5 s is checked for absent faces, extra people and phones.',
    off: 'No camera during the exam and no video checks.',
  },
  {
    key: 'audio_monitoring',
    label: 'Microphone monitoring',
    on: 'Short audio samples are checked for talking.',
    off: 'The microphone is not used.',
  },
  {
    key: 'fullscreen_required',
    label: 'Fullscreen required',
    on: 'The exam only shows in fullscreen; leaving it is recorded.',
    off: 'Students may use a normal window.',
  },
  {
    key: 'tab_switch_detection',
    label: 'Tab and window switch detection',
    on: 'Switching tabs or apps and connecting a second display are recorded.',
    off: 'Switching tabs or windows is not recorded.',
  },
  {
    key: 'clipboard_blocking',
    label: 'Block copy/paste and shortcuts',
    on: 'Copy, paste, right-click and developer-tools keys are blocked and recorded.',
    off: 'Clipboard and shortcuts work normally.',
  },
]

const PRESETS: { name: string; hint: string; value: Proctoring }[] = [
  {
    name: 'Full proctoring',
    hint: 'Everything on',
    value: { face_verification: true, camera_monitoring: true, audio_monitoring: true, fullscreen_required: true, tab_switch_detection: true, clipboard_blocking: true },
  },
  {
    name: 'Locked browser, no camera',
    hint: 'Closed-book without video (works over plain http)',
    value: { face_verification: false, camera_monitoring: false, audio_monitoring: false, fullscreen_required: true, tab_switch_detection: true, clipboard_blocking: true },
  },
  {
    name: 'Open book',
    hint: 'All monitoring off',
    value: { face_verification: false, camera_monitoring: false, audio_monitoring: false, fullscreen_required: false, tab_switch_detection: false, clipboard_blocking: false },
  },
]

/** Per-exam on/off switches for every proctoring feature. Changes are saved
 * immediately and reach students who are already writing within ~30 s. */
export function ProctoringSettings({ exam, canManage, onChanged }: { exam: Exam; canManage: boolean; onChanged: (e: Exam) => void }) {
  const [p, setP] = useState<Proctoring>(exam.proctoring)
  const [saving, setSaving] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const editable = canManage && exam.status !== 'CLOSED'

  const save = async (patch: Partial<Proctoring>, label: string) => {
    const prev = p
    setP({ ...p, ...patch }) // optimistic
    setSaving(label)
    setError(null)
    try {
      const updated = await staffApi.setProctoring(exam.id, patch)
      setP(updated.proctoring)
      onChanged(updated)
    } catch (e) {
      setP(prev)
      setError(errorMessage(e))
    } finally {
      setSaving(null)
    }
  }

  const needsHttps = p.face_verification || p.camera_monitoring || p.audio_monitoring
  const activePreset = PRESETS.find((pr) => JSON.stringify(pr.value) === JSON.stringify(p))?.name

  return (
    <div className="space-y-4">
      {editable && (
        <Card>
          <p className="text-sm font-semibold text-slate-800">Quick presets</p>
          <div className="mt-3 grid gap-2 sm:grid-cols-3">
            {PRESETS.map((pr) => (
              <button
                key={pr.name}
                type="button"
                disabled={!!saving}
                onClick={() => save(pr.value, pr.name)}
                className={`rounded-lg p-3 text-left ring-1 transition ${
                  activePreset === pr.name ? 'bg-brand-50 ring-brand-500' : 'ring-slate-200 hover:bg-slate-50'
                }`}
              >
                <span className="block text-sm font-semibold">{pr.name}</span>
                <span className="block text-xs text-slate-500">{pr.hint}</span>
              </button>
            ))}
          </div>
        </Card>
      )}

      <Card className="divide-y divide-slate-100 !p-0">
        {SWITCHES.map((s) => {
          const value = p[s.key]
          return (
            <div key={s.key} className="flex items-start justify-between gap-4 px-5 py-4">
              <div>
                <p className="font-medium text-slate-900" id={`lbl-${s.key}`}>
                  {s.label}
                </p>
                <p className="mt-0.5 text-sm text-slate-500">{value ? s.on : s.off}</p>
              </div>
              <button
                type="button"
                role="switch"
                aria-checked={value}
                aria-labelledby={`lbl-${s.key}`}
                disabled={!editable || !!saving}
                onClick={() => save({ [s.key]: !value }, s.label)}
                className={`relative mt-1 h-6 w-11 shrink-0 rounded-full transition focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 disabled:opacity-50 ${
                  value ? 'bg-brand-600' : 'bg-slate-300'
                }`}
              >
                <span
                  aria-hidden
                  className={`absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-all ${value ? 'left-[22px]' : 'left-0.5'}`}
                />
              </button>
            </div>
          )
        })}
      </Card>

      <ErrorText>{error}</ErrorText>
      <div className="space-y-1 text-sm text-slate-500">
        {saving && <p>Saving {saving}…</p>}
        {!editable && <p>{exam.status === 'CLOSED' ? 'The exam is closed; settings are read-only.' : 'Only the exam owner or an admin can change these settings.'}</p>}
        {exam.status === 'ACTIVE' && editable && <p>Students already writing pick up changes within about 30 seconds.</p>}
        <p>
          {needsHttps
            ? 'Camera, microphone or face checks are on: students must open the exam over https://.'
            : 'No camera or microphone is used, so the exam also works over plain http://.'}
        </p>
      </div>
    </div>
  )
}
