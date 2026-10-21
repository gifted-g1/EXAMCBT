import { expect, test, type APIRequestContext } from '@playwright/test'
import fs from 'node:fs'
import path from 'node:path'

const face = fs.readFileSync(path.resolve('../internal/testdata/face.jpg'))

async function staffToken(request: APIRequestContext, username = '20010078', password = 'MUSTAPHA') {
  const r = await request.post('/api/staff/auth/login', { data: { username, password } })
  expect(r.ok()).toBeTruthy()
  return (await r.json()).access_token as string
}

test('staff sign-in shows the dashboard and the IP/PIN/QR access card', async ({ page }) => {
  await page.goto('/staff')
  await page.getByLabel('Staff ID').fill('19990045')
  await page.getByLabel(/^Password/).fill('olawale') // upper-cased by the form
  const pw = page.getByLabel(/^Password/)
  await expect(pw).toHaveAttribute('type', 'password')
  await page.getByRole('button', { name: 'Show password' }).click()
  await expect(pw).toHaveAttribute('type', 'text')
  await expect(pw).toHaveValue('OLAWALE')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { name: 'Exams' })).toBeVisible()
  await page.getByRole('link', { name: /Demo Proctored Quiz/ }).click()
  await page.getByRole('tab', { name: 'Student access' }).click()
  await expect(page.getByTestId('access-ip')).toHaveText('127.0.0.1:18090')
  await expect(page.getByAltText(/QR code/)).toBeVisible()
  // Proctors cannot see admin-only navigation.
  await expect(page.getByRole('link', { name: 'Staff accounts' })).toHaveCount(0)
})

test('wrong staff password is rejected', async ({ page }) => {
  await page.goto('/staff')
  await page.getByLabel('Staff ID').fill('19980012')
  await page.getByLabel(/^Password/).fill('WRONGPASS')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('alert')).toContainText('incorrect')
})

test('student: locked until open, then verify → fullscreen exam → submit', async ({ page, request }) => {
  // Before any exam is open, the student page explains the 423 state.
  await page.goto('/exam')
  await expect(page.getByRole('heading', { name: 'No exam is open right now' })).toBeVisible()

  // Staff prepares and opens an exam on the loopback "LAN" with a PIN.
  const token = await staffToken(request)
  const auth = { Authorization: `Bearer ${token}` }
  const students = await (await request.get('/api/staff/students?q=IFT/23/9002', { headers: auth })).json()
  const studentId = students.students[0].id
  const photo = await request.post(`/api/staff/students/${studentId}/photo`, {
    headers: auth,
    multipart: { photo: { name: 'p.jpg', mimeType: 'image/jpeg', buffer: face } },
  })
  expect(photo.status()).toBe(204)
  const now = Date.now()
  const exam = await (
    await request.post('/api/staff/exams', {
      headers: auth,
      data: {
        title: 'E2E Networking',
        starts_at: new Date(now - 60_000).toISOString(),
        ends_at: new Date(now + 3_600_000).toISOString(),
        lan_cidr: '127.0.0.0/8',
        require_pin: true,
      },
    })
  ).json()
  await request.post(`/api/staff/exams/${exam.id}/questions`, {
    headers: auth,
    data: { questions: [{ prompt: 'Which HTTP status means Locked?', options: ['403', '423'], correct_option: 1 }] },
  })
  expect((await request.post(`/api/staff/exams/${exam.id}/status`, { headers: auth, data: { action: 'open' } })).ok()).toBeTruthy()

  // Student without PIN is asked for it; a wrong PIN is refused.
  await page.goto('/exam')
  await expect(page.getByRole('heading', { name: 'Enter the exam PIN' })).toBeVisible()
  await page.getByLabel('Exam PIN', { exact: true }).fill(exam.pin_code === '0000' ? '1111' : '0000')
  await page.getByRole('button', { name: 'Continue' }).click()
  await expect(page.getByRole('heading', { name: 'That PIN is not correct' })).toBeVisible()

  // QR-style URL carries the PIN; it is stripped from the address bar.
  await page.goto(`/?pin=${exam.pin_code}`)
  await expect(page.getByRole('heading', { name: 'E2E Networking' })).toBeVisible()
  expect(page.url()).not.toContain('pin=')

  await page.getByLabel(/Matric number/).fill('ift/23/9002')
  await expect
    .poll(() => page.locator('video').first().evaluate((v) => (v as HTMLVideoElement).videoWidth), { timeout: 15_000 })
    .toBeGreaterThan(0)
  await page.getByRole('button', { name: 'Take selfie' }).click()
  await expect(page.getByAltText('Your selfie')).toBeVisible()
  await page.getByLabel(/consent/).check()
  await page.getByRole('button', { name: 'Verify and start exam' }).click()

  // Fullscreen gate, then the exam itself — no navigation anywhere.
  await page.getByRole('button', { name: 'Enter fullscreen' }).click()
  await expect(page.getByRole('heading', { name: 'Which HTTP status means Locked?' })).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'Main' })).toHaveCount(0)
  await expect(page.getByRole('timer')).toBeVisible()

  await page.getByLabel(/423/).check()
  await expect(page.getByText('All answers saved')).toBeVisible()

  await page.getByRole('button', { name: 'Submit exam' }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Submit' }).click()
  await expect(page.getByRole('heading', { name: 'Exam submitted' })).toBeVisible()

  // The proctor report shows the graded result.
  const report = await (await request.get(`/api/staff/exams/${exam.id}/report`, { headers: auth })).json()
  expect(report.rows[0].matric_no).toBe('IFT/23/9002')
  expect(report.rows[0].score).toBe(1)
})

