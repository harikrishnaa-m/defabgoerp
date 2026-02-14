package variant

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/variants")

	g.Post("/", h.Create)
	g.Post("/generate", h.Generate)

	g.Get("/product/:productId", h.ListByProduct)

	g.Patch("/:id", h.Update)
	g.Patch("/:id/deactivate", h.Deactivate)
	g.Patch("/:id/activate", h.Activate)

	g.Post("/:id/images", h.AddImages)
	g.Get("/:id/images", h.ListImages)
	g.Delete("/images/:imageId", h.DeleteImage)
}
