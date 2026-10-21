package middleware

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"examshield/internal/config"
	"examshield/internal/httpx"
	"examshield/internal/models"
	"examshield/internal/netutil"
)

// OpenExamSource returns the exams that are ACTIVE and inside their window
// right now (cached briefly in Redis by the implementation).
type OpenExamSource interface {
	OpenExams(ctx context.Context) ([]models.Exam, error)
}

// Limiter is a fixed-window rate limiter (Redis-backed in production).
type Limiter interface {
	Hit(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error)
}

// LANGate enforces the network-locked access model in two layers:
//
//  1. Network() — on every request: the TCP peer (or, only behind a
//     configured trusted proxy, the forwarded client) must be inside the
//     server's LAN CIDRs and outside LAN_DENY_CIDRS (e.g. VPN pools).
//  2. ExamEntry() — on student entry routes: an ACTIVE exam whose window
//     contains now and whose lan_cidr contains the client must exist, and its
//     PIN must match when one is set.
type LANGate struct {
	allow         []*net.IPNet
	deny          []*net.IPNet
	loopbackStaff bool
	loopbackExam  bool
	open          bool // ACCESS_MODE=open: no subnet lock (cloud/demo hosting)
	exams         OpenExamSource
	limiter       Limiter
	log           *slog.Logger
}

const (
	pinAttemptLimit  = 10
	pinAttemptWindow = 5 * time.Minute
)

func NewLANGate(cfg *config.Config, lanNets []*net.IPNet, exams OpenExamSource, limiter Limiter, log *slog.Logger) *LANGate {
	return &LANGate{
		allow:         lanNets,
		deny:          cfg.LANDenyCIDRs,
		loopbackStaff: cfg.AllowLoopbackStaff,
		loopbackExam:  cfg.AllowLoopbackExam,
		open:          cfg.OpenAccess(),
		exams:         exams,
		limiter:       limiter,
		log:           log,
	}
}

// ClientIP returns the normalised client IP stored by Network().
func ClientIP(c *gin.Context) net.IP {
	if v, ok := c.Get(httpx.KeyClientIP); ok {
		if ip, ok := v.(net.IP); ok {
			return ip
		}
	}
	return netutil.ParseClientIP(c.ClientIP())
}

func isStudentAPI(path string) bool { return strings.HasPrefix(path, "/api/student") }

// Allowed reports whether ip may reach the server at all for path.
func (g *LANGate) Allowed(ip net.IP, path string) bool {
	if ip == nil {
		return false
	}
	if g.open {
		return true
	}
	if ip.IsLoopback() {
		if isStudentAPI(path) {
			return g.loopbackExam
		}
		return g.loopbackStaff || g.loopbackExam
	}
	if netutil.InAny(ip, g.deny) {
		return false
	}
	return netutil.InAny(ip, g.allow)
}

// Network rejects every request from outside the LAN.
func (g *LANGate) Network() gin.HandlerFunc {
	return func(c *gin.Context) {
		// c.ClientIP() honours X-Forwarded-For/X-Real-IP only when the TCP
		// peer is in the engine's TrustedProxies; otherwise it is RemoteAddr.
		ip := netutil.ParseClientIP(c.ClientIP())
		// Container/orchestrator health probes come from loopback.
		healthProbe := c.Request.URL.Path == "/healthz" && ip != nil && ip.IsLoopback()
		if !healthProbe && !g.Allowed(ip, c.Request.URL.Path) {
			g.log.Warn("lan gate: blocked", "ip", ipString(ip), "path", c.Request.URL.Path)
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				httpx.Abort(c, http.StatusForbidden, httpx.CodeLANRequired,
					"Connect to the exam hall network and open the address your invigilator gave you.")
				return
			}
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.AbortWithStatus(http.StatusForbidden)
			_, _ = c.Writer.WriteString(blockedPage)
			return
		}
		c.Set(httpx.KeyClientIP, ip)
		c.Next()
	}
}

