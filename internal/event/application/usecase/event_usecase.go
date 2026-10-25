package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/dto"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
	"github.com/refreshTechRusuh/event-service/internal/event/port/outbound"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
)

type EventUsecase struct {
	repo        outbound.IEventRepository
	producer    outbound.IEventProducer   // opsional — nil bila tidak ada Kafka
	broadcaster outbound.IStockBroadcaster // opsional — nil bila tidak ada Valkey
	log         *slog.Logger
}

func NewEventUsecase(repo outbound.IEventRepository, producer outbound.IEventProducer, broadcaster outbound.IStockBroadcaster, log *slog.Logger) *EventUsecase {
	return &EventUsecase{repo: repo, producer: producer, broadcaster: broadcaster, log: log}
}

// ---- Public read -----------------------------------------------------------

// ListEvents mengembalikan daftar event ter-paginate beserta info meta.
// page 1-based, size di-clamp ke rentang aman di handler sebelum sampai sini.
func (u *EventUsecase) ListEvents(ctx context.Context, page, size int) ([]dto.EventListItem, dto.PageMeta, error) {
	total, err := u.repo.CountAll(ctx)
	if err != nil {
		return nil, dto.PageMeta{}, apperror.Internal(err)
	}
	offset := (page - 1) * size
	events, err := u.repo.ListPaged(ctx, size, offset)
	if err != nil {
		return nil, dto.PageMeta{}, apperror.Internal(err)
	}
	result := make([]dto.EventListItem, 0, len(events))
	for _, e := range events {
		item := dto.EventListItem{ID: e.ID, Name: e.Name, Status: string(e.Status)}
		if e.SaleOpensAt != nil {
			item.SaleOpensAt = e.SaleOpensAt.UTC().Format(time.RFC3339)
		}
		result = append(result, item)
	}
	totalPages := 0
	if size > 0 {
		totalPages = (total + size - 1) / size // ceil
	}
	meta := dto.PageMeta{Page: page, Size: size, TotalItems: total, TotalPages: totalPages}
	return result, meta, nil
}

func (u *EventUsecase) GetEventDetail(ctx context.Context, eventID string) (dto.EventDetail, error) {
	e, err := u.repo.GetByID(ctx, eventID)
	if err != nil {
		return dto.EventDetail{}, apperror.NotFound("event")
	}
	cats, err := u.repo.ListCategories(ctx, eventID)
	if err != nil {
		return dto.EventDetail{}, apperror.Internal(err)
	}
	catDTOs := make([]dto.CategoryDTO, 0, len(cats))
	for _, c := range cats {
		catDTOs = append(catDTOs, dto.CategoryDTO{
			ID: c.ID, Name: c.Name, Price: c.Price, Available: c.Available,
		})
	}
	return dto.EventDetail{ID: e.ID, Name: e.Name, Status: string(e.Status), Categories: catDTOs}, nil
}

// ---- Admin -----------------------------------------------------------------

func (u *EventUsecase) AdminListEvents(ctx context.Context, page, size int) ([]dto.AdminEventListItem, dto.PageMeta, error) {
	total, err := u.repo.CountAll(ctx)
	if err != nil {
		return nil, dto.PageMeta{}, apperror.Internal(err)
	}
	offset := (page - 1) * size
	events, err := u.repo.ListPaged(ctx, size, offset)
	if err != nil {
		return nil, dto.PageMeta{}, apperror.Internal(err)
	}
	result := make([]dto.AdminEventListItem, 0, len(events))
	for _, e := range events {
		cats, _ := u.repo.ListCategories(ctx, e.ID)
		catDTOs := make([]dto.CategoryDTO, 0, len(cats))
		for _, c := range cats {
			catDTOs = append(catDTOs, dto.CategoryDTO{
				ID: c.ID, Name: c.Name, Price: c.Price,
				Available: c.Available, Capacity: c.Capacity,
			})
		}
		result = append(result, dto.AdminEventListItem{
			ID: e.ID, Name: e.Name, Status: string(e.Status), Categories: catDTOs,
		})
	}
	totalPages := 0
	if size > 0 {
		totalPages = (total + size - 1) / size // ceil
	}
	meta := dto.PageMeta{Page: page, Size: size, TotalItems: total, TotalPages: totalPages}
	return result, meta, nil
}

func (u *EventUsecase) CreateEvent(ctx context.Context, req dto.CreateEventRequest) (dto.AdminEventListItem, error) {
	status := req.Status
	if status == "" {
		status = "draft" // default sesuai openapi.yaml AdminCreateEventRequest
	}
	// Uniqueness check: tolak nama yang sudah ada (case-insensitive)
	exists, err := u.repo.ExistsByName(ctx, req.Name)
	if err != nil {
		return dto.AdminEventListItem{}, apperror.Internal(err)
	}
	if exists {
		return dto.AdminEventListItem{}, apperror.DuplicateEvent(req.Name)
	}
	e := entity.Event{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Status:    entity.EventStatus(status),
		CreatedAt: time.Now(),
	}
	if req.SaleOpensAt != "" {
		t, err := time.Parse(time.RFC3339, req.SaleOpensAt)
		if err != nil {
			return dto.AdminEventListItem{}, apperror.Validation("saleOpensAt format harus ISO-8601")
		}
		e.SaleOpensAt = &t
	}
	if err := u.repo.Create(ctx, e); err != nil {
		return dto.AdminEventListItem{}, apperror.Internal(err)
	}
	return dto.AdminEventListItem{ID: e.ID, Name: e.Name, Status: string(e.Status)}, nil
}

