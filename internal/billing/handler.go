package billing

import (
	"database/sql"
	"log"
	"strconv"
	"strings"

	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

// AccountingRecorder is an optional hook for auto-recording vouchers.
type AccountingRecorder interface {
	RecordSalesInvoice(salesInvoiceID, userID string) error
	RecordSalesPayment(salesPaymentID, userID string) error
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

// Create handles POST /billing — the main POS endpoint.
func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateBillInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.CustomerPhone == "" {
		return httperr.BadRequest(c, "customer_phone is required")
	}
	if in.CustomerName == "" {
		return httperr.BadRequest(c, "customer_name is required")
	}
	if len(in.Items) == 0 {
		return httperr.BadRequest(c, "at least one item is required")
	}
	if len(in.Payments) == 0 {
		return httperr.BadRequest(c, "at least one payment is required")
	}

	for i, item := range in.Items {
		if item.VariantID == "" {
			return httperr.BadRequest(c, "items["+strconv.Itoa(i)+"].variant_id is required")
		}
		if item.Quantity <= 0 {
			return httperr.BadRequest(c, "items["+strconv.Itoa(i)+"].quantity must be > 0")
		}
	}

	for i, p := range in.Payments {
		if p.Amount <= 0 {
			return httperr.BadRequest(c, "payments["+strconv.Itoa(i)+"].amount must be > 0")
		}
		if p.Method == "" {
			return httperr.BadRequest(c, "payments["+strconv.Itoa(i)+"].method is required")
		}
	}

	user := c.Locals("user").(*model.User)

	// Auto-resolve warehouse from branch
	if user.BranchID == nil {
		return httperr.BadRequest(c, "Your account has no branch assigned")
	}
	branchID := *user.BranchID

	warehouseID, err := h.store.GetWarehouseByBranch(branchID)
	if err != nil {
		return httperr.BadRequest(c, "No warehouse found for your branch")
	}
	in.WarehouseID = warehouseID

	// Auto-set salesperson_id if logged-in user is a Salesperson
	if user.Role.Name == model.RoleSalesPerson && in.SalesPersonID == "" {
		spID, err := h.store.GetSalespersonByUserID(user.ID.String())
		if err != nil {
			return httperr.BadRequest(c, "No salesperson profile found for your account")
		}
		in.SalesPersonID = spID
	}

	result, err := h.store.CreateBill(in, user.ID.String(), branchID)
	if err != nil {
		log.Println("create bill error:", err)
		errMsg := err.Error()
		if len(errMsg) >= 18 && errMsg[:18] == "insufficient stock" {
			return httperr.BadRequest(c, errMsg)
		}
		return httperr.Internal(c)
	}

	// Auto-record in accounting (non-blocking)
	if h.recorder != nil {
		if invoiceID, ok := result["sales_invoice_id"].(string); ok {
			go func() {
				if err := h.recorder.RecordSalesInvoice(invoiceID, user.ID.String()); err != nil {
					log.Println("accounting auto-record sales invoice error:", err)
				}
			}()
		}
	}

	return c.Status(201).JSON(result)
}

// GetByID handles GET /billing/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	result, err := h.store.GetByID(id)
	if err == sql.ErrNoRows {
		return httperr.NotFound(c, "Bill not found")
	}
	if err != nil {
		log.Println("get bill error:", err)
		return httperr.Internal(c)
	}

	// Branch check for StoreManager
	user := c.Locals("user").(*model.User)
	if user.Role.Name == model.RoleStoreManager {
		if user.BranchID != nil {
			billBranch, ok := result["branch_id"].(string)
			if ok && billBranch != *user.BranchID {
				return c.Status(403).JSON(fiber.Map{"error": "Access denied to this bill"})
			}
		}
	}

	return c.JSON(result)
}

// List handles GET /billing
func (h *Handler) List(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var branchID *string
	if user.Role.Name == model.RoleStoreManager || user.Role.Name == model.RoleSalesPerson {
		branchID = user.BranchID
	}

	results, err := h.store.List(branchID, limit, offset)
	if err != nil {
		log.Println("list bills error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"bills":  results,
		"limit":  limit,
		"offset": offset,
	})
}

