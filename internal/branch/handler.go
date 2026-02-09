package branch

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateBranchInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if in.Name == "" {
		return c.Status(400).SendString("name required")
	}

	if err := h.store.Create(in.Name, in.Address, in.ManagerID); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(201)
}

func (h *Handler) List(c *fiber.Ctx) error {
	rows, err := h.store.List()
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var out []map[string]any

	for rows.Next() {
		var id int
		var name, address string
		var managerID *string
		var created string

		rows.Scan(&id, &name, &address, &managerID, &created)

		out = append(out, map[string]any{
			"id": id,
			"name": name,
			"address": address,
			"manager_id": managerID,
			"created_at": created,
		})
	}

	return c.JSON(out)
}


func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).SendString("invalid id")
	}

	var in UpdateBranchInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if err := h.store.Update(id, in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}
