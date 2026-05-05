package directgrn

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, h *Handler) {
	router.Post("/", h.Create)
	router.Get("/", h.List)
	router.Get("/:id", h.GetByID)
}
