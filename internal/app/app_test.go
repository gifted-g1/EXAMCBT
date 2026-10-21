package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"examshield/internal/config"
	"examshield/internal/models"
)

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	app    *App
	client *http.Client
	token  string
	face   []byte
}

func newHarness(t *testing.T, opts ...func(*config.Config)) *harness {
	t.Helper()
	dir := t.TempDir()
	_, loop, _ := net.ParseCIDR("127.0.0.0/8")
	cfg := &config.Config{
		Env: "test", Mode: config.ModeStandalone, HTTPAddr: "127.0.0.1:0",
		SQLitePath: filepath.Join(dir, "es.db"), BlobDir: filepath.Join(dir, "blobs"), SeedOnStart: true,
		JWTSecret: []byte("0123456789abcdef0123456789abcdef"), AccessTTL: 15 * time.Minute, RefreshTTL: time.Hour,
		BcryptCost: 4, LANCIDRs: []*net.IPNet{loop}, LANAdvertiseIP: "192.168.1.45", PublicPort: 80,
		AllowLoopbackStaff: true, AllowLoopbackExam: true,
		SessionLockTTL: time.Minute, SubmitGrace: 30 * time.Second, FaceMatchThreshold: 0.75, FaceMatchEnforce: "auto",
		FrameMinInterval: 2 * time.Second, AIWorkers: 2, AIBackend: "go", ModelDir: "../../models",
		InProcessAI: true, RetentionDays: 30, ConsentVersion: "test",
	}
	for _, o := range opts {
		o(cfg)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	a, err := Build(ctx, cfg, log)
	require.NoError(t, err)
	done := a.RunBackground(ctx)
	srv := httptest.NewServer(NewRouter(ctx, a))
	t.Cleanup(func() {
		srv.Close()
		cancel()
		<-done
		a.Close()
	})
	face, err := os.ReadFile("../testdata/face.jpg")
	require.NoError(t, err)
	jar, _ := cookiejar.New(nil)
	return &harness{t: t, srv: srv, app: a, client: &http.Client{Jar: jar}, face: face}
}

type resp struct {
	Code int
	Body map[string]any
	Raw  string
}

func (h *harness) do(method, path string, body any, hdr map[string]string) resp {
	h.t.Helper()
	var rd io.Reader
	ctype := ""
	switch b := body.(type) {
	case nil:
	case *multipart.Writer:
		panic("use doMultipart")
	case []byte:
		rd, ctype = bytes.NewReader(b), "application/octet-stream"
	default:
		buf, _ := json.Marshal(b)
		rd, ctype = bytes.NewReader(buf), "application/json"
	}
	req, err := http.NewRequest(method, h.srv.URL+path, rd)
	require.NoError(h.t, err)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if h.token != "" && strings.HasPrefix(path, "/api/staff") {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return h.send(req)
}

func (h *harness) send(req *http.Request) resp {
	h.t.Helper()
	res, err := h.client.Do(req)
	require.NoError(h.t, err)
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{Code: res.StatusCode, Raw: string(raw)}
	_ = json.Unmarshal(raw, &out.Body)
	return out
}

func (h *harness) multipart(path string, fields map[string]string, files map[string][]byte, hdr map[string]string) resp {
	h.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	for k, v := range files {
		w, _ := mw.CreateFormFile(k, k+".jpg")
		_, _ = w.Write(v)
	}
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if h.token != "" && strings.HasPrefix(path, "/api/staff") {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return h.send(req)
}

func (h *harness) login(user, pass string) {
	r := h.do(http.MethodPost, "/api/staff/auth/login", map[string]string{"username": user, "password": pass}, nil)
	require.Equal(h.t, http.StatusOK, r.Code, r.Raw)
	h.token = r.Body["access_token"].(string)
}

func num(v any) int64 { return int64(v.(float64)) }

const devA = "device-A-0123456789abcdef"
const devB = "device-B-0123456789abcdef"

func fp(d string) map[string]string { return map[string]string{"X-Device-FP": d} }

func TestFullExamLifecycle(t *testing.T) {
	h := newHarness(t)
	h.login("20010078", "MUSTAPHA") // admin

	// Roster: look up the seeded student and give them a reference photo.
	r := h.do(http.MethodGet, "/api/staff/students?q=IFT/22/9203", nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	stID := num(r.Body["students"].([]any)[0].(map[string]any)["id"])
	r = h.multipart(fmt.Sprintf("/api/staff/students/%d/photo", stID), nil, map[string][]byte{"photo": h.face}, nil)
	require.Equal(t, http.StatusNoContent, r.Code, r.Raw)

	// Exam: create, add questions, open.
	now := time.Now().UTC()
	r = h.do(http.MethodPost, "/api/staff/exams", map[string]any{"title": "Networks 101",
		"starts_at": now.Add(-time.Minute), "ends_at": now.Add(time.Hour), "lan_cidr": "127.0.0.0/8"}, nil)
	require.Equal(t, http.StatusCreated, r.Code, r.Raw)
	examID := num(r.Body["id"])
	r = h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/questions", examID), map[string]any{"questions": []map[string]any{
		{"prompt": "Which layer routes?", "options": []string{"Link", "Network"}, "correct_option": 1, "marks": 2},
		{"kind": "TEXT", "prompt": "Explain CIDR."},
	}}, nil)
	require.Equal(t, http.StatusCreated, r.Code, r.Raw)
	q1 := num(r.Body["questions"].([]any)[0].(map[string]any)["id"])

	// Students cannot get in before it opens.
	r = h.do(http.MethodGet, "/api/student/gate", nil, nil)
	assert.Equal(t, http.StatusLocked, r.Code)

	r = h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/access-card", examID), nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, "http://192.168.1.45", r.Body["url"])
	assert.NotEmpty(t, r.Body["qr_png_base64"])

	// Student enters: gate, then verify with consent + selfie.
	r = h.do(http.MethodGet, "/api/student/gate", nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, "Networks 101", r.Body["title"])

	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9203", "consent": "false"},
		map[string][]byte{"selfie": h.face}, fp(devA))
	assert.Equal(t, http.StatusBadRequest, r.Code)
	assert.Equal(t, "CONSENT_REQUIRED", r.Body["error"])

	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "ift/22/9203", "consent": "true"},
		map[string][]byte{"selfie": h.face}, fp(devA))
	require.Equal(t, 200, r.Code, r.Raw)
	assert.GreaterOrEqual(t, r.Body["face_match_score"].(float64), 0.75)
	state := r.Body["state"].(map[string]any)
	assert.NotContains(t, r.Raw, "correct_option", "answers must never reach students")
	assert.Len(t, state["questions"], 2)

	// Device lock: another student on device A is refused.
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9101", "consent": "true"},
		map[string][]byte{"selfie": h.face}, fp(devA))
	assert.Equal(t, http.StatusConflict, r.Code)
	assert.Equal(t, "DEVICE_IN_USE", r.Body["error"])

	// Student lock: same student from device B is refused.
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9203", "consent": "true"},
		map[string][]byte{"selfie": h.face}, fp(devB))
	assert.Equal(t, http.StatusConflict, r.Code)
	assert.Equal(t, "DEVICE_MISMATCH", r.Body["error"])

	// Cookie replayed from a different device fingerprint is rejected.
	r = h.do(http.MethodGet, "/api/student/session", nil, fp(devB))
	assert.Equal(t, http.StatusUnauthorized, r.Code)

	r = h.do(http.MethodGet, "/api/student/session", nil, fp(devA))
	require.Equal(t, 200, r.Code, r.Raw)

	// Answers.
	r = h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": q1, "response": "1"}, fp(devA))
	require.Equal(t, 200, r.Code, r.Raw)
	r = h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": q1, "response": "7"}, fp(devA))
	assert.Equal(t, http.StatusBadRequest, r.Code)
	r = h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": 99999, "response": "x"}, fp(devA))
	assert.Equal(t, http.StatusBadRequest, r.Code)

	// Client events: allowed types only, with cooldown.
	r = h.do(http.MethodPost, "/api/student/events", map[string]any{"type": "TAB_SWITCH"}, fp(devA))
	assert.Equal(t, http.StatusAccepted, r.Code, r.Raw)
	r = h.do(http.MethodPost, "/api/student/events", map[string]any{"type": "TAB_SWITCH"}, fp(devA))
	assert.Equal(t, http.StatusAccepted, r.Code)
	r = h.do(http.MethodPost, "/api/student/events", map[string]any{"type": "FACE_MISMATCH"}, fp(devA))
	assert.Equal(t, http.StatusBadRequest, r.Code, "browser cannot forge AI events")

	// Frames: accepted, then throttled; the in-process worker analyses it.
	r = h.do(http.MethodPost, "/api/student/frames", h.face, fp(devA))
	assert.Equal(t, http.StatusAccepted, r.Code, r.Raw)
	r = h.do(http.MethodPost, "/api/student/frames", h.face, fp(devA))
	assert.Equal(t, http.StatusTooManyRequests, r.Code)
	r = h.do(http.MethodPost, "/api/student/frames", []byte("garbage"), fp(devA))
	assert.Equal(t, http.StatusBadRequest, r.Code)

	var sessionID int64
	require.Eventually(t, func() bool {
		var n int64
		h.app.DB.Model(&models.AIFinding{}).Count(&n)
		return n > 0
	}, 10*time.Second, 100*time.Millisecond, "AI findings should be persisted via the findings stream")

	// Live grid.
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/sessions", examID), nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	row := r.Body["sessions"].([]any)[0].(map[string]any)
	sessionID = num(row["id"])
	assert.EqualValues(t, 1, row["violations_count"], "cooldown must collapse duplicate TAB_SWITCH")
	assert.EqualValues(t, 1, row["answered_count"])
	assert.Equal(t, 97.0, row["integrity_score"])

	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/sessions/%d/image/latest", sessionID), nil, nil)
	assert.Equal(t, 200, r.Code)

	// Review: dismissing restores integrity.
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/violations", examID), nil, nil)
	require.Equal(t, 200, r.Code)
	vid := num(r.Body["violations"].([]any)[0].(map[string]any)["id"])
	r = h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/violations/%d/review", examID, vid), map[string]string{"status": "DISMISSED"}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, 100.0, r.Body["integrity_score"])

	// Submit, then nothing more is accepted.
	r = h.do(http.MethodPost, "/api/student/submit", nil, fp(devA))
	require.Equal(t, 200, r.Code, r.Raw)
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9203", "consent": "true"},
		map[string][]byte{"selfie": h.face}, fp(devA))
	assert.Equal(t, "ALREADY_SUBMITTED", r.Body["error"])

	// Device A is free again for the next student after submit (lock released).
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9101", "consent": "true"},
		map[string][]byte{"selfie": h.face}, fp(devA))
	require.Equal(t, 200, r.Code, "low-assurance matcher admits and asks a proctor to check: %s", r.Raw)
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/violations", examID), nil, nil)
	assert.Contains(t, r.Raw, "FACE_REVIEW_REQUIRED")

	// Report.
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/report", examID), nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	var rep map[string]any
	for _, row := range r.Body["rows"].([]any) {
		if row.(map[string]any)["matric_no"] == "IFT/22/9203" {
			rep = row.(map[string]any)
		}
	}
	require.NotNil(t, rep)
	assert.Equal(t, 2.0, rep["score"])
	assert.Equal(t, 3.0, rep["max_score"])
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/report?format=csv", examID), nil, nil)
	assert.Contains(t, r.Raw, "IFT/22/9203")

	// Close: students locked out.
	r = h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "close"}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	r = h.do(http.MethodGet, "/api/student/gate", nil, nil)
	assert.Equal(t, http.StatusLocked, r.Code)

	// Right to erasure removes the student and cascades.
	r = h.do(http.MethodDelete, fmt.Sprintf("/api/staff/students/%d", stID), nil, nil)
	require.Equal(t, http.StatusNoContent, r.Code, r.Raw)
	var n int64
	h.app.DB.Model(&models.ExamSession{}).Where("id = ?", sessionID).Count(&n)
	assert.Zero(t, n)
	h.app.DB.Model(&models.AIFinding{}).Where("session_id = ?", sessionID).Count(&n)
	assert.Zero(t, n)
}

