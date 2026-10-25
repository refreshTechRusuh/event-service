// event_crud_test.go — Test untuk fungsi CRUD event dan kategori yang belum dicakup.
// Bersama event_usecase_test.go ini membawa coverage usecase dari 36% ke >80%.
package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/refreshTechRusuh/event-service/internal/event/domain/dto"
	"github.com/refreshTechRusuh/event-service/internal/event/domain/entity"
	"github.com/refreshTechRusuh/event-service/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// ListEvents
// ============================================================

func TestListEvents_Empty(t *testing.T) {
	uc, _, _ := newUC(t)
	result, meta, err := uc.ListEvents(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Empty(t, result, "daftar kosong bila tidak ada event")
	assert.Equal(t, 0, meta.TotalItems)
	assert.Equal(t, 0, meta.TotalPages)
	assert.Equal(t, 1, meta.Page)
	assert.Equal(t, 10, meta.Size)
}

func TestListEvents_ReturnsAllEvents(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Konser A", Status: "on_sale"}
	repo.events["evt-2"] = entity.Event{ID: "evt-2", Name: "Festival B", Status: "sold_out"}

	result, meta, err := uc.ListEvents(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, meta.TotalItems)
	assert.Equal(t, 1, meta.TotalPages)

	names := make(map[string]string)
	for _, r := range result {
		names[r.ID] = r.Name
	}
	assert.Equal(t, "Konser A", names["evt-1"])
	assert.Equal(t, "Festival B", names["evt-2"])
}

func TestListEvents_SaleOpensAt_Formatted(t *testing.T) {
	uc, repo, _ := newUC(t)
	saleOpensAt := time.Date(2026, 12, 20, 19, 0, 0, 0, time.UTC)
	repo.events["evt-1"] = entity.Event{
		ID:          "evt-1",
		Name:        "Konser C",
		Status:      "on_sale",
		SaleOpensAt: &saleOpensAt,
	}

	result, _, err := uc.ListEvents(context.Background(), 1, 10)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "2026-12-20T19:00:00Z", result[0].SaleOpensAt,
		"SaleOpensAt harus di-format sebagai RFC3339 UTC")
}

func TestListEvents_Pagination(t *testing.T) {
	uc, repo, _ := newUC(t)
	for i := 0; i < 5; i++ {
		id := "evt-" + string(rune('a'+i))
		repo.events[id] = entity.Event{ID: id, Name: "Event " + id, Status: "on_sale"}
	}

	// size=2 → 3 halaman (ceil(5/2)); halaman 1 & 2 berisi 2 item, halaman 3 berisi 1.
	page1, meta1, err := uc.ListEvents(context.Background(), 1, 2)
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.Equal(t, 5, meta1.TotalItems)
	assert.Equal(t, 3, meta1.TotalPages)
	assert.Equal(t, 1, meta1.Page)
	assert.Equal(t, 2, meta1.Size)

	page3, meta3, err := uc.ListEvents(context.Background(), 3, 2)
	require.NoError(t, err)
	assert.Len(t, page3, 1, "halaman terakhir berisi sisa 1 item")
	assert.Equal(t, 3, meta3.Page)

	// Halaman di luar rentang → kosong, meta tetap konsisten.
	pageOver, metaOver, err := uc.ListEvents(context.Background(), 99, 2)
	require.NoError(t, err)
	assert.Empty(t, pageOver)
	assert.Equal(t, 5, metaOver.TotalItems)
	assert.Equal(t, 3, metaOver.TotalPages)
}

// ============================================================
// GetEventDetail
// ============================================================

