package entity

import "time"

type EventStatus string

const (
	EventStatusDraft   EventStatus = "draft"
	EventStatusOnSale  EventStatus = "on_sale"
	EventStatusSoldOut EventStatus = "sold_out"
	EventStatusClosed  EventStatus = "closed"
)

type Event struct {
	ID          string
	Name        string
	PosterURL   string
	Description string
	SaleOpensAt *time.Time
	Status      EventStatus
	CreatedAt   time.Time
}

type TicketCategory struct {
	ID        string
	EventID   string
	Name      string
	Price     int64
	Capacity  int
	Available int
}
