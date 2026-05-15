package paymentreport

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

// List handles GET /payment-report
//
// Query params:
//
//	branch_id      — filter by branch (auto-scoped for StoreManager/SalesPerson)
//	from_date      — YYYY-MM-DD
//	to_date        — YYYY-MM-DD
//	payment_method — CASH | UPI | CARD | DEBIT_CARD | CREDIT_CARD | BANK_TRANSFER  (omit for all methods)
//	page           — 1-based (omit for all rows)
//	limit          — per page (omit for all rows)
func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "0"))
	limit, _ := strconv.Atoi(c.Query("limit", "0"))

	user := c.Locals("user").(*model.User)

	branchID := c.Query("branch_id")
	if user.Role.Name == model.RoleStoreManager || user.Role.Name == model.RoleSalesPerson {
		if user.BranchID != nil {
			branchID = *user.BranchID
		}
	}

	f := Filter{
		BranchID:      branchID,
		FromDate:      c.Query("from_date"),
		ToDate:        c.Query("to_date"),
		PaymentMethod: c.Query("payment_method"),
		Page:          page,
		Limit:         limit,
	}

	result, err := h.store.List(f)
	if err != nil {
		log.Println("paymentreport list error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}
