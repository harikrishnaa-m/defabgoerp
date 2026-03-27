package purchaseinvoice

import (
	"database/sql"
	"log"
	"strings"

	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

// AccountingRecorder is an optional hook for auto-recording vouchers.
type AccountingRecorder interface {
	RecordPurchaseInvoice(purchaseInvoiceID, userID string) error
	RecordSupplierPayment(supplierPaymentID, userID string) error
}

type Handler struct {
	store    *Store
	recorder AccountingRecorder
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// SetRecorder injects the accounting recorder for auto-recording.
func (h *Handler) SetRecorder(r AccountingRecorder) {
	h.recorder = r
}

// Create handles POST /purchase-invoices
func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreatePurchaseInvoiceInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.PurchaseOrderID == "" {
		return httperr.BadRequest(c, "purchase_order_id is required")
	}
	if in.InvoiceDate == "" {
		return httperr.BadRequest(c, "invoice_date is required")
	}

	// Validate payment fields if provided
	if in.PaymentAmount > 0 {
		if in.PaymentMethod == "" {
			return httperr.BadRequest(c, "payment_method is required when payment_amount is provided")
		}
		allowed := map[string]bool{"CASH": true, "UPI": true, "CARD": true, "BANK_TRANSFER": true}
		in.PaymentMethod = strings.ToUpper(in.PaymentMethod)
		if !allowed[in.PaymentMethod] {
			return httperr.BadRequest(c, "payment_method must be CASH, UPI, CARD, or BANK_TRANSFER")
		}
	}

	user := c.Locals("user").(*model.User)

	invoiceID, err := h.store.Create(in, user.ID.String())
	if err != nil {
		log.Println("create purchase invoice error:", err)
		return httperr.Internal(c)
	}

	// Auto-record in accounting (non-blocking)
	if h.recorder != nil {
		go func() {
			if err := h.recorder.RecordPurchaseInvoice(invoiceID, user.ID.String()); err != nil {
				log.Println("accounting auto-record purchase invoice error:", err)
			}
		}()
	}

	invoice, err := h.store.GetByID(invoiceID)
	if err != nil {
		log.Println("fetch purchase invoice error:", err)
		return httperr.Internal(c)
	}

	return c.Status(201).JSON(invoice)
}

// GetByID handles GET /purchase-invoices/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	invoice, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Purchase invoice not found")
		}
		log.Println("get purchase invoice error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(invoice)
}

// List handles GET /purchase-invoices
func (h *Handler) List(c *fiber.Ctx) error {
	list, err := h.store.List()
	if err != nil {
		log.Println("list purchase invoices error:", err)
		return httperr.Internal(c)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return c.JSON(list)
}

// RecordPayment handles POST /purchase-invoices/:id/payments
func (h *Handler) RecordPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	var in RecordPaymentInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.Amount <= 0 {
		return httperr.BadRequest(c, "amount must be greater than 0")
	}
	if in.PaymentMethod == "" {
		return httperr.BadRequest(c, "payment_method is required")
	}
	allowed := map[string]bool{"CASH": true, "UPI": true, "CARD": true, "BANK_TRANSFER": true}
	if !allowed[strings.ToUpper(in.PaymentMethod)] {
		return httperr.BadRequest(c, "payment_method must be CASH, UPI, CARD, or BANK_TRANSFER")
	}
	in.PaymentMethod = strings.ToUpper(in.PaymentMethod)

	if err := h.store.RecordPayment(id, in); err != nil {
		if strings.Contains(err.Error(), "exceeds balance") || strings.Contains(err.Error(), "cancelled") {
			return httperr.BadRequest(c, err.Error())
		}
		if strings.Contains(err.Error(), "not found") {
			return httperr.NotFound(c, "Purchase invoice not found")
		}
		log.Println("record payment error:", err)
		return httperr.Internal(c)
	}

	// Auto-record supplier payment in accounting (non-blocking)
	if h.recorder != nil {
		// Get the latest supplier_payment ID for this invoice
		var paymentID string
		err := h.store.DB().QueryRow(
			`SELECT id FROM supplier_payments WHERE purchase_invoice_id = $1 ORDER BY created_at DESC LIMIT 1`, id,
		).Scan(&paymentID)
		if err == nil && paymentID != "" {
			user := c.Locals("user").(*model.User)
			go func() {
				if err := h.recorder.RecordSupplierPayment(paymentID, user.ID.String()); err != nil {
					log.Println("accounting auto-record supplier payment error:", err)
				}
			}()
		}
	}

	invoice, err := h.store.GetByID(id)
	if err != nil {
		log.Println("fetch invoice after payment error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(invoice)
}

// ListAllPayments handles GET /supplier-payments
func (h *Handler) ListAllPayments(c *fiber.Ctx) error {
	list, err := h.store.ListAllPayments()
	if err != nil {
		log.Println("list all payments error:", err)
		return httperr.Internal(c)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return c.JSON(list)
}

// ListPaymentsBySupplier handles GET /supplier-payments/supplier/:supplierId
func (h *Handler) ListPaymentsBySupplier(c *fiber.Ctx) error {
	supplierID := c.Params("supplierId")
	list, err := h.store.ListPaymentsBySupplier(supplierID)
	if err != nil {
		log.Println("list payments by supplier error:", err)
		return httperr.Internal(c)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return c.JSON(list)
}

// OutstandingSummary handles GET /supplier-payments/outstanding
func (h *Handler) OutstandingSummary(c *fiber.Ctx) error {
	list, err := h.store.OutstandingSummary()
	if err != nil {
		log.Println("outstanding summary error:", err)
		return httperr.Internal(c)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return c.JSON(list)
}

// Cancel handles DELETE /purchase-invoices/:id
func (h *Handler) Cancel(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.store.Cancel(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return httperr.NotFound(c, "Purchase invoice not found")
		}
		if strings.Contains(err.Error(), "only PENDING") {
			return httperr.BadRequest(c, err.Error())
		}
		log.Println("cancel purchase invoice error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "Purchase invoice cancelled"})
}
