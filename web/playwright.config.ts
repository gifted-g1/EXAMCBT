import { defineConfig } from '@playwright/test'
import path from 'node:path'
import os from 'node:os'

const port = 18090
const tmp = path.join(os.tmpdir(), `exam-shield-e2e-${Date.now()}`)
const channel = process.env.PW_CHANNEL ?? (process.platform === 'win32' ? 'msedge' : undefined)

/**
 * E2E runs the real Go server in standalone mode (SQLite + embedded Redis +
 * in-process AI) with the built UI embedded. Loopback is allowed into the
 * exam so the browser on this machine counts as "on the LAN".
 * Build the UI first: `npm run build`.
 */
export default defineConfig({
  testDir: './e2e',
  globalSetup: './e2e/global-setup.ts',
  timeout: 90_000,
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    channel,
    launchOptions: {
      args: [
        '--use-fake-ui-for-media-stream',
        '--use-fake-device-for-media-stream',
        ...(process.env.PW_FAKE_FILE === '0' ? [] : [`--use-file-for-fake-video-capture=${path.resolve('e2e/face.y4m')}`]),
      ],
    },
    permissions: ['camera', 'microphone'],
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'go run ./cmd/api',
    cwd: '..',
    url: `http://127.0.0.1:${port}/healthz`,
    timeout: 240_000,
    reuseExistingServer: false,
    env: {
      MODE: 'standalone',
      HTTP_ADDR: `127.0.0.1:${port}`,
      SQLITE_PATH: path.join(tmp, 'e2e.db'),
      BLOB_DIR: path.join(tmp, 'blobs'),
      SEED_ON_START: 'true',
      LAN_CIDRS: '127.0.0.0/8',
      LAN_ADVERTISE_IP: '127.0.0.1',
      ALLOW_LOOPBACK_EXAM: 'true',
      BCRYPT_COST: '10',
      JWT_SECRET: 'e2e-secret-e2e-secret-e2e-secret-0123',
      FRAME_MIN_INTERVAL: '1s',
      LOG_LEVEL: 'warn',
    },
  },
})
