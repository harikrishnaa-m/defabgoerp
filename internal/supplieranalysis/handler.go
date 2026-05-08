package supplieranalysis

import (
	"log"
	"strconv"

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
//	month        — number of months to show (1=current only, 2=current+prev, …)
//	warehouse_id — optional
//	search_item  — optional, partial item name/code filter within supplier
//
// Mode 2 — Search by Item (search_by_item=true):
//
//	search_item  — required
//	month        — number of months to show
//	warehouse_id — optional
func (h *Handler) Get(c *fiber.Ctx) error {
	searchByItem := c.Query("search_by_item") == "true"

	lastN, err := strconv.Atoi(c.Query("month", "1"))
	if err != nil || lastN < 1 {
		return httperr.BadRequest(c, "month must be a positive number (1 = current month, 2 = current + previous, …)")
	}

	f := Filter{
		SupplierID:   c.Query("supplier_id"),
		LastNMonths:  lastN,
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