func TestAutoSubmitOnClose(t *testing.T) {
	h := newHarness(t)
	h.login("20010078", "MUSTAPHA")
	r := h.do(http.MethodGet, "/api/staff/students?q=IFT/22/9203", nil, nil)
	stID := num(r.Body["students"].([]any)[0].(map[string]any)["id"])
	h.multipart(fmt.Sprintf("/api/staff/students/%d/photo", stID), nil, map[string][]byte{"photo": h.face}, nil)
	now := time.Now().UTC()
	r = h.do(http.MethodPost, "/api/staff/exams", map[string]any{"title": "Auto close", "starts_at": now.Add(-time.Minute),
		"ends_at": now.Add(time.Hour), "lan_cidr": "127.0.0.0/8", "require_pin": true}, nil)
	examID := num(r.Body["id"])
	pin := r.Body["pin_code"].(string)
	h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/questions", examID), map[string]any{"prompt": "Q", "options": []string{"a", "b"}, "correct_option": 0}, nil)
	h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil)

	hdr := fp(devA)
	hdr["X-Exam-PIN"] = pin
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9203", "consent": "true"}, map[string][]byte{"selfie": h.face}, hdr)
	require.Equal(t, 200, r.Code, r.Raw)

	r = h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "close"}, nil)
	require.Equal(t, 200, r.Code)
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/sessions", examID), nil, nil)
	row := r.Body["sessions"].([]any)[0].(map[string]any)
	assert.Equal(t, "SUBMITTED", row["status"])
	assert.Equal(t, true, row["auto_submitted"])

	// Answers after close are refused.
	r = h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": 1, "response": "0"}, fp(devA))
	assert.Equal(t, http.StatusGone, r.Code, r.Raw)
}

