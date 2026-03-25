package customer

import "github.com/gofiber/fiber/v2"

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	customers, total, err := h.store.List(limit, offset, search)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":  customers,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	customer, err := h.store.GetByID(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "customer not found"})
	}

	return c.JSON(customer)
}