func TestGetEventDetail_HappyPath(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Konser A", Status: "on_sale"}
	repo.categories["cat-1"] = entity.TicketCategory{
		ID: "cat-1", EventID: "evt-1", Name: "VIP", Price: 500000,
		Available: 37, Capacity: 100,
	}
	repo.categories["cat-2"] = entity.TicketCategory{
		ID: "cat-2", EventID: "evt-1", Name: "Regular", Price: 250000,
		Available: 120, Capacity: 200,
	}

	result, err := uc.GetEventDetail(context.Background(), "evt-1")
	require.NoError(t, err)
	assert.Equal(t, "evt-1", result.ID)
	assert.Equal(t, "Konser A", result.Name)
	assert.Len(t, result.Categories, 2)

	// Verifikasi satu kategori ada di detail
	found := false
	for _, c := range result.Categories {
		if c.Name == "VIP" {
			assert.Equal(t, int64(500000), c.Price)
			assert.Equal(t, 37, c.Available)
			found = true
		}
	}
	assert.True(t, found, "kategori VIP harus ada di detail")
}

func TestGetEventDetail_EventNotFound(t *testing.T) {
	uc, _, _ := newUC(t)
	_, err := uc.GetEventDetail(context.Background(), "nonexistent")
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrNotFound, appErr.Code)
}

func TestGetEventDetail_NoCategories(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Konser", Status: "on_sale"}
	// Tidak ada kategori

	result, err := uc.GetEventDetail(context.Background(), "evt-1")
	require.NoError(t, err)
	assert.Empty(t, result.Categories, "categories harus kosong bila belum ada")
}

// ============================================================
// AdminListEvents
// ============================================================

func TestAdminListEvents_IncludesCapacity(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Konser A", Status: "on_sale"}
	repo.categories["cat-1"] = entity.TicketCategory{
		ID: "cat-1", EventID: "evt-1", Name: "VIP",
		Price: 500000, Available: 37, Capacity: 100,
	}

	result, _, err := uc.AdminListEvents(context.Background(), 1, 10)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Len(t, result[0].Categories, 1)

	// Admin view harus menampilkan Capacity (public view tidak)
	assert.Equal(t, 100, result[0].Categories[0].Capacity,
		"admin list harus sertakan Capacity per kategori")
	assert.Equal(t, 37, result[0].Categories[0].Available)
}

func TestAdminListEvents_MultipleEvents(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Konser A", Status: "on_sale"}
	repo.events["evt-2"] = entity.Event{ID: "evt-2", Name: "Festival B", Status: "sold_out"}

	result, meta, err := uc.AdminListEvents(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, meta.TotalItems)
	assert.Equal(t, 1, meta.TotalPages)
}

// ============================================================
// CreateEvent
// ============================================================

func TestCreateEvent_HappyPath(t *testing.T) {
	uc, repo, _ := newUC(t)

	result, err := uc.CreateEvent(context.Background(), dto.CreateEventRequest{
		Name:   "Konser Baru",
		Status: "on_sale",
	})
	require.NoError(t, err)
	assert.Equal(t, "Konser Baru", result.Name)
	assert.Equal(t, "on_sale", result.Status)
	assert.NotEmpty(t, result.ID)

	// Event harus tersimpan di repo
	_, ok := repo.events[result.ID]
	assert.True(t, ok, "event harus tersimpan di repo setelah CreateEvent")
}

func TestCreateEvent_WithSaleOpensAt(t *testing.T) {
	uc, _, _ := newUC(t)

	result, err := uc.CreateEvent(context.Background(), dto.CreateEventRequest{
		Name:        "Konser Besok",
		Status:      "on_sale",
		SaleOpensAt: "2026-12-20T19:00:00Z",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
}

func TestCreateEvent_InvalidSaleOpensAt(t *testing.T) {
	uc, _, _ := newUC(t)

	_, err := uc.CreateEvent(context.Background(), dto.CreateEventRequest{
		Name:        "Test",
		Status:      "on_sale",
		SaleOpensAt: "bukan-tanggal-valid",
	})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrValidation, appErr.Code)
}

// ============================================================
// UpdateEvent
// ============================================================

func TestUpdateEvent_HappyPath(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Lama", Status: "on_sale"}

	result, err := uc.UpdateEvent(context.Background(), "evt-1", dto.UpdateEventRequest{
		Name:   "Baru",
		Status: "sold_out",
	})
	require.NoError(t, err)
	assert.Equal(t, "Baru", result.Name)
	assert.Equal(t, "sold_out", result.Status)

	// Verifikasi perubahan di repo
	assert.Equal(t, "Baru", repo.events["evt-1"].Name)
}

func TestUpdateEvent_PartialUpdate_OnlyName(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Lama", Status: "on_sale"}

	result, err := uc.UpdateEvent(context.Background(), "evt-1", dto.UpdateEventRequest{
		Name: "Nama Baru", // hanya nama yang diubah
	})
	require.NoError(t, err)
	assert.Equal(t, "Nama Baru", result.Name)
	assert.Equal(t, "on_sale", result.Status, "status tidak boleh berubah bila tidak di-set")
}

func TestUpdateEvent_WithSaleOpensAt(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Konser", Status: "on_sale"}

	_, err := uc.UpdateEvent(context.Background(), "evt-1", dto.UpdateEventRequest{
		SaleOpensAt: "2027-01-15T10:00:00Z",
	})
	require.NoError(t, err)
}

func TestUpdateEvent_InvalidSaleOpensAt(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Konser", Status: "on_sale"}

	_, err := uc.UpdateEvent(context.Background(), "evt-1", dto.UpdateEventRequest{
		SaleOpensAt: "invalid-date",
	})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrValidation, appErr.Code)
}

