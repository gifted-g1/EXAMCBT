package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"examshield/internal/auth"
	"examshield/internal/models"
	"examshield/internal/repositories"
	"examshield/internal/store"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrLoginThrottled     = errors.New("too many login attempts")
	ErrThrottleDown       = errors.New("login throttle unavailable")
	ErrRefreshInvalid     = errors.New("refresh token invalid")
)

const (
	loginFailLimit    = 5
	loginFailWindow   = 15 * time.Minute
	loginIPFailLimit  = 30
	refreshTokenBytes = 32
)

type AuthService struct {
	repo       *repositories.Repo
	rdb        *store.Redis
	tokens     *auth.Tokens
	refreshTTL time.Duration
	bcryptCost int
	dummyHash  []byte
}

func NewAuthService(repo *repositories.Repo, rdb *store.Redis, tokens *auth.Tokens, refreshTTL time.Duration, bcryptCost int) *AuthService {
	// Comparing against a real hash when the user doesn't exist keeps
	// response time constant, so usernames can't be enumerated by timing.
	dummy, _ := bcrypt.GenerateFromPassword([]byte("DUMMYPASSWORD"), bcryptCost)
	return &AuthService{repo: repo, rdb: rdb, tokens: tokens, refreshTTL: refreshTTL, bcryptCost: bcryptCost, dummyHash: dummy}
}

type LoginResult struct {
	AccessToken  string       `json:"access_token"`
	ExpiresAt    time.Time    `json:"expires_at"`
	User         *models.User `json:"user"`
	RefreshToken string       `json:"-"`
	RefreshExp   time.Time    `json:"-"`
}

func failKey(username string) string { return "rl:login-fail:" + username }
func ipFailKey(ip string) string     { return "rl:login-fail-ip:" + ip }

// Login verifies staff credentials. Only failures count toward the limit of
// 5 per username per 15 minutes (plus 30 per IP, against spraying). If Redis
// is down the login is refused (fail-closed) rather than unthrottled.
func (s *AuthService) Login(ctx context.Context, username, password, ip string) (*LoginResult, error) {
	username = strings.TrimSpace(username)
	for _, k := range []struct {
		key   string
		limit int
	}{{failKey(username), loginFailLimit}, {ipFailKey(ip), loginIPFailLimit}} {
		n, err := s.rdb.Get(ctx, k.key).Int()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, ErrThrottleDown
		}
		if n >= k.limit {
			return nil, ErrLoginThrottled
		}
	}

	fail := func() (*LoginResult, error) {
		_, _, err1 := s.rdb.Hit(ctx, failKey(username), loginFailLimit, loginFailWindow)
		_, _, err2 := s.rdb.Hit(ctx, ipFailKey(ip), loginIPFailLimit, loginFailWindow)
		if err1 != nil || err2 != nil {
			return nil, ErrThrottleDown
		}
		return nil, ErrInvalidCredentials
	}

	if !auth.ValidUsername(username) || !auth.ValidPassword(password) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return fail()
	}
	u, err := s.repo.UserByUsername(ctx, username)
	if errors.Is(err, repositories.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return fail()
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil || !u.IsActive {
		return fail()
	}
	s.rdb.Del(ctx, failKey(username))

	return s.issue(ctx, u, uuid.NewString())
}

func (s *AuthService) issue(ctx context.Context, u *models.User, family string) (*LoginResult, error) {
	access, exp, err := s.tokens.IssueStaff(u)
	if err != nil {
		return nil, err
	}
	refresh := auth.RandomHex(refreshTokenBytes)
	rexp := time.Now().UTC().Add(s.refreshTTL)
	if err := s.repo.CreateRefresh(ctx, &models.RefreshToken{
		UserID: u.ID, FamilyID: family, TokenHash: auth.HashToken(refresh), ExpiresAt: rexp,
	}); err != nil {
		return nil, err
	}
	return &LoginResult{AccessToken: access, ExpiresAt: exp, User: u, RefreshToken: refresh, RefreshExp: rexp}, nil
}

// Refresh rotates a refresh token. Presenting an already-rotated (revoked)
// token is treated as theft: the whole token family is revoked.
func (s *AuthService) Refresh(ctx context.Context, raw string) (*LoginResult, error) {
	if len(raw) != refreshTokenBytes*2 {
		return nil, ErrRefreshInvalid
	}
	var res *LoginResult
	var reuseFamily string
	err := s.repo.Tx(ctx, func(tx *repositories.Repo) error {
		t, err := tx.RefreshByHashForUpdate(ctx, auth.HashToken(raw))
		if err != nil {
			return ErrRefreshInvalid
		}
		if t.RevokedAt != nil {
			reuseFamily = t.FamilyID
			return ErrRefreshInvalid
		}
		if time.Now().After(t.ExpiresAt) {
			return ErrRefreshInvalid
		}
		u, err := tx.UserByID(ctx, t.UserID)
		if err != nil || !u.IsActive {
			return ErrRefreshInvalid
		}
		if err := tx.RevokeRefresh(ctx, t.ID); err != nil {
			return err
		}
		res, err = (&AuthService{repo: tx, tokens: s.tokens, refreshTTL: s.refreshTTL}).issue(ctx, u, t.FamilyID)
		return err
	})
	if reuseFamily != "" {
		// Outside the rolled-back transaction so the revocation sticks.
		_ = s.repo.RevokeFamily(ctx, reuseFamily)
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Logout revokes the presented refresh token's family.
func (s *AuthService) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	return s.repo.Tx(ctx, func(tx *repositories.Repo) error {
		t, err := tx.RefreshByHashForUpdate(ctx, auth.HashToken(raw))
		if err != nil {
			return nil
		}
		return tx.RevokeFamily(ctx, t.FamilyID)
	})
}

// HashPassword validates the policy and hashes with the configured cost.
func (s *AuthService) HashPassword(pw string) (string, error) {
	if !auth.ValidPassword(pw) {
		return "", errors.New("password must be at least 6 uppercase letters")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pw), s.bcryptCost)
	return string(h), err
}
