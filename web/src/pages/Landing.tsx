import { useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { useReveal } from '../hooks/useReveal'
import { loadExamCache } from '../lib/sessionCache'

/**
 * Public landing page: the first thing anyone sees at the exam server's
 * address. Explains how Exam Shield works for students and staff, and offers
 * the two entry points (start my exam / staff sign in). Fully self-contained:
 * no external fonts, images or scripts, so it works on an offline hall LAN.
 */
export default function Landing() {
  const root = useReveal<HTMLDivElement>()
  const [resume, setResume] = useState(false)
  useEffect(() => setResume(!!loadExamCache()), [])

  return (
    <div ref={root} className="min-h-screen overflow-x-hidden bg-slate-950 text-slate-100">
      <Nav />
      {resume && (
        <div className="relative z-20 bg-amber-400 px-4 py-2 text-center text-sm font-medium text-slate-900">
          You have an exam in progress on this computer.{' '}
          <Link to="/exam" className="underline underline-offset-2">
            Continue where you left off →
          </Link>
        </div>
      )}
      <Hero />
      <TrustStrip />
      <HowItWorks />
      <PowerCut />
      <Features />
      <Modes />
      <Faq />
      <FinalCta />
      <footer className="border-t border-white/10 px-4 py-8 text-center text-xs text-slate-500">
        Exam Shield · network-locked, AI-assisted examinations · AI flags are always reviewed by a person.
      </footer>
    </div>
  )
}

// --- Layout pieces ------------------------------------------------------------

function Nav() {
  return (
    <header className="sticky top-0 z-30 border-b border-white/10 bg-slate-950/70 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-3">
        <a href="#top" className="flex items-center gap-2 font-semibold">
          <ShieldMark className="h-7 w-7" />
          Exam Shield
        </a>
        <nav aria-label="Main" className="hidden items-center gap-6 text-sm text-slate-300 md:flex">
          <a href="#how" className="hover:text-white">
            How it works
          </a>
          <a href="#resilience" className="hover:text-white">
            Power &amp; network
          </a>
          <a href="#features" className="hover:text-white">
            Features
          </a>
          <a href="#faq" className="hover:text-white">
            FAQ
          </a>
        </nav>
        <div className="flex items-center gap-2">
          <Link to="/staff" className="rounded-lg px-3 py-1.5 text-sm text-slate-300 hover:bg-white/10 hover:text-white">
            Staff sign in
          </Link>
          <Link to="/exam" className="rounded-lg bg-brand-500 px-3 py-1.5 text-sm font-semibold text-white hover:bg-brand-400">
            Start my exam
          </Link>
        </div>
      </div>
    </header>
  )
}

function Hero() {
  return (
    <section id="top" className="relative isolate px-4 pb-24 pt-16 sm:pt-24">
      {/* animated colour blobs */}
      <div aria-hidden className="pointer-events-none absolute inset-0 -z-10 overflow-hidden">
        <div className="absolute -left-32 top-0 h-96 w-96 animate-blob rounded-full bg-brand-600/30 blur-3xl" />
        <div className="absolute right-0 top-24 h-80 w-80 animate-blob rounded-full bg-fuchsia-600/20 blur-3xl [animation-delay:-6s]" />
        <div className="absolute bottom-0 left-1/3 h-72 w-72 animate-blob rounded-full bg-cyan-500/20 blur-3xl [animation-delay:-12s]" />
        <div className="absolute inset-0 bg-[linear-gradient(to_right,rgba(255,255,255,.04)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,.04)_1px,transparent_1px)] bg-[size:48px_48px] [mask-image:radial-gradient(ellipse_at_center,black_40%,transparent_75%)]" />
      </div>

      <div className="mx-auto grid max-w-6xl items-center gap-12 lg:grid-cols-2">
        <div>
          <p className="inline-flex animate-fade-up items-center gap-2 rounded-full border border-white/15 bg-white/5 px-3 py-1 text-xs text-slate-300">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400" /> Works on the exam hall network — no internet needed
          </p>
          <h1 className="mt-5 animate-fade-up text-4xl font-bold leading-tight tracking-tight [animation-delay:100ms] sm:text-6xl">
            Fair exams,{' '}
            <span className="animate-gradient-x bg-gradient-to-r from-brand-300 via-fuchsia-300 to-cyan-300 bg-[length:200%_auto] bg-clip-text text-transparent">
              locked to the hall.
            </span>
          </h1>
          <p className="mt-5 max-w-xl animate-fade-up text-lg leading-relaxed text-slate-300 [animation-delay:200ms]">
            Students open the exam by typing one address — only from the exam hall&apos;s Wi-Fi. Answers save as they type and
            survive Wi-Fi drops and power cuts. Invigilators watch a live dashboard where every AI alert is a prompt for a
            human, never a verdict.
          </p>
          <div className="mt-8 flex animate-fade-up flex-wrap gap-3 [animation-delay:300ms]">
            <Link
              to="/exam"
              className="group inline-flex items-center gap-2 rounded-xl bg-brand-500 px-5 py-3 font-semibold text-white shadow-lg shadow-brand-500/30 transition hover:-translate-y-0.5 hover:bg-brand-400"
            >
              I&apos;m a student — start my exam
              <span className="transition group-hover:translate-x-1">→</span>
            </Link>
            <Link
              to="/staff"
              className="inline-flex items-center gap-2 rounded-xl border border-white/20 px-5 py-3 font-semibold text-white transition hover:-translate-y-0.5 hover:bg-white/10"
            >
              Staff sign in
            </Link>
            <a href="#how" className="inline-flex items-center px-2 py-3 text-sm text-slate-400 hover:text-white">
              See how it works ↓
            </a>
          </div>
        </div>

        <HeroVisual />
      </div>
    </section>
  )
}

/** Browser mock typing the hall IP, with a scanning shield over a live exam. */
function HeroVisual() {
  const ip = '192.168.1.45'
  return (
    <div className="relative mx-auto w-full max-w-md animate-fade-up [animation-delay:250ms] lg:max-w-none">
      <div className="animate-float rounded-2xl border border-white/10 bg-slate-900/80 shadow-2xl shadow-brand-900/50 backdrop-blur">
        <div className="flex items-center gap-2 border-b border-white/10 px-4 py-3">
          <span className="h-3 w-3 rounded-full bg-rose-400/80" />
          <span className="h-3 w-3 rounded-full bg-amber-400/80" />
          <span className="h-3 w-3 rounded-full bg-emerald-400/80" />
          <div className="ml-3 flex flex-1 items-center gap-2 rounded-md bg-slate-800 px-3 py-1.5 font-mono text-sm text-slate-300">
            <span className="text-emerald-400">🔒</span>
            <span className="es-typing" style={{ '--chars': `${ip.length}ch`, '--steps': ip.length } as CSSProperties}>
              {ip}
            </span>
          </div>
        </div>

        <div className="relative grid gap-4 p-5 sm:grid-cols-[1fr_auto]">
          <div className="space-y-3">
            <div className="h-3 w-24 rounded bg-brand-400/60" />
            <div className="h-4 w-full rounded bg-white/15" />
            <div className="h-4 w-4/5 rounded bg-white/15" />
            {['A', 'B', 'C', 'D'].map((l, i) => (
              <div
                key={l}
                className={`flex animate-slide-in items-center gap-3 rounded-lg px-3 py-2 ring-1 ${
                  i === 1 ? 'bg-brand-500/20 ring-brand-400' : 'ring-white/10'
                }`}
                style={{ animationDelay: `${600 + i * 120}ms` }}
              >
                <span className={`h-3.5 w-3.5 rounded-full ring-2 ${i === 1 ? 'bg-brand-300 ring-brand-300' : 'ring-white/30'}`} />
                <span className="text-xs text-slate-400">{l}.</span>
                <span className="h-2.5 flex-1 rounded bg-white/10" />
              </div>
            ))}
            <p className="flex items-center gap-2 pt-1 text-xs text-emerald-300">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" /> All answers saved
            </p>
          </div>

          <div className="relative mx-auto grid h-36 w-36 place-items-center">
            <span className="absolute inset-0 animate-pulse-ring rounded-full border-2 border-brand-400/50" />
            <span className="absolute inset-0 animate-pulse-ring rounded-full border-2 border-brand-400/40 [animation-delay:1.2s]" />
            <div className="relative h-28 w-28 overflow-hidden rounded-3xl bg-slate-800 ring-1 ring-white/10">
              <ShieldMark className="absolute inset-3 h-[calc(100%-1.5rem)] w-[calc(100%-1.5rem)] opacity-90" />
              <div className="absolute inset-x-0 top-0 h-6 animate-scan bg-gradient-to-b from-transparent via-cyan-300/50 to-transparent" />
            </div>
          </div>
        </div>
      </div>
      <div className="absolute -bottom-5 -left-3 animate-float rounded-xl border border-white/10 bg-slate-900/90 px-3 py-2 text-xs shadow-xl [animation-delay:-3s]">
        <p className="font-semibold text-white">PIN 4821</p>
        <p className="text-slate-400">shown on the projector</p>
      </div>
      <div className="absolute -right-3 -top-5 animate-float rounded-xl border border-emerald-400/30 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-200 shadow-xl [animation-delay:-1.5s]">
        LAN only · 192.168.1.0/24
      </div>
    </div>
  )
}

function TrustStrip() {
  const items = [
    ['0', 'internet connections needed'],
    ['1', 'address to type — or a QR to scan'],
    ['Every', 'keystroke saved on the device'],
    ['100%', 'of AI alerts reviewed by a human'],
  ]
  return (
    <section className="border-y border-white/10 bg-white/[0.02] px-4 py-10">
      <dl className="mx-auto grid max-w-6xl grid-cols-2 gap-6 md:grid-cols-4">
        {items.map(([n, l], i) => (
          <div key={l} className="reveal text-center" style={{ '--delay': `${i * 90}ms` } as CSSProperties}>
            <dt className="text-3xl font-bold text-white">{n}</dt>
            <dd className="mt-1 text-sm text-slate-400">{l}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}

// --- How it works ---------------------------------------------------------------

type Step = { title: string; body: ReactNode; icon: ReactNode }

const studentSteps: Step[] = [
  {
    title: 'Join the exam hall Wi-Fi',
    body: 'Connect your computer to the hall network. Exam Shield refuses every device that is not on it — home Wi-Fi and mobile data cannot reach the exam.',
    icon: <IconWifi />,
  },
  {
    title: 'Type the address on the projector',
    body: (
      <>
        Open a browser and type the address your invigilator shows, e.g. <code className="text-brand-300">192.168.1.45</code>
        — or scan the QR code. You land on this page; press <b>Start my exam</b>.
      </>
    ),
    icon: <IconGlobe />,
  },
  {
    title: 'Enter the PIN and your matric number',
    body: 'Type the 4-digit exam PIN, then your matric number (for example IFT/22/9203) and accept the proctoring notice. The notice lists exactly what this exam records.',
    icon: <IconKey />,
  },
  {
    title: 'Confirm it is you',
    body: 'If the exam uses face verification, take a selfie (and optionally a photo of your ID card). Face the camera in good light — only you in view.',
    icon: <IconFace />,
  },
  {
    title: 'Write your exam',
    body: 'Answer at your own pace; the countdown uses the server clock. Every answer saves as you type — a green “All answers saved” means the server has it.',
    icon: <IconPencil />,
  },
  {
    title: 'Submit — or let the timer do it',
    body: 'Press Submit and confirm. When time runs out your answers are submitted automatically, even if you were offline at that moment.',
    icon: <IconCheck />,
  },
]

const staffSteps: Step[] = [
  {
    title: 'Sign in with your staff ID',
    body: (
      <>
        Go to <code className="text-brand-300">/staff</code> and sign in with your 8-digit staff ID. Admins also manage staff accounts
        and the student roster.
      </>
    ),
    icon: <IconKey />,
  },
  {
    title: 'Load the roster',
    body: 'Import students from a CSV (matric_no, full_name, department) and upload reference photos named like IFT-22-9203.jpg.',
    icon: <IconUsers />,
  },
  {
    title: 'Create the exam and add questions',
    body: 'Set the title, time window and hall network. Add multiple-choice and written questions by hand, or import a whole paper from a PDF and review it before saving.',
    icon: <IconDoc />,
  },
  {
    title: 'Choose the proctoring level',
    body: 'Turn each check on or off per exam: face verification, webcam, microphone, fullscreen, tab-switch detection, copy/paste blocking — or pick a preset.',
    icon: <IconSliders />,
  },
  {
    title: 'Open the exam and project the access card',
    body: 'Press Open exam now. The Student access tab shows the address, a fresh PIN and a QR code to put on the projector.',
    icon: <IconQr />,
  },
  {
    title: 'Monitor, review, report',
    body: 'Watch the live grid and alert feed, confirm or dismiss each alert, unlock a student who changed computers, then download the results as CSV.',
    icon: <IconChart />,
  },
]

function HowItWorks() {
  const [who, setWho] = useState<'student' | 'staff'>('student')
  const steps = who === 'student' ? studentSteps : staffSteps
  return (
    <section id="how" className="scroll-mt-20 px-4 py-24">
      <div className="mx-auto max-w-6xl">
        <SectionTitle kicker="How it works" title="From the projector to submitted — step by step" />
        <div role="tablist" aria-label="Choose your role" className="reveal mx-auto mt-8 flex w-fit rounded-xl bg-white/5 p-1 ring-1 ring-white/10">
          {(['student', 'staff'] as const).map((k) => (
            <button
              key={k}
              role="tab"
              type="button"
              aria-selected={who === k}
              onClick={() => setWho(k)}
              className={`rounded-lg px-5 py-2 text-sm font-semibold transition ${who === k ? 'bg-brand-500 text-white shadow' : 'text-slate-300 hover:text-white'}`}
            >
              {k === 'student' ? 'I’m a student' : 'I’m staff'}
            </button>
          ))}
        </div>

        <ol key={who} className="relative mt-12 grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {steps.map((s, i) => (
            <li
              key={s.title}
              className="group relative animate-fade-up rounded-2xl border border-white/10 bg-white/[0.03] p-6 transition hover:-translate-y-1 hover:border-brand-400/50 hover:bg-white/[0.06]"
              style={{ animationDelay: `${i * 90}ms` }}
            >
              <div className="flex items-center gap-3">
                <span className="grid h-11 w-11 place-items-center rounded-xl bg-brand-500/15 text-brand-300 ring-1 ring-brand-400/30 transition group-hover:scale-110">
                  {s.icon}
                </span>
                <span className="text-sm font-semibold text-brand-300">Step {i + 1}</span>
              </div>
              <h3 className="mt-4 text-lg font-semibold text-white">{s.title}</h3>
              <p className="mt-2 text-sm leading-relaxed text-slate-400">{s.body}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  )
}

// --- Power cut / network resilience -------------------------------------------------

function PowerCut() {
  const stages = [
    {
      tone: 'emerald',
      title: 'Writing normally',
      body: 'Each answer is written to this computer’s disk first, then sent to the exam server, which confirms it.',
      badge: 'All answers saved',
    },
    {
      tone: 'amber',
      title: 'Wi-Fi drops or the lights go off',
      body: 'Keep typing. Answers queue on the computer. If it switches off, everything up to the last keystroke is already on disk.',
      badge: 'Offline · 3 saved on this computer',
    },
    {
      tone: 'sky',
      title: 'Power or network returns',
      body: 'Reopen the page on the same computer: the exam comes back where you left it, and waiting answers are sent automatically.',
      badge: 'Syncing… then all saved',
    },
  ] as const
  const tone = {
    emerald: 'border-emerald-400/40 bg-emerald-400/10 text-emerald-200',
    amber: 'border-amber-400/40 bg-amber-400/10 text-amber-200',
    sky: 'border-sky-400/40 bg-sky-400/10 text-sky-200',
  }
  return (
    <section id="resilience" className="scroll-mt-20 border-y border-white/10 bg-gradient-to-b from-white/[0.02] to-transparent px-4 py-24">
      <div className="mx-auto max-w-6xl">
        <SectionTitle
          kicker="Power cuts & bad networks"
          title="What happens if the light goes off mid-exam?"
          sub="Nothing the student wrote is lost. Exam Shield is built for halls with unreliable power and Wi-Fi."
        />
        <div className="relative mt-14 grid gap-6 md:grid-cols-3">
          <svg aria-hidden className="absolute left-0 top-10 hidden h-2 w-full md:block" preserveAspectRatio="none">
            <line x1="8%" y1="1" x2="92%" y2="1" stroke="rgba(129,140,248,.6)" strokeWidth="2" strokeDasharray="8 4" className="animate-dash-flow" />
          </svg>
          {stages.map((s, i) => (
            <div key={s.title} className="reveal relative" style={{ '--delay': `${i * 160}ms` } as CSSProperties}>
              <div className="mx-auto grid h-20 w-20 place-items-center rounded-full border border-white/10 bg-slate-900 text-2xl shadow-lg">
                {i === 0 ? '💡' : i === 1 ? '⚡' : '🔄'}
              </div>
              <div className="mt-5 rounded-2xl border border-white/10 bg-slate-900/60 p-6 text-center">
                <h3 className="font-semibold text-white">{s.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-400">{s.body}</p>
                <span className={`mt-4 inline-block rounded-full border px-3 py-1 text-xs ${tone[s.tone]}`}>{s.badge}</span>
              </div>
            </div>
          ))}
        </div>
        <ul className="reveal mx-auto mt-12 grid max-w-4xl gap-3 text-sm text-slate-300 sm:grid-cols-2">
          {[
            'Answers are kept on the computer’s disk until the server confirms them.',
            'If the exam server itself restarts, students stay signed in and simply continue.',
            'Time-up while offline? The exam keeps trying and submits as soon as it can reach the server.',
            'Answers saved offline are still accepted for 2 minutes after the official end.',
            'Webcam uploads pause on a weak connection so answers always go first.',
            'Moved to another computer? An invigilator unlocks the session and the student continues.',
          ].map((t) => (
            <li key={t} className="flex gap-3">
              <span className="mt-0.5 text-emerald-400">✓</span>
              {t}
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}

// --- Features -------------------------------------------------------------------

function Features() {
  const items: [ReactNode, string, string][] = [
    [<IconWifi key="i" />, 'Locked to the hall network', 'Only devices on the hall’s subnet get in; everyone else sees “exam hall network required”.'],
    [<IconQr key="i" />, 'PIN & QR access', 'A fresh 4-digit PIN per exam; two exams can share a hall without mix-ups.'],
    [<IconFace key="i" />, 'Identity check', 'Selfie matched to the roster photo, with a manual-check fallback instead of false rejections.'],
    [<IconEye key="i" />, 'AI alerts, human decisions', 'No face, extra person, phone, talking, tab switches — each alert is confirmed or dismissed by staff.'],
    [<IconDoc key="i" />, 'PDF question import', 'Upload a question paper; Exam Shield extracts questions, options, answers and marks for review.'],
    [<IconSliders key="i" />, 'Per-exam proctoring switches', 'Full proctoring, locked browser without camera, or open book — your choice per exam.'],
    [<IconBolt key="i" />, 'Autosave that survives outages', 'Every keystroke stored locally and synced; works through Wi-Fi drops and power cuts.'],
    [<IconChart key="i" />, 'Results & audit trail', 'Automatic marking for multiple choice, CSV export, and a log of every staff action.'],
  ]
  return (
    <section id="features" className="scroll-mt-20 px-4 py-24">
      <div className="mx-auto max-w-6xl">
        <SectionTitle kicker="Features" title="Everything an exam hall needs" />
        <div className="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {items.map(([icon, t, d], i) => (
            <div
              key={t}
              className="reveal group rounded-2xl border border-white/10 bg-white/[0.03] p-5 transition hover:border-brand-400/40 hover:bg-white/[0.06]"
              style={{ '--delay': `${(i % 4) * 80}ms` } as CSSProperties}
            >
              <span className="grid h-10 w-10 place-items-center rounded-lg bg-brand-500/15 text-brand-300 transition group-hover:rotate-6 group-hover:scale-110">
                {icon}
              </span>
              <h3 className="mt-4 font-semibold text-white">{t}</h3>
              <p className="mt-1.5 text-sm leading-relaxed text-slate-400">{d}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function Modes() {
  const modes = [
    { name: 'Full proctoring', items: ['Face verification', 'Webcam + microphone', 'Fullscreen', 'Tab & clipboard checks'], hl: true },
    { name: 'Locked browser, no camera', items: ['Matric number + consent', 'Fullscreen', 'Tab & clipboard checks', 'No camera or microphone'] },
    { name: 'Open book', items: ['Matric number + consent', 'Normal window', 'Copy/paste allowed', 'Answers still autosave'] },
  ]
  return (
    <section className="border-t border-white/10 px-4 py-24">
      <div className="mx-auto max-w-6xl">
        <SectionTitle kicker="One switchboard per exam" title="Pick how strict each exam should be" />
        <div className="mt-12 grid gap-6 md:grid-cols-3">
          {modes.map((m, i) => (
            <div
              key={m.name}
              className={`reveal rounded-2xl border p-6 ${m.hl ? 'border-brand-400/60 bg-brand-500/10' : 'border-white/10 bg-white/[0.03]'}`}
              style={{ '--delay': `${i * 120}ms` } as CSSProperties}
            >
              <h3 className="text-lg font-semibold text-white">{m.name}</h3>
              <ul className="mt-4 space-y-2 text-sm text-slate-300">
                {m.items.map((it) => (
                  <li key={it} className="flex gap-2">
                    <span className="text-brand-300">•</span>
                    {it}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function Faq() {
  const qa: [string, ReactNode][] = [
    ['The page says “Exam hall network required”.', 'Your device is not on the hall network. Turn off mobile data, connect to the hall Wi-Fi and type the address again.'],
    ['It says “No exam is open right now”.', 'The invigilator has not opened the exam yet. Leave the page open — it checks again every 15 seconds.'],
    ['My face was not verified.', 'Face the camera directly, light your face from the front (not a window behind you) and make sure only you are in view, then retake the selfie.'],
    [
      'The power went off. Did I lose my answers?',
      'No. Answers are saved on the computer as you type. When power returns, open the same address on the same computer and continue; anything not yet sent goes automatically.',
    ],
    ['I had to move to another computer.', 'Ask an invigilator to unlock your session from the dashboard, then sign in again on the new computer.'],
    ['Does the AI decide if I cheated?', 'No. The AI only raises alerts. A member of staff reviews every one and can dismiss it, which restores your integrity score.'],
    ['Why does it ask for https://?', 'Browsers only allow the camera on secure connections. Exams without camera or microphone also work on plain http://.'],
  ]
  return (
    <section id="faq" className="scroll-mt-20 border-t border-white/10 px-4 py-24">
      <div className="mx-auto max-w-3xl">
        <SectionTitle kicker="Questions" title="Frequently asked" />
        <div className="mt-10 space-y-3">
          {qa.map(([q, a], i) => (
            <details
              key={q}
              className="reveal group rounded-xl border border-white/10 bg-white/[0.03] p-5 open:bg-white/[0.06]"
              style={{ '--delay': `${i * 50}ms` } as CSSProperties}
            >
              <summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-medium text-white">
                {q}
                <span className="text-brand-300 transition group-open:rotate-45">+</span>
              </summary>
              <p className="mt-3 text-sm leading-relaxed text-slate-400">{a}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  )
}

function FinalCta() {
  return (
    <section className="px-4 pb-24">
      <div className="reveal relative mx-auto max-w-5xl overflow-hidden rounded-3xl border border-white/10 bg-gradient-to-br from-brand-600 via-brand-700 to-fuchsia-700 px-6 py-14 text-center">
        <div aria-hidden className="absolute -right-10 -top-10 h-56 w-56 animate-blob rounded-full bg-white/10 blur-2xl" />
        <h2 className="relative text-3xl font-bold text-white sm:text-4xl">Ready when you are.</h2>
        <p className="relative mx-auto mt-3 max-w-xl text-brand-100">
          Students: make sure you are on the hall Wi-Fi, then start. Staff: sign in to prepare the exam.
        </p>
        <div className="relative mt-8 flex flex-wrap justify-center gap-3">
          <Link to="/exam" className="rounded-xl bg-white px-6 py-3 font-semibold text-brand-700 transition hover:-translate-y-0.5">
            Start my exam →
          </Link>
          <Link to="/staff" className="rounded-xl border border-white/40 px-6 py-3 font-semibold text-white transition hover:-translate-y-0.5 hover:bg-white/10">
            Staff sign in
          </Link>
        </div>
      </div>
    </section>
  )
}

function SectionTitle({ kicker, title, sub }: { kicker: string; title: string; sub?: string }) {
  return (
    <div className="reveal mx-auto max-w-2xl text-center">
      <p className="text-sm font-semibold uppercase tracking-widest text-brand-300">{kicker}</p>
      <h2 className="mt-3 text-3xl font-bold tracking-tight text-white sm:text-4xl">{title}</h2>
      {sub && <p className="mt-4 text-slate-400">{sub}</p>}
    </div>
  )
}

// --- Icons (inline SVG, no external assets) -------------------------------------------

function ShieldMark({ className = '' }: { className?: string }) {
  return (
    <svg viewBox="0 0 64 64" className={className} aria-hidden>
      <defs>
        <linearGradient id="es-g" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#a5b4fc" />
          <stop offset="1" stopColor="#6366f1" />
        </linearGradient>
      </defs>
      <path d="M32 4 8 13v17c0 15 10 26 24 30 14-4 24-15 24-30V13L32 4Z" fill="url(#es-g)" />
      <path d="m22 32 7 7 14-15" fill="none" stroke="#fff" strokeWidth="5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

const ic = 'h-5 w-5'
const sv = { fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, viewBox: '0 0 24 24', 'aria-hidden': true }
function IconWifi() {
  return (
    <svg className={ic} {...sv}>
      <path d="M2 8.8a15 15 0 0 1 20 0M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0" />
      <circle cx="12" cy="19.5" r="1" fill="currentColor" />
    </svg>
  )
}
function IconGlobe() {
  return (
    <svg className={ic} {...sv}>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18" />
    </svg>
  )
}
function IconKey() {
  return (
    <svg className={ic} {...sv}>
      <circle cx="7.5" cy="15.5" r="3.5" />
      <path d="m10 13 9-9M16 7l2 2M14 9l2 2" />
    </svg>
  )
}
function IconFace() {
  return (
    <svg className={ic} {...sv}>
      <path d="M3 7V5a2 2 0 0 1 2-2h2M17 3h2a2 2 0 0 1 2 2v2M21 17v2a2 2 0 0 1-2 2h-2M7 21H5a2 2 0 0 1-2-2v-2" />
      <path d="M9 10h.01M15 10h.01M9.5 15a3.5 3.5 0 0 0 5 0" />
    </svg>
  )
}
function IconPencil() {
  return (
    <svg className={ic} {...sv}>
      <path d="M12 20h9M16.5 3.5a2.1 2.1 0 1 1 3 3L7 19l-4 1 1-4Z" />
    </svg>
  )
}
function IconCheck() {
  return (
    <svg className={ic} {...sv}>
      <circle cx="12" cy="12" r="9" />
      <path d="m8 12 3 3 5-6" />
    </svg>
  )
}
function IconUsers() {
  return (
    <svg className={ic} {...sv}>
      <circle cx="9" cy="8" r="3.5" />
      <path d="M2.5 20a6.5 6.5 0 0 1 13 0M16 4.5a3.5 3.5 0 0 1 0 7M21.5 20a6.5 6.5 0 0 0-4-6" />
    </svg>
  )
}
function IconDoc() {
  return (
    <svg className={ic} {...sv}>
      <path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9Z" />
      <path d="M14 3v6h6M8 13h8M8 17h5" />
    </svg>
  )
}
function IconSliders() {
  return (
    <svg className={ic} {...sv}>
      <path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12M20 18h0" />
      <circle cx="16" cy="6" r="2" />
      <circle cx="10" cy="12" r="2" />
      <circle cx="18" cy="18" r="2" />
    </svg>
  )
}
function IconQr() {
  return (
    <svg className={ic} {...sv}>
      <rect x="3" y="3" width="7" height="7" rx="1" />
      <rect x="14" y="3" width="7" height="7" rx="1" />
      <rect x="3" y="14" width="7" height="7" rx="1" />
      <path d="M14 14h3v3M21 14v.01M14 21h7v-4" />
    </svg>
  )
}
function IconChart() {
  return (
    <svg className={ic} {...sv}>
      <path d="M3 3v18h18" />
      <path d="m7 15 4-4 3 3 5-6" />
    </svg>
  )
}
function IconEye() {
  return (
    <svg className={ic} {...sv}>
      <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  )
}
function IconBolt() {
  return (
    <svg className={ic} {...sv}>
      <path d="M13 2 4 14h7l-1 8 9-12h-7Z" />
    </svg>
  )
}
