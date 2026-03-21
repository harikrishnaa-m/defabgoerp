package purchase

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/:id", h.Get)
	r.Patch("/:id", h.Update)
	r.Patch("/:id/status", h.UpdateStatus)
	r.Delete("/:id", h.Delete)

	// PO Item management
	r.Post("/:id/items", h.AddItem)
	r.Patch("/:id/items/:itemId", h.UpdateItem)
	r.Delete("/:id/items/:itemId", h.DeleteItem)
}
