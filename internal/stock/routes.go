package stock

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/stocks")

	g.Post("/", h.Create)                  //to create stock
	g.Patch("/:id", h.Update)              // to edit stock
	g.Get("/warehouse/:id", h.ByWarehouse) //to get all stock in a warehouse
	g.Get("/variant/:id", h.ByVariant)
	g.Get("/low", h.LowStock)
	g.Get("/branch/:id", h.ByBranch) // get all stocks in a branch

	// MUST ADD
	g.Get("/", h.All)                  // Shows every variant in every warehouse
	g.Get("/product/:id", h.ByProduct) // total stock per variant (across all warehouses).
	g.Get("/movements", h.Movements)

	// OPTIONAL
	g.Get("/warehouse/:id/products", h.ByWarehouseProductSummary)

}
