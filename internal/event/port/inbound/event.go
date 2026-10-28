package inbound

import (
	"context"

	"github.com/refreshTechRusuh/event-service/internal/event/domain/dto"
)

// IEventUsecase adalah kontrak yang dipanggil HTTP handler.
type IEventUsecase interface {
	// Public
	ListEvents(ctx context.Context, page, size int) ([]dto.EventListItem, dto.PageMeta, error)
	GetEventDetail(ctx context.Context, eventID string) (dto.EventDetail, error)

	// Admin
	AdminListEvents(ctx context.Context, page, size int) ([]dto.AdminEventListItem, dto.PageMeta, error)
	CreateEvent(ctx context.Context, req dto.CreateEventRequest) (dto.AdminEventListItem, error)
	UpdateEvent(ctx context.Context, eventID string, req dto.UpdateEventRequest) (dto.AdminEventListItem, error)
	DeleteEvent(ctx context.Context, eventID string) error

	CreateCategory(ctx context.Context, eventID string, req dto.CreateCategoryRequest) (dto.CategoryDTO, error)
	UpdateCategory(ctx context.Context, categoryID string, req dto.UpdateCategoryRequest) (dto.CategoryDTO, error)

	// Internal — dipanggil saat Booking Service mendeteksi sold-out
	MarkSoldOut(ctx context.Context, eventID string) error
}
