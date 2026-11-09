package usecase_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/refreshTechRusuh/event-service/internal/event/application/usecase"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/dto"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- CreateEvent branches --------------------------------------------------

func TestCreateEvent_WithExplicitStatus(t *testing.T) {
	uc, _, _ := newUC(t)
	result, err := uc.CreateEvent(context.Background(), dto.CreateEventRequest{
		Name:   "Festival Musik",
		Status: "on_sale", // explicit status, bukan default draft
	})
	require.NoError(t, err)
	assert.Equal(t, "on_sale", result.Status)
}

func TestCreateEvent_InvalidSaleOpensAt_Validation(t *testing.T) {
	uc, _, _ := newUC(t)
	_, err := uc.CreateEvent(context.Background(), dto.CreateEventRequest{
		Name:        "Event Baru",
		SaleOpensAt: "bukan-tanggal-valid",
	})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrValidation, appErr.Code)
}

func TestCreateEvent_DuplicateName(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-dup")
	repo.events["evt-dup"] = entity.Event{ID: "evt-dup", Name: "Test Event", Status: "draft"}

	_, err := uc.CreateEvent(context.Background(), dto.CreateEventRequest{
		Name: "Test Event", // nama sudah ada
	})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrDuplicateEvent, appErr.Code)
}

func TestCreateEvent_RepoError(t *testing.T) {
	repo := newMockRepo()
	errRepo := &errCreateRepo{mockRepo: repo}
	uc := usecase.NewEventUsecase(errRepo, &mockProducer{}, nil, slog.Default())

	_, err := uc.CreateEvent(context.Background(), dto.CreateEventRequest{Name: "New Event"})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrInternal, appErr.Code)
}

// ---- MarkSoldOut branches --------------------------------------------------

func TestMarkSoldOut_AlreadySoldOut_Idempoten(t *testing.T) {
	uc, repo, prod := newUC(t)
	repo.events["evt-sold"] = entity.Event{
		ID:     "evt-sold",
		Name:   "Sold Out Event",
		Status: entity.EventStatusSoldOut, // sudah sold out
	}

	err := uc.MarkSoldOut(context.Background(), "evt-sold")
	require.NoError(t, err)
	// Tidak ada event yang dipublish karena sudah sold out
	assert.Empty(t, prod.published)
}

func TestMarkSoldOut_NotFound(t *testing.T) {
	uc, _, _ := newUC(t)
	err := uc.MarkSoldOut(context.Background(), "evt-not-exist")
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrNotFound, appErr.Code)
}

func TestMarkSoldOut_UpdateError(t *testing.T) {
	repo := newMockRepo()
	// Event ada di repo tapi tidak bisa di-update (repo tidak mengenal ID ini untuk update)
	// Pakai errUpdateRepo yang return error saat Update
	errRepo := &errUpdateRepo{mockRepo: repo}
	errRepo.events["evt-fail"] = entity.Event{ID: "evt-fail", Name: "Event", Status: "on_sale"}
	uc := usecase.NewEventUsecase(errRepo, &mockProducer{}, nil, slog.Default())

	err := uc.MarkSoldOut(context.Background(), "evt-fail")
	require.Error(t, err)
}

func TestMarkSoldOut_PublishesKafka(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-publish"] = entity.Event{ID: "evt-publish", Name: "Event", Status: "on_sale"}

	err := uc.MarkSoldOut(context.Background(), "evt-publish")
	require.NoError(t, err)
	// Booking seharusnya ter-update ke sold_out di repo
	assert.Equal(t, entity.EventStatusSoldOut, repo.events["evt-publish"].Status)
}

// ---- ListEvents repo error -------------------------------------------------

func TestListEvents_RepoError(t *testing.T) {
	errRepo := &errListRepo{mockRepo: newMockRepo()}
	uc := usecase.NewEventUsecase(errRepo, &mockProducer{}, nil, slog.Default())

	_, _, err := uc.ListEvents(context.Background(), 1, 10)
	require.Error(t, err)
}

func TestAdminListEvents_RepoError(t *testing.T) {
	errRepo := &errListRepo{mockRepo: newMockRepo()}
	uc := usecase.NewEventUsecase(errRepo, &mockProducer{}, nil, slog.Default())

	_, _, err := uc.AdminListEvents(context.Background(), 1, 10)
	require.Error(t, err)
}

// ---- CreateCategory event not found ----------------------------------------

func TestCreateCategory_EventNotFound_Branch(t *testing.T) {
	uc, _, _ := newUC(t)
	// evt-nonexist tidak ada di repo
	_, err := uc.CreateCategory(context.Background(), "evt-nonexist", dto.CreateCategoryRequest{
		Name: "VIP", Price: 500000, Capacity: 100,
	})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrNotFound, appErr.Code)
}

// ---- UpdateEvent SaleOpensAt invalid ---------------------------------------

func TestUpdateEvent_InvalidSaleOpensAt_Branch(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-upd")

	_, err := uc.UpdateEvent(context.Background(), "evt-upd", dto.UpdateEventRequest{
		SaleOpensAt: "invalid-date",
	})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrValidation, appErr.Code)
}

// ---- Mock overrides --------------------------------------------------------

type errCreateRepo struct {
	*mockRepo
}

func (r *errCreateRepo) Create(_ context.Context, _ entity.Event) error {
	return errors.New("db create failed")
}

type errUpdateRepo struct {
	*mockRepo
}

func (r *errUpdateRepo) Update(_ context.Context, _ entity.Event) error {
	return errors.New("db update failed")
}

type errListRepo struct {
	*mockRepo
}

func (r *errListRepo) ListPaged(_ context.Context, _, _ int) ([]entity.Event, error) {
	return nil, errors.New("db list failed")
}

func (r *errListRepo) ListPagedPublic(_ context.Context, _, _ int) ([]entity.Event, error) {
	return nil, errors.New("db list failed")
}

func (r *errListRepo) CountAll(_ context.Context) (int, error) {
	return 0, errors.New("db count failed")
}

func (r *errListRepo) CountPublic(_ context.Context) (int, error) {
	return 0, errors.New("db count failed")
}
