package authjwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	TokenTypeAdmin TokenType = "admin"
	TokenTypeApp   TokenType = "app"
)

type Subject struct {
	UserID    string
	TenantID  string
	Username  string
	SessionID string
	TokenType TokenType
}

type Claims struct {
	UserID    string    `json:"userId"`
	TenantID  string    `json:"tenantId"`
	Username  string    `json:"username"`
	SessionID string    `json:"sessionId"`
	TokenType TokenType `json:"tokenType"`
	jwt.RegisteredClaims
}

type Service struct {
	secret string
	now    func() time.Time
}

func NewService(secret string, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{secret: secret, now: now}
}

func (s *Service) Sign(subject Subject, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	now := s.now()
	expiresAt := now.Add(ttl)
	claims := Claims{
		UserID:    subject.UserID,
		TenantID:  subject.TenantID,
		Username:  subject.Username,
		SessionID: subject.SessionID,
		TokenType: subject.TokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.secret))
	return signed, expiresAt, err
}

func (s *Service) Parse(tokenValue string) (Claims, error) {
	parser := jwt.NewParser(jwt.WithTimeFunc(s.now))
	parsed, err := parser.ParseWithClaims(tokenValue, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected jwt signing method")
		}
		return []byte(s.secret), nil
	})
	if err != nil {
		return Claims{}, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("invalid jwt token")
	}
	return *claims, nil
}