func TestLoginThrottleAndRefreshReuse(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 5; i++ {
		r := h.do(http.MethodPost, "/api/staff/auth/login", map[string]string{"username": "19990045", "password": "WRONGPASS"}, nil)
		require.Equal(t, http.StatusUnauthorized, r.Code)
	}
	r := h.do(http.MethodPost, "/api/staff/auth/login", map[string]string{"username": "19990045", "password": "OLAWALE"}, nil)
	assert.Equal(t, http.StatusTooManyRequests, r.Code, "6th attempt within 15 min is throttled even with the right password")

	h.login("19980012", "ADEGOKE")
	u, _ := neturl.Parse(h.srv.URL + "/api/staff/auth/refresh")
	first := h.client.Jar.Cookies(u)
	require.NotEmpty(t, first)

	r = h.do(http.MethodPost, "/api/staff/auth/refresh", nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)

	// Replay the first (rotated) refresh token: rejected, and the family dies.
	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/api/staff/auth/refresh", nil)
	plain := &http.Client{}
	for _, c := range first {
		req.AddCookie(c)
	}
	res, err := plain.Do(req)
	require.NoError(t, err)
	res.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)

	r = h.do(http.MethodPost, "/api/staff/auth/refresh", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, r.Code, "reuse detection revokes the current token too")
}

