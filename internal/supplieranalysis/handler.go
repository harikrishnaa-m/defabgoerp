package supplieranalysis

import (
	"log"
	"strconv"
	"time"

	"defab-erp/internal/core/httperr"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// Get handles GET /supplier-analysis
// Mode 1 — Search by Supplier (search_by_item absent or false):
//
//	supplier_id  — required
//	month        — 1-12, required
//	year         — default: current year
//	warehouse_id — optional
//	search_item  — optional, partial item name/code filter within supplier
//
// Mode 2 — Search by Item (search_by_item=true):
//
//	search_item  — required (item code / name, partial match)
//	month        — 1-12, required
//	year         — default: current year
//	warehouse_id — optional
func (h *Handler) Get(c *fiber.Ctx) error {
	searchByItem := c.Query("search_by_item") == "true"

	month, err := strconv.Atoi(c.Query("month"))
	if err != nil || month < 1 || month > 12 {
		return httperr.BadRequest(c, "month must be a number between 1 and 12")
	}

	year, _ := strconv.Atoi(c.Query("year", strconv.Itoa(time.Now().Year())))
	if year == 0 {
		year = time.Now().Year()
	}

	f := Filter{
		SupplierID:   c.Query("supplier_id"),
		Month:        month,
		Year:         year,
		WarehouseID:  c.Query("warehouse_id"),
		SearchItem:   c.Query("search_item"),
		SearchByItem: searchByItem,
	}

	if !searchByItem && f.SupplierID == "" {
		return httperr.BadRequest(c, "supplier_id is required")
	}
	if searchByItem && f.SearchItem == "" {
		return httperr.BadRequest(c, "search_item is required when search_by_item=true")
	}

	result, err := h.store.Get(f)
	if err != nil {
		log.Println("supplier analysis error:", err)
		if err.Error() == "supplier not found" {
			return httperr.NotFound(c, "Supplier not found")
		}
		return httperr.Internal(c)
	}

	return c.JSON(result)
}
