package coupon

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)

	r.Get("/", h.List) // pagination supports
	r.Get("/:id", h.Get)
	r.Patch("/:id", h.Update)
	r.Patch("/:id/activate", h.Activate)
	r.Patch("/:id/deactivate", h.Deactivate)

	// 🔹 NEW
	r.Post("/:id/variants", h.AttachVariants)
	r.Post("/:id/categories", h.AttachCategories)

	r.Delete("/variants/:mappingId", h.RemoveVariant)
	r.Delete("/categories/:mappingId", h.RemoveCategory)
}
