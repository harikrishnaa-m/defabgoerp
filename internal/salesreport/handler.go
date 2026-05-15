package salesreport

import (
	"log"
	"strconv"

	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// List handles GET /sales-report
//
// Query params:
//
//	branch_id        — filter by branch/location (SuperAdmin/AccountsManager only; StoreManager auto-scoped)
//	from_date        — start date inclusive, YYYY-MM-DD
//	to_date          — end date inclusive, YYYY-MM-DD
//	salesperson_id   — filter by sales person
//	created_by_id    — filter by user who created the bill
//	payment_type     — filter by payment method: CASH | UPI | CARD | BANK_TRANSFER
//	channel          — filter by sale type: STORE | ONLINE
//	page             — 1-based page number (default 1)
//	limit            — results per page (default 50)
func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	user := c.Locals("user").(*model.User)

	branchID := c.Query("branch_id")
	// StoreManager and SalesPerson are scoped to their own branch
	if user.Role.Name == model.RoleStoreManager || user.Role.Name == model.RoleSalesPerson {
		if user.BranchID != nil {
			branchID = *user.BranchID
		}
	}

	f := Filter{
		BranchID:      branchID,
		FromDate:      c.Query("from_date"),
		ToDate:        c.Query("to_date"),
		SalespersonID: c.Query("salesperson_id"),
		CreatedByID:   c.Query("created_by_id"),
		PaymentType:   c.Query("payment_type"),
		Channel:       c.Query("channel"),
		Page:          page,
		Limit:         limit,
	}

	result, err := h.store.List(f)
	if err != nil {
		log.Println("salesreport list error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}