func TestUpdateEvent_NotFound(t *testing.T) {
	uc, _, _ := newUC(t)
	_, err := uc.UpdateEvent(context.Background(), "nonexistent", dto.UpdateEventRequest{Name: "baru"})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrNotFound, appErr.Code)
}

// ============================================================
// DeleteEvent
// ============================================================

func TestDeleteEvent_SoftDeletesEvent(t *testing.T) {
	uc, repo, _ := newUC(t)
	repo.events["evt-1"] = entity.Event{ID: "evt-1", Name: "Konser", Status: "on_sale"}

	err := uc.DeleteEvent(context.Background(), "evt-1")
	require.NoError(t, err)

	// Event masih ada tapi status = closed (soft delete)
	updated := repo.events["evt-1"]
	assert.Equal(t, entity.EventStatus("closed"), updated.Status)
}

func TestDeleteEvent_NotFound(t *testing.T) {
	uc, _, _ := newUC(t)
	err := uc.DeleteEvent(context.Background(), "nonexistent")
	require.Error(t, err)
}

// ============================================================
// UpdateCategory — kategori guard boundary tests
// (ini adalah inti test yang diminta: available < 0 ditolak)
// ============================================================

// TestUpdateCategory_Guard_ExactBoundary memverifikasi kondisi batas:
//   - capacity_baru = tiketTerjual → available_baru = 0 → DITERIMA (batas bawah valid)
//   - capacity_baru = tiketTerjual - 1 → available_baru = -1 → DITOLAK
//
// Ini adalah "close timing" guard yang paling penting: admin tidak boleh
// mengurangi kapasitas di bawah tiket yang sudah terjual/di-hold.
func TestUpdateCategory_Guard_ExactBoundary(t *testing.T) {
	t.Run("capacity_baru == tiketTerjual → available=0 diterima", func(t *testing.T) {
		uc, repo, _ := newUC(t)
		seedEvent(repo, "evt-1")
		// capacity=100, available=40 → tiketTerjual=60
		repo.categories["cat-1"] = entity.TicketCategory{
			ID: "cat-1", EventID: "evt-1", Name: "VIP",
			Price: 500000, Capacity: 100, Available: 40,
		}

		// Kapasitas = tepat jumlah terjual → available_baru = 0 → boleh
		result, err := uc.UpdateCategory(context.Background(), "cat-1", dto.UpdateCategoryRequest{Capacity: 60})
		require.NoError(t, err)
		assert.Equal(t, 60, result.Capacity)
		assert.Equal(t, 0, result.Available,
			"available=0 adalah batas bawah valid — tepat menutupi tiket terjual")
	})

	t.Run("capacity_baru < tiketTerjual → available < 0 ditolak", func(t *testing.T) {
		uc, repo, _ := newUC(t)
		seedEvent(repo, "evt-1")
		// capacity=100, available=40 → tiketTerjual=60
		repo.categories["cat-1"] = entity.TicketCategory{
			ID: "cat-1", EventID: "evt-1", Name: "VIP",
			Price: 500000, Capacity: 100, Available: 40,
		}

		// Kapasitas < tiket terjual → available_baru = 59 - 60 = -1 → ditolak
		_, err := uc.UpdateCategory(context.Background(), "cat-1", dto.UpdateCategoryRequest{Capacity: 59})
		require.Error(t, err)
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperror.ErrValidation, appErr.Code,
			"available < 0 harus ditolak (Req 13.4) — tidak boleh 'menghapus' tiket yang sudah di-hold/terjual")
	})
}

