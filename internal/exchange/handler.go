package exchange

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"defab-erp/internal/accounting"
	"defab-erp/internal/core/httperr"
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

// POST /exchanges
func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateExchangeInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.OriginalSalesInvoiceID == "" || len(in.ItemsOut) == 0 || len(in.ItemsIn) == 0 {
		return httperr.BadRequest(c, "original_sales_invoice_id, items_out and items_in are required")
	}

	for i, s := range in.Settlements {
		if s.PaymentMethod == "" {
			return httperr.BadRequest(c, "settlements["+strconv.Itoa(i)+"].payment_method is required")
		}
		if s.Amount <= 0 {
			return httperr.BadRequest(c, "settlements["+strconv.Itoa(i)+"].amount must be > 0")
		}
	}

	user := c.Locals("user").(*model.User)
	branchID := ""
	if user.BranchID != nil {
		branchID = *user.BranchID
	}

	id, err := h.store.Create(in, user.ID.String(), branchID)
	if err != nil {
		log.Println("create exchange error:", err)
		// Surface validation errors to the client
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.recorder.RecordExchange(id, user.ID.String()); err != nil {
		log.Println("record exchange voucher error:", err)
	}

	result, err := h.store.GetByID(id)
	if err != nil {
		log.Println("get exchange after create error:", err)
		return c.Status(http.StatusCreated).JSON(fiber.Map{"exchange_order_id": id})
	}
	return c.Status(http.StatusCreated).JSON(result)
}

// GET /exchanges
func (h *Handler) List(c *fiber.Ctx) error {
	f := ExchangeListFilter{}
	if bid := c.Query("branch_id"); bid != "" {
		f.BranchID = &bid
	}
	f.Status = c.Query("status")
	f.Search = c.Query("search")
	if l := c.QueryInt("limit"); l > 0 {
		f.Limit = l
	}
	if o := c.QueryInt("offset"); o >= 0 {
		f.Offset = o
	}

	list, total, err := h.store.List(f)
	if err != nil {
		log.Println("list exchanges error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"exchanges": list, "total": total, "limit": f.Limit, "offset": f.Offset})
}

// GET /exchanges/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	exc, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Exchange order not found")
		}
		log.Println("get exchange error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(exc)
}

// DELETE /exchanges/:id
func (h *Handler) Cancel(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)

	if err := h.store.Cancel(id); err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Exchange order not found")
		}
		log.Println("cancel exchange error:", err)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.recorder.CancelVoucherByRef("exchange_order", id); err != nil {
		log.Println("cancel exchange voucher error:", err)
	}
	_ = user
	return c.JSON(fiber.Map{"message": "Exchange order cancelled"})
}
