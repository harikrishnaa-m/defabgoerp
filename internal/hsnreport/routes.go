package hsnreport

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, h *Handler) {
	router.Get("/sales", h.ListSales)
	router.Get("/purchase", h.ListPurchase)
	router.Get("/job-orders", h.ListJobOrders)
}
