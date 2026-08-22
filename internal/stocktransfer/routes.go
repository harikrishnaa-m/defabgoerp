package stocktransfer

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)

	r.Post("/transfers/:id/receive", h.Receive)
}

func RegisterInterWarehouseRoute(r fiber.Router, h *Handler) {
	r.Get("/variant/:variant_code", h.GetBranchVariantStock)
	r.Post("/inter-warehouse", h.InterWarehouse)
}
