package outbound

import (
	"context"

	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
)

type IEventRepository interface {
	ListAll(ctx context.Context) ([]entity.Event, error)
	GetByID(ctx context.Context, id string) (entity.Event, error)
	Create(ctx context.Context, e entity.Event) error
	Update(ctx context.Context, e entity.Event) error
	Delete(ctx context.Context, id string) error

	ListCategories(ctx context.Context, eventID string) ([]entity.TicketCategory, error)
	CreateCategory(ctx context.Context, c entity.TicketCategory) error
	UpdateCategory(ctx context.Context, c entity.TicketCategory) error
}
