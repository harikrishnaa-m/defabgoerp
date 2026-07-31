package joborder

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/:id", h.GetByID)
	r.Put("/:id", h.Update)
	r.Patch("/:id/items/:itemId/staff", h.UpdateItemStaff)
	r.Post("/:id/items/:itemId/work-logs", h.CreateWorkLog)
	r.Patch("/:id/items/:itemId/work-logs/:logId", h.UpdateWorkLog)
	r.Delete("/:id/items/:itemId/work-logs/:logId", h.DeleteWorkLog)
	r.Post("/:id/status", h.PushStatus)
	r.Post("/:id/payments", h.AddPayment)
	r.Delete("/:id", h.Cancel)
}
