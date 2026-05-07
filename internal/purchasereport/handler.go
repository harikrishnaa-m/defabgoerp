package purchasereport

import (
	"log"
	"strconv"

	"defab-erp/internal/core/httperr"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// List handles GET /purchase-report
//
// Top-level filters (dropdowns):
//
//	warehouse_id  — filter by location
//	supplier_id   — filter by supplier
//
// Column-level search (per-column text boxes):
//
//	search_invoice       — partial match on Invoice/Bill#
//	search_date          — partial match on Bill Date (DD/MM/YYYY)
//	search_party         — partial match on Party Name
//	search_purchase_cost — partial match on Purchase Cost
//	search_round_off     — partial match on RoundOff
//	search_other_charges — partial match on OtherCharges
//	search_tax           — partial match on Tax
//	search_discount      — partial match on Discount
//	search_net_amount    — partial match on Net Amount
//	search_location      — partial match on Location
//
// Pagination:
//
//	page  — 1-based (default 1)
//	limit — entries per page (default 10)
func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	f := Filter{
		WarehouseID:        c.Query("warehouse_id"),
		SupplierID:         c.Query("supplier_id"),
		SearchInvoice:      c.Query("search_invoice"),
		SearchDate:         c.Query("search_date"),
		SearchParty:        c.Query("search_party"),
		SearchPurchaseCost: c.Query("search_purchase_cost"),
		SearchRoundOff:     c.Query("search_round_off"),
		SearchOtherCharges: c.Query("search_other_charges"),
		SearchTax:          c.Query("search_tax"),
		SearchDiscount:     c.Query("search_discount"),
		SearchNetAmount:    c.Query("search_net_amount"),
		SearchLocation:     c.Query("search_location"),
		Page:               page,
		Limit:              limit,
	}

	result, err := h.store.List(f)
	if err != nil {
		log.Println("purchase report error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}