func TestRBAC(t *testing.T) {
	h := newHarness(t)
	h.login("19990045", "OLAWALE") // proctor
	assert.Equal(t, http.StatusForbidden, h.do(http.MethodGet, "/api/staff/users", nil, nil).Code)
	assert.Equal(t, http.StatusForbidden, h.do(http.MethodDelete, "/api/staff/students/1", nil, nil).Code)

	// The demo exam belongs to 19990045; another proctor may view but not manage it.
	h.login("19980012", "ADEGOKE")
	assert.Equal(t, 200, h.do(http.MethodGet, "/api/staff/exams/1", nil, nil).Code)
	assert.Equal(t, http.StatusForbidden, h.do(http.MethodPost, "/api/staff/exams/1/status", map[string]string{"action": "open"}, nil).Code)

	h.token = ""
	assert.Equal(t, http.StatusUnauthorized, h.do(http.MethodGet, "/api/staff/exams", nil, nil).Code)
}

func TestEnforcedFaceMatchRequiresReference(t *testing.T) {
	h := newHarness(t, func(c *config.Config) { c.FaceMatchEnforce = "always" })
	h.login("20010078", "MUSTAPHA")
	now := time.Now().UTC()
	r := h.do(http.MethodPost, "/api/staff/exams", map[string]any{"title": "Strict", "starts_at": now.Add(-time.Minute),
		"ends_at": now.Add(time.Hour), "lan_cidr": "127.0.0.0/8"}, nil)
	examID := num(r.Body["id"])
	h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/questions", examID), map[string]any{"prompt": "Q", "options": []string{"a", "b"}, "correct_option": 0}, nil)
	require.Equal(t, 200, h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil).Code)

	// No roster photo and no ID photo: refused when enforcement is on.
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9101", "consent": "true"}, map[string][]byte{"selfie": h.face}, fp(devA))
	assert.Equal(t, http.StatusUnprocessableEntity, r.Code)
	assert.Equal(t, "NO_REFERENCE_PHOTO", r.Body["error"])

	// With the ID card photo as reference it passes.
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9101", "consent": "true"},
		map[string][]byte{"selfie": h.face, "id_photo": h.face}, fp(devA))
	require.Equal(t, 200, r.Code, r.Raw)

	// A blank selfie never passes, whatever the policy.
	blank := bytes.Repeat([]byte{0}, 10)
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9187", "consent": "true"}, map[string][]byte{"selfie": blank}, fp(devB))
	assert.Equal(t, http.StatusBadRequest, r.Code)
}

