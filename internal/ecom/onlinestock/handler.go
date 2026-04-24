package onlinestock

import "github.com/gofiber/fiber/v2"

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
	items, err := h.store.List()
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"data": items})
}
