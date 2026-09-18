# Exam Shield

AI-powered examination integrity, invigilation, and monitoring platform.

Stack (as specified): **React** (frontend) · **Go** (all application/business APIs)
· **Python** (AI/computer-vision service only). No Node/Express, PHP, Django,
Flask/FastAPI, Next.js, Firebase/Supabase, Java, or C# anywhere in the stack.

```
React  ──REST + WebSocket──>  Go backend  ──HTTP──>  Python AI service
                                    │
                                    └──> PostgreSQL
```

React never talks to the Python AI service directly — every face-verification
or frame-analysis request is validated and relayed through Go.

## Project layout

```
backend/        Go application/business API (cmd/server, internal/*)
ai-service/     Python AI/computer-vision service (stdlib http.server, OpenCV)
frontend/       React app (Vite), white + purple design system
docker-compose.yml
```

## Running everything locally (online-mode exams)

```bash
docker compose up --build
```

- Frontend: http://localhost:5173
- Backend API: http://localhost:8080/api/v1
- AI service: http://localhost:9000
- Postgres: localhost:5432 (user/pass/db: `examshield`)

The backend auto-migrates the schema on startup (see `internal/db/db.go`).
There is no seed data yet — create your first Super Admin directly in
Postgres (hash a password with bcrypt, cost 12) or add a one-off seed script
before first use.

## Running for a LAN-mode examination

**Important:** LAN mode works by having the Go backend detect the *host
machine's* real network interface and IP address (see
`internal/exam/lan_server.go`) so students on the same Wi-Fi can reach it
directly. If the backend runs inside Docker's default bridge network, it will
detect the container's internal IP, not the admin's real LAN IP, and students
won't be able to connect.

For a genuine LAN exam, run the backend **directly on the invigilating
machine** rather than through Docker:

```bash
cd backend
export DATABASE_URL="host=localhost user=examshield password=examshield dbname=examshield port=5432 sslmode=disable"
export AI_SERVICE_URL="http://localhost:9000"
go run ./cmd/server
```

(Postgres and the AI service can still run in Docker — only the Go backend
needs direct access to the real network interface.) Alternatively, run the
backend container with `--network host` on Linux hosts.

When an exam configured for LAN mode is started, the API response includes
the detected IP, an available port, the full access URL, and a base64 PNG QR
code students can scan — rendered in the admin "Exam Server" panel.

## Backend (`backend/`)

Clean/modular architecture:

```
cmd/server/main.go        entrypoint: config, DB connect, migrate, serve
internal/
  config/                 env-driven configuration, no hard-coded values
  models/                 GORM entities (users, exams, monitoring, audit…)
  db/                     connection + AutoMigrate
  repositories/           all DB access, one repo per aggregate
  services/               business logic (auth, exam lifecycle, monitoring…)
  handlers/                HTTP handlers — thin, call into services
  middleware/             JWT auth, RBAC, security headers, rate limiting
  auth/                   JWT + bcrypt
  websocket/              real-time monitoring hub + handler
  exam/                   LAN IP detection, port scan, QR code generation
  router.go               wires everything, mounts routes
```

Notable design choices:

- **RBAC is enforced server-side only**, from verified JWT claims — the
  client's claimed role is never trusted.
- **Exam lifecycle** is a strict state machine (`DRAFT → SCHEDULED →
  PUBLISHED → ACTIVE → ENDED → ARCHIVED`); invalid transitions are rejected.
- **AI events never carry a verdict.** The AI service classifies severity
  (`INFO`/`LOW`/`MEDIUM`/`HIGH`/`CRITICAL`); a human always reviews before any
  conclusion is drawn.
- **Duplicate submissions are prevented at the database level** via a unique
  `(exam_id, student_id)` index on attempts, not just application logic.
- **Face verification stores only the match result and confidence** — never
  the raw image — in line with the platform's privacy-by-design requirement.
- go.mod contains `replace` directives pointing vanity import paths
  (`gorm.io/*`, `golang.org/x/*`, `gopkg.in/*`) at the GitHub repositories
  they resolve to anyway. This was needed to build in this restricted
  sandbox and is harmless to keep in any environment.

## AI service (`ai-service/`)

Deliberately built on Python's standard-library `http.server`
(`ThreadingHTTPServer`) rather than Flask/FastAPI/Django, per the stack
restriction that Python is used *only* for AI/CV logic, not as a general web
framework.

```
app/
  main.py                 HTTP routing (/verify-face, /analyze-frame,
                           /detect-face, /detect-multiple-faces, /healthz)
  face_detection/         OpenCV Haar-cascade face detection
  face_verification/      feature extraction + cosine-similarity comparison
  behavior_detection/     per-student rolling state -> classified events
```

The face-verification "embedding" here is a lightweight gradient-histogram
feature vector so the service has no heavy model-download dependency. It is
built behind a stable `extract()` interface — swap in a proper embedding
model (ArcFace/FaceNet) for production accuracy without touching anything
else in the service.

## Frontend (`frontend/`)

Vite + React + React Router. Design system lives in `src/styles/theme.css`
(white background, purple primary, light/dark aware) with a small reusable
component kit in `src/components/ui/`. Pages are organized by role under
`src/pages/{admin,lecturer,student}`.

```bash
cd frontend
npm install
npm run dev      # http://localhost:5173
```

## What's scaffolded vs. what's left for a production rollout

This is a complete, working, and verified-to-build scaffold covering every
architectural piece in the spec (auth, RBAC, exam CRUD, LAN discovery + QR,
face verification, AI monitoring pipeline, WebSocket dashboards, audit
logging, reporting). Before a real deployment you'd still want to:

- Swap the AI service's lightweight face-embedding comparator for a proper
  face-recognition model.
- Add versioned SQL migrations (the `migrations/` folder is a placeholder;
  `AutoMigrate` is used for fast bootstrap) via a tool like `golang-migrate`.
- Add a seed script for the first Super Admin account.
- Move the in-memory rate limiter and AI reference-face store to Redis/DB
  for multi-instance deployments.
- Add automated tests (unit + integration) — none are included here.
- Front the AI service and backend with TLS in production; both currently
  assume a trusted internal network or a reverse proxy terminating TLS.
