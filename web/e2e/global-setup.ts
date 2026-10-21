import { execFileSync } from 'node:child_process'
import path from 'node:path'

/** Build the fake webcam video (a still of the test face) before the run. */
export default function globalSetup() {
  execFileSync('go', ['run', './scripts/fakecam', 'internal/testdata/face.jpg', path.join('web', 'e2e', 'face.y4m')], {
    cwd: path.resolve('..'),
    stdio: 'inherit',
  })
}
