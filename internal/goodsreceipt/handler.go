package goodsreceipt

import (
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

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateGoodsReceiptInput

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.SupplierID == "" || in.WarehouseID == "" {
		return httperr.BadRequest(c, "supplier_id and warehouse_id required")
	}

	if len(in.Items) == 0 {
		return httperr.BadRequest(c, "at least one item required")
	}

	user := c.Locals("user").(*model.User)

	if err := h.store.Create(in, user.ID.String()); err != nil {
		log.Println("goods receipt error:", err)
		return httperr.Internal(c)
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "Goods receipt recorded successfully",
	})
}
