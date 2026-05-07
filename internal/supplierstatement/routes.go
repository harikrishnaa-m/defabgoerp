package supplierstatement

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, h *Handler) {
	router.Get("/", h.Get)
}
