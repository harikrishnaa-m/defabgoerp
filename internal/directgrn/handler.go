package directgrn

import (
	"database/sql"
	"log"
	"strings"

	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

// AccountingRecorder mirrors the interface in purchaseinvoice/handler.go.
type AccountingRecorder interface {
	RecordPurchaseInvoice(purchaseInvoiceID, userID string) error
}

type Handler struct {
	store    *Store
	recorder AccountingRecorder
}

func NewHandler(s *Store, recorder AccountingRecorder) *Handler {
	return &Handler{store: s, recorder: recorder}
}

// Create handles POST /direct-grn
func (h *Handler) Create(c *fiber.Ctx) error {
	var in DirectGRNInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.SupplierID == "" {
		return httperr.BadRequest(c, "supplier_id is required")
	}
	if in.WarehouseID == "" {
		return httperr.BadRequest(c, "warehouse_id is required")
	}
	if in.InvoiceDate == "" {
		return httperr.BadRequest(c, "invoice_date is required")
	}
	if len(in.Items) == 0 {
		return httperr.BadRequest(c, "at least one item is required")
	}

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

	result, err := h.store.Create(in, user.ID.String())
	if err != nil {
		log.Println("direct grn create error:", err)
		return httperr.Internal(c)
	}

	// Trigger accounting auto-record (non-blocking)
	if h.recorder != nil {
		go func() {
			if err := h.recorder.RecordPurchaseInvoice(result.InvoiceID, user.ID.String()); err != nil {
				log.Println("direct grn accounting record error:", err)
			}
		}()
	}

	return c.Status(201).JSON(result)
}

// List handles GET /direct-grn?grn_number=&supplier_name=&date_from=&date_to=
func (h *Handler) List(c *fiber.Ctx) error {
	f := ListFilter{
		GRNNumber:    c.Query("grn_number"),
		SupplierName: c.Query("supplier_name"),
		DateFrom:     c.Query("date_from"),
		DateTo:       c.Query("date_to"),
	}
	list, err := h.store.List(f)
	if err != nil {
		log.Println("direct grn list error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(list)
}

// GetByID handles GET /direct-grn/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	detail, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Direct GRN not found")
		}
		log.Println("direct grn get error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(detail)
}
