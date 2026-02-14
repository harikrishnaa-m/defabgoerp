package product

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/products")

	g.Post("/", h.Create)


	g.Put("/:id/main-image", h.ReplaceMainImage)
	g.Post("/:id/images", h.AddGalleryImages)
	g.Delete("/images/:imageId", h.DeleteGalleryImage)


	
	g.Get("/", h.List)
	g.Get("/:id", h.Get)
	g.Patch("/:id", h.Update)

	g.Patch("/:id/deactivate", h.Deactivate)
	g.Patch("/:id/activate", h.Activate)
}