func TestExtensionReissuesStudentToken(t *testing.T) {
	h := newHarness(t)
	h.login("20010078", "MUSTAPHA")
	r := h.do(http.MethodGet, "/api/staff/students?q=IFT/22/9203", nil, nil)
	stID := num(r.Body["students"].([]any)[0].(map[string]any)["id"])
	h.multipart(fmt.Sprintf("/api/staff/students/%d/photo", stID), nil, map[string][]byte{"photo": h.face}, nil)
	start, end := time.Now().UTC().Add(-time.Minute).Truncate(time.Second), time.Now().UTC().Add(30*time.Minute).Truncate(time.Second)
	r = h.do(http.MethodPost, "/api/staff/exams", map[string]any{"title": "Extend me", "starts_at": start, "ends_at": end, "lan_cidr": "127.0.0.0/8"}, nil)
	examID := num(r.Body["id"])
	h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/questions", examID), map[string]any{"prompt": "Q", "options": []string{"a", "b"}, "correct_option": 0}, nil)
	require.Equal(t, 200, h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil).Code)
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9203", "consent": "true"}, map[string][]byte{"selfie": h.face}, fp(devA))
	require.Equal(t, 200, r.Code, r.Raw)

	// Heartbeat before any change: no new cookie.
	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/api/student/heartbeat", nil)
	req.Header.Set("X-Device-FP", devA)
	res, err := h.client.Do(req)
	require.NoError(t, err)
	res.Body.Close()
	assert.Empty(t, res.Header.Values("Set-Cookie"))

	// Extend by an hour; the next heartbeat re-issues the session cookie.
	r = h.do(http.MethodPut, fmt.Sprintf("/api/staff/exams/%d", examID), map[string]any{"title": "Extend me",
		"starts_at": start, "ends_at": end.Add(time.Hour), "lan_cidr": "127.0.0.0/8"}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	req, _ = http.NewRequest(http.MethodPost, h.srv.URL+"/api/student/heartbeat", nil)
	req.Header.Set("X-Device-FP", devA)
	res, err = h.client.Do(req)
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, 200, res.StatusCode)
	cookies := res.Cookies()
	require.Len(t, cookies, 1)
	assert.WithinDuration(t, end.Add(time.Hour+30*time.Second), cookies[0].Expires, 2*time.Second)
}

