package stock

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/stocks")

	g.Post("/", h.Create)            // create or upsert stock
	g.Get("/", h.All)                // all stocks (paginated with total)
	g.Get("/low", h.LowStock)        // low stock alert
	g.Get("/movements", h.Movements) // movement audit log (filterable)

	g.Get("/warehouse/:id", h.ByWarehouse) // stocks in a warehouse (paginated with total)
	g.Get("/warehouse/:id/products", h.ByWarehouseProductSummary)
	g.Get("/branch/:id", h.ByBranch)   // stocks in a branch (paginated with total)
	g.Get("/variant/:id", h.ByVariant) // stock for variant across warehouses
	g.Get("/product/:id", h.ByProduct) // total stock per variant for a product

	g.Get("/:id", h.GetByID)        // single stock detail
	g.Patch("/:id", h.Update)       // raw update (backward compat)
	g.Post("/:id/adjust", h.Adjust) // audited adjustment with movement
	g.Delete("/:id", h.Delete)      // delete stock record
}
