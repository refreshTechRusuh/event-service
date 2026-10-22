package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
)

type EventRepository struct{ pool *pgxpool.Pool }

func NewEventRepository(pool *pgxpool.Pool) *EventRepository { return &EventRepository{pool: pool} }

func (r *EventRepository) ListAll(ctx context.Context) ([]entity.Event, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, sale_opens_at, status, created_at FROM event ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []entity.Event
	for rows.Next() {
		var e entity.Event
		if err := rows.Scan(&e.ID, &e.Name, &e.SaleOpensAt, &e.Status, &e.CreatedAt); err != nil { return nil, err }
		result = append(result, e)
	}
	return result, nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (entity.Event, error) {
	var e entity.Event
	err := r.pool.QueryRow(ctx, `SELECT id, name, sale_opens_at, status, created_at FROM event WHERE id = $1`, id).
		Scan(&e.ID, &e.Name, &e.SaleOpensAt, &e.Status, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return entity.Event{}, apperror.NotFound("event") }
		return entity.Event{}, fmt.Errorf("event get: %w", err)
	}
	return e, nil
}

func (r *EventRepository) Create(ctx context.Context, e entity.Event) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO event (id, name, sale_opens_at, status, created_at) VALUES ($1,$2,$3,$4,$5)`,
		e.ID, e.Name, e.SaleOpensAt, e.Status, e.CreatedAt)
	return err
}

func (r *EventRepository) Update(ctx context.Context, e entity.Event) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE event SET name=$2, sale_opens_at=$3, status=$4 WHERE id=$1`,
		e.ID, e.Name, e.SaleOpensAt, e.Status)
	return err
}

func (r *EventRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE event SET status='closed' WHERE id=$1`, id)
	return err
}

func (r *EventRepository) ListCategories(ctx context.Context, eventID string) ([]entity.TicketCategory, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, event_id, name, price, capacity, available FROM ticket_category WHERE event_id=$1`, eventID)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []entity.TicketCategory
	for rows.Next() {
		var c entity.TicketCategory
		if err := rows.Scan(&c.ID, &c.EventID, &c.Name, &c.Price, &c.Capacity, &c.Available); err != nil { return nil, err }
		result = append(result, c)
	}
	return result, nil
}

func (r *EventRepository) CreateCategory(ctx context.Context, c entity.TicketCategory) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO ticket_category (id, event_id, name, price, capacity, available) VALUES ($1,$2,$3,$4,$5,$6)`,
		c.ID, c.EventID, c.Name, c.Price, c.Capacity, c.Available)
	return err
}

func (r *EventRepository) UpdateCategory(ctx context.Context, c entity.TicketCategory) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE ticket_category SET name=$2, price=$3, capacity=$4 WHERE id=$1`,
		c.ID, c.Name, c.Price, c.Capacity)
	return err
}

var _ = time.Now // suppress unused import
