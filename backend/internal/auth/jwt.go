package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"examshield/internal/models"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

// Claims embeds the standard JWT claims plus the fields the RBAC
// middleware needs to authorize a request without a DB round-trip.
type Claims struct {
	UserID         uuid.UUID  `json:"uid"`
	OrganizationID uuid.UUID  `json:"org"`
	Role           models.Role `json:"role"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// GenerateAccessToken issues a short-lived signed JWT carrying identity
// and role, used to authorize every subsequent API request.
func (m *TokenManager) GenerateAccessToken(u *models.User) (string, error) {
	claims := Claims{
		UserID:         u.ID,
		OrganizationID: u.OrganizationID,
		Role:           u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *TokenManager) ParseAccessToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// GenerateRefreshToken returns a random opaque token (not a JWT) plus
// its SHA-256 hash for storage. Only the hash is persisted, so a
// leaked database never yields usable refresh tokens.
func (m *TokenManager) GenerateRefreshToken() (plain string, hash string, expiresAt time.Time, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", time.Time{}, err
	}
	plain = hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(sum[:])
	expiresAt = time.Now().Add(m.refreshTTL)
	return plain, hash, expiresAt, nil
}

func HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
