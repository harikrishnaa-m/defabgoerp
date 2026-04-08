package production

import (
	"database/sql"
	"log"
	"net/http"

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
	var in CreateProductionOrderInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.OutputVariantID == "" || in.OutputQuantity <= 0 {
		return httperr.BadRequest(c, "output_variant_id and output_quantity are required")
	}

	user := c.Locals("user").(*model.User)
	branchID := ""
	if user.BranchID != nil {
		branchID = *user.BranchID
	}
	warehouseID := ""
	if branchID != "" {
		_ = h.store.db.QueryRow(`SELECT warehouse_id FROM branches WHERE id = $1`, branchID).Scan(&warehouseID)
	}

	id, err := h.store.CreateProductionOrder(in, user.ID.String(), branchID, warehouseID)
	if err != nil {
		log.Println("create production order error:", err)
		return httperr.Internal(c)
	}
	return c.Status(http.StatusCreated).JSON(fiber.Map{"production_order_id": id})
}

func (h *Handler) List(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	status := c.Query("status")
	search := c.Query("search")
	var branchID *string
	if bid := c.Query("branch_id"); bid != "" {
		branchID = &bid
	}

	list, total, err := h.store.List(branchID, status, search, limit, offset)
	if err != nil {
		log.Println("list production orders error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"production_orders": list, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	result, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Production order not found")
		}
		log.Println("get production order error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(result)
}

func (h *Handler) PushStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var in StatusUpdateInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.Status == "" {
		return httperr.BadRequest(c, "status is required")
	}
	user := c.Locals("user").(*model.User)
	if err := h.store.PushStatus(id, in, user.ID.String()); err != nil {
		log.Println("push production status error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "status updated"})
}

func (h *Handler) Complete(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)
	if err := h.store.Complete(id, user.ID.String()); err != nil {
		log.Println("complete production order error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "production completed, stock updated"})
}

func (h *Handler) Cancel(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)
	if err := h.store.Cancel(id, user.ID.String()); err != nil {
		log.Println("cancel production order error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "cancelled"})
}