func (u *EventUsecase) UpdateEvent(ctx context.Context, eventID string, req dto.UpdateEventRequest) (dto.AdminEventListItem, error) {
	e, err := u.repo.GetByID(ctx, eventID)
	if err != nil {
		return dto.AdminEventListItem{}, apperror.NotFound("event")
	}
	if req.Name != "" {
		e.Name = req.Name
	}
	if req.Status != "" {
		e.Status = entity.EventStatus(req.Status)
	}
	if req.SaleOpensAt != "" {
		t, err := time.Parse(time.RFC3339, req.SaleOpensAt)
		if err != nil {
			return dto.AdminEventListItem{}, apperror.Validation("saleOpensAt format harus ISO-8601")
		}
		e.SaleOpensAt = &t
	}
	if err := u.repo.Update(ctx, e); err != nil {
		return dto.AdminEventListItem{}, apperror.Internal(err)
	}
	return dto.AdminEventListItem{ID: e.ID, Name: e.Name, Status: string(e.Status)}, nil
}

func (u *EventUsecase) DeleteEvent(ctx context.Context, eventID string) error {
	// Soft delete: set status = closed
	return u.repo.Delete(ctx, eventID)
}

// ---- Category CRUD ---------------------------------------------------------

func (u *EventUsecase) CreateCategory(ctx context.Context, eventID string, req dto.CreateCategoryRequest) (dto.CategoryDTO, error) {
	// Pastikan event ada
	if _, err := u.repo.GetByID(ctx, eventID); err != nil {
		return dto.CategoryDTO{}, apperror.NotFound("event")
	}

	c := entity.TicketCategory{
		ID:        uuid.New().String(),
		EventID:   eventID,
		Name:      req.Name,
		Price:     req.Price,
		Capacity:  req.Capacity,
		Available: req.Capacity, // available = capacity saat pertama dibuat (Req 13.2)
	}
	if err := u.repo.CreateCategory(ctx, c); err != nil {
		return dto.CategoryDTO{}, apperror.Internal(err)
	}
	return dto.CategoryDTO{
		ID: c.ID, Name: c.Name, Price: c.Price,
		Available: c.Available, Capacity: c.Capacity,
	}, nil
}

// UpdateCategory mengubah kategori dengan guard: available_baru >= 0.
// Rumus: available_baru = capacity_baru - (capacity_lama - available_lama)
// = available_lama + (capacity_baru - capacity_lama)
// Artinya: kenaikan capacity → naikkan available; penurunan capacity → kurangi available.
// Tolak bila capacity_baru < (capacity_lama - available_lama) karena available_baru < 0. (Req 13.3, 13.4)
func (u *EventUsecase) UpdateCategory(ctx context.Context, categoryID string, req dto.UpdateCategoryRequest) (dto.CategoryDTO, error) {
	current, err := u.repo.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return dto.CategoryDTO{}, apperror.NotFound("kategori")
	}

	updated := current // copy

	if req.Name != "" {
		updated.Name = req.Name
	}
	if req.Price > 0 {
		updated.Price = req.Price
	}
	if req.Capacity > 0 && req.Capacity != current.Capacity {
		// Hitung available baru
		tiketTerjual := current.Capacity - current.Available // tiket yang sudah terpesan/terjual
		availableBaru := req.Capacity - tiketTerjual
		if availableBaru < 0 {
			return dto.CategoryDTO{}, apperror.New(apperror.ErrValidation,
				"capacity baru tidak boleh membuat available < 0 (sudah ada tiket terjual/hold)")
		}
		updated.Capacity = req.Capacity
		updated.Available = availableBaru
	}

	if err := u.repo.UpdateCategory(ctx, updated); err != nil {
		return dto.CategoryDTO{}, apperror.Internal(err)
	}
	return dto.CategoryDTO{
		ID: updated.ID, Name: updated.Name, Price: updated.Price,
		Available: updated.Available, Capacity: updated.Capacity,
	}, nil
}

// MarkSoldOut dipanggil oleh Booking Service (via event Kafka atau internal call)
// saat semua available = 0. (Req 15.1)
func (u *EventUsecase) MarkSoldOut(ctx context.Context, eventID string) error {
	e, err := u.repo.GetByID(ctx, eventID)
	if err != nil {
		return apperror.NotFound("event")
	}
	if e.Status == entity.EventStatusSoldOut {
		return nil // idempoten
	}
	e.Status = entity.EventStatusSoldOut
	if err := u.repo.Update(ctx, e); err != nil {
		return apperror.Internal(err)
	}
	// Publish event.sold_out ke Kafka agar Queue Service menutup antrian
	if u.producer != nil {
		go u.producer.PublishEventSoldOut(context.WithoutCancel(ctx), eventID,
			map[string]string{"eventId": eventID})
	}
	// Broadcast sold-out ke klien SSE (booking-service) via Redis Pub/Sub.
	if u.broadcaster != nil {
		go func(bctx context.Context) {
			if err := u.broadcaster.BroadcastSoldOut(bctx, eventID); err != nil {
				u.log.ErrorContext(bctx, "broadcast sold-out to SSE failed",
					slog.String("eventId", eventID), slog.Any("err", err))
			}
		}(context.WithoutCancel(ctx))
	}
	u.log.InfoContext(ctx, "event marked sold_out", slog.String("eventId", eventID))
	return nil
}
