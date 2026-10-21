# Exam Shield — Error Audit

Self-audit of the v3 codebase across the ten required categories. The audit
covers what was built, the draft schema and middleware sketch in the build
prompt, and the assumptions the prompt makes.

**Severity:** Critical (breaks the core promise or security) · High (likely
production incident or data/security exposure) · Medium (real but bounded) ·
Low (hygiene).
**Status:** ✅ Fixed (code + test where practical) · 🟡 Accepted (documented
trade-off) · ⚠️ Not verified (could not be exercised in the build environment).

## Summary

| Severity | Found | Fixed | Accepted | Not verified |
|---|---|---|---|---|
| Critical | 3 | 3 | 0 | 0 |
| High | 8 | 6 | 1 | 1 |
| Medium | 13 | 4 | 6 | 3 |
| Low | 10 | 6 | 4 | 0 |

**Build environment limits.** The build machine is Windows with Go 1.26,
Node 20, and Edge. It has no C compiler, no running Docker daemon, no MySQL,
and no Redis server. The following were verified: `go build`, `go vet`, all Go
unit and integration tests (SQLite + embedded Redis), 14 Vitest tests, and 3
Playwright E2E tests in Edge against the real server, including camera → face
verification → fullscreen exam → submit. **Not executed here:** the `onnx`
build tag (cgo), the Dockerfile and Compose stack, the MySQL migration, and
`go test -race` (needs cgo). These items are marked ⚠️ below.

---

## 1. Go syntax & compile

| Check | Result |
|---|---|
| `go build ./...`, `go vet ./...` | ✅ clean |
| Unused imports / shadowed vars | ✅ compiler + vet clean; `gofmt -l` empty |
| Ignored `err` | ✅ remaining ignores are deliberate best-effort (`_ =` on audit writes, lock release on failure paths) |
| Goroutine leaks | ✅ every long-running goroutine (hub, scheduler, findings consumer, worker pool, WS pumps) exits on `ctx.Done()`; `RunBackground` waits for all of them |
| `context` propagation | ✅ request contexts flow to GORM/Redis; post-response cleanup uses `context.WithoutCancel` |

### L1 · Low · Worker back-off ignored shutdown — ✅ Fixed
`internal/worker/worker.go:80` — the stream-read error path used `time.Sleep(2s)`, which delays shutdown.
```go
// before
time.Sleep(2 * time.Second)
// after
select {
case <-ctx.Done():
	return nil
case <-time.After(2 * time.Second):
}
```

### L4 · Low · Audit write error silently dropped; sentinel compared with `==` — ✅ Fixed
`internal/handlers/auth.go:33` — now `errors.Is(...)`, and the failed-login audit write is an explicit `_ =` (best effort; the login result must not depend on the audit log).

---

## 2. React / TypeScript

| Check | Result |
|---|---|
| `any` misuse | ✅ none; `strict`, `noUnusedLocals`, `noImplicitReturns` on; `tsc -b` clean |
| Missing `useEffect` deps / stale closures | ✅ callbacks reach effects through refs (`useFullscreenLock.ts:11`, `useTabSwitchDetect`, `useSocket`, `CountdownTimer`); parent callbacks are `useCallback`-stable so effects don't re-run (the camera isn't reopened on re-render) |
| Leaks on unmount | ✅ media tracks stopped, intervals cleared, object URLs revoked (`StudentTile.tsx:24`), WebSocket closed and reconnect timer cancelled (`useSocket.ts:13`) |

### H2 · High · Pending answers could leak to the next student on a shared PC — ✅ Fixed
`web/src/lib/offlineQueue.ts:25`, `web/src/pages/StudentExam.tsx:331`.
The offline answer queue persisted to a single `sessionStorage` key. Suppose
student A left with unsent answers (offline at submit) and student B then
verified in the same tab. B's queue loaded A's answers and flushed them into
B's session, because question IDs are shared within an exam.
```ts
// before
const STORAGE_KEY = 'es_pending_answers'
new AnswerQueue(save, isFatal)
// after: one key per exam session
new AnswerQueue(save, isFatal, `es_pending_answers:${initial.session_id}`)
```
Test: `web/src/lib/lib.test.ts` → "does not hand one session's pending answers to the next session".

### L8 · Low · Audio clips are approximate — 🟡 Accepted
`web/src/lib/media.ts:72` samples an `AnalyserNode` on a timer. Timer jitter can drop or overlap a few milliseconds. That is acceptable for voice-activity detection. A precise recorder would need an AudioWorklet module.

---

## 3. SQL

