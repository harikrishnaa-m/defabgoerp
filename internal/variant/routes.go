package variant

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Post("/generate", h.Generate)

	r.Get("/item-master", h.ItemMaster)
	r.Get("/search", h.Search)
	r.Get("/by-code/:code", h.GetByCode)
	r.Get("/product/:productId", h.ListByProduct)
	r.Get("/:id", h.GetByID)

	r.Patch("/:id", h.Update)
	r.Patch("/:id/deactivate", h.Deactivate)
	r.Patch("/:id/activate", h.Activate)

	r.Post("/:id/images", h.AddImages)
	r.Get("/:id/images", h.ListImages)
	r.Delete("/images/:imageId", h.DeleteImage)

	r.Post("/backfill-codes", h.BackfillVariantCodes)
}

//tested
