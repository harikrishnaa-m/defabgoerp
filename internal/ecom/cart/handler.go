package cart

import (
	ecomMw "defab-erp/internal/ecom/middleware"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// GET /ecom/cart
func (h *Handler) Get(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	items, err := h.store.GetCart(cust.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch cart"})
	}
	if items == nil {
		items = []map[string]interface{}{}
	}

	count, total, _ := h.store.CartSummary(cust.ID)

	return c.JSON(fiber.Map{
		"items":      items,
		"item_count": count,
		"total":      total,
	})
}

// POST /ecom/cart/items
func (h *Handler) AddItem(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	var in AddItemInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if in.VariantID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "variant_id is required"})
	}
	if in.Quantity <= 0 {
		in.Quantity = 1
	}

	if err := h.store.AddItem(cust.ID, in.VariantID, in.Quantity); err != nil {
		if err.Error() == "variant not found or inactive" {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to add item"})
	}

	return c.Status(201).JSON(fiber.Map{"message": "item added to cart"})
}

// PATCH /ecom/cart/items/:id
func (h *Handler) UpdateItem(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	itemID := c.Params("id")

	var in UpdateQtyInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if in.Quantity <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "quantity must be > 0"})
	}

	if err := h.store.UpdateItemQty(cust.ID, itemID, in.Quantity); err != nil {
		if err.Error() == "cart item not found" {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).JSON(fiber.Map{"error": "update failed"})
	}

	return c.JSON(fiber.Map{"message": "cart item updated"})
}

// DELETE /ecom/cart/items/:id
func (h *Handler) RemoveItem(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	itemID := c.Params("id")

	if err := h.store.RemoveItem(cust.ID, itemID); err != nil {
		if err.Error() == "cart item not found" {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).JSON(fiber.Map{"error": "remove failed"})
	}

	return c.JSON(fiber.Map{"message": "item removed from cart"})
}

// DELETE /ecom/cart
func (h *Handler) Clear(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	if err := h.store.ClearCart(cust.ID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "clear failed"})
	}

	return c.JSON(fiber.Map{"message": "cart cleared"})
}
