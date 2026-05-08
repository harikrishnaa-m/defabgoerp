package supplieraging

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// GET /?date_from=2026-04-01&date_to=2026-05-05
func (h *Handler) Get(c *fiber.Ctx) error {
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	result, err := h.store.Get(dateFrom, dateTo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}
