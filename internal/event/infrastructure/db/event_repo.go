package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
)

// sqlVerb menentukan jenis query dari KATA PERTAMA SQL asli (select/insert/update/delete/…).
// Akurat karena membaca SQL yang benar-benar dieksekusi, bukan menebak dari nama op.
func sqlVerb(sql string) string {
	s := strings.ToLower(strings.TrimSpace(sql))
	if i := strings.IndexAny(s, " \t\n"); i >= 0 {
		return s[:i]
	}
	if s == "" {
		return "query"
	}
	return s
}

// dbLog adalah helper untuk log query DB secara konsisten.
//   - INFO : ringkas — verb SQL (select/insert/update/delete) + op + duration + args.
//   - DEBUG: full SQL template (TANPA nilai args, jadi aman dari bocor PII seperti nik/email).
//   - ERROR: verb + full SQL + error, agar langsung terlihat tanpa menurunkan level.
//
// sql = query template dengan placeholder $1,$2 (bukan nilainya).
func dbLog(ctx context.Context, log *slog.Logger, op, sql string, start time.Time, err error, args ...any) {
	elapsed := time.Since(start)
	verb := sqlVerb(sql)
	base := append([]any{slog.String("op", op), slog.String("duration", elapsed.String())}, args...)
	if err != nil {
		log.ErrorContext(ctx, verb+" failed",
			append(base, slog.String("sql", sql), slog.String("error", err.Error()))...)
		return
	}
	log.InfoContext(ctx, verb, base...)
	// Full SQL hanya di DEBUG agar senyap di produksi (LOG_LEVEL=info).
	log.DebugContext(ctx, verb+" sql", append(base, slog.String("sql", sql))...)
}

type EventRepository struct {
	pool IPool
	log  *slog.Logger
}

func NewEventRepository(pool IPool, log *slog.Logger) *EventRepository {
	return &EventRepository{pool: pool, log: log}
}