test('camera-free exam: no selfie, no fullscreen gate; admin toggles in dashboard', async ({ page, request }) => {
  const token = await staffToken(request)
  const auth = { Authorization: `Bearer ${token}` }
  const now = Date.now()
  const exam = await (
    await request.post('/api/staff/exams', {
      headers: auth,
      data: {
        title: 'E2E Open Book',
        starts_at: new Date(now - 60_000).toISOString(),
        ends_at: new Date(now + 3_600_000).toISOString(),
        lan_cidr: '127.0.0.0/8',
        require_pin: true,
      },
    })
  ).json()
  await request.post(`/api/staff/exams/${exam.id}/questions`, {
    headers: auth,
    data: { questions: [{ prompt: 'Which port does HTTPS use by default?', options: ['80', '443'], correct_option: 1 }] },
  })

  // Admin switches everything off with the "Open book" preset in the dashboard.
  await page.goto('/staff')
  await page.getByLabel('Staff ID').fill('20010078')
  await page.getByLabel(/^Password/).fill('MUSTAPHA')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { name: 'Exams' })).toBeVisible()
  // Full page load: the session is restored from the refresh cookie.
  await page.goto(`/staff/exams/${exam.id}`)
  await page.getByRole('tab', { name: 'Proctoring' }).click()
  await page.getByRole('button', { name: /Open book/ }).click()
  await expect(page.getByRole('switch', { name: 'Webcam monitoring' })).toHaveAttribute('aria-checked', 'false')
  await expect(page.getByRole('switch', { name: 'Face verification at entry' })).toHaveAttribute('aria-checked', 'false')
  // Turn one switch back on individually.
  await page.getByRole('switch', { name: 'Tab and window switch detection' }).click()
  await expect(page.getByRole('switch', { name: 'Tab and window switch detection' })).toHaveAttribute('aria-checked', 'true')
  expect((await request.post(`/api/staff/exams/${exam.id}/status`, { headers: auth, data: { action: 'open' } })).ok()).toBeTruthy()

  // Student: matric + consent only, straight into the exam, no fullscreen overlay, no webcam.
  const student = await page.context().browser()!.newPage()
  await student.goto(`/?pin=${exam.pin_code}`)
  await expect(student.getByRole('heading', { name: 'E2E Open Book' })).toBeVisible()
  await expect(student.getByRole('button', { name: 'Take selfie' })).toHaveCount(0)
  await student.getByLabel(/Matric number/).fill('IFT/24/9501')
  await student.getByLabel(/consent/).check()
  await student.getByRole('button', { name: 'Start exam' }).click()
  await expect(student.getByRole('heading', { name: 'Which port does HTTPS use by default?' })).toBeVisible()
  await expect(student.getByRole('button', { name: 'Enter fullscreen' })).toHaveCount(0)
  await expect(student.locator('video')).toHaveCount(0)
  await student.close()
})

