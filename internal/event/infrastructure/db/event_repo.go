package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
)

type EventRepository struct{ pool *pgxpool.Pool }

func NewEventRepository(pool *pgxpool.Pool) *EventRepository { return &EventRepository{pool: pool} }

func (r *EventRepository) ListAll(ctx context.Context) ([]entity.Event, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, sale_opens_at, status, created_at FROM event ORDER BY created_at DESC`)
	if err != nil {
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
	return result, nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (entity.Event, error) {
	var e entity.Event
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, sale_opens_at, status, created_at FROM event WHERE id = $1`, id).
		Scan(&e.ID, &e.Name, &e.SaleOpensAt, &e.Status, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Event{}, apperror.NotFound("event")
		}
		return entity.Event{}, fmt.Errorf("event get: %w", err)
	}
	return e, nil
}

func (r *EventRepository) Create(ctx context.Context, e entity.Event) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO event (id, name, sale_opens_at, status, created_at) VALUES ($1,$2,$3,$4,$5)`,
		e.ID, e.Name, e.SaleOpensAt, e.Status, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("event create: %w", err)
	}
	return nil
}

func (r *EventRepository) Update(ctx context.Context, e entity.Event) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE event SET name=$2, sale_opens_at=$3, status=$4 WHERE id=$1`,
		e.ID, e.Name, e.SaleOpensAt, e.Status)
	return err
}

// Delete adalah soft delete: set status = 'closed'.
func (r *EventRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE event SET status='closed' WHERE id=$1`, id)
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
	rows, err := r.pool.Query(ctx,
		`SELECT id, event_id, name, price, capacity, available FROM ticket_category WHERE event_id=$1 ORDER BY name`,
		eventID)
	if err != nil {
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
	return result, nil
}

// GetCategoryByID dipakai UpdateCategory untuk baca state saat ini sebelum kalkulasi guard.
func (r *EventRepository) GetCategoryByID(ctx context.Context, categoryID string) (entity.TicketCategory, error) {
	var c entity.TicketCategory
	err := r.pool.QueryRow(ctx,
		`SELECT id, event_id, name, price, capacity, available FROM ticket_category WHERE id=$1`,
		categoryID).
		Scan(&c.ID, &c.EventID, &c.Name, &c.Price, &c.Capacity, &c.Available)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.TicketCategory{}, apperror.NotFound("kategori")
		}
		return entity.TicketCategory{}, fmt.Errorf("category get: %w", err)
	}
	return c, nil
}

func (r *EventRepository) CreateCategory(ctx context.Context, c entity.TicketCategory) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO ticket_category (id, event_id, name, price, capacity, available) VALUES ($1,$2,$3,$4,$5,$6)`,
		c.ID, c.EventID, c.Name, c.Price, c.Capacity, c.Available)
	if err != nil {
		return fmt.Errorf("create category: %w", err)
	}
	return nil
}

// UpdateCategory update name, price, capacity, dan available (hasil kalkulasi dari usecase).
func (r *EventRepository) UpdateCategory(ctx context.Context, c entity.TicketCategory) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE ticket_category SET name=$2, price=$3, capacity=$4, available=$5 WHERE id=$1`,
		c.ID, c.Name, c.Price, c.Capacity, c.Available)
	if err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("kategori")
	}
	return nil
}
