package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	ctxUserID   contextKey = "userId"
	ctxUserRole contextKey = "userRole"
)

func OptionalAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token := extractBearer(r); token != "" {
				if userID, role, ok := parseUserJWT(token, secret); ok {
					r = r.WithContext(context.WithValue(r.Context(), ctxUserID, userID))
					r = r.WithContext(context.WithValue(r.Context(), ctxUserRole, role))
				} else {
					slog.WarnContext(r.Context(), "OptionalAuth: JWT tidak valid atau expired, request dilanjutkan tanpa user",
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
						slog.Int("secret_len", len(secret)),
					)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAccessToken(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Access-Token") == "" {
				slog.ErrorContext(r.Context(), "RequireAccessToken: header X-Access-Token tidak ada",
					slog.String("path", r.URL.Path),
					slog.String("method", r.Method),
					slog.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, `{"status":"401","message":"Access Token tidak ada","data":{}}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Cek apakah user sudah di-inject oleh OptionalAuth
			userRole, _ := r.Context().Value(ctxUserRole).(string)
			if userRole != role {
				slog.ErrorContext(r.Context(), "RequireRole: akses ditolak, role tidak sesuai",
					slog.String("path", r.URL.Path),
					slog.String("method", r.Method),
					slog.String("required_role", role),
					slog.String("user_role", userRole),
					slog.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, `{"status":"403","message":"Akses ditolak","data":{}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func TransactionID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		txID := r.Header.Get("X-Transaction-Id")
		if txID == "" {
			txID = "tx-placeholder"
		}
		w.Header().Set("X-Transaction-Id", txID)
		next.ServeHTTP(w, r)
	})
}

func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func parseUserJWT(tokenStr, secret string) (string, string, bool) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		slog.Error("parseUserJWT: gagal parse JWT", slog.String("error", err.Error()))
		return "", "", false
	}
	if !token.Valid {
		slog.Error("parseUserJWT: token tidak valid")
		return "", "", false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		slog.Error("parseUserJWT: gagal cast claims ke MapClaims")
		return "", "", false
	}
	sub, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	if sub == "" {
		slog.Error("parseUserJWT: claim 'sub' kosong")
		return "", "", false
	}
	return sub, role, true
}