| Check | Result |
|---|---|
| Missing indexes | ✅ every hot query is covered: gate `idx_status_window`; grid/alerts `uq_exam_student` prefix + `idx_session_time`; retention `idx_findings_created`; refresh `uq_refresh_hash`; answers `uq_session_q` + `idx_answers_question` |
| N+1 queries | ✅ live grid = 3 queries for any class size (`repositories/sessions.go:61`); exam list counts questions in one grouped query |
| Missing transactions | ✅ violation + integrity (`violations.go:44`), submit + grade (`sessions.go:571`), review + rescore, lifecycle transitions (`SELECT … FOR UPDATE`), refresh rotation |
| SQLi in raw GORM | ✅ no string concatenation; `gorm.Expr` with bound args; LIKE input escaped with `ESCAPE '!'` (`repositories/students.go:33`) |
| ON DELETE behaviour | ✅ see H7/M6/M7 below |

### H7 · High · Draft schema: `answers.question_id` had no table to reference — ✅ Fixed
The draft had no `questions` table, so answers could neither be validated nor graded, and orphans were possible.
```sql
-- after (migrations/000001_init.up.sql:123)
CREATE TABLE questions ( ... exam_id BIGINT NOT NULL, FOREIGN KEY (exam_id) REFERENCES exams(id) ON DELETE CASCADE );
CONSTRAINT fk_answers_question FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
```

### M6 · Medium · `lan_cidr VARCHAR(18)` cannot hold IPv6 — ✅ Fixed
`migrations/000001_init.up.sql:37` — `VARCHAR(43)` fits `ffff:…:ffff/128`.

### M7 · Medium · `integrity_score DEFAULT 0` — ✅ Fixed
The score is deductive (100 − penalties), so the draft default made every new session look fully compromised. `migrations/000001_init.up.sql:70` now uses `DEFAULT 100`.

Other schema changes, with reasons:
- `exams.created_by` → `ON DELETE RESTRICT` (explicit). Staff are deactivated, never deleted.
- `violations.reviewed_by` → `ON DELETE SET NULL`.
- Added `refresh_tokens` (rotation) and `audit_logs`.
- Added `CHECK` constraints for username, matric number, and the exam window.
- `DATETIME(3)` everywhere, stored in UTC.

### M10 · Medium · Migration not executed against MySQL — ⚠️ Not verified
The SQL is MySQL 8.0.16+ syntax (`CHECK … REGEXP_LIKE(…, 'c')`). Tests run on SQLite via AutoMigrate of the same models. **Before first use:** `docker compose up migrate`, then `SHOW CREATE TABLE` on each table.

---

## 4. LAN gate

| Check | Result |
|---|---|
| `ClientIP()` = `127.0.0.1` behind Nginx | ✅ see H8 |
| IPv6 | ✅ zone IDs stripped, IPv4-mapped IPv6 normalised to 4 bytes, IPv6 CIDRs accepted (`netutil/lan.go:16`), tested |
| CIDR parse errors | ✅ see C2 |
| VPN bypass | ✅ `LAN_DENY_CIDRS` excludes VPN pools inside the LAN range (`lan_gate.go:80`), tested |
| Trusted-proxy misconfig | ✅ forwarded headers ignored unless the TCP peer is in `TRUSTED_PROXIES`; Nginx overwrites (never appends) `X-Forwarded-For` |
| PIN brute force | ✅ 10 wrong PINs / 5 min / IP → 429 (`lan_gate.go:51`); constant-time compare (`lan_gate.go:181`) |
| Two exams on one subnet | ✅ PIN routes the student; opening a second overlapping exam without PINs on both is refused (`exams.go:276`) |

### C1 · Critical · `http://<LAN_IP>` cannot use the webcam — ✅ Fixed
Browsers expose `navigator.mediaDevices.getUserMedia` only in a **secure
context** (HTTPS, or localhost). The prompt's access model — students type
`http://192.168.1.45` — would make face verification and webcam proctoring
impossible for every student.
- `cmd/api/main.go:126`: native TLS (`TLS_CERT_FILE`/`TLS_KEY_FILE`) plus an HTTP→HTTPS redirect, so students can still type the bare IP. The redirect only reflects IP literals, which prevents an open redirect via `Host`.
- `deploy/nginx/templates/default.conf.template`: port 80 → 308 → 443.
- `scripts/gen-lan-cert.sh`: local CA + IP-SAN certificate. Install `ca.crt` on hall PCs once to avoid warnings.
- `web/src/lib/media.ts:11` + `Locked.tsx`: an explicit `INSECURE_CONTEXT` screen instead of a silent failure.
```ts
// after
export function mediaAvailable(): boolean {
  return typeof window !== 'undefined' && window.isSecureContext && !!navigator.mediaDevices?.getUserMedia
}
```

