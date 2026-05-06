package purchasereturn

import (
	"github.com/gofiber/fiber/v2"

	"defab-erp/internal/core/model"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// POST /
func (h *Handler) Create(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	var in CreatePurchaseReturnInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if in.SupplierID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "supplier_id is required"})
	}
	if len(in.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "items are required"})
	}

	id, err := h.store.Create(in, user.ID.String())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

// GET /
func (h *Handler) List(c *fiber.Ctx) error {
	f := ListFilter{
		SupplierName:  c.Query("supplier_name"),
		PRNumber:      c.Query("pr_number"),
		InvoiceNumber: c.Query("invoice_number"),
		DateFrom:      c.Query("date_from"),
		DateTo:        c.Query("date_to"),
	}
	rows, err := h.store.List(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": rows})
}

// GET /:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	detail, err := h.store.GetByID(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	return c.JSON(detail)
}

// GET /invoice-lookup?invoice_number=INV-001
func (h *Handler) InvoiceLookup(c *fiber.Ctx) error {
	invoiceNumber := c.Query("invoice_number")
	if invoiceNumber == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invoice_number query param is required"})
	}
	resp, err := h.store.GetInvoiceLookup(invoiceNumber)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}
