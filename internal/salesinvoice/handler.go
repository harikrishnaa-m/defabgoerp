package salesinvoice

import (
	"database/sql"
	"log"

	"defab-erp/internal/accounting"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store    *Store
	recorder *accounting.Recorder
}

func NewHandler(s *Store, r *accounting.Recorder) *Handler {
	return &Handler{store: s, recorder: r}
}

func (h *Handler) List(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}

	// limit=0 means "no limit" — send everything.
	// Only apply a positive limit when the caller explicitly sets one.
	limitStr := c.Query("limit")
	limit := 0
	if limitStr != "" {
		limit = c.QueryInt("limit", 0)
		if limit < 1 {
			limit = 0
		}
	}

	offset := 0
	if limit > 0 {
		offset = (page - 1) * limit
	}

	status := c.Query("status")
	search := c.Query("search")

	var branchID *string
	if user.Role.Name == model.RoleStoreManager || user.Role.Name == model.RoleSalesPerson {
		branchID = user.BranchID
	} else if c.Query("branch_id") != "" {
		bid := c.Query("branch_id")
		branchID = &bid
	}

	invoices, total, err := h.store.List(branchID, status, search, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":  invoices,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	invoice, err := h.store.GetByID(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "sales invoice not found"})
	}

	return c.JSON(invoice)
}

func (h *Handler) GetByInvoiceNumber(c *fiber.Ctx) error {
	num := c.Params("invoiceNumber")
	log.Println("GetByInvoiceNumber called with:", num)

	invoice, err := h.store.GetByInvoiceNumber(num)
	if err != nil {
		log.Println("GetByInvoiceNumber error:", err)
		return c.Status(404).JSON(fiber.Map{"error": "sales invoice not found"})
	}

	return c.JSON(invoice)
}

func (h *Handler) UpdateSalesperson(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		SalespersonID string `json:"salesperson_id"`
	}
	if err := c.BodyParser(&body); err != nil || body.SalespersonID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "salesperson_id is required"})
	}
	err := h.store.UpdateSalesperson(id, body.SalespersonID)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "sales invoice not found"})
	}
	if err != nil {
		if err.Error() == "cannot update a cancelled invoice" {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		log.Println("update salesperson error:", err)
		return c.Status(500).JSON(fiber.Map{"error": "failed to update salesperson"})
	}
	return c.JSON(fiber.Map{"message": "Salesperson updated"})
}

func (h *Handler) CancelInvoice(c *fiber.Ctx) error {
	id := c.Params("id")

	paymentIDs, err := h.store.CancelInvoice(id)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "sales invoice not found"})
	}
	if err != nil {
		msg := err.Error()
		if msg == "invoice is already cancelled" || msg == "cannot cancel a returned invoice" {
			return c.Status(400).JSON(fiber.Map{"error": msg})
		}
		log.Println("cancel invoice error:", err)
		return c.Status(500).JSON(fiber.Map{"error": "failed to cancel invoice"})
	}

	// Cancel accounting vouchers (best-effort — DB is already committed)
	if h.recorder != nil {
		if err := h.recorder.CancelVoucherByRef("sales_invoice", id); err != nil {
			log.Println("cancel sales_invoice voucher error:", err)
		}
		for _, pid := range paymentIDs {
			if err := h.recorder.CancelVoucherByRef("sales_payment", pid); err != nil {
				log.Println("cancel sales_payment voucher error:", pid, err)
			}
		}
	}

	return c.JSON(fiber.Map{"message": "Invoice cancelled successfully"})
}
