package coupon

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/coupons")

	g.Post("/", h.Create)
	
	g.Get("/", h.List) // pagination supports
	g.Get("/:id", h.Get)
	g.Patch("/:id", h.Update)
	g.Patch("/:id/activate", h.Activate)
	g.Patch("/:id/deactivate", h.Deactivate)

	// 🔹 NEW
	g.Post("/:id/variants", h.AttachVariants)
	g.Post("/:id/categories", h.AttachCategories)

	g.Delete("/variants/:mappingId", h.RemoveVariant)
	g.Delete("/categories/:mappingId", h.RemoveCategory)
}
