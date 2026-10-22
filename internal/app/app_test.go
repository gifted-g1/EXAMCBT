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
	"examshield/internal/services"
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

// Cloud/demo hosting: visitors arrive from public IPs through a load
// balancer. With ACCESS_MODE=open they can sit an exam; PIN still applies.
func TestOpenAccessModeBehindLoadBalancer(t *testing.T) {
	h := newHarness(t, func(c *config.Config) {
		c.AccessMode = "open"
		c.AllowLoopbackExam = false
		c.TrustedProxies = []string{"127.0.0.0/8"} // the "load balancer"
		c.PublicURL = "https://examcbt.onrender.com"
	})
	h.login("20010078", "MUSTAPHA")
	now := time.Now().UTC()
	r := h.do(http.MethodPost, "/api/staff/exams", map[string]any{"title": "Online demo",
		"starts_at": now.Add(-time.Minute), "ends_at": now.Add(time.Hour), "require_pin": true,
		"proctoring": map[string]bool{"face_verification": false}}, nil)
	require.Equal(t, http.StatusCreated, r.Code, r.Raw)
	assert.Equal(t, "0.0.0.0/0", r.Body["lan_cidr"], "no hall network in open mode")
	examID := num(r.Body["id"])
	pin := r.Body["pin_code"].(string)
	h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/questions", examID), map[string]any{"prompt": "Q", "options": []string{"a", "b"}, "correct_option": 0}, nil)
	require.Equal(t, 200, h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil).Code)

	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/access-card", examID), nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, "https://examcbt.onrender.com", r.Body["url"])
	assert.Equal(t, "https://examcbt.onrender.com/exam?pin="+pin, r.Body["qr_url"])

	public := map[string]string{"X-Forwarded-For": "203.0.113.9", "X-Device-FP": devA}
	r = h.do(http.MethodGet, "/api/student/gate", nil, public)
	assert.Equal(t, http.StatusUnauthorized, r.Code, "PIN still required")
	public["X-Exam-PIN"] = pin
	r = h.do(http.MethodGet, "/api/student/gate", nil, public)
	require.Equal(t, 200, r.Code, r.Raw)
	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9101", "consent": "true"}, nil, public)
	require.Equal(t, 200, r.Code, r.Raw)
	r = h.do(http.MethodPost, "/api/student/heartbeat", nil, public)
	assert.Equal(t, 200, r.Code, r.Raw)
}

func TestAnswerScriptAndItemAnalysis(t *testing.T) {
	h := newHarness(t)
	h.login("20010078", "MUSTAPHA")
	now := time.Now().UTC()
	r := h.do(http.MethodPost, "/api/staff/exams", map[string]any{"title": "Results", "starts_at": now.Add(-time.Minute),
		"ends_at": now.Add(time.Hour), "lan_cidr": "127.0.0.0/8", "proctoring": map[string]bool{"face_verification": false}}, nil)
	examID := num(r.Body["id"])
	r = h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/questions", examID), map[string]any{"questions": []map[string]any{
		{"prompt": "Q1", "options": []string{"a", "b", "c"}, "correct_option": 1, "marks": 2},
		{"prompt": "Q2", "options": []string{"a", "b", "c"}, "correct_option": 2},
		{"prompt": "Q3", "options": []string{"a", "b"}, "correct_option": 0},
		{"kind": "TEXT", "prompt": "Explain", "marks": 5},
	}}, nil)
	qs := r.Body["questions"].([]any)
	qid := func(i int) int64 { return num(qs[i].(map[string]any)["id"]) }
	require.Equal(t, 200, h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil).Code)

	sit := func(matric, device string, answers map[int64]string) {
		r := h.multipart("/api/student/verify", map[string]string{"matric_no": matric, "consent": "true"}, nil, fp(device))
		require.Equal(t, 200, r.Code, r.Raw)
		for q, a := range answers {
			require.Equal(t, 200, h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": q, "response": a}, fp(device)).Code)
		}
		require.Equal(t, 200, h.do(http.MethodPost, "/api/student/submit", nil, fp(device)).Code)
	}
	// A: Q1 right, Q2 wrong (a), Q3 skipped, written answer given.
	sit("IFT/22/9203", devA, map[int64]string{qid(0): "1", qid(1): "0", qid(3): "Because..."})
	// B: Q1 wrong (a), Q2 wrong (a), Q3 right.
	sit("IFT/22/9101", devB, map[int64]string{qid(0): "0", qid(1): "0", qid(2): "0"})

	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/report", examID), nil, nil)
	var sidA int64
	for _, row := range r.Body["rows"].([]any) {
		if row.(map[string]any)["matric_no"] == "IFT/22/9203" {
			sidA = num(row.(map[string]any)["session_id"])
		}
	}
	require.NotZero(t, sidA)

	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/sessions/%d/answers", sidA), nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, 2.0, r.Body["score"])
	assert.Equal(t, 9.0, r.Body["max_score"])
	counts := r.Body["counts"].(map[string]any)
	assert.EqualValues(t, 1, counts["correct"])
	assert.EqualValues(t, 1, counts["incorrect"])
	assert.EqualValues(t, 1, counts["unanswered"])
	assert.EqualValues(t, 1, counts["needs_marking"])
	items := r.Body["items"].([]any)
	first := items[0].(map[string]any)
	assert.Equal(t, "correct", first["status"])
	assert.EqualValues(t, 1, first["correct_option"])
	assert.Equal(t, "1", first["response"])
	assert.Equal(t, "Because...", items[3].(map[string]any)["response"])
	assert.Nil(t, items[3].(map[string]any)["awarded"], "written answers wait for an examiner")

	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/item-analysis", examID), nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.EqualValues(t, 2, r.Body["candidates"])
	stats := r.Body["questions"].([]any)
	q2 := stats[1].(map[string]any)
	assert.EqualValues(t, 0, q2["correct"])
	assert.EqualValues(t, 2, q2["incorrect"])
	assert.Equal(t, 0.0, q2["percent_right"])
	assert.EqualValues(t, 0, q2["common_wrong"], "both picked option a")
	q1 := stats[0].(map[string]any)
	assert.Equal(t, 50.0, q1["percent_right"])
	q3 := stats[2].(map[string]any)
	assert.EqualValues(t, 1, q3["unanswered"])

	// Viewing is shared among staff (like the live monitor), but students
	// can never reach these endpoints.
	h.login("19980012", "ADEGOKE")
	assert.Equal(t, 200, h.do(http.MethodGet, fmt.Sprintf("/api/staff/sessions/%d/answers", sidA), nil, nil).Code)
	h.token = ""
	assert.Equal(t, http.StatusUnauthorized, h.do(http.MethodGet, fmt.Sprintf("/api/staff/sessions/%d/answers", sidA), nil, nil).Code)
}