func TestPDFQuestionImport(t *testing.T) {
	h := newHarness(t)
	h.login("20010078", "MUSTAPHA")

	// The example PDF is downloadable by staff.
	r := h.do(http.MethodGet, "/api/staff/questions/sample.pdf", nil, nil)
	require.Equal(t, 200, r.Code)
	require.True(t, strings.HasPrefix(r.Raw, "%PDF-"))
	sample := []byte(r.Raw)

	now := time.Now().UTC()
	r = h.do(http.MethodPost, "/api/staff/exams", map[string]any{"title": "From PDF", "starts_at": now,
		"ends_at": now.Add(time.Hour), "lan_cidr": "127.0.0.0/8"}, nil)
	examID := num(r.Body["id"])

	r = h.multipart(fmt.Sprintf("/api/staff/exams/%d/questions/parse-pdf", examID), nil, map[string][]byte{"file": sample}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	qs := r.Body["questions"].([]any)
	require.Len(t, qs, 6)
	assert.EqualValues(t, 1, r.Body["pages"])

	// Nothing was saved by the preview.
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/questions", examID), nil, nil)
	assert.Empty(t, r.Body["questions"])

	// Staff fill in the missing answer (question 4) and import.
	var in []map[string]any
	for _, q := range qs {
		m := q.(map[string]any)
		item := map[string]any{"kind": m["kind"], "prompt": m["prompt"], "marks": m["marks"]}
		if m["kind"] == "MCQ" {
			item["options"] = m["options"]
			item["correct_option"] = m["correct_option"]
			if m["correct_option"] == nil {
				item["correct_option"] = 2
			}
		}
		in = append(in, item)
	}
	r = h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/questions", examID), map[string]any{"questions": in}, nil)
	require.Equal(t, http.StatusCreated, r.Code, r.Raw)
	assert.Len(t, r.Body["questions"], 6)

	// Garbage and non-PDF files are rejected clearly.
	r = h.multipart(fmt.Sprintf("/api/staff/exams/%d/questions/parse-pdf", examID), nil, map[string][]byte{"file": []byte("not a pdf")}, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, r.Code)
	assert.Equal(t, "PDF_UNREADABLE", r.Body["error"])

	// Locked once the exam is open.
	require.Equal(t, 200, h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil).Code)
	r = h.multipart(fmt.Sprintf("/api/staff/exams/%d/questions/parse-pdf", examID), nil, map[string][]byte{"file": sample}, nil)
	assert.Equal(t, http.StatusConflict, r.Code)
}