### C2 · Critical · Draft sketch ignores `net.ParseCIDR` error → nil-pointer panic — ✅ Fixed
```go
// before (prompt sketch)
_, lanNet, _ := net.ParseCIDR(cfg.LANCIDR)   // lanNet == nil on bad input
... lanNet.Contains(ip)                       // panics on every request
// after: internal/config/config.go — parse at startup, refuse to boot on error
if c.LANCIDRs, err = parseCIDRs(list("LAN_CIDRS")); err != nil {
	return nil, fmt.Errorf("LAN_CIDRS: %w", err)
}
```
Each exam's `lan_cidr` is validated on create and update (private range, not broader than /8, must overlap the server LAN). A stored bad value is logged and skipped by the gate; it does not panic.

### H8 · High · `ClientIP()` behind a proxy — ✅ Fixed
Without trusted-proxy configuration, Gin's `ClientIP()` behind Nginx returns
the proxy address for every request. Depending on configuration, that either
blocks all students or admits all clients, including spoofers.
```go
// internal/app/router.go:30
if len(a.Cfg.TrustedProxies) > 0 {
	_ = r.SetTrustedProxies(a.Cfg.TrustedProxies)
	r.ForwardedByClientIP = true
	r.RemoteIPHeaders = []string{"X-Real-IP", "X-Forwarded-For"}
} else {
	_ = r.SetTrustedProxies(nil) // ClientIP() == TCP peer
	r.ForwardedByClientIP = false
}
```
Tests: `TestNetworkIgnoresSpoofedForwardedForWithoutTrustedProxy` and `TestNetworkHonoursTrustedProxy`. With no trusted proxy configured, a misconfigured deployment fails **closed**.

### M1 · Medium · Exam subnet not re-checked after entry — ✅ Fixed
In-exam routes checked only the global LAN, so a student could move to another LAN subnet outside the exam's `lan_cidr`. `internal/services/sessions.go:424` now re-checks it on every in-exam call and returns `403 LAN_REQUIRED`.

### L6 · Low · Container health probe blocked by the gate — ✅ Fixed
`internal/middleware/lan_gate.go:103` exempts loopback `/healthz` only.

---

## 5. Auth

| Check | Result |
|---|---|
| Weak JWT secret | ✅ production refuses < 32 bytes or placeholder values (`config.go:205`); dev generates a random per-process secret |
| `alg` confusion / `none` | ✅ algorithm pinned to HS256 + issuer + audience + `typ` + `exp` required (`auth/tokens.go:132`); tests cover `none`, HS512, wrong key, and staff↔student token swap |
| bcrypt cost | ✅ 12 (production refuses < 12); constant-time path for unknown users (`services/auth.go:45`) |
| Refresh rotation | ✅ opaque 256-bit token, SHA-256 stored, rotated on each use; reuse of a rotated token revokes the whole family (`services/auth.go:136`); tested |
| Regex bypass | ✅ Go regexps are anchored, ASCII `\d`, inputs length-capped; tests cover trailing `\n`, full-width digits, SQL-like strings |
| Login throttle | ✅ 5 failures / 15 min / username + 30 / IP, Redis-backed and **fail-closed** (`services/auth.go:71`) |

### H4 · High · Mock staff with published passwords could reach production — ✅ Fixed
`cmd/seed/main.go:40` and `internal/app/app.go:92` refuse to seed when `APP_ENV=production` unless `-force` is passed explicitly.

### H5 · High · Spec-mandated password policy is weak — 🟡 Accepted (spec requirement)
`internal/auth/validate.go:15`: "uppercase letters, ≥ 6" is about 3×10⁸ combinations, and the mock passwords are surnames. Mitigations in place: bcrypt 12, per-username and per-IP throttling, LAN-only exposure, and refresh-token revocation on password change. **Recommendation:** raise the minimum to 12+ characters with mixed classes, or add TOTP for admins.

### M2 · Medium · Per-username lockout can be used to lock out a staff member — 🟡 Accepted
Anyone on the LAN who knows a staff ID can trigger the 15-minute lockout. This is the standard trade-off of per-account throttling. The lockout is time-boxed, and the audit log records `login_failed` with the source IP.

### M3 · Medium · Access tokens live up to 15 min after deactivation — 🟡 Accepted
Refresh tokens are revoked immediately on deactivation or password change. The short access TTL bounds the window.

