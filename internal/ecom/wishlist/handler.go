package wishlist

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

// GET /ecom/wishlist
func (h *Handler) List(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	items, err := h.store.List(cust.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch wishlist"})
	}
	if items == nil {
		items = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{"items": items, "count": len(items)})
}

// POST /ecom/wishlist/items
func (h *Handler) Add(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	var in struct {
		ProductID string `json:"product_id"`
	}
	if err := c.BodyParser(&in); err != nil || in.ProductID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "product_id required"})
	}

	if err := h.store.Add(cust.ID, in.ProductID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(201)
}

// DELETE /ecom/wishlist/items/:product_id
func (h *Handler) Remove(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	productID := c.Params("product_id")

	if err := h.store.Remove(cust.ID, productID); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(200)
}
