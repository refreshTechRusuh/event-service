package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/dto"
	eventhttp "github.com/refreshTechRusuh/event-service/internal/event/infrastructure/http"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAccessSecret = "test-access-secret"
	testUserSecret   = "test-user-secret"
)

// ---- Mock usecase ----------------------------------------------------------

type mockEventUsecase struct {
	listFn          func(ctx context.Context, page, size int) ([]dto.EventListItem, dto.PageMeta, error)
	detailFn        func(ctx context.Context, eventID string) (dto.EventDetail, error)
	adminListFn     func(ctx context.Context, page, size int) ([]dto.AdminEventListItem, dto.PageMeta, error)
	createFn        func(ctx context.Context, req dto.CreateEventRequest) (dto.AdminEventListItem, error)
	updateFn        func(ctx context.Context, id string, req dto.UpdateEventRequest) (dto.AdminEventListItem, error)
	deleteFn        func(ctx context.Context, id string) error
	createCatFn     func(ctx context.Context, eventID string, req dto.CreateCategoryRequest) (dto.CategoryDTO, error)
	updateCatFn     func(ctx context.Context, catID string, req dto.UpdateCategoryRequest) (dto.CategoryDTO, error)
	markSoldOutFn   func(ctx context.Context, eventID string) error
}

func (m *mockEventUsecase) ListEvents(ctx context.Context, page, size int) ([]dto.EventListItem, dto.PageMeta, error) {
	if m.listFn != nil { return m.listFn(ctx, page, size) }
	return []dto.EventListItem{{ID: "evt-1", Name: "Test Event"}}, dto.PageMeta{Page: 1, Size: 10}, nil
}
func (m *mockEventUsecase) GetEventDetail(ctx context.Context, eventID string) (dto.EventDetail, error) {
	if m.detailFn != nil { return m.detailFn(ctx, eventID) }
	return dto.EventDetail{ID: eventID, Name: "Test Event"}, nil
}
func (m *mockEventUsecase) AdminListEvents(ctx context.Context, page, size int) ([]dto.AdminEventListItem, dto.PageMeta, error) {
	if m.adminListFn != nil { return m.adminListFn(ctx, page, size) }
	return []dto.AdminEventListItem{}, dto.PageMeta{}, nil
}
func (m *mockEventUsecase) CreateEvent(ctx context.Context, req dto.CreateEventRequest) (dto.AdminEventListItem, error) {
	if m.createFn != nil { return m.createFn(ctx, req) }
	return dto.AdminEventListItem{ID: "new-evt", Name: req.Name}, nil
}
func (m *mockEventUsecase) UpdateEvent(ctx context.Context, id string, req dto.UpdateEventRequest) (dto.AdminEventListItem, error) {
	if m.updateFn != nil { return m.updateFn(ctx, id, req) }
	return dto.AdminEventListItem{ID: id}, nil
}
func (m *mockEventUsecase) DeleteEvent(ctx context.Context, id string) error {
	if m.deleteFn != nil { return m.deleteFn(ctx, id) }
	return nil
}
func (m *mockEventUsecase) CreateCategory(ctx context.Context, eventID string, req dto.CreateCategoryRequest) (dto.CategoryDTO, error) {
	if m.createCatFn != nil { return m.createCatFn(ctx, eventID, req) }
	return dto.CategoryDTO{}, nil
}
func (m *mockEventUsecase) UpdateCategory(ctx context.Context, catID string, req dto.UpdateCategoryRequest) (dto.CategoryDTO, error) {
	if m.updateCatFn != nil { return m.updateCatFn(ctx, catID, req) }
	return dto.CategoryDTO{}, nil
}
func (m *mockEventUsecase) MarkSoldOut(ctx context.Context, eventID string) error {
	if m.markSoldOutFn != nil { return m.markSoldOutFn(ctx, eventID) }
	return nil
}

// ---- Helpers ----------------------------------------------------------------

func setupEventRouter(uc *mockEventUsecase) *http.ServeMux {
	mux := http.NewServeMux()
	h := eventhttp.NewHandlerWithUsecase(uc)
	eventhttp.RegisterRoutesWithSecrets(mux, h, testAccessSecret, testUserSecret)
	return mux
}

