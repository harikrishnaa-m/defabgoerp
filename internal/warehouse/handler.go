package warehouse

import "github.com/gofiber/fiber/v2"

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateWarehouseInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if in.Name == "" {
		return c.Status(400).SendString("name required")
	}

	if in.Type == "" {
		in.Type = "STORE"
	}

	if err := h.store.Create(in); err != nil {
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
		var id string
		var branchID *int
		var name, typ, created string

		rows.Scan(&id, &branchID, &name, &typ, &created)

		out = append(out, map[string]any{
			"id": id,
			"branch_id": branchID,
			"name": name,
			"type": typ,
			"created_at": created,
		})
	}

	return c.JSON(out)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdateWarehouseInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if err := h.store.Update(id, in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}
