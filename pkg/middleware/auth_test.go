package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/refreshTechRusuh/event-service/pkg/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-for-middleware-tests"

// okHandler adalah handler sederhana yang selalu return 200.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// ---- helpers ----------------------------------------------------------------

func makeAccessJWT(t *testing.T, secret string, ttlExpiry int64, expired bool) string {
	t.Helper()
	type claims struct {
		EventID             string `json:"eventId"`
		BookingTTLExpiresAt int64  `json:"bookingTtlExpiresAt"`
		jwt.RegisteredClaims
	}
	exp := time.Now().Add(5 * time.Minute)
	if expired {
		exp = time.Now().Add(-1 * time.Minute)
	}
	c := claims{
		EventID:             "evt-001",
		BookingTTLExpiresAt: ttlExpiry,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func makeAdminJWT(t *testing.T, secret, role string) string {
	t.Helper()
	// Pakai RegisteredClaims.Subject agar encode sebagai "sub"
	// dan MapClaims untuk custom field agar di-encode dengan key yang benar
	claims := jwt.MapClaims{
		"sub":  "user-1",
		"role": role,
		"exp":  time.Now().Add(1 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

// ---- RequireAccessToken -----------------------------------------------------

func TestRequireAccessToken_NoHeader_401(t *testing.T) {
	h := middleware.RequireAccessToken(testSecret)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAccessToken_OpaqueToken_Pass(t *testing.T) {
	// Token non-JWT (opaque) harus lolos karena BE-1 pakai opaque token
	h := middleware.RequireAccessToken(testSecret)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Access-Token", "opaque-token-from-be1")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireAccessToken_ValidJWT_Pass(t *testing.T) {
	ttl := time.Now().Add(5 * time.Minute).Unix()
	token := makeAccessJWT(t, testSecret, ttl, false)
	h := middleware.RequireAccessToken(testSecret)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Access-Token", token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireAccessToken_ExpiredJWT_401(t *testing.T) {
	token := makeAccessJWT(t, testSecret, 0, true) // expired JWT
	h := middleware.RequireAccessToken(testSecret)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Access-Token", token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAccessToken_ExpiredTTL_401(t *testing.T) {
	// JWT valid tapi bookingTtlExpiresAt sudah lewat
	ttl := time.Now().Add(-2 * time.Minute).Unix()
	token := makeAccessJWT(t, testSecret, ttl, false)
	h := middleware.RequireAccessToken(testSecret)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Access-Token", token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAccessToken_WrongSecret_401(t *testing.T) {
	token := makeAccessJWT(t, "wrong-secret", 0, false)
	h := middleware.RequireAccessToken(testSecret)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Access-Token", token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	// JWT dengan wrong secret di-detect sebagai invalid JWT → 401
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAccessToken_ClaimsInjected(t *testing.T) {
	// Verifikasi claims dimasukkan ke context
	ttl := time.Now().Add(5 * time.Minute).Unix()
	token := makeAccessJWT(t, testSecret, ttl, false)

	var gotClaims *middleware.AccessTokenClaims
	checkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims, _ = middleware.GetAccessTokenClaims(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	h := middleware.RequireAccessToken(testSecret)(checkHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Access-Token", token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, gotClaims)
	assert.Equal(t, "evt-001", gotClaims.EventID)
}

// ---- OptionalAuth -----------------------------------------------------------

func TestOptionalAuth_NoToken_PassThrough(t *testing.T) {
	h := middleware.OptionalAuth(testSecret)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOptionalAuth_ValidAdminToken_InjectsRole(t *testing.T) {
	token := makeAdminJWT(t, testSecret, "admin")

	var gotRole string
	checkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRole, _ = r.Context().Value(middleware.ExportCtxUserRole).(string)
		w.WriteHeader(http.StatusOK)
	})

	h := middleware.OptionalAuth(testSecret)(checkHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "admin", gotRole)
}

func TestOptionalAuth_InvalidToken_PassThrough(t *testing.T) {
	// Invalid token harus tetap lolos (optional — tidak blokir request)
	h := middleware.OptionalAuth(testSecret)(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ---- RequireRole ------------------------------------------------------------

func TestRequireRole_NoRole_403(t *testing.T) {
	h := middleware.RequireRole("admin")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireRole_WrongRole_403(t *testing.T) {
	// Chain(handler, OptionalAuth, RequireRole): OptionalAuth di index 0 → outermost (runs first, injects role)
	// kemudian RequireRole di index 1 → inner (runs after, checks role)
	token := makeAdminJWT(t, testSecret, "user")
	h := middleware.Chain(
		okHandler,
		middleware.OptionalAuth(testSecret),
		middleware.RequireRole("admin"),
	)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireRole_CorrectRole_Pass(t *testing.T) {
	token := makeAdminJWT(t, testSecret, "admin")
	// OptionalAuth di index 0 (outermost) → inject role dulu, lalu RequireRole cek
	h := middleware.Chain(
		okHandler,
		middleware.OptionalAuth(testSecret),
		middleware.RequireRole("admin"),
	)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ---- Chain ------------------------------------------------------------------

func TestChain_MultipleMiddlewares(t *testing.T) {
	order := []string{}
	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}

	h := middleware.Chain(okHandler, m1, m2)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Chain(handler, m1, m2): m1 adalah outermost (di-apply terakhir karena loop dari akhir ke awal)
	// Execution order: m1-before → m2-before → handler → m2-after → m1-after
	assert.Equal(t, []string{"m1-before", "m2-before", "m2-after", "m1-after"}, order)
}
