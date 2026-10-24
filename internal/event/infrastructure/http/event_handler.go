package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/event-service/internal/event/application/usecase"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/dto"
	"github.com/refreshTechRusuh/event-service/internal/event/infrastructure/db"
	infraevent "github.com/refreshTechRusuh/event-service/internal/event/infrastructure/event"
	"github.com/refreshTechRusuh/event-service/internal/event/port/inbound"
	"github.com/refreshTechRusuh/event-service/pkg/apinizer"
	"github.com/refreshTechRusuh/event-service/pkg/kafka"
	"github.com/refreshTechRusuh/event-service/pkg/middleware"
)

var validate = validator.New()

// Handler memegang semua HTTP handler untuk event domain.
type Handler struct{ usecase inbound.IEventUsecase }

// NewWiredHandler mewiring semua dependency (factory).
func NewWiredHandler(pool *pgxpool.Pool, producer *kafka.Producer) *Handler {
	h, _ := NewWiredHandlerWithUC(pool, producer)
	return h
}

// NewWiredHandlerWithUC mewiring dependency dan mengembalikan Handler + EventUsecase.
// EventUsecase diekspos agar Kafka consumer bisa memanggil MarkSoldOut.
func NewWiredHandlerWithUC(pool *pgxpool.Pool, producer *kafka.Producer) (*Handler, *usecase.EventUsecase) {
	log := slog.Default()
	repo := db.NewEventRepository(pool, log)
	var ep *infraevent.KafkaProducer
	if producer != nil {
		ep = infraevent.NewKafkaProducer(producer, slog.Default())
	}
	uc := usecase.NewEventUsecase(repo, ep, slog.Default())
	return &Handler{usecase: uc}, uc
}

// RegisterRoutes mendaftarkan semua route event ke mux.
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

	// Internal (dipanggil booking-service saat sold-out terdeteksi — tidak perlu auth publik)
	mux.HandleFunc("POST /internal/events/{eventId}/sold-out", h.MarkSoldOut)

	// Health
	mux.HandleFunc("GET /sys/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"00000","message":"pong","data":{}}`))
	})
}

// ---- Handlers ---------------------------------------------------------------

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	result, err := h.usecase.ListEvents(r.Context())
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "OK", result)
}

func (h *Handler) GetEventDetail(w http.ResponseWriter, r *http.Request) {
	result, err := h.usecase.GetEventDetail(r.Context(), r.PathValue("eventId"))
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "OK", result)
}

func (h *Handler) AdminListEvents(w http.ResponseWriter, r *http.Request) {
	result, err := h.usecase.AdminListEvents(r.Context())
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "OK", result)
}

func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.WriteError(w, fmt.Errorf("invalid JSON body"))
		return
	}
	if err := validate.Struct(req); err != nil {
		apinizer.WriteError(w, fmt.Errorf("validation: %s", err.Error()))
		return
	}
	result, err := h.usecase.CreateEvent(r.Context(), req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "Event dibuat", result)
}

func (h *Handler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.WriteError(w, fmt.Errorf("invalid JSON body"))
		return
	}
	result, err := h.usecase.UpdateEvent(r.Context(), r.PathValue("eventId"), req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "Event diperbarui", result)
}

func (h *Handler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	if err := h.usecase.DeleteEvent(r.Context(), r.PathValue("eventId")); err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "Event dihapus", map[string]bool{"ok": true})
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.WriteError(w, fmt.Errorf("invalid JSON body"))
		return
	}
	if err := validate.Struct(req); err != nil {
		apinizer.WriteError(w, fmt.Errorf("validation: %s", err.Error()))
		return
	}
	result, err := h.usecase.CreateCategory(r.Context(), r.PathValue("eventId"), req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "Kategori dibuat", result)
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.WriteError(w, fmt.Errorf("invalid JSON body"))
		return
	}
	result, err := h.usecase.UpdateCategory(r.Context(), r.PathValue("categoryId"), req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "Kategori diperbarui", result)
}

// MarkSoldOut adalah endpoint internal — dipanggil Booking Service via HTTP saat sold-out.
func (h *Handler) MarkSoldOut(w http.ResponseWriter, r *http.Request) {
	if err := h.usecase.MarkSoldOut(r.Context(), r.PathValue("eventId")); err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "Event ditandai sold out", map[string]bool{"ok": true})
}
