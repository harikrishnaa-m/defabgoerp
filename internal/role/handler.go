package role

import "github.com/gofiber/fiber/v2"

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var in struct {
		Name        string `json:"name"`
		Permissions string `json:"permissions"`
	}

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if in.Name == "" {
		return c.Status(400).SendString("name required")
	}

	if err := h.store.Create(in.Name, in.Permissions); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(201)
}

func (h *Handler) List(c *fiber.Ctx) error {
	roles, err := h.store.List()
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.JSON(roles)
}
