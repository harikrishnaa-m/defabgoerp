package stock

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)                           // create or upsert stock
	r.Get("/", h.All)                               // all stocks (paginated with total)
	r.Get("/low", h.LowStock)                       // low stock alert
	r.Get("/movements", h.Movements)                // movement audit log (filterable)
	r.Get("/movements/branch", h.MovementsByBranch) // movements for particular branch
	r.Get("/movements/:id", h.MovementByID)         // single movement detail
	r.Get("/available", h.Available)                // all central warehouse stocks
	r.Get("/available/new", h.AvailableNew)         // central stocks NOT in my branch

	r.Get("/warehouse/:id", h.ByWarehouse) // stocks in a warehouse (paginated with total)
	r.Get("/warehouse/:id/products", h.ByWarehouseProductSummary)
	r.Get("/branch/:id", h.ByBranch)   // stocks in a branch (paginated with total)
	r.Get("/variant/:id", h.ByVariant) // stock for variant across warehouses
	r.Get("/product/:id", h.ByProduct) // total stock per variant for a product

	r.Get("/:id", h.GetByID)        // single stock detail
	r.Patch("/:id", h.Update)       // raw update (backward compat)
	r.Post("/:id/adjust", h.Adjust) // audited adjustment with movement
	r.Delete("/:id", h.Delete)      // delete stock record
}
