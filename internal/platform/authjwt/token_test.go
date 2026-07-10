package authjwt

import (
	"testing"
	"time"
)

func TestTokenServiceSignsAndParsesAdminToken(t *testing.T) {
	service := NewService("secret", func() time.Time { return time.Unix(100, 0).UTC() })
	token, expiresAt, err := service.Sign(Subject{
		UserID: "user-1", TenantID: "platform", Username: "admin", SessionID: "session-1", TokenType: TokenTypeAdmin,
	}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	if expiresAt.IsZero() {
		t.Fatalf("expiresAt is zero")
	}
	claims, err := service.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if claims.UserID != "user-1" || claims.TokenType != TokenTypeAdmin {
		t.Fatalf("claims = %+v, want admin user-1", claims)
	}
}

func TestTokenServiceRejectsWrongSecret(t *testing.T) {
	service := NewService("secret", func() time.Time { return time.Unix(100, 0).UTC() })
	token, _, err := service.Sign(Subject{
		UserID: "user-1", TenantID: "platform", Username: "admin", SessionID: "session-1", TokenType: TokenTypeAdmin,
	}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	wrongSecretService := NewService("wrong-secret", func() time.Time { return time.Unix(100, 0).UTC() })
	if _, err := wrongSecretService.Parse(token); err == nil {
		t.Fatalf("Parse() with wrong secret error = nil, want error")
	}
}

func TestTokenServiceRejectsExpiredToken(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	service := NewService("secret", func() time.Time { return now })
	token, _, err := service.Sign(Subject{
		UserID: "user-1", TenantID: "platform", Username: "admin", SessionID: "session-1", TokenType: TokenTypeAdmin,
	}, time.Minute)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	now = now.Add(2 * time.Minute)
	if _, err := service.Parse(token); err == nil {
		t.Fatalf("Parse() expired token error = nil, want error")
	}
}
