package usecase_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/refreshTechRusuh/event-service/internal/event/application/usecase"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/dto"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Mock implementations --------------------------------------------------

type mockRepo struct {
	events     map[string]entity.Event
	categories map[string]entity.TicketCategory
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		events:     make(map[string]entity.Event),
		categories: make(map[string]entity.TicketCategory),
	}
}

func (m *mockRepo) ListAll(_ context.Context) ([]entity.Event, error) {
	result := make([]entity.Event, 0)
	for _, e := range m.events {
		result = append(result, e)
	}
	return result, nil
}

func (m *mockRepo) ListPaged(_ context.Context, limit, offset int) ([]entity.Event, error) {
	all := make([]entity.Event, 0)
	for _, e := range m.events {
		all = append(all, e)
	}
	if offset >= len(all) {
		return []entity.Event{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (m *mockRepo) CountAll(_ context.Context) (int, error) {
	return len(m.events), nil
}

func (m *mockRepo) GetByID(_ context.Context, id string) (entity.Event, error) {
	e, ok := m.events[id]
	if !ok {
		return entity.Event{}, apperror.NotFound("event")
	}
	return e, nil
}

func (m *mockRepo) ExistsByName(_ context.Context, name string) (bool, error) {
	for _, e := range m.events {
		if strings.EqualFold(e.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockRepo) Create(_ context.Context, e entity.Event) error {
	m.events[e.ID] = e
	return nil
}

func (m *mockRepo) Update(_ context.Context, e entity.Event) error {
	if _, ok := m.events[e.ID]; !ok {
		return apperror.NotFound("event")
	}
	m.events[e.ID] = e
	return nil
}

func (m *mockRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.events[id]; !ok {
		return apperror.NotFound("event")
	}
	e := m.events[id]
	e.Status = "closed"
	m.events[id] = e
	return nil
}

func (m *mockRepo) ListCategories(_ context.Context, eventID string) ([]entity.TicketCategory, error) {
	var result []entity.TicketCategory
	for _, c := range m.categories {
		if c.EventID == eventID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockRepo) GetCategoryByID(_ context.Context, id string) (entity.TicketCategory, error) {
	c, ok := m.categories[id]
	if !ok {
		return entity.TicketCategory{}, apperror.NotFound("kategori")
	}
	return c, nil
}

func (m *mockRepo) CreateCategory(_ context.Context, c entity.TicketCategory) error {
	m.categories[c.ID] = c
	return nil
}

func (m *mockRepo) UpdateCategory(_ context.Context, c entity.TicketCategory) error {
	if _, ok := m.categories[c.ID]; !ok {
		return apperror.NotFound("kategori")
	}
	m.categories[c.ID] = c
	return nil
}

// mockProducer
type mockProducer struct{ published []string }

func (p *mockProducer) PublishEventSoldOut(_ context.Context, eventID string, _ any) error {
	p.published = append(p.published, eventID)
	return nil
}

// ---- Helpers ---------------------------------------------------------------

func newUC(t *testing.T) (*usecase.EventUsecase, *mockRepo, *mockProducer) {
	t.Helper()
	repo := newMockRepo()
	prod := &mockProducer{}
	uc := usecase.NewEventUsecase(repo, prod, nil, slog.Default())
	return uc, repo, prod
}

func seedEvent(repo *mockRepo, id string) {
	repo.events[id] = entity.Event{ID: id, Name: "Test Event", Status: "on_sale"}
}

// ---- Tests -----------------------------------------------------------------

func TestCreateCategory_HappyPath(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-1")

	result, err := uc.CreateCategory(context.Background(), "evt-1", dto.CreateCategoryRequest{
		Name: "VIP", Price: 500000, Capacity: 100,
	})
	require.NoError(t, err)
	assert.Equal(t, "VIP", result.Name)
	assert.Equal(t, 100, result.Available)
	assert.Equal(t, 100, result.Capacity) // available = capacity saat dibuat
	assert.NotEmpty(t, result.ID)
}

func TestCreateCategory_EventNotFound(t *testing.T) {
	uc, _, _ := newUC(t)

	_, err := uc.CreateCategory(context.Background(), "nonexistent", dto.CreateCategoryRequest{
		Name: "VIP", Price: 500000, Capacity: 100,
	})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrNotFound, appErr.Code)
}

func TestUpdateCategory_CapacityIncrease(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-1")
	// Setup: capacity=100, available=40 (60 tiket terjual/hold)
	repo.categories["cat-1"] = entity.TicketCategory{
		ID: "cat-1", EventID: "evt-1", Name: "VIP", Price: 500000, Capacity: 100, Available: 40,
	}

	result, err := uc.UpdateCategory(context.Background(), "cat-1", dto.UpdateCategoryRequest{
		Capacity: 120, // naikkan 20 → available naik 20 → 60
	})
	require.NoError(t, err)
	assert.Equal(t, 120, result.Capacity)
	assert.Equal(t, 60, result.Available) // 120 - 60 (tiket terjual) = 60
}

func TestUpdateCategory_CapacityDecrease_Valid(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-1")
	// capacity=100, available=40 → tiketTerjual=60
	repo.categories["cat-1"] = entity.TicketCategory{
		ID: "cat-1", EventID: "evt-1", Name: "VIP", Price: 500000, Capacity: 100, Available: 40,
	}

	// Turunkan ke 80 → available_baru = 80 - 60 = 20 (masih >= 0, valid)
	result, err := uc.UpdateCategory(context.Background(), "cat-1", dto.UpdateCategoryRequest{Capacity: 80})
	require.NoError(t, err)
	assert.Equal(t, 80, result.Capacity)
	assert.Equal(t, 20, result.Available)
}

func TestUpdateCategory_CapacityDecrease_BelowSold_Rejected(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-1")
	// capacity=100, available=40 → tiketTerjual=60
	repo.categories["cat-1"] = entity.TicketCategory{
		ID: "cat-1", EventID: "evt-1", Name: "VIP", Price: 500000, Capacity: 100, Available: 40,
	}

	// Turunkan ke 50 → available_baru = 50 - 60 = -10 → DITOLAK (Req 13.4)
	_, err := uc.UpdateCategory(context.Background(), "cat-1", dto.UpdateCategoryRequest{Capacity: 50})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrValidation, appErr.Code)
}

func TestUpdateCategory_CategoryNotFound(t *testing.T) {
	uc, _, _ := newUC(t)
	_, err := uc.UpdateCategory(context.Background(), "nonexistent", dto.UpdateCategoryRequest{Name: "baru"})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrNotFound, appErr.Code)
}

func TestMarkSoldOut_PublishesEvent(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-1")

	err := uc.MarkSoldOut(context.Background(), "evt-1")
	require.NoError(t, err)

	// Event status harus berubah ke sold_out di repo
	updated := repo.events["evt-1"]
	assert.Equal(t, entity.EventStatusSoldOut, updated.Status)
}

func TestMarkSoldOut_Idempoten(t *testing.T) {
	uc, repo, prod := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Test", Status: entity.EventStatusSoldOut}

	err := uc.MarkSoldOut(context.Background(), "evt-1")
	require.NoError(t, err)
	// Idempoten: tidak publish ulang
	assert.Empty(t, prod.published)
}

func TestMarkSoldOut_EventNotFound(t *testing.T) {
	uc, _, _ := newUC(t)
	err := uc.MarkSoldOut(context.Background(), "nonexistent")
	require.Error(t, err)
}
