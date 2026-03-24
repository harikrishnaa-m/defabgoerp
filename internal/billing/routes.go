package billing

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/search", h.Search)
	r.Get("/lookup", h.Lookup)
	r.Get("/customer", h.CustomerLookup)
	r.Get("/cache", h.CacheStatus)
	r.Post("/:id/payments", h.AddPayment) //use invoice id to complete payments
	r.Get("/:id", h.GetByID)
}