test('landing page explains the app and leads into the exam', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: /locked to the hall/i })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Join the exam hall Wi-Fi' })).toBeVisible()
  await page.getByRole('tab', { name: /staff/i }).click()
  await expect(page.getByRole('heading', { name: 'Choose the proctoring level' })).toBeVisible()
  await page.getByRole('link', { name: /I.m a student/ }).click()
  await expect(page).toHaveURL(/\/exam$/)
})

test('answers survive going offline and a reload while offline', async ({ page, request, context }) => {
  const token = await staffToken(request)
  const auth = { Authorization: `Bearer ${token}` }
  const now = Date.now()
  const exam = await (
    await request.post('/api/staff/exams', {
      headers: auth,
      data: {
        title: 'E2E Power Cut',
        starts_at: new Date(now - 60_000).toISOString(),
        ends_at: new Date(now + 3_600_000).toISOString(),
        lan_cidr: '127.0.0.0/8',
        require_pin: true,
        proctoring: { face_verification: false, camera_monitoring: false, audio_monitoring: false, fullscreen_required: false },
      },
    })
  ).json()
  await request.post(`/api/staff/exams/${exam.id}/questions`, {
    headers: auth,
    data: {
      questions: [
        { prompt: 'Pick B', options: ['A', 'B'], correct_option: 1 },
        { prompt: 'Pick A', options: ['A', 'B'], correct_option: 0 },
        { kind: 'TEXT', prompt: 'Describe a power cut.' },
      ],
    },
  })
  expect((await request.post(`/api/staff/exams/${exam.id}/status`, { headers: auth, data: { action: 'open' } })).ok()).toBeTruthy()

  await page.goto(`/exam?pin=${exam.pin_code}`)
  await page.getByLabel(/Matric number/).fill('IFT/24/9567')
  await page.getByLabel(/consent/).check()
  await page.getByRole('button', { name: 'Start exam' }).click()
  await page.getByLabel(/^B\./).check()
  await expect(page.getByText('All answers saved')).toBeVisible()
  // Let the service worker take control so the app can reload offline.
  await page.waitForFunction(() => !!navigator.serviceWorker?.controller, null, { timeout: 15_000 }).catch(() => undefined)
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Pick B' })).toBeVisible()

  // The Wi-Fi / power goes: keep answering.
  await context.setOffline(true)
  await page.getByRole('button', { name: 'Question 2' }).click()
  await page.getByLabel(/^A\./).check()
  await page.getByRole('button', { name: 'Question 3' }).click()
  await page.getByLabel('Your answer').fill('The lights went off but my work stayed.')
  await expect(page.getByText(/Connection lost/)).toBeVisible({ timeout: 30_000 })

  // Reload while still offline: the exam reopens from this computer's copy.
  await page.reload()
  await expect(page.getByText(/Connection lost|Offline/).first()).toBeVisible({ timeout: 30_000 })
  await page.getByRole('button', { name: 'Question 3' }).click()
  await expect(page.getByLabel('Your answer')).toHaveValue('The lights went off but my work stayed.')

  // Connection returns: everything syncs, then submit.
  await context.setOffline(false)
  await expect(page.getByText('All answers saved')).toBeVisible({ timeout: 30_000 })
  await page.getByRole('button', { name: 'Submit exam' }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Submit' }).click()
  await expect(page.getByRole('heading', { name: 'Exam submitted' })).toBeVisible()

  const report = await (await request.get(`/api/staff/exams/${exam.id}/report`, { headers: auth })).json()
  expect(report.rows[0].score).toBe(2)
})