// Lookup handles GET /billing/lookup?sku=XXX
// Searches by SKU or barcode. Warehouse is auto-resolved from the user's branch.
func (h *Handler) Lookup(c *fiber.Ctx) error {
	sku := c.Query("sku")
	if sku == "" {
		return httperr.BadRequest(c, "sku query param is required")
	}

	user := c.Locals("user").(*model.User)
	if user.BranchID == nil {
		return httperr.BadRequest(c, "Your account has no branch assigned")
	}

	warehouseID, err := h.store.GetWarehouseByBranch(*user.BranchID)
	if err != nil {
		log.Println("resolve warehouse error:", err)
		return httperr.BadRequest(c, "No warehouse found for your branch")
	}

	result, err := h.store.LookupVariant(sku, warehouseID)
	if err == sql.ErrNoRows {
		return httperr.NotFound(c, "No active variant found for this SKU/barcode")
	}
	if err != nil {
		log.Println("lookup variant error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}

// CacheStatus handles GET /billing/cache — shows all cached variants.
func (h *Handler) CacheStatus(c *fiber.Ctx) error {
	variants, err := h.store.GetCachedVariants()
	if err != nil {
		return httperr.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{
		"cached_variants": len(variants),
		"variants":        variants,
	})
}

// Search handles GET /billing/search?q=ban — autocomplete for SKU/barcode/product name.
func (h *Handler) Search(c *fiber.Ctx) error {
	q := c.Query("q")
	if q == "" {
		return httperr.BadRequest(c, "q query param is required")
	}

	user := c.Locals("user").(*model.User)
	if user.BranchID == nil {
		return httperr.BadRequest(c, "Your account has no branch assigned")
	}

	warehouseID, err := h.store.GetWarehouseByBranch(*user.BranchID)
	if err != nil {
		return httperr.BadRequest(c, "No warehouse found for your branch")
	}

	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	results, err := h.store.SearchVariants(q, warehouseID, limit)
	if err != nil {
		log.Println("search variants error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"results": results,
		"count":   len(results),
	})
}

// AddPayment handles POST /billing/:id/payments — add payment to an existing bill.
func (h *Handler) AddPayment(c *fiber.Ctx) error {
	id := c.Params("id")

	var p PaymentInput
	if err := c.BodyParser(&p); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if p.Amount <= 0 {
		return httperr.BadRequest(c, "amount must be > 0")
	}
	if p.Method == "" {
		return httperr.BadRequest(c, "method is required")
	}

	result, err := h.store.AddPayment(id, p)
	if err == sql.ErrNoRows {
		return httperr.NotFound(c, "Invoice not found")
	}
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already fully paid") || strings.Contains(msg, "exceeds balance due") {
			return httperr.BadRequest(c, msg)
		}
		log.Println("add payment error:", err)
		return httperr.Internal(c)
	}

	// Auto-record payment in accounting (non-blocking)
	if h.recorder != nil {
		user := c.Locals("user").(*model.User)
		go func() {
			// Get latest sales_payment ID for this invoice
			var paymentID string
			h.store.QueryLatestSalesPaymentID(id, &paymentID)
			if paymentID != "" {
				if err := h.recorder.RecordSalesPayment(paymentID, user.ID.String()); err != nil {
					log.Println("accounting auto-record sales payment error:", err)
				}
			}
		}()
	}

	return c.JSON(result)
}

// CustomerLookup handles GET /billing/customer?phone=9876543210
func (h *Handler) CustomerLookup(c *fiber.Ctx) error {
	phone := c.Query("phone")
	if phone == "" {
		return httperr.BadRequest(c, "phone query param is required")
	}

	result, err := h.store.GetCustomerByPhone(phone)
	if err == sql.ErrNoRows {
		return c.JSON(fiber.Map{"exists": false})
	}
	if err != nil {
		log.Println("customer lookup error:", err)
		return httperr.Internal(c)
	}

	result["exists"] = true
	return c.JSON(result)
}
