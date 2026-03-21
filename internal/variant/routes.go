package variant

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Post("/generate", h.Generate)

	r.Get("/product/:productId", h.ListByProduct)
	r.Get("/:id", h.GetByID)

	r.Patch("/:id", h.Update)
	r.Patch("/:id/deactivate", h.Deactivate)
	r.Patch("/:id/activate", h.Activate)

	r.Post("/:id/images", h.AddImages)
	r.Get("/:id/images", h.ListImages)
	r.Delete("/images/:imageId", h.DeleteImage)
}

//tested
