package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

type Claims struct {
	UserID string    `json:"uid"`
	Type   TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// Manager issues and verifies signed JWTs.
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(secret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (m *Manager) issue(userID string, typ TokenType, ttl time.Duration, now time.Time) (string, error) {
	claims := Claims{
		UserID: userID,
		Type:   typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Issue returns a fresh (access, refresh) token pair for the user.
func (m *Manager) Issue(userID string) (access, refresh string, err error) {
	now := time.Now()
	access, err = m.issue(userID, AccessToken, m.accessTTL, now)
	if err != nil {
		return "", "", err
	}
	refresh, err = m.issue(userID, RefreshToken, m.refreshTTL, now)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func (m *Manager) Parse(token string, want TokenType) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims.Type != want {
		return nil, errors.New("unexpected token type")
	}
	return claims, nil
}
