package salesorder

import (
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) List(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	status := c.Query("status")
	paymentStatus := c.Query("payment_status")

	var branchID *string
	if user.Role.Name == model.RoleStoreManager || user.Role.Name == model.RoleSalesPerson {
		branchID = user.BranchID
	} else if c.Query("branch_id") != "" {
		bid := c.Query("branch_id")
		branchID = &bid
	}

	orders, total, err := h.store.List(branchID, status, paymentStatus, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":  orders,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	order, err := h.store.GetByID(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "sales order not found"})
	}

	return c.JSON(order)
}
