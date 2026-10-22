package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/refreshTechRusuh/event-service/internal/event/domain/dto"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
	"github.com/refreshTechRusuh/event-service/internal/event/port/outbound"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
)

type EventUsecase struct {
	repo outbound.IEventRepository
	log  *slog.Logger
}

func NewEventUsecase(repo outbound.IEventRepository, log *slog.Logger) *EventUsecase {
	return &EventUsecase{repo: repo, log: log}
}

func (u *EventUsecase) ListEvents(ctx context.Context) ([]dto.EventListItem, error) {
	events, err := u.repo.ListAll(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	result := make([]dto.EventListItem, 0, len(events))
	for _, e := range events {
		item := dto.EventListItem{ID: e.ID, Name: e.Name, Status: string(e.Status)}
		if e.SaleOpensAt != nil {
			item.SaleOpensAt = e.SaleOpensAt.UTC().Format(time.RFC3339)
		}
		result = append(result, item)
	}
	return result, nil
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
		catDTOs = append(catDTOs, dto.CategoryDTO{ID: c.ID, Name: c.Name, Price: c.Price, Available: c.Available})
	}
	return dto.EventDetail{ID: e.ID, Name: e.Name, Status: string(e.Status), Categories: catDTOs}, nil
}

func (u *EventUsecase) AdminListEvents(ctx context.Context) ([]dto.AdminEventListItem, error) {
	events, err := u.repo.ListAll(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	result := make([]dto.AdminEventListItem, 0, len(events))
	for _, e := range events {
		cats, _ := u.repo.ListCategories(ctx, e.ID)
		catDTOs := make([]dto.CategoryDTO, 0, len(cats))
		for _, c := range cats {
			catDTOs = append(catDTOs, dto.CategoryDTO{ID: c.ID, Name: c.Name, Price: c.Price, Available: c.Available, Capacity: c.Capacity})
		}
		result = append(result, dto.AdminEventListItem{ID: e.ID, Name: e.Name, Status: string(e.Status), Categories: catDTOs})
	}
	return result, nil
}

func (u *EventUsecase) CreateEvent(ctx context.Context, req dto.CreateEventRequest) (dto.AdminEventListItem, error) {
	e := entity.Event{ID: newUUID(), Name: req.Name, Status: entity.EventStatus(req.Status), CreatedAt: time.Now()}
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
	if req.Name != "" { e.Name = req.Name }
	if req.Status != "" { e.Status = entity.EventStatus(req.Status) }
	if err := u.repo.Update(ctx, e); err != nil {
		return dto.AdminEventListItem{}, apperror.Internal(err)
	}
	return dto.AdminEventListItem{ID: e.ID, Name: e.Name, Status: string(e.Status)}, nil
}

func (u *EventUsecase) DeleteEvent(ctx context.Context, eventID string) error {
	return u.repo.Delete(ctx, eventID)
}

func (u *EventUsecase) CreateCategory(ctx context.Context, eventID string, req dto.CreateCategoryRequest) (dto.CategoryDTO, error) {
	c := entity.TicketCategory{
		ID: newUUID(), EventID: eventID, Name: req.Name,
		Price: req.Price, Capacity: req.Capacity, Available: req.Capacity, // available = capacity saat dibuat
	}
	if err := u.repo.CreateCategory(ctx, c); err != nil {
		return dto.CategoryDTO{}, apperror.Internal(err)
	}
	return dto.CategoryDTO{ID: c.ID, Name: c.Name, Price: c.Price, Available: c.Available, Capacity: c.Capacity}, nil
}

func (u *EventUsecase) UpdateCategory(ctx context.Context, categoryID string, req dto.UpdateCategoryRequest) (dto.CategoryDTO, error) {
	// Usecase mengambil category saat ini, menghitung available_baru = capacity_baru - (capacity_lama - available_lama)
	// Implementasi lengkap di Task B2-1.4
	return dto.CategoryDTO{ID: categoryID}, nil
}

func newUUID() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}
