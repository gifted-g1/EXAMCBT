# 🛡️ Exam Shield

Network-locked, AI-assisted exam proctoring.

Students open the exam by typing the IP address their invigilator shows. They
can only do so from the exam hall network, and they land straight in a
fullscreen exam: no student accounts, no dashboard. Staff run everything from
a proctor dashboard with a live webcam grid and AI alerts. Every alert is a
prompt for a human to review; nothing is decided automatically.

**Stack:** React 18 + Vite + TypeScript · Go 1.22 (Gin, GORM, ONNX Runtime) · MySQL 8 · Redis 7 · MinIO · Nginx

## Quick start (one laptop, no Docker)

```bash
cd web && npm ci && npm run build && cd ..
SEED_ON_START=true go run ./cmd/api
```

- Staff dashboard: <http://localhost:8080/staff>
- Students: `http(s)://<this machine's LAN IP>:8080` from the same network. Webcams require HTTPS; see [Deployment](DOCUMENTATION.md#13-deployment).

Standalone mode runs SQLite, an embedded Redis, disk storage and the AI worker
inside one process. For MySQL, Redis, MinIO, separate AI workers and Nginx:

```bash
cp .env.example .env            # set LAN_IP, LAN_CIDRS and every secret
./scripts/gen-lan-cert.sh 192.168.1.45
./scripts/download-models.sh    # optional ONNX models; pure-Go fallback otherwise
docker compose up -d --build
```

## Online demo (Render, Railway, Fly…)

Exam Shield is designed for an exam hall LAN. To host a public demo, deploy
the Docker image and set `ACCESS_MODE=open`: this switches **off** the
hall-network lock (PINs, identity checks and proctoring still apply) and
trusts the platform's load balancer for the real visitor IP. `render.yaml`
is a ready Render Blueprint.

| Variable | Value |
|---|---|
| `ACCESS_MODE` | `open` (use `lan`, the default, in real exam halls) |
| `SEED_ON_START` | `true` for demo accounts |
| `JWT_SECRET` | 32+ random characters |
| `PUBLIC_URL` | your service URL, shown on access cards and QR codes |
| `PORT` | set by the platform automatically |

On plans without a persistent disk the SQLite database resets on every
restart; mount a disk at `/app/data` to keep data.

## Mock data (development / training only)

Seeding is refused when `APP_ENV=production`.

| Staff ID | Password | Name | Role |
|---|---|---|---|
| 19990045 | OLAWALE | Olawale Johnson | proctor |
| 19980012 | ADEGOKE | Adegoke Bolanle | proctor |
| 20010078 | MUSTAPHA | Mustapha Kabir | admin |
| 19970023 | CHUKWU | Chukwu Emeka | proctor |
| 20030091 | FASHOLA | Fashola Ronke | admin |
| 20100056 | GARBA | Garba Suleiman | proctor |
| 20050044 | ADELEKE | Adeleke Yewande | admin |
| 20120019 | ONYEKA | Onyeka Chidi | proctor |

There are 12 roster students, `IFT/22/9203` … `IFT/24/9589`. See
`seed/seed.go`, and `seed/seed.sql` for a MySQL script with bcrypt-hashed
passwords. A DRAFT demo exam is created when `SEED_ON_START=true` or
`go run ./cmd/seed -demo-exam` is used.

## Adding students and questions

- **One student + photo** (no server needed):
  `go run ./cmd/student -matric IFT/22/9203 -name "Awodele Kehinde Gabriel" -department "Information Technology" -photo photo.png`
- **Whole roster:** Dashboard → Student roster → CSV (`matric_no,full_name[,department]`) + bulk photos named `IFT-22-9203.jpg`.
- **Questions from a PDF:** Dashboard → exam → Questions → *Import from PDF*. Review the preview, then import. See the format in [docs/sample-questions.pdf](docs/sample-questions.pdf).
- **Fill-in-the-gap questions:** choose *Fill in the gap*, type `_____` for each blank and enter the accepted answers (alternatives separated by `/`). Marked automatically, with part marks.
- **Time limit:** set *Time limit (minutes)* when creating an exam (e.g. 25), or *Time limit → Change* on the exam page. Each student gets that long from when they start; they are submitted automatically when it runs out.
- **Example exam paper:** [docs/exam-paper/computing-comprehensive-exam.pdf](docs/exam-paper/computing-comprehensive-exam.pdf) (30 questions, 100 marks, print-ready). To run it in the app, import [the staff copy](docs/exam-paper/computing-comprehensive-exam-import.pdf).
- **Marking theory answers:** Dashboard → exam → Marking. Write a marking guide (key points, marks, keywords) to get suggested marks; you confirm every mark. Optional AI suggestions: set `AI_MARKING=true` and `ANTHROPIC_API_KEY` (answers are sent to Anthropic; needs internet). See DOCUMENTATION §7.

## Repository layout

```
cmd/            api · ai-worker · migrate · seed
internal/
  middleware/   lan_gate.go (heart of the system) · auth · ratelimit · cors · misc
  handlers/     auth · exam · session · violation · proctor · roster · users
  services/     auth · exams · sessions · roster · proctor · scoring
                face_match · gaze · object_detect · audio · background (findings + scheduler)
  ai/           engine · ONNX backend (build tag) · Go fallback · pre/post-processing
  worker/       AI worker (Redis Streams → models → findings)
  realtime/     WebSocket hub
  store/        MySQL/SQLite · Redis locks/limits/streams · MinIO/disk blobs
  repositories/ models/ config/ netutil/ auth/ httpx/ app/
migrations/     golang-migrate SQL (embedded)
seed/           seed.go + seed.sql
web/            React app (embedded into the Go binary) + Vitest + Playwright
api/            openapi.yaml
deploy/         nginx template, certs/
scripts/        gen-lan-cert.sh · download-models.sh · fakecam (E2E webcam)
```

## Tests

```bash
go test ./...                                    # unit + full HTTP integration
cd web && npm test                               # Vitest
cd web && npm run build && npx playwright test   # E2E (Edge/Chrome, fake webcam)
```

## Documentation

- [DOCUMENTATION.md](DOCUMENTATION.md) — architecture, access model, install, env vars, RBAC, API, schema, AI pipeline, frontend, testing, deployment, troubleshooting
- [ERROR_AUDIT.md](ERROR_AUDIT.md) — self-audit across 10 categories, with fixes and open items
- [api/openapi.yaml](api/openapi.yaml) — OpenAPI 3 spec
