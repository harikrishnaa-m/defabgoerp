package stock

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/stocks")

	g.Get("/warehouse/:id", h.ByWarehouse)
	g.Get("/variant/:id", h.ByVariant)
	g.Get("/low", h.LowStock)

		// MUST ADD
	g.Get("/", h.All)  // Shows every variant in every warehouse
	g.Get("/product/:id", h.ByProduct) // total stock per variant (across all warehouses).
	g.Get("/movements", h.Movements)
	
	// OPTIONAL
	g.Get("/warehouse/:id/products", h.ByWarehouseProductSummary)


}
