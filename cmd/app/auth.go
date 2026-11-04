package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

const googleKeysURL = "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com"

type ctxKey string

const userCtxKey ctxKey = "user"

// AuthUser: кто сделал запрос (данные берём из токена)
type AuthUser struct {
	UID   string
	Email string
}

// UserFromContext достаёт пользователя, которого туда положил Auth
func UserFromContext(ctx context.Context) (AuthUser, bool) {
	u, ok := ctx.Value(userCtxKey).(AuthUser)
	return u, ok
}

// NewAuth возвращает middleware, который проверяет токен Firebase
func NewAuth(projectID string) (func(http.Handler) http.Handler, error) {
	keys, err := keyfunc.NewDefault([]string{googleKeysURL})
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Метрики открыты для Prometheus
			if r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			tokenStr, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !found || tokenStr == "" {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenStr, keys.Keyfunc,
				jwt.WithValidMethods([]string{"RS256"}),
				jwt.WithIssuer("https://securetoken.google.com/"+projectID),
				jwt.WithAudience(projectID),
				jwt.WithExpirationRequired(),
			)
			if err != nil || !token.Valid {
				slog.Warn("bad token", "error", err)
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			claims, _ := token.Claims.(jwt.MapClaims)
			uid, _ := claims["sub"].(string)
			email, _ := claims["email"].(string)
			if uid == "" {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userCtxKey, AuthUser{UID: uid, Email: email})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}