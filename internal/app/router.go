package app

import (
	"context"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"examshield/internal/middleware"
	"examshield/internal/models"
	"examshield/internal/services"
	"examshield/web"
)

// NewRouter builds the Gin engine. Middleware order matters: the LAN gate
// runs before anything else touches the request.
func NewRouter(shutdown context.Context, a *App) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.MaxMultipartMemory = 8 << 20
	r.HandleMethodNotAllowed = true

	// ClientIP() trusts X-Real-IP / X-Forwarded-For only from these peers.
	// With none configured it always returns the TCP peer address.
	if len(a.Cfg.TrustedProxies) > 0 {
		_ = r.SetTrustedProxies(a.Cfg.TrustedProxies)
		r.ForwardedByClientIP = true
		r.RemoteIPHeaders = []string{"X-Real-IP", "X-Forwarded-For"}
	} else {
		_ = r.SetTrustedProxies(nil)
		r.ForwardedByClientIP = false
	}

	h := a.Handlers
	log := a.Log
	r.Use(gin.CustomRecovery(func(c *gin.Context, rec any) {
		log.Error("panic recovered", "path", c.Request.URL.Path, "panic", rec)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "INTERNAL_ERROR"})
	}))
	r.Use(middleware.RequestLog(log))
	r.Use(a.Gate.Network())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.SameOriginOnly())

	r.GET("/healthz", h.Health)

	api := r.Group("/api")

	// --- Students: no login, no dashboard ---------------------------------------
	st := api.Group("/student")
	verifyLimit := middleware.RateLimit(a.Redis, "verify", 10, 5*time.Minute, middleware.ByIP, log)
	st.GET("/gate", a.Gate.ExamEntry(), h.Gate)
	st.POST("/verify", verifyLimit, a.Gate.ExamEntry(), h.Verify)

	sess := st.Group("", middleware.StudentAuth(a.Tokens, services.DeriveFP))
	sess.GET("/session", h.SessionState)
	sess.PUT("/answers", middleware.RateLimit(a.Redis, "answers", 120, time.Minute, middleware.ByIP, log), h.SaveAnswer)
	sess.POST("/heartbeat", h.Heartbeat)
	sess.POST("/events", middleware.RateLimit(a.Redis, "events", 60, time.Minute, middleware.ByIP, log), h.ClientEvent)
	sess.POST("/frames", h.Frame)
	sess.POST("/audio", h.Audio)
	sess.POST("/submit", h.Submit)

	// --- Staff ---------------------------------------------------------------
	staff := api.Group("/staff")
	authLimit := middleware.RateLimit(a.Redis, "staff-auth", 60, 15*time.Minute, middleware.ByIP, log)
	staff.POST("/auth/login", authLimit, h.Login)
	staff.POST("/auth/refresh", authLimit, h.Refresh)
	staff.POST("/auth/logout", h.Logout)
	staff.GET("/ws", h.WS(shutdown))

	s := staff.Group("", middleware.StaffAuth(a.Tokens))
	admin := middleware.RequireRole(models.RoleAdmin)

	s.GET("/me", h.Me)
	s.GET("/system", h.System)
	s.POST("/ws-ticket", h.WSTicket)

	s.GET("/exams", h.ListExams)
	s.POST("/exams", h.CreateExam)
	s.GET("/exams/:id", h.GetExam)
	s.PUT("/exams/:id", h.UpdateExam)
	s.DELETE("/exams/:id", h.DeleteExam)
	s.POST("/exams/:id/status", h.ExamStatus)
	s.POST("/exams/:id/pin", h.ExamPIN)
	s.PUT("/exams/:id/proctoring", h.SetProctoring)
	s.GET("/exams/:id/access-card", h.AccessCard)
	s.GET("/exams/:id/questions", h.ListQuestions)
	s.POST("/exams/:id/questions", h.AddQuestions)
	s.PUT("/exams/:id/questions/:qid", h.UpdateQuestion)
	s.DELETE("/exams/:id/questions/:qid", h.DeleteQuestion)
	s.POST("/exams/:id/questions/parse-pdf", h.ParseQuestionsPDF)
	s.GET("/questions/sample.pdf", h.SampleQuestionsPDF)
	s.GET("/exams/:id/sessions", h.ListSessions)
	s.GET("/exams/:id/violations", h.ListViolations)
	s.POST("/exams/:id/violations/:vid/review", h.ReviewViolation)
	s.GET("/exams/:id/report", h.Report)
	s.GET("/exams/:id/item-analysis", h.ItemAnalysis)
	s.POST("/exams/:id/purge-media", admin, h.PurgeMedia)

	s.GET("/sessions/:sid", h.SessionDetail)
	s.GET("/sessions/:sid/answers", h.AnswerScript)
	s.GET("/sessions/:sid/image/:which", h.SessionImage)
	s.POST("/sessions/:sid/unlock", h.UnlockSession)
	s.POST("/sessions/:sid/terminate", h.TerminateSession)
	s.POST("/sessions/:sid/flag", h.FlagSession)
	s.GET("/media", h.Media)

	s.GET("/students", h.ListStudents)
	s.POST("/students", h.CreateStudent)
	s.POST("/students/import", h.ImportStudents)
	s.POST("/students/photos", h.BulkPhotos)
	s.POST("/students/:id/photo", h.UploadPhoto)
	s.GET("/students/:id/photo", h.StudentPhoto)
	s.GET("/students/:id/export", admin, h.ExportStudent)
	s.DELETE("/students/:id", admin, h.DeleteStudent)

	s.GET("/users", admin, h.ListUsers)
	s.POST("/users", admin, h.CreateUser)
	s.PATCH("/users/:id", admin, h.UpdateUser)
	s.GET("/audit", admin, h.AuditLog)

	r.NoRoute(spaHandler())
	return r
}

// spaHandler serves the embedded React build: real files directly (hashed
// assets cached for a year), anything else falls back to index.html so the
// client router can handle "/" and "/staff/...".
func spaHandler() gin.HandlerFunc {
	dist, _ := fs.Sub(web.Dist, "dist")
	files := http.FileServer(http.FS(dist))
	index, indexErr := fs.ReadFile(dist, "index.html")
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
			c.JSON(http.StatusNotFound, gin.H{"error": "NOT_FOUND"})
			return
		}
		p := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(dist, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		if indexErr != nil {
			c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8",
				[]byte("<h1>Exam Shield</h1><p>The web UI has not been built. Run <code>npm run build</code> in <code>web/</code> and rebuild the server.</p>"))
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}
