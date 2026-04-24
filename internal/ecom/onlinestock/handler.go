package onlinestock

import (
	"math"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// POST / — upsert online stock for a variant
func (h *Handler) Set(c *fiber.Ctx) error {
	var in SetStockInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}
	if in.VariantID == "" {
		return c.Status(400).SendString("variant_id required")
	}
	if in.Quantity < 0 {
		return c.Status(400).SendString("quantity cannot be negative")
	}
	if err := h.store.Upsert(in.VariantID, in.Quantity); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.SendStatus(200)
}

// PATCH /:variant_id — update quantity for a specific variant
func (h *Handler) Update(c *fiber.Ctx) error {
	variantID := c.Params("variant_id")
	var in UpdateStockInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}
	if in.Quantity < 0 {
		return c.Status(400).SendString("quantity cannot be negative")
	}
	if err := h.store.Upsert(variantID, in.Quantity); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.SendStatus(200)
}

// GET / — list all online stocks
func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	search := c.Query("q")
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	items, total, err := h.store.List(search, page, limit)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	if items == nil {
		items = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{
		"data":        items,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
	})
}

// GET /web-visible — sync missing variants then list all web-visible stocks
func (h *Handler) SyncWebVisible(c *fiber.Ctx) error {
	items, newlyAdded, err := h.store.SyncAndListWebVisible()
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	if items == nil {
		items = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{
		"data":        items,
		"newly_added": newlyAdded,
	})
}

// GET /low-stock — variants with online stock < 10
func (h *Handler) LowStock(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	search := c.Query("q")
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	items, total, err := h.store.ListLowStock(search, page, limit)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	if items == nil {
		items = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{
		"data":        items,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
	})
}
