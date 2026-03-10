package variant

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateVariantInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad input"})
	}

	id, err := h.store.Create(in)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "variant created",
		"id":      id,
	})
}

func (h *Handler) Generate(c *fiber.Ctx) error {
	var in GenerateVariantsInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad input"})
	}

	// Convert map to ordered groups for cartesian product
	var groups [][]string
	var attrOrder []string
	for attrID, valueIDs := range in.AttributeValues {
		groups = append(groups, valueIDs)
		attrOrder = append(attrOrder, attrID)
	}
	// Debug output
	fmt.Printf("Generate handler: attrOrder = %v\n", attrOrder)
	fmt.Printf("Generate handler: groups = %v\n", groups)

	ids, err := h.store.GenerateWithAttrOrder(in.ProductID, in.BasePrice, attrOrder, groups)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "variants generated",
		"count":   len(ids),
	})
}

func (h *Handler) ListByProduct(c *fiber.Ctx) error {
	rows, err := h.store.ListByProduct(c.Params("productId"))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, name, sku string
		var price, cost float64
		var active bool

		rows.Scan(&id, &name, &sku, &price, &cost, &active)

		out = append(out, fiber.Map{
			"id":         id,
			"name":       name,
			"sku":        sku,
			"price":      price,
			"cost_price": cost,
			"is_active":  active,
		})
	}

	return c.JSON(out)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	var in UpdateVariantInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad input"})
	}

	if err := h.store.Update(c.Params("id"), in); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "variant updated"})
}

func (h *Handler) Deactivate(c *fiber.Ctx) error {
	return h.toggle(c, false)
}

func (h *Handler) Activate(c *fiber.Ctx) error {
	return h.toggle(c, true)
}

func (h *Handler) toggle(c *fiber.Ctx, active bool) error {
	if err := h.store.SetActive(c.Params("id"), active); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "status updated"})
}
