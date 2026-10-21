package middleware

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"examshield/internal/config"
	"examshield/internal/httpx"
	"examshield/internal/models"
	"examshield/internal/netutil"
)

func cidrs(t *testing.T, ss ...string) []*net.IPNet {
	t.Helper()
	var out []*net.IPNet
	for _, s := range ss {
		n, err := config.ParseCIDR(s)
		require.NoError(t, err)
		out = append(out, n)
	}
	return out
}

type fakeExams struct{ exams []models.Exam }

func (f fakeExams) OpenExams(context.Context) ([]models.Exam, error) { return f.exams, nil }

type fakeLimiter struct{ hits map[string]int }

func (f *fakeLimiter) Hit(_ context.Context, key string, limit int, _ time.Duration) (bool, time.Duration, error) {
	f.hits[key]++
	return f.hits[key] <= limit, time.Minute, nil
}

func newGate(t *testing.T, cfg *config.Config, allow []string, exams []models.Exam) *LANGate {
	return NewLANGate(cfg, cidrs(t, allow...), fakeExams{exams}, &fakeLimiter{hits: map[string]int{}},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestAllowed(t *testing.T) {
	cfg := &config.Config{LANDenyCIDRs: cidrs(t, "192.168.1.200/29"), AllowLoopbackStaff: true}
	g := newGate(t, cfg, []string{"192.168.1.0/24", "fd00:1::/64"}, nil)

	cases := []struct {
		ip, path string
		want     bool
	}{
		{"192.168.1.45", "/", true},
		{"192.168.1.45", "/api/student/gate", true},
		{"192.168.2.45", "/", false},                // other subnet
		{"8.8.8.8", "/api/staff/me", false},         // internet
		{"::ffff:192.168.1.45", "/", true},          // IPv4-mapped IPv6
		{"fd00:1::42", "/", true},                   // IPv6 ULA in range
		{"fd00:2::42", "/", false},                  // IPv6 ULA out of range
		{"fe80::1%eth0", "/", false},                // link-local with zone
		{"192.168.1.201", "/", false},               // deny list (VPN pool)
		{"127.0.0.1", "/staff", true},               // staff on the server itself
		{"127.0.0.1", "/api/student/verify", false}, // students never via loopback by default
		{"::1", "/api/student/gate", false},
		{"garbage", "/", false},
	}
	for _, tc := range cases {
		got := g.Allowed(netutil.ParseClientIP(tc.ip), tc.path)
		assert.Equal(t, tc.want, got, "%s %s", tc.ip, tc.path)
	}
}

func serve(t *testing.T, g *LANGate, trusted []string, remote string, hdr map[string]string, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if len(trusted) > 0 {
		require.NoError(t, r.SetTrustedProxies(trusted))
		r.ForwardedByClientIP = true
		r.RemoteIPHeaders = []string{"X-Real-IP", "X-Forwarded-For"}
	} else {
		require.NoError(t, r.SetTrustedProxies(nil))
		r.ForwardedByClientIP = false
	}
	r.Use(g.Network())
	r.GET("/api/student/gate", g.ExamEntry(), func(c *gin.Context) {
		c.JSON(200, gin.H{"exam_id": ExamFromContext(c).ID})
	})
	r.GET("/api/x", func(c *gin.Context) { c.String(200, ClientIP(c).String()) })
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remote
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestNetworkIgnoresSpoofedForwardedForWithoutTrustedProxy(t *testing.T) {
	g := newGate(t, &config.Config{}, []string{"192.168.1.0/24"}, nil)
	w := serve(t, g, nil, "203.0.113.9:5555", map[string]string{"X-Forwarded-For": "192.168.1.10", "X-Real-IP": "192.168.1.10"}, "/api/x")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), httpx.CodeLANRequired)
}

func TestNetworkHonoursTrustedProxy(t *testing.T) {
	g := newGate(t, &config.Config{}, []string{"192.168.1.0/24"}, nil)
	// Behind Nginx on 172.28.0.1 the TCP peer is the proxy; X-Real-IP is the student.
	w := serve(t, g, []string{"172.28.0.1"}, "172.28.0.1:40000", map[string]string{"X-Real-IP": "192.168.1.10"}, "/api/x")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "192.168.1.10", w.Body.String())

	// A forged header from an untrusted peer is ignored.
	w = serve(t, g, []string{"172.28.0.1"}, "10.9.9.9:40000", map[string]string{"X-Real-IP": "192.168.1.10"}, "/api/x")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestNetworkServesHTMLBlockPageOutsideAPI(t *testing.T) {
	g := newGate(t, &config.Config{}, []string{"192.168.1.0/24"}, nil)
	w := serve(t, g, nil, "203.0.113.9:1", nil, "/")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func pin(s string) *string { return &s }

func TestExamEntry(t *testing.T) {
	a := models.Exam{ID: 1, LANCIDR: "192.168.1.0/24"}
	b := models.Exam{ID: 2, LANCIDR: "192.168.1.0/24", PinCode: pin("1234")}
	c := models.Exam{ID: 3, LANCIDR: "192.168.1.0/24", PinCode: pin("9876")}
	other := models.Exam{ID: 4, LANCIDR: "10.0.0.0/8"}

	cases := []struct {
		name   string
		exams  []models.Exam
		query  string
		status int
		body   string
	}{
		{"no exam", nil, "", http.StatusLocked, httpx.CodeExamNotOpen},
		{"exam on another subnet", []models.Exam{other}, "", http.StatusLocked, httpx.CodeExamNotOpen},
		{"single open exam", []models.Exam{a}, "", 200, `"exam_id":1`},
		{"stale pin tolerated for single pinless exam", []models.Exam{a}, "?pin=5555", 200, `"exam_id":1`},
		{"pin required", []models.Exam{b}, "", http.StatusUnauthorized, httpx.CodePINRequired},
		{"wrong pin", []models.Exam{b}, "?pin=0000", http.StatusUnauthorized, httpx.CodePINInvalid},
		{"right pin", []models.Exam{b}, "?pin=1234", 200, `"exam_id":2`},
		{"pin routes between two exams", []models.Exam{b, c}, "?pin=9876", 200, `"exam_id":3`},
		{"ambiguous pinless exams", []models.Exam{a, {ID: 5, LANCIDR: "192.168.0.0/16"}}, "", http.StatusConflict, httpx.CodeConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newGate(t, &config.Config{}, []string{"192.168.0.0/16", "10.0.0.0/8"}, tc.exams)
			w := serve(t, g, nil, "192.168.1.45:1000", nil, "/api/student/gate"+tc.query)
			assert.Equal(t, tc.status, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tc.body)
		})
	}
}

func TestPINBruteForceIsRateLimited(t *testing.T) {
	g := newGate(t, &config.Config{}, []string{"192.168.1.0/24"}, []models.Exam{{ID: 2, LANCIDR: "192.168.1.0/24", PinCode: pin("1234")}})
	var last int
	for i := 0; i < pinAttemptLimit+1; i++ {
		last = serve(t, g, nil, "192.168.1.45:1000", nil, "/api/student/gate?pin=0000").Code
	}
	assert.Equal(t, http.StatusTooManyRequests, last)
}

func TestOpenAccessMode(t *testing.T) {
	cfg := &config.Config{AccessMode: "open"}
	exam := models.Exam{ID: 9, LANCIDR: "0.0.0.0/0"}
	g := newGate(t, cfg, []string{"192.168.1.0/24"}, []models.Exam{exam})
	assert.True(t, g.Allowed(netutil.ParseClientIP("203.0.113.9"), "/api/student/gate"))
	assert.False(t, g.Allowed(nil, "/"), "unparseable addresses are still refused")

	w := serve(t, g, nil, "203.0.113.9:1000", nil, "/api/student/gate")
	assert.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"exam_id":9`)

	// LAN mode with the same request is refused.
	lan := newGate(t, &config.Config{}, []string{"192.168.1.0/24"}, []models.Exam{exam})
	assert.Equal(t, http.StatusForbidden, serve(t, lan, nil, "203.0.113.9:1000", nil, "/api/student/gate").Code)
}
