package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	ctxUserID            contextKey = "userId"
	ctxUserRole          contextKey = "userRole"
	ctxAccessTokenClaims contextKey = "accessTokenClaims"
)

// AccessTokenClaims adalah claim dari Access Token yang diterbitkan Queue Service.
type AccessTokenClaims struct {
	EventID             string `json:"eventId"`
	QueueToken          string `json:"queueToken"`
	BookingTTLExpiresAt int64  `json:"bookingTtlExpiresAt"` // unix timestamp
}

// GetAccessTokenClaims mengambil AccessTokenClaims dari context.
func GetAccessTokenClaims(ctx context.Context) (*AccessTokenClaims, bool) {
	v, ok := ctx.Value(ctxAccessTokenClaims).(*AccessTokenClaims)
	return v, ok
}

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
			tokenStr := r.Header.Get("X-Access-Token")
			if tokenStr == "" {
				slog.ErrorContext(r.Context(), "RequireAccessToken: header X-Access-Token tidak ada",
					slog.String("path", r.URL.Path),
					slog.String("method", r.Method),
					slog.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, `{"status":"401","message":"Access Token tidak ada","data":{}}`, http.StatusUnauthorized)
				return
			}
			// Parse JWT untuk ambil bookingTtlExpiresAt — dipakai hitung remainingSeconds di event detail.
			// Kalau token expired, tolak dengan 401.
			// Kalau token bukan JWT (opaque dari BE-1), tetap lanjut tanpa claims.
			claims, err := parseAccessTokenClaims(tokenStr, secret)
			if err != nil {
				// Hanya tolak kalau token adalah JWT tapi expired/invalid.
				// Token opaque (non-JWT) tidak akan bisa di-parse dan dibiarkan lewat.
				if isJWT(tokenStr) {
					slog.ErrorContext(r.Context(), "RequireAccessToken: Access Token expired atau tidak valid",
						slog.String("path", r.URL.Path),
						slog.String("error", err.Error()),
					)
					http.Error(w, `{"status":"401","message":"Access Token expired atau tidak valid","data":{}}`, http.StatusUnauthorized)
					return
				}
			} else {
				// Token valid -- cek apakah sudah expired berdasarkan bookingTtlExpiresAt
				if claims.BookingTTLExpiresAt > 0 && time.Now().Unix() > claims.BookingTTLExpiresAt {
					slog.ErrorContext(r.Context(), "RequireAccessToken: Access Token sudah expired (bookingTtlExpiresAt)",
						slog.String("path", r.URL.Path),
						slog.Int64("expired_at", claims.BookingTTLExpiresAt),
					)
					http.Error(w, `{"status":"401","message":"Access Token sudah expired","data":{}}`, http.StatusUnauthorized)
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), ctxAccessTokenClaims, claims))
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

// parseAccessTokenClaims mem-parse Access Token JWT (diterbitkan Queue Service)
// dan mengekstrak AccessTokenClaims. Dipanggil oleh RequireAccessToken.
// Kalau token bukan JWT (opaque) atau gagal verify, kembalikan error.
func parseAccessTokenClaims(tokenStr, secret string) (*AccessTokenClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}
	claims := &AccessTokenClaims{}
	claims.EventID, _ = mc["eventId"].(string)
	claims.QueueToken, _ = mc["queueToken"].(string)
	if v, ok := mc["bookingTtlExpiresAt"].(float64); ok {
		claims.BookingTTLExpiresAt = int64(v)
	}
	return claims, nil
}

// isJWT cek apakah string adalah format JWT (3 bagian dipisah titik).
func isJWT(s string) bool {
	return strings.Count(s, ".") == 2
}
