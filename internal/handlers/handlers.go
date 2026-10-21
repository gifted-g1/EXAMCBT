// Package handlers holds the Gin HTTP handlers. Handlers only parse input,
// call a service and map the outcome to HTTP; all rules live in services.
package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"examshield/internal/ai"
	"examshield/internal/auth"
	"examshield/internal/config"
	"examshield/internal/httpx"
	"examshield/internal/middleware"
	"examshield/internal/models"
	"examshield/internal/realtime"
	"examshield/internal/repositories"
	"examshield/internal/services"
	"examshield/internal/store"
)

type Handlers struct {
	Cfg       *config.Config
	Repo      *repositories.Repo
	Redis     *store.Redis
	Blobs     store.Blobs
	Tokens    *auth.Tokens
	Auth      *services.AuthService
	Exams     *services.ExamService
	Students  *services.StudentService
	Roster    *services.RosterService
	Proctor   *services.ProctorService
	Scheduler *services.Scheduler
	Matcher   *services.FaceMatcher
	Engine    ai.Engine
	Hub       *realtime.Hub
	Log       *slog.Logger
}

// fail maps a service/repository error to an HTTP response.
func fail(c *gin.Context, err error) {
	var ve *services.ValidationError
	var fe *services.FaceError
	switch {
	case errors.As(err, &ve):
		httpx.BadRequest(c, ve.Msg)
	case errors.Is(err, repositories.ErrNotFound):
		httpx.NotFound(c, "resource")
	case errors.Is(err, services.ErrForbidden):
		httpx.Abort(c, http.StatusForbidden, httpx.CodeForbidden, "you cannot manage this resource")

	case errors.Is(err, services.ErrInvalidCredentials):
		httpx.Abort(c, http.StatusUnauthorized, httpx.CodeInvalidCreds, "Staff ID or password is incorrect.")
	case errors.Is(err, services.ErrLoginThrottled):
		c.Header("Retry-After", "900")
		httpx.Abort(c, http.StatusTooManyRequests, httpx.CodeRateLimited, "Too many failed attempts. Try again in 15 minutes.")
	case errors.Is(err, services.ErrThrottleDown):
		httpx.Abort(c, http.StatusServiceUnavailable, httpx.CodeInternal, "Login is temporarily unavailable.")
	case errors.Is(err, services.ErrRefreshInvalid):
		httpx.Abort(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "session expired; sign in again")

	case errors.Is(err, services.ErrConsentRequired):
		httpx.Abort(c, http.StatusBadRequest, httpx.CodeConsentRequired, "You must accept the proctoring notice to continue.")
	case errors.Is(err, services.ErrNotOnRoster):
		httpx.Abort(c, http.StatusNotFound, httpx.CodeNotOnRoster, "That matric number is not on the roster for this exam.")
	case errors.Is(err, services.ErrAlreadySubmitted):
		httpx.Abort(c, http.StatusConflict, httpx.CodeAlreadySubmitted, "You have already submitted this exam.")
	case errors.Is(err, services.ErrDeviceMismatch):
		httpx.Abort(c, http.StatusConflict, httpx.CodeDeviceMismatch, "Your exam is bound to another computer. Ask an invigilator to unlock it.")
	case errors.Is(err, store.ErrDeviceInUse):
		httpx.Abort(c, http.StatusConflict, httpx.CodeDeviceInUse, "This computer is already running another student's exam.")
	case errors.Is(err, store.ErrStudentInUse):
		httpx.Abort(c, http.StatusConflict, httpx.CodeStudentElsewhere, "This student is already writing on another computer.")
	case errors.Is(err, services.ErrBadFingerprint):
		httpx.BadRequest(c, "missing device identifier; reload the page")
	case errors.As(err, &fe) && errors.Is(err, services.ErrFaceMismatch):
		httpx.AbortDetails(c, http.StatusUnprocessableEntity, httpx.CodeFaceMismatch,
			"Your face did not match the roster photo. Face the camera in good light and try again.",
			gin.H{"score": fe.Score, "threshold": fe.Threshold})
	case errors.Is(err, services.ErrNoReference):
		httpx.Abort(c, http.StatusUnprocessableEntity, "NO_REFERENCE_PHOTO", "There is no roster photo for you. Take a clear photo of your student ID card and try again.")
	case errors.Is(err, services.ErrNoFace):
		httpx.Abort(c, http.StatusUnprocessableEntity, httpx.CodeNoFace, "No face was found in the photo. Center your face and try again.")
	case errors.Is(err, services.ErrMultipleFaces):
		httpx.Abort(c, http.StatusUnprocessableEntity, "MULTIPLE_FACES", "More than one face is visible. Only you should be in view.")
	case errors.Is(err, services.ErrOutsideExamNetwork):
		httpx.Abort(c, http.StatusForbidden, httpx.CodeLANRequired, "You are no longer on the exam network. Reconnect to the hall network.")
	case errors.Is(err, services.ErrFeatureDisabled):
		httpx.Abort(c, http.StatusConflict, "FEATURE_DISABLED", err.Error())
	case errors.Is(err, services.ErrSessionEnded):
		httpx.Abort(c, http.StatusGone, httpx.CodeSessionEnded, "This exam session has ended.")
	case errors.Is(err, services.ErrLockLost):
		httpx.Abort(c, http.StatusConflict, httpx.CodeSessionLost, "Your session was taken over or unlocked by an invigilator.")
	case errors.Is(err, services.ErrTooFrequent):
		httpx.Abort(c, http.StatusTooManyRequests, httpx.CodeRateLimited, "too frequent")
	case errors.Is(err, services.ErrBadMedia), errors.Is(err, services.ErrBadAudio):
		httpx.BadRequest(c, "media is missing, too large, or not a valid image/audio clip")

	case errors.Is(err, services.ErrInvalidState), errors.Is(err, services.ErrWindowPassed),
		errors.Is(err, services.ErrNoQuestions), errors.Is(err, services.ErrSubnetConflict),
		errors.Is(err, services.ErrPinInUse), errors.Is(err, services.ErrCIDROutsideLAN):
		httpx.Abort(c, http.StatusConflict, httpx.CodeConflict, err.Error())
	default:
		httpx.Internal(c, err)
	}
}

func idParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		httpx.BadRequest(c, "invalid "+name)
		return 0, false
	}
	return id, true
}

// examFor loads :id; when manage is true the caller must be its owner or an
// admin. Any staff member may view any exam (monitoring is shared).
func (h *Handlers) examFor(c *gin.Context, manage bool) (*models.Exam, bool) {
	id, ok := idParam(c, "id")
	if !ok {
		return nil, false
	}
	e, err := h.Repo.ExamByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return nil, false
	}
	p := middleware.Staff(c)
	if manage && !services.CanManage(p.ID, p.IsAdmin(), e) {
		fail(c, services.ErrForbidden)
		return nil, false
	}
	return e, true
}

func (h *Handlers) audit(c *gin.Context, action, target string, details map[string]any) {
	p := middleware.Staff(c)
	var actor *int64
	if p.ID > 0 {
		actor = &p.ID
	}
	if err := h.Repo.Audit(c.Request.Context(), actor, action, target, middleware.ClientIP(c).String(), details); err != nil {
		h.Log.Error("audit log write failed", "action", action, "err", err)
	}
}

// Health reports liveness plus dependency status.
func (h *Handlers) Health(c *gin.Context) {
	status := http.StatusOK
	checks := gin.H{}
	if sqlDB, err := h.Repo.DB().DB(); err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
		checks["database"], status = "down", http.StatusServiceUnavailable
	} else {
		checks["database"] = "ok"
	}
	if err := h.Redis.Ping(c.Request.Context()).Err(); err != nil {
		checks["redis"], status = "down", http.StatusServiceUnavailable
	} else {
		checks["redis"] = "ok"
	}
	c.JSON(status, gin.H{"status": map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK], "checks": checks})
}
