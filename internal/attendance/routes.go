package attendance

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/punch-in", h.PunchIn)
	r.Post("/punch-out", h.PunchOut)
	r.Post("/upload", h.UploadExcel)
	r.Get("/", h.List)
	r.Get("/:id", h.GetByID)
}