func TestProctoringToggles(t *testing.T) {
	h := newHarness(t)
	h.login("20010078", "MUSTAPHA") // admin
	now := time.Now().UTC()
	r := h.do(http.MethodPost, "/api/staff/exams", map[string]any{"title": "Closed book, no camera",
		"starts_at": now.Add(-time.Minute), "ends_at": now.Add(time.Hour), "lan_cidr": "127.0.0.0/8",
		"proctoring": map[string]bool{"face_verification": false, "camera_monitoring": false, "tab_switch_detection": false}}, nil)
	require.Equal(t, http.StatusCreated, r.Code, r.Raw)
	examID := num(r.Body["id"])
	p := r.Body["proctoring"].(map[string]any)
	assert.Equal(t, false, p["camera_monitoring"])
	assert.Equal(t, true, p["audio_monitoring"], "unspecified switches default on")
	h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/questions", examID), map[string]any{"prompt": "Q", "options": []string{"a", "b"}, "correct_option": 0}, nil)
	require.Equal(t, 200, h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil).Code)

	// The gate tells the page what is required.
	r = h.do(http.MethodGet, "/api/student/gate", nil, nil)
	require.Equal(t, 200, r.Code)
	assert.Equal(t, false, r.Body["proctoring"].(map[string]any)["face_verification"])

	// Face verification off: matric + consent only, no selfie, no roster photo.
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9101", "consent": "true"}, nil, fp(devA))
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Nil(t, r.Body["face_match_score"])

	// Camera off: frames refused; microphone still on: audio accepted.
	r = h.do(http.MethodPost, "/api/student/frames", h.face, fp(devA))
	assert.Equal(t, http.StatusConflict, r.Code)
	assert.Equal(t, "FEATURE_DISABLED", r.Body["error"])
	r = h.do(http.MethodPost, "/api/student/audio", make([]byte, 32000), fp(devA))
	assert.Equal(t, http.StatusAccepted, r.Code, r.Raw)

	// Tab detection off: accepted but not recorded. Clipboard still on: recorded.
	assert.Equal(t, http.StatusAccepted, h.do(http.MethodPost, "/api/student/events", map[string]any{"type": "TAB_SWITCH"}, fp(devA)).Code)
	assert.Equal(t, http.StatusAccepted, h.do(http.MethodPost, "/api/student/events", map[string]any{"type": "COPY_PASTE"}, fp(devA)).Code)
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/violations", examID), nil, nil)
	assert.NotContains(t, r.Raw, "TAB_SWITCH")
	assert.Contains(t, r.Raw, "COPY_PASTE")

	// Toggle tab detection back on mid-exam; heartbeat delivers it and events count.
	r = h.do(http.MethodPut, fmt.Sprintf("/api/staff/exams/%d/proctoring", examID), map[string]bool{"tab_switch_detection": true}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	pr := r.Body["proctoring"].(map[string]any)
	assert.Equal(t, true, pr["tab_switch_detection"])
	assert.Equal(t, false, pr["camera_monitoring"], "unchanged switches keep their value")
	r = h.do(http.MethodPost, "/api/student/heartbeat", nil, fp(devA))
	require.Equal(t, 200, r.Code)
	assert.Equal(t, true, r.Body["proctoring"].(map[string]any)["tab_switch_detection"])
	h.do(http.MethodPost, "/api/student/events", map[string]any{"type": "TAB_SWITCH"}, fp(devA))
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/violations", examID), nil, nil)
	assert.Contains(t, r.Raw, "TAB_SWITCH")

	// Another proctor may not change someone else's exam.
	h.login("19980012", "ADEGOKE")
	r = h.do(http.MethodPut, fmt.Sprintf("/api/staff/exams/%d/proctoring", examID), map[string]bool{"camera_monitoring": true}, nil)
	assert.Equal(t, http.StatusForbidden, r.Code)
}

// A standalone server that restarts (e.g. after a power cut) must keep the
// same session key, or every student and proctor is logged out mid-exam.
func TestSessionKeySurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", ".jwt_secret")
	a, err := loadOrCreateSecret(path)
	require.NoError(t, err)
	b, err := loadOrCreateSecret(path)
	require.NoError(t, err)
	assert.Len(t, a, 64)
	assert.Equal(t, a, b)
}