const (
	sqlEventListAll      = `SELECT id, name, sale_opens_at, status, created_at FROM event ORDER BY created_at DESC`
	sqlEventListPaged    = `SELECT id, name, sale_opens_at, status, created_at FROM event ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	sqlEventCountAll     = `SELECT COUNT(*) FROM event`
	sqlEventGetByID      = `SELECT id, name, sale_opens_at, status, created_at FROM event WHERE id = $1`
	sqlEventExistsByName = `SELECT EXISTS(SELECT 1 FROM event WHERE LOWER(name) = LOWER($1))`
	sqlEventInsert       = `INSERT INTO event (id, name, sale_opens_at, status, created_at) VALUES ($1,$2,$3,$4,$5)`
	sqlEventUpdate       = `UPDATE event SET name=$2, sale_opens_at=$3, status=$4 WHERE id=$1`
	sqlEventSoftDelete   = `UPDATE event SET status='closed' WHERE id=$1`

	sqlCategoryList    = `SELECT id, event_id, name, price, capacity, available FROM ticket_category WHERE event_id=$1 ORDER BY name`
	sqlCategoryGetByID = `SELECT id, event_id, name, price, capacity, available FROM ticket_category WHERE id=$1`
	sqlCategoryInsert  = `INSERT INTO ticket_category (id, event_id, name, price, capacity, available) VALUES ($1,$2,$3,$4,$5,$6)`
	sqlCategoryUpdate  = `UPDATE ticket_category SET name=$2, price=$3, capacity=$4, available=$5 WHERE id=$1`
)

func (r *EventRepository) ListAll(ctx context.Context) ([]entity.Event, error) {
	start := time.Now()
	rows, err := r.pool.Query(ctx, sqlEventListAll)
	if err != nil {
		dbLog(ctx, r.log, "event.list_all", sqlEventListAll, start, err)
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
	dbLog(ctx, r.log, "event.list_all", sqlEventListAll, start, nil, slog.Int("count", len(result)))
	return result, nil
}

func (r *EventRepository) ListPaged(ctx context.Context, limit, offset int) ([]entity.Event, error) {
	start := time.Now()
	rows, err := r.pool.Query(ctx, sqlEventListPaged, limit, offset)
	if err != nil {
		dbLog(ctx, r.log, "event.list_paged", sqlEventListPaged, start, err,
			slog.Int("limit", limit), slog.Int("offset", offset))
		return nil, fmt.Errorf("event list paged: %w", err)
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
	dbLog(ctx, r.log, "event.list_paged", sqlEventListPaged, start, nil,
		slog.Int("limit", limit), slog.Int("offset", offset), slog.Int("count", len(result)))
	return result, nil
}

func (r *EventRepository) CountAll(ctx context.Context) (int, error) {
	start := time.Now()
	var total int
	if err := r.pool.QueryRow(ctx, sqlEventCountAll).Scan(&total); err != nil {
		dbLog(ctx, r.log, "event.count_all", sqlEventCountAll, start, err)
		return 0, fmt.Errorf("event count: %w", err)
	}
	dbLog(ctx, r.log, "event.count_all", sqlEventCountAll, start, nil, slog.Int("total", total))
	return total, nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (entity.Event, error) {
	start := time.Now()
	var e entity.Event
	err := r.pool.QueryRow(ctx, sqlEventGetByID, id).
		Scan(&e.ID, &e.Name, &e.SaleOpensAt, &e.Status, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			dbLog(ctx, r.log, "event.get_by_id", sqlEventGetByID, start, err, slog.String("eventId", id))
			return entity.Event{}, apperror.NotFound("event")
		}
		dbLog(ctx, r.log, "event.get_by_id", sqlEventGetByID, start, err, slog.String("eventId", id))
		return entity.Event{}, fmt.Errorf("event get: %w", err)
	}
	dbLog(ctx, r.log, "event.get_by_id", sqlEventGetByID, start, nil,
		slog.String("eventId", id),
		slog.String("name", e.Name),
		slog.String("status", string(e.Status)))
	return e, nil
}

func (r *EventRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	start := time.Now()
	var exists bool
	err := r.pool.QueryRow(ctx, sqlEventExistsByName, name).Scan(&exists)
	dbLog(ctx, r.log, "event.exists_by_name", sqlEventExistsByName, start, err,
		slog.String("name", name),
		slog.Bool("exists", exists))
	if err != nil {
		return false, fmt.Errorf("event exists by name: %w", err)
	}
	return exists, nil
}

func (r *EventRepository) Create(ctx context.Context, e entity.Event) error {
	start := time.Now()
	_, err := r.pool.Exec(ctx, sqlEventInsert,
		e.ID, e.Name, e.SaleOpensAt, e.Status, e.CreatedAt)
	dbLog(ctx, r.log, "event.create", sqlEventInsert, start, err,
		slog.String("eventId", e.ID),
		slog.String("name", e.Name))
	if err != nil {
		return fmt.Errorf("event create: %w", err)
	}
	return nil
}

func (r *EventRepository) Update(ctx context.Context, e entity.Event) error {
	start := time.Now()
	_, err := r.pool.Exec(ctx, sqlEventUpdate,
		e.ID, e.Name, e.SaleOpensAt, e.Status)
	dbLog(ctx, r.log, "event.update", sqlEventUpdate, start, err,
		slog.String("eventId", e.ID),
		slog.String("status", string(e.Status)))
	return err
}

// Delete adalah soft delete: set status = 'closed'.
func (r *EventRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	tag, err := r.pool.Exec(ctx, sqlEventSoftDelete, id)
	dbLog(ctx, r.log, "event.delete", sqlEventSoftDelete, start, err, slog.String("eventId", id))
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
	rows, err := r.pool.Query(ctx, sqlCategoryList, eventID)
	if err != nil {
		dbLog(ctx, r.log, "category.list", sqlCategoryList, start, err, slog.String("eventId", eventID))
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
	dbLog(ctx, r.log, "category.list", sqlCategoryList, start, nil,
		slog.String("eventId", eventID),
		slog.Int("count", len(result)))
	return result, nil
}

// GetCategoryByID dipakai UpdateCategory untuk baca state saat ini sebelum kalkulasi guard.
func (r *EventRepository) GetCategoryByID(ctx context.Context, categoryID string) (entity.TicketCategory, error) {
	start := time.Now()
	var c entity.TicketCategory
	err := r.pool.QueryRow(ctx, sqlCategoryGetByID, categoryID).
		Scan(&c.ID, &c.EventID, &c.Name, &c.Price, &c.Capacity, &c.Available)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			dbLog(ctx, r.log, "category.get_by_id", sqlCategoryGetByID, start, err, slog.String("categoryId", categoryID))
			return entity.TicketCategory{}, apperror.NotFound("kategori")
		}
		dbLog(ctx, r.log, "category.get_by_id", sqlCategoryGetByID, start, err, slog.String("categoryId", categoryID))
		return entity.TicketCategory{}, fmt.Errorf("category get: %w", err)
	}
	dbLog(ctx, r.log, "category.get_by_id", sqlCategoryGetByID, start, nil,
		slog.String("categoryId", categoryID),
		slog.Int("available", c.Available))
	return c, nil
}

func (r *EventRepository) CreateCategory(ctx context.Context, c entity.TicketCategory) error {
	start := time.Now()
	_, err := r.pool.Exec(ctx, sqlCategoryInsert,
		c.ID, c.EventID, c.Name, c.Price, c.Capacity, c.Available)
	dbLog(ctx, r.log, "category.create", sqlCategoryInsert, start, err,
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
	tag, err := r.pool.Exec(ctx, sqlCategoryUpdate,
		c.ID, c.Name, c.Price, c.Capacity, c.Available)
	dbLog(ctx, r.log, "category.update", sqlCategoryUpdate, start, err,
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