type fakeMarker struct{ calls int }

func (f *fakeMarker) Suggest(_ context.Context, req services.AIMarkRequest) (*services.AISuggestion, error) {
	f.calls++
	// Over the maximum on purpose: the service must clamp it.
	return &services.AISuggestion{Marks: req.MaxMarks + 3, Reason: "Covers every point.", PointsMet: []int{0, 1}, Model: "fake", At: time.Now()}, nil
}

func TestTheoryMarking(t *testing.T) {
	h := newHarness(t)
	h.login("20010078", "MUSTAPHA")
	now := time.Now().UTC()
	r := h.do(http.MethodPost, "/api/staff/exams", map[string]any{"title": "Theory", "starts_at": now.Add(-time.Minute),
		"ends_at": now.Add(time.Hour), "lan_cidr": "127.0.0.0/8", "proctoring": map[string]bool{"face_verification": false}}, nil)
	examID := num(r.Body["id"])
	r = h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/questions", examID), map[string]any{"questions": []map[string]any{
		{"prompt": "Pick", "options": []string{"a", "b"}, "correct_option": 0, "marks": 2},
		{"kind": "TEXT", "prompt": "Explain encryption.", "marks": 10},
	}}, nil)
	qs := r.Body["questions"].([]any)
	mcq, text := num(qs[0].(map[string]any)["id"]), num(qs[1].(map[string]any)["id"])
	guidePath := fmt.Sprintf("/api/staff/exams/%d/questions/%d/guide", examID, text)

	// Guide validation: points may not add up to more than the question.
	r = h.do(http.MethodPut, guidePath, map[string]any{"points": []map[string]any{{"point": "x", "marks": 11}}}, nil)
	assert.Equal(t, 400, r.Code, r.Raw)
	r = h.do(http.MethodPut, fmt.Sprintf("/api/staff/exams/%d/questions/%d/guide", examID, mcq), map[string]any{"points": []map[string]any{{"point": "x", "marks": 1}}}, nil)
	assert.Equal(t, 400, r.Code, "MCQ questions have no marking guide")
	r = h.do(http.MethodPut, guidePath, map[string]any{"model_answer": "Encryption turns plaintext into ciphertext.", "points": []map[string]any{
		{"point": "Defines encryption", "marks": 4, "keywords": []string{"ciphertext", "encrypt"}},
		{"point": "Names an algorithm", "marks": 6, "keywords": []string{"AES", "RSA"}},
	}}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	require.Equal(t, 200, h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil).Code)

	sit := func(matric, device, answer string) {
		r := h.multipart("/api/student/verify", map[string]string{"matric_no": matric, "consent": "true"}, nil, fp(device))
		require.Equal(t, 200, r.Code, r.Raw)
		st := h.do(http.MethodGet, "/api/student/session", nil, fp(device))
		assert.NotContains(t, st.Raw, "ciphertext", "the marking guide never reaches students")
		require.Equal(t, 200, h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": mcq, "response": "0"}, fp(device)).Code)
		require.Equal(t, 200, h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": text, "response": answer}, fp(device)).Code)
		require.Equal(t, 200, h.do(http.MethodPost, "/api/student/submit", nil, fp(device)).Code)
	}
	sit("IFT/22/9203", devA, "Encrypting data produces ciphertext, e.g. with AES.")
	sit("IFT/22/9101", devB, "It hides data.")

	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/marking", examID), nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, false, r.Body["ai_enabled"])
	mq := r.Body["questions"].([]any)
	require.Len(t, mq, 1, "only written questions are listed")
	assert.EqualValues(t, 2, mq[0].(map[string]any)["answered"])
	assert.EqualValues(t, 0, mq[0].(map[string]any)["marked"])

	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/marking/%d", examID, text), nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.NotContains(t, r.Raw, "IFT/22/", "marking is anonymous")
	scripts := r.Body["scripts"].([]any)
	require.Len(t, scripts, 2)
	first := scripts[0].(map[string]any)
	assert.Equal(t, "Candidate 1", first["candidate"])
	assert.Equal(t, 10.0, first["keyword_suggestion"].(map[string]any)["marks"])
	assert.Equal(t, 0.0, scripts[1].(map[string]any)["keyword_suggestion"].(map[string]any)["marks"])
	ansA, ansB := num(first["answer_id"]), num(scripts[1].(map[string]any)["answer_id"])

	// AI is off unless the server enables it.
	aiPath := fmt.Sprintf("/api/staff/exams/%d/marking/%d/ai-suggest", examID, text)
	r = h.do(http.MethodPost, aiPath, nil, nil)
	assert.Equal(t, http.StatusConflict, r.Code)
	assert.Contains(t, r.Raw, "AI_MARKING_OFF")

	fake := &fakeMarker{}
	h.app.Handlers.Marking = services.NewMarkingService(h.app.Handlers.Repo, fake)
	r = h.do(http.MethodPost, aiPath, nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.EqualValues(t, 2, r.Body["suggested"])
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/marking/%d", examID, text), nil, nil)
	ai := r.Body["scripts"].([]any)[1].(map[string]any)["ai_suggestion"].(map[string]any)
	assert.Equal(t, 10.0, ai["marks"], "AI suggestion clamped to the question's maximum: %s", r.Raw)
	assert.Equal(t, "Covers every point.", ai["reason"])
	assert.Nil(t, r.Body["scripts"].([]any)[1].(map[string]any)["awarded"], "a suggestion is not a mark")

	// The examiner decides. Score = MCQ 2 + written mark.
	markPath := func(id int64) string { return fmt.Sprintf("/api/staff/exams/%d/answers/%d/mark", examID, id) }
	assert.Equal(t, 400, h.do(http.MethodPut, markPath(ansA), map[string]any{"marks": 10.5}, nil).Code)
	r = h.do(http.MethodPut, markPath(ansA), map[string]any{"marks": 8, "comment": "Good, but explain keys.", "source": "keyword"}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, 10.0, r.Body["session_score"])
	r = h.do(http.MethodPut, markPath(ansB), map[string]any{"marks": 1.5}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, 3.5, r.Body["session_score"])

	// Already-marked answers are skipped by a new AI batch.
	fake.calls = 0
	r = h.do(http.MethodPost, aiPath, nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, 0, fake.calls)

	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/report", examID), nil, nil)
	var sidA int64
	for _, row := range r.Body["rows"].([]any) {
		m := row.(map[string]any)
		if m["matric_no"] == "IFT/22/9203" {
			sidA = num(m["session_id"])
			assert.Equal(t, 10.0, m["score"], "report shows the marked total")
		}
	}
	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/sessions/%d/answers", sidA), nil, nil)
	item := r.Body["items"].([]any)[1].(map[string]any)
	assert.Equal(t, "marked", item["status"])
	assert.Equal(t, 8.0, item["awarded"])
	assert.Equal(t, "Good, but explain keys.", item["comment"])

	// Clearing a mark puts it back to "needs marking".
	r = h.do(http.MethodPut, markPath(ansA), map[string]any{"marks": nil}, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, 2.0, r.Body["session_score"])

	// A proctor can view but not mark.
	h.login("19980012", "ADEGOKE")
	assert.Equal(t, 200, h.do(http.MethodGet, fmt.Sprintf("/api/staff/exams/%d/marking/%d", examID, text), nil, nil).Code)
	assert.Equal(t, http.StatusForbidden, h.do(http.MethodPut, markPath(ansA), map[string]any{"marks": 10}, nil).Code)
	assert.Equal(t, http.StatusForbidden, h.do(http.MethodPut, guidePath, map[string]any{"points": []any{}}, nil).Code)
}

func TestGapQuestionsAndTimeLimit(t *testing.T) {
	h := newHarness(t)
	h.login("20010078", "MUSTAPHA")
	now := time.Now().UTC()
	exam := map[string]any{"title": "Timed", "starts_at": now.Add(-time.Minute), "ends_at": now.Add(time.Hour),
		"lan_cidr": "127.0.0.0/8", "proctoring": map[string]bool{"face_verification": false}, "duration_minutes": 90}
	assert.Equal(t, 400, h.do(http.MethodPost, "/api/staff/exams", exam, nil).Code, "time limit longer than the window")
	exam["duration_minutes"] = 25
	r := h.do(http.MethodPost, "/api/staff/exams", exam, nil)
	require.Equal(t, 201, r.Code, r.Raw)
	examID := num(r.Body["id"])
	assert.EqualValues(t, 25, r.Body["duration_minutes"])

	qpath := fmt.Sprintf("/api/staff/exams/%d/questions", examID)
	r = h.do(http.MethodPost, qpath, map[string]any{"questions": []map[string]any{
		{"kind": "GAP", "prompt": "TCP opens with (i) _____ then (ii) _____.", "blanks": [][]string{{"SYN"}}},
	}}, nil)
	assert.Equal(t, 400, r.Code, "answers must match the number of blanks")
	r = h.do(http.MethodPost, qpath, map[string]any{"questions": []map[string]any{
		{"kind": "GAP", "prompt": "TCP opens with (i) _____ then (ii) _____.", "blanks": [][]string{{"SYN"}, {"SYN-ACK", "SYN ACK"}}, "marks": 2},
	}}, nil)
	require.Equal(t, 201, r.Code, r.Raw)
	gap := num(r.Body["questions"].([]any)[0].(map[string]any)["id"])
	require.Equal(t, 200, h.do(http.MethodPost, fmt.Sprintf("/api/staff/exams/%d/status", examID), map[string]string{"action": "open"}, nil).Code)

	r = h.multipart("/api/student/verify", map[string]string{"matric_no": "IFT/22/9203", "consent": "true"}, nil, fp(devA))
	require.Equal(t, 200, r.Code, r.Raw)
	st := h.do(http.MethodGet, "/api/student/session", nil, fp(devA))
	assert.NotContains(t, st.Raw, "SYN-ACK", "accepted answers never reach students")
	q0 := st.Body["questions"].([]any)[0].(map[string]any)
	assert.EqualValues(t, 2, q0["blanks"])
	ends, err := time.Parse(time.RFC3339Nano, st.Body["ends_at"].(string))
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(25*time.Minute), ends, 5*time.Second, "deadline is start + 25 min, not the window end")

	assert.Equal(t, 400, h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": gap, "response": "SYN"}, fp(devA)).Code)
	require.Equal(t, 200, h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": gap, "response": `["syn","FIN"]`}, fp(devA)).Code)

	// Pretend the student started 30 minutes ago: the sweep submits them.
	sid := num(st.Body["session_id"])
	require.NoError(t, h.app.Handlers.Repo.UpdateSession(context.Background(), sid, map[string]any{"started_at": time.Now().Add(-30 * time.Minute)}))
	assert.Equal(t, http.StatusGone, h.do(http.MethodPut, "/api/student/answers", map[string]any{"question_id": gap, "response": `["SYN","SYN-ACK"]`}, fp(devA)).Code, "time is up")
	h.app.Handlers.Students.AutoSubmitOverdue(context.Background())

	r = h.do(http.MethodGet, fmt.Sprintf("/api/staff/sessions/%d/answers", sid), nil, nil)
	require.Equal(t, 200, r.Code, r.Raw)
	assert.Equal(t, "SUBMITTED", r.Body["status"])
	assert.Equal(t, 1.0, r.Body["score"], "one of two blanks right = half of 2 marks")
	item := r.Body["items"].([]any)[0].(map[string]any)
	assert.Equal(t, "partial", item["status"])
	assert.Equal(t, []any{true, false}, item["blank_correct"])
}
