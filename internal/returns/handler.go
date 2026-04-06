package returns

import (
	"database/sql"
	"log"
	"net/http"

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

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateReturnOrderInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.SalesInvoiceID == "" || len(in.Items) == 0 {
		return httperr.BadRequest(c, "sales_invoice_id and items are required")
	}
	user := c.Locals("user").(*model.User)
	id, err := h.store.CreateReturnOrder(in, user.ID.String())
	if err != nil {
		log.Println("create return order error:", err)
		return httperr.Internal(c)
	}
	if err := h.recorder.RecordSalesReturn(id, user.ID.String()); err != nil {
		log.Println("record return voucher error:", err)
	}
	return c.Status(http.StatusCreated).JSON(fiber.Map{"return_order_id": id})
}

func (h *Handler) List(c *fiber.Ctx) error {
	filter := ReturnListFilter{Limit: 20, Offset: 0}
	if bid := c.Query("branch_id"); bid != "" {
		filter.BranchID = &bid
	}
	filter.Status = c.Query("status")
	filter.Search = c.Query("search")
	if limit := c.QueryInt("limit"); limit > 0 {
		filter.Limit = limit
	}
	if offset := c.QueryInt("offset"); offset >= 0 {
		filter.Offset = offset
	}

	list, total, err := h.store.List(filter.BranchID, filter.Status, filter.Search, filter.Limit, filter.Offset)
	if err != nil {
		log.Println("list return orders error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"returns": list, "total": total, "limit": filter.Limit, "offset": filter.Offset})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	ret, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Return order not found")
		}
		log.Println("get return order error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(ret)
}

func (h *Handler) Complete(c *fiber.Ctx) error {
	id := c.Params("id")
	var in CompleteReturnInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.RefundType == "" {
		in.RefundType = RefundTypeCash
	}
	user := c.Locals("user").(*model.User)
	ret, err := h.store.CompleteReturnOrder(id, in, user.ID.String())
	if err != nil {
		log.Println("complete return order error:", err)
		return httperr.Internal(c)
	}
	if err := h.recorder.RecordSalesReturn(id, user.ID.String()); err != nil {
		log.Println("record return voucher error:", err)
	}
	return c.JSON(ret)
}

func (h *Handler) Cancel(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.store.CancelReturnOrder(id); err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Return order not found")
		}
		log.Println("cancel return order error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "Return order cancelled"})
}
