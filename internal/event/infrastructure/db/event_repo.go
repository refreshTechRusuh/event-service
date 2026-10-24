package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
)

// dbLog adalah helper untuk log query DB secara konsisten.
func dbLog(ctx context.Context, log *slog.Logger, op string, start time.Time, err error, args ...any) {
	elapsed := time.Since(start)
	base := append([]any{slog.String("op", op), slog.String("duration", elapsed.String())}, args...)
	if err != nil {
		log.ErrorContext(ctx, "db query failed", append(base, slog.String("error", err.Error()))...)
	} else {
		log.InfoContext(ctx, "db query ok", base...)
	}
}

type EventRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewEventRepository(pool *pgxpool.Pool, log *slog.Logger) *EventRepository {
	return &EventRepository{pool: pool, log: log}
}

func (r *EventRepository) ListAll(ctx context.Context) ([]entity.Event, error) {
	start := time.Now()
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, sale_opens_at, status, created_at FROM event ORDER BY created_at DESC`)
	if err != nil {
		dbLog(ctx, r.log, "event.list_all", start, err)
		return nil, fmt.Errorf("event list: %w", err)
	}
	defer rows.Close()
	var result []entity.Event
	for rows.Next() {
		var e entity.Event
		if err := rows.Scan(&e.ID, &e.Name, &e.SaleOpensAt, &e.Status, &e.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	dbLog(ctx, r.log, "event.list_all", start, nil, slog.Int("count", len(result)))
	return result, nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (entity.Event, error) {
	start := time.Now()
	var e entity.Event
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, sale_opens_at, status, created_at FROM event WHERE id = $1`, id).
		Scan(&e.ID, &e.Name, &e.SaleOpensAt, &e.Status, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			dbLog(ctx, r.log, "event.get_by_id", start, err, slog.String("eventId", id))
			return entity.Event{}, apperror.NotFound("event")
		}
		dbLog(ctx, r.log, "event.get_by_id", start, err, slog.String("eventId", id))
		return entity.Event{}, fmt.Errorf("event get: %w", err)
	}
	dbLog(ctx, r.log, "event.get_by_id", start, nil,
		slog.String("eventId", id),
		slog.String("name", e.Name),
		slog.String("status", string(e.Status)))
	return e, nil
}

func (r *EventRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	start := time.Now()
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM event WHERE LOWER(name) = LOWER($1))`, name).
		Scan(&exists)
	dbLog(ctx, r.log, "event.exists_by_name", start, err,
		slog.String("name", name),
		slog.Bool("exists", exists))
	if err != nil {
		return false, fmt.Errorf("event exists by name: %w", err)
	}
	return exists, nil
}

func (r *EventRepository) Create(ctx context.Context, e entity.Event) error {
	start := time.Now()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO event (id, name, sale_opens_at, status, created_at) VALUES ($1,$2,$3,$4,$5)`,
		e.ID, e.Name, e.SaleOpensAt, e.Status, e.CreatedAt)
	dbLog(ctx, r.log, "event.create", start, err,
		slog.String("eventId", e.ID),
		slog.String("name", e.Name))
	if err != nil {
		return fmt.Errorf("event create: %w", err)
	}
	return nil
}

func (r *EventRepository) Update(ctx context.Context, e entity.Event) error {
	start := time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE event SET name=$2, sale_opens_at=$3, status=$4 WHERE id=$1`,
		e.ID, e.Name, e.SaleOpensAt, e.Status)
	dbLog(ctx, r.log, "event.update", start, err,
		slog.String("eventId", e.ID),
		slog.String("status", string(e.Status)))
	return err
}

// Delete adalah soft delete: set status = 'closed'.
func (r *EventRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	tag, err := r.pool.Exec(ctx, `UPDATE event SET status='closed' WHERE id=$1`, id)
	dbLog(ctx, r.log, "event.delete", start, err, slog.String("eventId", id))
	if err != nil {
		return fmt.Errorf("event delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("event")
	}
	return nil
}

// ---- Category ---------------------------------------------------------------

func (r *EventRepository) ListCategories(ctx context.Context, eventID string) ([]entity.TicketCategory, error) {
	start := time.Now()
	rows, err := r.pool.Query(ctx,
		`SELECT id, event_id, name, price, capacity, available FROM ticket_category WHERE event_id=$1 ORDER BY name`,
		eventID)
	if err != nil {
		dbLog(ctx, r.log, "category.list", start, err, slog.String("eventId", eventID))
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()
	var result []entity.TicketCategory
	for rows.Next() {
		var c entity.TicketCategory
		if err := rows.Scan(&c.ID, &c.EventID, &c.Name, &c.Price, &c.Capacity, &c.Available); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	dbLog(ctx, r.log, "category.list", start, nil,
		slog.String("eventId", eventID),
		slog.Int("count", len(result)))
	return result, nil
}

// GetCategoryByID dipakai UpdateCategory untuk baca state saat ini sebelum kalkulasi guard.
func (r *EventRepository) GetCategoryByID(ctx context.Context, categoryID string) (entity.TicketCategory, error) {
	start := time.Now()
	var c entity.TicketCategory
	err := r.pool.QueryRow(ctx,
		`SELECT id, event_id, name, price, capacity, available FROM ticket_category WHERE id=$1`,
		categoryID).
		Scan(&c.ID, &c.EventID, &c.Name, &c.Price, &c.Capacity, &c.Available)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			dbLog(ctx, r.log, "category.get_by_id", start, err, slog.String("categoryId", categoryID))
			return entity.TicketCategory{}, apperror.NotFound("kategori")
		}
		dbLog(ctx, r.log, "category.get_by_id", start, err, slog.String("categoryId", categoryID))
		return entity.TicketCategory{}, fmt.Errorf("category get: %w", err)
	}
	dbLog(ctx, r.log, "category.get_by_id", start, nil,
		slog.String("categoryId", categoryID),
		slog.Int("available", c.Available))
	return c, nil
}

func (r *EventRepository) CreateCategory(ctx context.Context, c entity.TicketCategory) error {
	start := time.Now()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO ticket_category (id, event_id, name, price, capacity, available) VALUES ($1,$2,$3,$4,$5,$6)`,
		c.ID, c.EventID, c.Name, c.Price, c.Capacity, c.Available)
	dbLog(ctx, r.log, "category.create", start, err,
		slog.String("categoryId", c.ID),
		slog.String("eventId", c.EventID),
		slog.Int("capacity", c.Capacity))
	if err != nil {
		return fmt.Errorf("create category: %w", err)
	}
	return nil
}

// UpdateCategory update name, price, capacity, dan available.
func (r *EventRepository) UpdateCategory(ctx context.Context, c entity.TicketCategory) error {
	start := time.Now()
	tag, err := r.pool.Exec(ctx,
		`UPDATE ticket_category SET name=$2, price=$3, capacity=$4, available=$5 WHERE id=$1`,
		c.ID, c.Name, c.Price, c.Capacity, c.Available)
	dbLog(ctx, r.log, "category.update", start, err,
		slog.String("categoryId", c.ID),
		slog.Int("available", c.Available))
	if err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("kategori")
	}
	return nil
}
