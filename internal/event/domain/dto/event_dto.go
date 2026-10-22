package dto

// ---- Public read ----

type EventListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	SaleOpensAt string `json:"saleOpensAt,omitempty"`
}

type CategoryDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Price     int64  `json:"price"`
	Available int    `json:"available"`
	Capacity  int    `json:"capacity,omitempty"`
}

type EventDetail struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Status     string        `json:"status"`
	Categories []CategoryDTO `json:"categories"`
}

// ---- Admin ----

type CreateEventRequest struct {
	Name        string `json:"name"        validate:"required"`
	SaleOpensAt string `json:"saleOpensAt"`
	Status      string `json:"status"      validate:"oneof=draft on_sale closed"`
}

type UpdateEventRequest struct {
	Name        string `json:"name"`
	SaleOpensAt string `json:"saleOpensAt"`
	Status      string `json:"status"`
}

type AdminEventListItem struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Status     string        `json:"status"`
	Categories []CategoryDTO `json:"categories"`
}

type CreateCategoryRequest struct {
	Name     string `json:"name"     validate:"required"`
	Price    int64  `json:"price"    validate:"required,min=0"`
	Capacity int    `json:"capacity" validate:"required,min=1"`
}

type UpdateCategoryRequest struct {
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Capacity int    `json:"capacity"`
}