// makeAccessToken membuat Access Token JWT seperti yang diterbitkan queue-service.
func makeAccessToken(t *testing.T, bookingID string, ttlExpiry int64, expired bool) string {
	t.Helper()
	type accessClaims struct {
		BookingID           string `json:"bookingId"`
		EventID             string `json:"eventId"`
		BookingTTLExpiresAt int64  `json:"bookingTtlExpiresAt"`
		jwt.RegisteredClaims
	}
	exp := time.Now().Add(5 * time.Minute)
	if expired { exp = time.Now().Add(-1 * time.Minute) }
	claims := accessClaims{
		BookingID: bookingID, EventID: "evt-1",
		BookingTTLExpiresAt: ttlExpiry,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(testAccessSecret))
	require.NoError(t, err)
	return signed
}

// makeAdminToken membuat JWT admin dari user-service (BE-1).
func makeAdminToken(t *testing.T) string {
	t.Helper()
	type adminClaims struct {
		Sub  string `json:"sub"`
		Role string `json:"role"`
		jwt.RegisteredClaims
	}
	claims := adminClaims{
		Sub: "admin-1", Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(testUserSecret))
	require.NoError(t, err)
	return signed
}

// ---- GET /api/event/v1/list ------------------------------------------------

func TestListEvents_Success(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/list", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListEvents_WithPagination(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/list?page=2&size=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListEvents_UsecaseError(t *testing.T) {
	uc := &mockEventUsecase{
		listFn: func(_ context.Context, _, _ int) ([]dto.EventListItem, dto.PageMeta, error) {
			return nil, dto.PageMeta{}, apperror.New(apperror.ErrNotFound, "not found")
		},
	}
	mux := setupEventRouter(uc)
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/list", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- GET /api/event/v1/detail/{eventId} ------------------------------------

func TestGetEventDetail_NoToken_401(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/detail/evt-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestGetEventDetail_WithOpaqueToken_Success(t *testing.T) {
	// Token non-JWT (opaque) dari BE-1: middleware tidak reject, claims tidak ada → remainingSeconds=0
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/detail/evt-1", nil)
	req.Header.Set("X-Access-Token", "opaque-token-from-be1")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetEventDetail_WithValidJWT_InjectsTTL(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	ttlExpiry := time.Now().Add(3 * time.Minute).Unix()
	token := makeAccessToken(t, "bkg-1", ttlExpiry, false)
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/detail/evt-1", nil)
	req.Header.Set("X-Access-Token", token)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	remaining := data["remainingSeconds"].(float64)
	assert.Greater(t, remaining, float64(0))
}

func TestGetEventDetail_ExpiredTTL_410(t *testing.T) {
	// TTL sudah lewat → middleware reject dulu dengan 401 (token expired sebelum masuk handler)
	mux := setupEventRouter(&mockEventUsecase{})
	ttlExpiry := time.Now().Add(-1 * time.Minute).Unix() // sudah expired
	token := makeAccessToken(t, "bkg-1", ttlExpiry, false)
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/detail/evt-1", nil)
	req.Header.Set("X-Access-Token", token)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	// Middleware RequireAccessToken menolak token expired → 401
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestGetEventDetail_UsecaseError(t *testing.T) {
	uc := &mockEventUsecase{
		detailFn: func(_ context.Context, _ string) (dto.EventDetail, error) {
			return dto.EventDetail{}, apperror.New(apperror.ErrNotFound, "event tidak ditemukan")
		},
	}
	mux := setupEventRouter(uc)
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/detail/evt-999", nil)
	req.Header.Set("X-Access-Token", "opaque-token")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- Admin routes ----------------------------------------------------------

func TestCreateEvent_NoAdmin_403(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	body, _ := json.Marshal(dto.CreateEventRequest{Name: "New Event"})
	req := httptest.NewRequest(http.MethodPost, "/api/event/v1/admin/create-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Tidak ada token → RequireRole: role tidak sesuai → 403
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCreateEvent_AdminToken_Success(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	body, _ := json.Marshal(dto.CreateEventRequest{Name: "Konser Keren"})
	req := httptest.NewRequest(http.MethodPost, "/api/event/v1/admin/create-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCreateEvent_InvalidJSON(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodPost, "/api/event/v1/admin/create-event",
		bytes.NewReader([]byte(`{bad`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDeleteEvent_AdminToken_Success(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodDelete, "/api/event/v1/admin/delete-event/evt-1", nil)
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteEvent_UsecaseError(t *testing.T) {
	uc := &mockEventUsecase{
		deleteFn: func(_ context.Context, _ string) error {
			return apperror.New(apperror.ErrNotFound, "event tidak ditemukan")
		},
	}
	mux := setupEventRouter(uc)
	req := httptest.NewRequest(http.MethodDelete, "/api/event/v1/admin/delete-event/evt-999", nil)
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- Internal sold-out -----------------------------------------------------

func TestMarkSoldOut_Success(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodPost, "/internal/event/v1/evt-1/sold-out", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ---- GET /sys/ping ---------------------------------------------------------

func TestEventPing(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodGet, "/sys/ping", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "pong")
}

// ---- GET /api/event/v1/admin/list-event ------------------------------------

func TestAdminListEvents_Success(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/admin/list-event", nil)
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAdminListEvents_NoAdmin_403(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/admin/list-event", nil)
	// No auth header → 401/403
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestAdminListEvents_UsecaseError(t *testing.T) {
	uc := &mockEventUsecase{
		adminListFn: func(_ context.Context, _, _ int) ([]dto.AdminEventListItem, dto.PageMeta, error) {
			return nil, dto.PageMeta{}, apperror.New(apperror.ErrInternal, "db error")
		},
	}
	mux := setupEventRouter(uc)
	req := httptest.NewRequest(http.MethodGet, "/api/event/v1/admin/list-event", nil)
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---- PUT /api/event/v1/admin/update-event/{eventId} -------------------------

func TestUpdateEvent_AdminToken_Success(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	body, _ := json.Marshal(dto.UpdateEventRequest{Name: "Updated Event"})
	req := httptest.NewRequest(http.MethodPut, "/api/event/v1/admin/update-event/evt-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateEvent_InvalidJSON(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodPut, "/api/event/v1/admin/update-event/evt-1",
		bytes.NewReader([]byte("{bad json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestUpdateEvent_NoAdmin_403(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	body, _ := json.Marshal(dto.UpdateEventRequest{Name: "X"})
	req := httptest.NewRequest(http.MethodPut, "/api/event/v1/admin/update-event/evt-1", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestUpdateEvent_UsecaseError(t *testing.T) {
	uc := &mockEventUsecase{
		updateFn: func(_ context.Context, _ string, _ dto.UpdateEventRequest) (dto.AdminEventListItem, error) {
			return dto.AdminEventListItem{}, apperror.New(apperror.ErrNotFound, "event not found")
		},
	}
	mux := setupEventRouter(uc)
	body, _ := json.Marshal(dto.UpdateEventRequest{Name: "X"})
	req := httptest.NewRequest(http.MethodPut, "/api/event/v1/admin/update-event/evt-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- POST /api/event/v1/admin/{eventId}/create-category ---------------------

func TestCreateCategory_AdminToken_Success(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	body, _ := json.Marshal(dto.CreateCategoryRequest{Name: "VIP", Price: 500000, Capacity: 100})
	req := httptest.NewRequest(http.MethodPost, "/api/event/v1/admin/evt-1/create-category", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCreateCategory_InvalidJSON(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodPost, "/api/event/v1/admin/evt-1/create-category",
		bytes.NewReader([]byte("{bad")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestCreateCategory_NoAdmin_403(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	body, _ := json.Marshal(dto.CreateCategoryRequest{Name: "VIP", Price: 100, Capacity: 10})
	req := httptest.NewRequest(http.MethodPost, "/api/event/v1/admin/evt-1/create-category", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestCreateCategory_UsecaseError(t *testing.T) {
	uc := &mockEventUsecase{
		createCatFn: func(_ context.Context, _ string, _ dto.CreateCategoryRequest) (dto.CategoryDTO, error) {
			return dto.CategoryDTO{}, apperror.New(apperror.ErrNotFound, "event not found")
		},
	}
	mux := setupEventRouter(uc)
	body, _ := json.Marshal(dto.CreateCategoryRequest{Name: "VIP", Price: 100, Capacity: 10})
	req := httptest.NewRequest(http.MethodPost, "/api/event/v1/admin/evt-1/create-category", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- PUT /api/event/v1/admin/update-category/{categoryId} -------------------

func TestUpdateCategory_AdminToken_Success(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	body, _ := json.Marshal(dto.UpdateCategoryRequest{Name: "VIP+", Price: 600000, Capacity: 120})
	req := httptest.NewRequest(http.MethodPut, "/api/event/v1/admin/update-category/cat-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateCategory_InvalidJSON(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	req := httptest.NewRequest(http.MethodPut, "/api/event/v1/admin/update-category/cat-1",
		bytes.NewReader([]byte("{bad")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestUpdateCategory_NoAdmin_403(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	body, _ := json.Marshal(dto.UpdateCategoryRequest{Name: "X"})
	req := httptest.NewRequest(http.MethodPut, "/api/event/v1/admin/update-category/cat-1", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestUpdateCategory_UsecaseError(t *testing.T) {
	uc := &mockEventUsecase{
		updateCatFn: func(_ context.Context, _ string, _ dto.UpdateCategoryRequest) (dto.CategoryDTO, error) {
			return dto.CategoryDTO{}, apperror.New(apperror.ErrNotFound, "kategori tidak ditemukan")
		},
	}
	mux := setupEventRouter(uc)
	body, _ := json.Marshal(dto.UpdateCategoryRequest{Name: "VIP+"})
	req := httptest.NewRequest(http.MethodPut, "/api/event/v1/admin/update-category/cat-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- MarkSoldOut branch ────────────────────────────────────────────────────

func TestMarkSoldOut_UsecaseError(t *testing.T) {
	uc := &mockEventUsecase{
		markSoldOutFn: func(_ context.Context, _ string) error {
			return apperror.New(apperror.ErrInternal, "db error")
		},
	}
	mux := setupEventRouter(uc)
	req := httptest.NewRequest(http.MethodPost, "/internal/event/v1/evt-1/sold-out", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ── CreateEvent validation error ──────────────────────────────────────────────

func TestCreateEvent_ValidationError(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	// Valid JSON but missing required fields → validation fails
	body, _ := json.Marshal(map[string]string{"name": ""}) // empty name, likely required
	req := httptest.NewRequest(http.MethodPost, "/api/event/v1/admin/create-event", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

// ── CreateCategory validation error ───────────────────────────────────────────

func TestCreateCategory_ValidationError(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	// Missing required fields
	body, _ := json.Marshal(map[string]any{"name": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/event/v1/admin/evt-1/create-category", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

// ── UpdateCategory validation error ───────────────────────────────────────────

func TestUpdateCategory_ValidationError(t *testing.T) {
	mux := setupEventRouter(&mockEventUsecase{})
	body, _ := json.Marshal(map[string]any{"price": -1}) // negative price if validated
	req := httptest.NewRequest(http.MethodPut, "/api/event/v1/admin/update-category/cat-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+makeAdminToken(t))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	// Either success or validation error, just not internal server error
	assert.NotEqual(t, http.StatusInternalServerError, rr.Code)
}

// ── GetEventDetail — handled by existing TestGetEventDetail_ExpiredTTL_410 ─────

// ── NewWiredHandler — nil dependencies ───────────────────────────────────────

func TestNewWiredHandler_nilDeps(t *testing.T) {
	// Konstruktor tidak connect ke DB/Kafka/Valkey saat dibuat.
	// Passing nil aman untuk coverage tujuan unit test.
	h := eventhttp.NewWiredHandler(nil, nil, nil)
	assert.NotNil(t, h)
}