// ExamEntry resolves which open exam this client is entering and stores it
// under httpx.KeyExam. The PIN may come from the X-Exam-PIN header or the
// ?pin= query parameter (the QR code embeds it).
func (g *LANGate) ExamEntry() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := ClientIP(c)
		open, err := g.exams.OpenExams(c.Request.Context())
		if err != nil {
			httpx.Internal(c, err)
			return
		}
		var candidates []models.Exam
		for _, e := range open {
			n, err := config.ParseCIDR(e.LANCIDR)
			if err != nil {
				g.log.Error("lan gate: exam has invalid lan_cidr", "exam_id", e.ID, "err", err)
				continue
			}
			if g.open || n.Contains(ip) || (ip.IsLoopback() && g.loopbackExam) {
				candidates = append(candidates, e)
			}
		}
		if len(candidates) == 0 {
			httpx.Abort(c, http.StatusLocked, httpx.CodeExamNotOpen, "No exam is open for this network right now.")
			return
		}

		pin := strings.TrimSpace(c.GetHeader("X-Exam-PIN"))
		if pin == "" {
			pin = strings.TrimSpace(c.Query("pin"))
		}

		exam, status, code := pickExam(candidates, pin)
		if code == httpx.CodePINInvalid {
			ok, retry, err := g.limiter.Hit(c.Request.Context(), "rl:pin:"+ip.String(), pinAttemptLimit, pinAttemptWindow)
			if err == nil && !ok {
				c.Header("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
				httpx.Abort(c, http.StatusTooManyRequests, httpx.CodeRateLimited, "Too many PIN attempts. Ask your invigilator.")
				return
			}
		}
		if exam == nil {
			msg := map[string]string{
				httpx.CodePINRequired: "Enter the 4-digit exam PIN shown by your invigilator.",
				httpx.CodePINInvalid:  "That PIN is not correct.",
				httpx.CodeConflict:    "More than one exam is open on this network; your invigilator must set PINs.",
			}[code]
			httpx.Abort(c, status, code, msg)
			return
		}
		c.Set(httpx.KeyExam, exam)
		c.Next()
	}
}

// pickExam chooses among exams open on the client's subnet.
func pickExam(candidates []models.Exam, pin string) (*models.Exam, int, string) {
	if pin != "" {
		for i := range candidates {
			e := &candidates[i]
			if e.PinRequired() && subtle.ConstantTimeCompare([]byte(*e.PinCode), []byte(pin)) == 1 {
				return e, 0, ""
			}
		}
		// A PIN was offered; if exactly one PIN-less exam exists, accept it so
		// a stale ?pin= in a bookmarked URL doesn't lock students out.
		if e := onlyPinless(candidates); e != nil && !anyPinned(candidates) {
			return e, 0, ""
		}
		return nil, http.StatusUnauthorized, httpx.CodePINInvalid
	}
	if anyPinned(candidates) {
		return nil, http.StatusUnauthorized, httpx.CodePINRequired
	}
	if len(candidates) > 1 {
		return nil, http.StatusConflict, httpx.CodeConflict
	}
	return &candidates[0], 0, ""
}

func anyPinned(es []models.Exam) bool {
	for i := range es {
		if es[i].PinRequired() {
			return true
		}
	}
	return false
}

func onlyPinless(es []models.Exam) *models.Exam {
	if len(es) == 1 && !es[0].PinRequired() {
		return &es[0]
	}
	return nil
}

// ExamFromContext returns the exam stored by ExamEntry.
func ExamFromContext(c *gin.Context) *models.Exam {
	if v, ok := c.Get(httpx.KeyExam); ok {
		if e, ok := v.(*models.Exam); ok {
			return e
		}
	}
	return nil
}

func ipString(ip net.IP) string {
	if ip == nil {
		return "invalid"
	}
	return ip.String()
}

const blockedPage = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Exam Shield — LAN required</title>
<style>body{font-family:system-ui,sans-serif;background:#0f172a;color:#e2e8f0;display:grid;place-items:center;min-height:100vh;margin:0;padding:16px}main{max-width:28rem;text-align:center}h1{font-size:1.4rem}p{color:#94a3b8;line-height:1.5}</style></head>
<body><main><h1>Exam hall network required</h1><p>This exam can only be opened from the exam hall's local network. Connect to the hall Wi-Fi or LAN, then type the address your invigilator gave you.</p><p><small>Error 403 · LAN_REQUIRED</small></p></main></body></html>`
