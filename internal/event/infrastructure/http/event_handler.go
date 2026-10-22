package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/event-service/internal/event/application/usecase"
	"github.com/refreshTechRusuh/event-service/internal/event/infrastructure/db"
	"github.com/refreshTechRusuh/event-service/internal/event/port/inbound"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/dto"
	"github.com/refreshTechRusuh/event-service/pkg/apinizer"
	"github.com/refreshTechRusuh/event-service/pkg/middleware"
)

var validate = validator.New()

type Handler struct{ usecase inbound.IEventUsecase }

func NewWiredHandler(pool *pgxpool.Pool) *Handler {
	repo := db.NewEventRepository(pool)
	uc := usecase.NewEventUsecase(repo, slog.Default())
	return &Handler{usecase: uc}
}

func RegisterRoutes(mux *http.ServeMux, h *Handler, accessTokenSecret string) {
	withAccessToken := func(next http.HandlerFunc) http.Handler {
		return middleware.Chain(http.HandlerFunc(next), middleware.RequireAccessToken(accessTokenSecret))
	}
	requireAdmin := func(next http.HandlerFunc) http.Handler {
		return middleware.Chain(http.HandlerFunc(next), middleware.RequireRole("admin"))
	}

	// Public
	mux.HandleFunc("GET /api/v1/events", h.ListEvents)
	mux.Handle("GET /api/v1/events/{eventId}", withAccessToken(h.GetEventDetail))

	// Admin
	mux.Handle("GET /api/v1/admin/events", requireAdmin(h.AdminListEvents))
	mux.Handle("POST /api/v1/admin/events", requireAdmin(h.CreateEvent))
	mux.Handle("PUT /api/v1/admin/events/{eventId}", requireAdmin(h.UpdateEvent))
	mux.Handle("DELETE /api/v1/admin/events/{eventId}", requireAdmin(h.DeleteEvent))
	mux.Handle("POST /api/v1/admin/events/{eventId}/categories", requireAdmin(h.CreateCategory))
	mux.Handle("PUT /api/v1/admin/categories/{categoryId}", requireAdmin(h.UpdateCategory))

	// Health
	mux.HandleFunc("GET /sys/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"00000","message":"pong","data":{}}`))
	})
}

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	result, err := h.usecase.ListEvents(r.Context())
	if err != nil { apinizer.WriteError(w, err); return }
	apinizer.Success(w, "OK", result)
}

func (h *Handler) GetEventDetail(w http.ResponseWriter, r *http.Request) {
	result, err := h.usecase.GetEventDetail(r.Context(), r.PathValue("eventId"))
	if err != nil { apinizer.WriteError(w, err); return }
	apinizer.Success(w, "OK", result)
}

func (h *Handler) AdminListEvents(w http.ResponseWriter, r *http.Request) {
	result, err := h.usecase.AdminListEvents(r.Context())
	if err != nil { apinizer.WriteError(w, err); return }
	apinizer.Success(w, "OK", result)
}

func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.WriteError(w, fmt.Errorf("invalid JSON")); return
	}
	if err := validate.Struct(req); err != nil {
		apinizer.WriteError(w, fmt.Errorf("validation: %s", err.Error())); return
	}
	result, err := h.usecase.CreateEvent(r.Context(), req)
	if err != nil { apinizer.WriteError(w, err); return }
	apinizer.Success(w, "Event dibuat", result)
}

func (h *Handler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateEventRequest
	json.NewDecoder(r.Body).Decode(&req)
	result, err := h.usecase.UpdateEvent(r.Context(), r.PathValue("eventId"), req)
	if err != nil { apinizer.WriteError(w, err); return }
	apinizer.Success(w, "Event diperbarui", result)
}

func (h *Handler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	if err := h.usecase.DeleteEvent(r.Context(), r.PathValue("eventId")); err != nil {
		apinizer.WriteError(w, err); return
	}
	apinizer.Success(w, "Event dihapus", map[string]bool{"ok": true})
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.WriteError(w, fmt.Errorf("invalid JSON")); return
	}
	result, err := h.usecase.CreateCategory(r.Context(), r.PathValue("eventId"), req)
	if err != nil { apinizer.WriteError(w, err); return }
	apinizer.Success(w, "Kategori dibuat", result)
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateCategoryRequest
	json.NewDecoder(r.Body).Decode(&req)
	result, err := h.usecase.UpdateCategory(r.Context(), r.PathValue("categoryId"), req)
	if err != nil { apinizer.WriteError(w, err); return }
	apinizer.Success(w, "Kategori diperbarui", result)
}
