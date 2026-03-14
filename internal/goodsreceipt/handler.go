package goodsreceipt

import (
	"database/sql"
	"log"

	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// Create handles POST /goods-receipts
func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateGoodsReceiptInput

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.PurchaseOrderID == "" || in.SupplierID == "" || in.WarehouseID == "" {
		return httperr.BadRequest(c, "purchase_order_id, supplier_id and warehouse_id required")
	}

	if len(in.Items) == 0 {
		return httperr.BadRequest(c, "at least one item required")
	}

	user := c.Locals("user").(*model.User)

	grnID, err := h.store.Create(in, user.ID.String())
	if err != nil {
		log.Println("goods receipt error:", err)
		return httperr.Internal(c)
	}

	grn, err := h.store.GetByID(grnID)
	if err != nil {
		log.Println("goods receipt fetch error:", err)
		return httperr.Internal(c)
	}

	return c.Status(201).JSON(grn)
}

// GetByID handles GET /goods-receipts/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	grn, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Goods receipt not found")
		}
		log.Println("goods receipt get error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(grn)
}

// ListByPO handles GET /goods-receipts/po/:poId
func (h *Handler) ListByPO(c *fiber.Ctx) error {
	poID := c.Params("poId")

	list, err := h.store.ListByPO(poID)
	if err != nil {
		log.Println("goods receipt list by po error:", err)
		return httperr.Internal(c)
	}

	if list == nil {
		list = []GoodsReceiptResponse{}
	}

	return c.JSON(list)
}

// List handles GET /goods-receipts
func (h *Handler) List(c *fiber.Ctx) error {
	list, err := h.store.List()
	if err != nil {
		log.Println("goods receipt list error:", err)
		return httperr.Internal(c)
	}

	if list == nil {
		list = []GoodsReceiptResponse{}
	}

	return c.JSON(list)
}
