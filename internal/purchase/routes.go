package purchase

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/purchase-orders")

	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Get("/:id", h.Get)
	g.Patch("/:id/status", h.UpdateStatus)

	// PO Item management
	g.Post("/:id/items", h.AddItem)
	g.Patch("/:id/items/:itemId", h.UpdateItem)
	g.Delete("/:id/items/:itemId", h.DeleteItem)
}