### M5 · Medium · Device fingerprint is client-supplied — 🟡 Accepted
`X-Device-FP` is a lock key, not an authenticator. The session cookie is bound to it, rejoining on a new device requires a proctor unlock plus face re-verification, and the random component distinguishes identically imaged lab PCs (`services/sessions.go:51`).

---

## 6. AI

| Check | Result |
|---|---|
| ONNX model version drift | ✅ input/output shapes and names checked at load (`ai/onnx.go:212`); a mismatching model is rejected and that capability falls back |
| Nil / empty inference output | ✅ empty outputs are errors (`ai/onnx.go:252`); YOLO decoder bounds-checks (tested with truncated output) |
| CPU fallback | ✅ CUDA is opt-in; if the CUDA provider can't be appended, the session stays on CPU (`ai/onnx.go:193`); any missing model falls back to pure Go per capability |
| Empty / hostile frames | ✅ nil or < 8 px rejected (`ai/engine.go:200`); dimensions checked before decode (decompression-bomb guard, `imageops.go:16`); frames capped at 512 KB |
| GPU OOM | ✅ inference errors fail that job only; it is ACKed and logged, and the worker continues |

### C3 · Critical · Ported Go face matcher could not recognise the same face — ✅ Fixed
Measured during E2E: the original Sobel-pixel embedding gave **cosine 0.44 for
the same photo** after a 1.2× rescale and JPEG re-encode. With the native
build (no ONNX), every student would have been refused at entry, and the
worker would have raised a stream of false CRITICAL `FACE_MISMATCH` alerts.
- `internal/ai/fallback.go:78`: replaced with a spatial uniform-LBP histogram descriptor (Hellinger-normalised). Same-face cosine is now **0.90**.
- `internal/services/face_match.go:53`: `FACE_MATCH_ENFORCE=auto` lets only the high-assurance model (ArcFace) block entry. With the Go fallback the student is admitted and `FACE_REVIEW_REQUIRED` (HIGH) asks a proctor to compare the entry selfie and roster photo, which are shown side by side in the session drawer.
- `internal/worker/worker.go:239`: live `FACE_MISMATCH` is raised only by the high-assurance model.
```go
// before
if score < limit { flags = FACE_MISMATCH }      // fired on the student's own face
// after
if w.matcher.Assurance() == "high" && !w.matcher.Matches(score) { ... }
```

### M8 · Medium · "face-match score ≥ 0.75" is not an ArcFace cosine threshold — ✅ Fixed
Same-person ArcFace cosines are typically 0.35–0.75, so a raw 0.75 cut would reject most genuine students. Scores are now **calibrated** with a logistic curve per backend (`services/face_match.go`, mid 0.30 / k 15 for ArcFace), and 0.75 applies to the calibrated score (≈ cosine 0.37).

### H6 · High · ONNX path not compiled or run in the build environment — ⚠️ Not verified
`internal/ai/onnx.go` (`//go:build onnx`) needs cgo and libonnxruntime 1.19.0. The build machine has no C compiler and no Docker daemon. All pre- and post-processing (letterbox, YOLO decode + NMS, L2CS bins, 6DRepNet rotation→Euler) lives in untagged files and **is** unit-tested; only the thin session wrapper is unverified. The URLs in `scripts/download-models.sh` were not exercised either. **Before production:** run `docker build .`, start with the models mounted, check `GET /api/staff/system` → `ai_backends`, and pin model SHA-256s.

---

## 7. Concurrency

| Check | Result |
|---|---|
| WebSocket hub races | ✅ a single goroutine owns all room state; producers only send on channels; slow clients are dropped, never block (`realtime/hub.go:45`); tested with concurrent publishers |
| Redis lock release without Lua | ✅ release, refresh and the atomic device↔student claim are all Lua compare-and-set (`store/redis.go:66`, `:87`); the rate limiter's INCR+PEXPIRE is atomic (`:164`) |
| Deadlocks | ✅ no nested mutexes; DB row locks taken in one order (exam → session); hub sends use `select` with ctx |

### H1 · High · Un-ACKed stream messages orphaned on restart — ✅ Fixed
Consumer names include the PID, so a restarted API or worker never re-read its predecessor's pending entries, and the AI findings in them (violations) were lost. `internal/store/stream.go:132` `ClaimStale` (XAUTOCLAIM, idle ≥ 60 s) now runs every 30 s in both the findings consumer (`services/background.go:45`) and the worker (`worker/worker.go:67`). Test: `TestClaimStaleRecoversOrphans`.

### M4 · Medium · At-least-once findings may duplicate `ai_findings` rows — 🟡 Accepted
If persisting a findings message fails half-way, redelivery can insert the AI-finding rows again. Duplicate *violations* are suppressed by the per-event cooldown. Duplicate raw findings only affect evidence volume.