// TestUpdateCategory_Guard_AllSoldOut memverifikasi kasus ekstrem:
// semua tiket terjual (available=0) → tidak bisa kurangi kapasitas sama sekali.
func TestUpdateCategory_Guard_AllSoldOut(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-1")
	// Semua 100 tiket terjual: available=0
	repo.categories["cat-sold"] = entity.TicketCategory{
		ID: "cat-sold", EventID: "evt-1", Name: "Sold", Capacity: 100, Available: 0,
	}

	// Mencoba kurangi 1 → available_baru = 99 - 100 = -1 → DITOLAK
	_, err := uc.UpdateCategory(context.Background(), "cat-sold", dto.UpdateCategoryRequest{Capacity: 99})
	require.Error(t, err)
	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperror.ErrValidation, appErr.Code)
}

// TestUpdateCategory_Guard_FullyAvailable memverifikasi kasus ekstrem lain:
// tidak ada tiket terjual (available=capacity) → bisa kurangi kapasitas sampai 0.
func TestUpdateCategory_Guard_FullyAvailable(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-1")
	// Tidak ada tiket terjual: available = capacity = 100
	repo.categories["cat-fresh"] = entity.TicketCategory{
		ID: "cat-fresh", EventID: "evt-1", Name: "Fresh", Capacity: 100, Available: 100,
	}

	// Kurangi ke 1 → available_baru = 1 - 0 = 1 → OK
	result, err := uc.UpdateCategory(context.Background(), "cat-fresh", dto.UpdateCategoryRequest{Capacity: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Capacity)
	assert.Equal(t, 1, result.Available)
}

// TestUpdateCategory_OnlyNameOrPrice memverifikasi bahwa update partial (tanpa Capacity)
// tidak mengubah available yang ada.
func TestUpdateCategory_OnlyNameOrPrice(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-1")
	repo.categories["cat-1"] = entity.TicketCategory{
		ID: "cat-1", EventID: "evt-1", Name: "VIP", Price: 500000, Capacity: 100, Available: 37,
	}

	result, err := uc.UpdateCategory(context.Background(), "cat-1", dto.UpdateCategoryRequest{
		Name:  "VIP Platinum",
		Price: 750000,
		// Capacity tidak di-set → available tidak berubah
	})
	require.NoError(t, err)
	assert.Equal(t, "VIP Platinum", result.Name)
	assert.Equal(t, int64(750000), result.Price)
	assert.Equal(t, 37, result.Available, "available tidak boleh berubah bila Capacity tidak di-set")
	assert.Equal(t, 100, result.Capacity, "Capacity tidak boleh berubah bila tidak di-set")
}

// TestUpdateCategory_SameCapacity memverifikasi bahwa update dengan kapasitas yang sama
// tidak mengubah apapun (no-op guard).
func TestUpdateCategory_SameCapacity(t *testing.T) {
	uc, repo, _ := newUC(t)
	seedEvent(repo, "evt-1")
	repo.categories["cat-1"] = entity.TicketCategory{
		ID: "cat-1", EventID: "evt-1", Name: "VIP", Price: 500000, Capacity: 100, Available: 40,
	}

	result, err := uc.UpdateCategory(context.Background(), "cat-1", dto.UpdateCategoryRequest{
		Capacity: 100, // sama seperti sekarang → tidak ada perubahan available
	})
	require.NoError(t, err)
	assert.Equal(t, 40, result.Available, "available tidak boleh berubah bila Capacity sama")
}
