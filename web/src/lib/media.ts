/** Webcam / microphone helpers for the exam page. */

export class InsecureContextError extends Error {
  constructor() {
    super('Camera access needs HTTPS (or a browser flag on lab PCs).')
    this.name = 'InsecureContextError'
  }
}

/** getUserMedia only exists in secure contexts (HTTPS or localhost). */
export function mediaAvailable(): boolean {
  return typeof window !== 'undefined' && window.isSecureContext && !!navigator.mediaDevices?.getUserMedia
}

export async function openCamera(withAudio: boolean): Promise<MediaStream> {
  return openMedia(true, withAudio)
}

/** Opens camera and/or microphone as the exam's proctoring settings require. */
export async function openMedia(video: boolean, audio: boolean): Promise<MediaStream> {
  if (!mediaAvailable()) throw new InsecureContextError()
  return navigator.mediaDevices.getUserMedia({
    video: video ? { width: { ideal: 640 }, height: { ideal: 480 }, facingMode: 'user' } : false,
    audio: audio ? { channelCount: 1, echoCancellation: false, noiseSuppression: false } : false,
  })
}

export function stopStream(stream: MediaStream | null | undefined): void {
  stream?.getTracks().forEach((t) => t.stop())
}

/** Grab the current video frame as a JPEG (downscaled to maxWidth). */
export function captureFrame(video: HTMLVideoElement, maxWidth = 640, quality = 0.8): Promise<Blob | null> {
  const w = video.videoWidth
  const h = video.videoHeight
  if (!w || !h) return Promise.resolve(null)
  const scale = Math.min(1, maxWidth / w)
  const canvas = document.createElement('canvas')
  canvas.width = Math.round(w * scale)
  canvas.height = Math.round(h * scale)
  const ctx = canvas.getContext('2d')
  if (!ctx) return Promise.resolve(null)
  ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
  return new Promise((resolve) => canvas.toBlob((b) => resolve(b), 'image/jpeg', quality))
}

/** Convert float samples to 16-bit little-endian PCM. */
export function floatToPCM16(samples: Float32Array): ArrayBuffer {
  const buf = new ArrayBuffer(samples.length * 2)
  const view = new DataView(buf)
  for (let i = 0; i < samples.length; i++) {
    const s = Math.max(-1, Math.min(1, samples[i]))
    view.setInt16(i * 2, s < 0 ? s * 0x8000 : s * 0x7fff, true)
  }
  return buf
}

/** Linear resample to 16 kHz. */
export function resampleTo16k(input: Float32Array, fromRate: number): Float32Array {
  if (fromRate === 16000) return input
  const ratio = fromRate / 16000
  const out = new Float32Array(Math.floor(input.length / ratio))
  for (let i = 0; i < out.length; i++) {
    const pos = i * ratio
    const i0 = Math.floor(pos)
    const i1 = Math.min(i0 + 1, input.length - 1)
    out[i] = input[i0] + (input[i1] - input[i0]) * (pos - i0)
  }
  return out
}

/**
 * Records `seconds` of mono audio from a stream and returns 16 kHz PCM16.
 * Uses an AnalyserNode snapshot loop (no deprecated ScriptProcessor, no
 * AudioWorklet module file needed).
 */
export async function recordClip(stream: MediaStream, seconds = 1): Promise<ArrayBuffer | null> {
  if (!stream.getAudioTracks().length) return null
  const Ctx = window.AudioContext ?? (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
  const ctx = new Ctx()
  try {
    const src = ctx.createMediaStreamSource(stream)
    const analyser = ctx.createAnalyser()
    analyser.fftSize = 2048
    src.connect(analyser)
    const total = Math.floor(ctx.sampleRate * seconds)
    const out = new Float32Array(total)
    const chunk = new Float32Array(analyser.fftSize)
    let filled = 0
    const step = analyser.fftSize / ctx.sampleRate
    while (filled < total) {
      await new Promise((r) => setTimeout(r, step * 1000))
      analyser.getFloatTimeDomainData(chunk)
      const n = Math.min(chunk.length, total - filled)
      out.set(chunk.subarray(0, n), filled)
      filled += n
    }
    src.disconnect()
    return floatToPCM16(resampleTo16k(out, ctx.sampleRate))
  } finally {
    void ctx.close()
  }
}
