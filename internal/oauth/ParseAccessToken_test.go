package oauth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseAccessToken(t *testing.T) {
	tests := []struct {
		name    string
		claims  jwt.MapClaims
		want    bool
		wantErr string
	}{
		{
			name:   "valid token",
			claims: jwt.MapClaims{"exp": time.Now().Add(10 * time.Minute).Unix()},
			want:   true,
		},
		{
			name:   "expired token",
			claims: jwt.MapClaims{"exp": time.Now().Add(-time.Minute).Unix()},
		},
		{
			name:   "token within refresh window",
			claims: jwt.MapClaims{"exp": time.Now().Add(4 * time.Minute).Unix()},
		},
		{
			name:    "missing exp",
			claims:  jwt.MapClaims{"sub": "user"},
			wantErr: "does not contain exp",
		},
		{
			name:    "invalid exp type",
			claims:  jwt.MapClaims{"exp": "not-a-timestamp"},
			wantErr: "invalid type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, tt.claims)
			accessToken, err := token.SignedString([]byte("test-secret"))
			if err != nil {
				t.Fatalf("sign test token: %v", err)
			}

			got, err := ParseAccessToken(accessToken)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAccessToken() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseAccessToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
