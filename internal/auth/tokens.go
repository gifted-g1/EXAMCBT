// Package auth issues and verifies the two token kinds Exam Shield uses:
// staff access tokens and student exam-session tokens. Both are HS256 JWTs
// whose algorithm, issuer, audience and type are all pinned on verification,
// so a token of one kind can never be replayed as the other and "alg":"none"
// or RS/HS confusion is rejected outright.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"examshield/internal/models"
)

const (
	issuer        = "exam-shield"
	AudStaff      = "staff"
	AudStudent    = "student"
	typAccess     = "access"
	typStudent    = "student"
	clockLeeway   = 5 * time.Second
	signingMethod = "HS256"
)

var ErrInvalidToken = errors.New("invalid token")

type StaffClaims struct {
	Role models.Role `json:"role"`
	Typ  string      `json:"typ"`
	jwt.RegisteredClaims
}

type StudentClaims struct {
	ExamID    int64  `json:"eid"`
	StudentID int64  `json:"stid"`
	DeviceFP  string `json:"fp"`
	Typ       string `json:"typ"`
	jwt.RegisteredClaims
}

// SessionID returns the exam_sessions.id the token is bound to.
func (s *StudentClaims) SessionID() int64 {
	id, _ := strconv.ParseInt(s.Subject, 10, 64)
	return id
}

type Tokens struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokens(secret []byte, accessTTL time.Duration) *Tokens {
	return &Tokens{secret: secret, accessTTL: accessTTL}
}

func (t *Tokens) AccessTTL() time.Duration { return t.accessTTL }

func (t *Tokens) IssueStaff(u *models.User) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(t.accessTTL)
	claims := StaffClaims{
		Role: u.Role,
		Typ:  typAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   strconv.FormatInt(u.ID, 10),
			Audience:  jwt.ClaimStrings{AudStaff},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-clockLeeway)),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        RandomHex(8),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	return s, exp, err
}

func (t *Tokens) ParseStaff(raw string) (*StaffClaims, error) {
	c := &StaffClaims{}
	if err := t.parse(raw, c, AudStaff); err != nil {
		return nil, err
	}
	if c.Typ != typAccess || !c.Role.Valid() {
		return nil, ErrInvalidToken
	}
	return c, nil
}

// IssueStudent binds a token to one exam session on one device. It expires
// with the exam window (plus grace), never later.
func (t *Tokens) IssueStudent(sessionID, examID, studentID int64, deviceFP string, expires time.Time) (string, error) {
	now := time.Now()
	claims := StudentClaims{
		ExamID: examID, StudentID: studentID, DeviceFP: deviceFP, Typ: typStudent,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   strconv.FormatInt(sessionID, 10),
			Audience:  jwt.ClaimStrings{AudStudent},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
			ID:        RandomHex(8),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

func (t *Tokens) ParseStudent(raw string) (*StudentClaims, error) {
	c := &StudentClaims{}
	if err := t.parse(raw, c, AudStudent); err != nil {
		return nil, err
	}
	if c.Typ != typStudent || c.SessionID() <= 0 || c.DeviceFP == "" {
		return nil, ErrInvalidToken
	}
	return c, nil
}

func (t *Tokens) parse(raw string, claims jwt.Claims, aud string) error {
	_, err := jwt.ParseWithClaims(raw, claims, func(tok *jwt.Token) (any, error) {
		if tok.Method.Alg() != signingMethod {
			return nil, fmt.Errorf("unexpected alg %q", tok.Method.Alg())
		}
		return t.secret, nil
	},
		jwt.WithValidMethods([]string{signingMethod}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(aud),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(clockLeeway),
	)
	if err != nil {
		return ErrInvalidToken
	}
	return nil
}

// RandomHex returns n random bytes hex-encoded (2n chars).
func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// HashToken returns the hex SHA-256 of an opaque token, for storage.
func HashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}
