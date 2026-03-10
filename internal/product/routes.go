package product

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/products")

	g.Post("/", h.Create) //tested

	g.Put("/:id/main-image", h.ReplaceMainImage)       //tested
	g.Post("/:id/images", h.AddGalleryImages)          //tested
	g.Delete("/images/:imageId", h.DeleteGalleryImage) //tested

	g.Get("/", h.List)        //tested
	g.Get("/:id", h.Get)      //tested
	g.Patch("/:id", h.Update) //tested

	g.Patch("/:id/deactivate", h.Deactivate) //tested
	g.Patch("/:id/activate", h.Activate)     //tested
}