### M11 · Medium · `go test -race` not run — ⚠️ Not verified
The race detector needs cgo. Run `CGO_ENABLED=1 go test -race ./...` in CI on Linux.

### M13 · Medium · Hub may drop events under burst — 🟡 Accepted
`Publish` never blocks a request (`realtime/hub.go:98`). If its 1024-event buffer is full, events are dropped and logged. The dashboard re-polls sessions every 10 s, so state always reconciles.

---

## 8. DevOps

| Check | Result |
|---|---|
| Hardcoded secrets | ✅ none in code or compose; compose uses `${VAR:?}` so it refuses to start without them |
| `.env.example` | ✅ every variable, with comments |
| MySQL `max_connections` | ✅ 300 (`docker-compose.yml:47`); API pool 50 open / 10 idle with lifetimes (`store/db.go:44`) |
| MinIO public bucket | ✅ private (no policy); all media served through staff-authenticated endpoints; console bound to 127.0.0.1 (`docker-compose.yml:86`) |
| Migrations | ✅ embedded in the binary; run by a one-shot `migrate` service; multi-statement mode only on that dedicated connection (`store/db.go:85`), never on the app pool |

### L5 · Low · Undocumented `BEHIND_PROXY` switch — ✅ Removed.

### M-Docker · Medium · Dockerfile / Compose never built here — ⚠️ Not verified
Image tags are pinned. `nginx` uses host networking so it sees real client IPs on Linux; on Docker Desktop (Windows/macOS) source IPs are NAT-ed — use standalone mode there.

---

## 9. Edge cases

| Case | Handling |
|---|---|
| Student rejoins (reload, crash) | Cookie resumes the session (`GET /api/student/session`); without the cookie, same device + face re-verify resumes; a different device needs a proctor **Unlock** |
| Exam ends mid-answer | Server accepts saves for `SUBMIT_GRACE` (30 s); client auto-submits at 0 after flushing; scheduler auto-closes and auto-submits every open session (`services/background.go:166`); tested |
| Network drop | Answers queue locally and retry with back-off; heartbeat re-acquires expired locks if nobody else took them (`sessions.go:466`) |
| Duplicate matric | `UNIQUE(matric_no)`; CSV import rejects in-file duplicates with line numbers (`roster.go:100`); re-import updates names |
| Two exams on one subnet | PIN routing; conflict check when opening |

### H3 · High · Extending an active exam logged students out at the old end time — ✅ Fixed
The student token's `exp` was fixed at verification (`ends_at + grace`). `internal/handlers/session.go:106`: the heartbeat now re-issues the cookie when `ends_at` moved. Test: `TestExtensionReissuesStudentToken`.

### L7 · Low · Two simultaneous verifies for one student → 500 — 🟡 Accepted
The unique key rejects the second insert. The UI disables the button while verifying.

---

## 10. Compliance

| Check | Result |
|---|---|
| Consent capture | ✅ required checkbox and notice; `consent_at` + `consent_version` stored per session; `CONSENT_REQUIRED` otherwise |
| PII in logs | ✅ request log has method, path **without query string**, status, IP — never names, matric numbers or bodies (`middleware/misc.go:15`); CSV export guards against formula injection (`services/proctor.go:176`) |
| Retention policy | ✅ `RETENTION_DAYS` (default 30): an hourly janitor deletes frames, audio, verification images and AI findings of closed exams (`services/background.go:192`); manual purge endpoint |
| Deletion endpoint | ✅ `DELETE /api/staff/students/:id` (admin) erases the student, sessions, answers, violations, findings and every stored image (`services/roster.go:150`); `GET …/export` for subject access |

### M12 · Medium · Violations, answers and audit logs have no automatic expiry — 🟡 Accepted
They are the academic record and are kept until the student is erased. Set an institutional schedule. Bulk erasure is per student today.

### L9 · Low · Standalone mode uses one SQLite connection — 🟡 Accepted
Intended for a single hall; use distributed mode for larger deployments.

### L-SocketIO · Low · Native WebSocket instead of Socket.IO — 🟡 Accepted (deviation)
The Go hub speaks RFC 6455 WebSocket via gorilla/websocket. A Socket.IO client can't talk to it, and Go has no maintained Socket.IO v4 server. `useSocket` implements reconnect with back-off and one-time tickets.

### L2/L3 · Low · Content-Type and CSP — ✅ Fixed
Media responses now sniff the content type (PNG roster photos). CSP `connect-src` is narrowed from `'self' ws: wss:` to `'self'`.
