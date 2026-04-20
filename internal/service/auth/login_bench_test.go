package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

func BenchmarkResolvePrincipal(b *testing.B) {
	service := NewLoginService()
	claims := &token.AdminCustomClaims{
		AdminUserInfo: token.AdminUserInfo{
			UserID:          12,
			Username:        "tester",
			Nickname:        "Tester",
			Email:           "tester@example.com",
			FullPhoneNumber: "+8613800000000",
			PhoneNumber:     "13800000000",
			CountryCode:     "+86",
			IsSuperAdmin:    0,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jwt-bench",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	service.blacklistLookupFn = func(_ string) (bool, error) {
		return false, nil
	}
	service.tokenRevokedLookupFn = func(_ string) bool {
		return false
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		principal, ok := service.resolvePrincipalFromClaims(claims)
		if !ok || principal == nil {
			b.Fatal("expected principal")
		}
	}
}
