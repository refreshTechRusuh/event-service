package outbound

import (
	"context"

	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
)

// IEventRepository mengelola persistensi event dan ticket_category.
type IEventRepository interface {
	ListAll(ctx context.Context) ([]entity.Event, error)
	ListPaged(ctx context.Context, limit, offset int) ([]entity.Event, error) // pagination untuk GET /events
	CountAll(ctx context.Context) (int, error)                                // total event untuk meta pagination
	GetByID(ctx context.Context, id string) (entity.Event, error)
	ExistsByName(ctx context.Context, name string) (bool, error) // uniqueness check
	Create(ctx context.Context, e entity.Event) error
	Update(ctx context.Context, e entity.Event) error
	Delete(ctx context.Context, id string) error // soft delete (set status=closed)

	ListCategories(ctx context.Context, eventID string) ([]entity.TicketCategory, error)
	GetCategoryByID(ctx context.Context, categoryID string) (entity.TicketCategory, error) // dibutuhkan UpdateCategory
	CreateCategory(ctx context.Context, c entity.TicketCategory) error
	UpdateCategory(ctx context.Context, c entity.TicketCategory) error
}

// IEventProducer mempublish event ke Kafka (opsional — nil di test/local tanpa Kafka).
type IEventProducer interface {
	// PublishEventSoldOut mempublish event.sold_out saat semua stok habis.
	// Dikonsumsi Queue Service untuk menutup antrian.
	PublishEventSoldOut(ctx context.Context, eventID string, payload any) error
}

// IStockBroadcaster mem-broadcast perubahan stok ke klien SSE lewat Redis/Valkey
// Pub/Sub channel "stock:{eventId}" — channel yang sama di-subscribe booking-service.
// Opsional: nil bila Valkey tidak dikonfigurasi (MarkSoldOut tetap jalan tanpa broadcast).
type IStockBroadcaster interface {
	// BroadcastSoldOut mengirim frame {eventId, soldOut:true} ke subscriber SSE.
	BroadcastSoldOut(ctx context.Context, eventID string) error
}
