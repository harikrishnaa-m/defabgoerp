package category

import "github.com/gofiber/fiber/v2"

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

//
// CREATE
//

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateCategoryInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if in.Name == "" {
		return c.Status(400).SendString("name required")
	}

	if err := h.store.Create(in.Name); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(201)
}

//
// LIST ACTIVE ONLY
//

func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	rows, err := h.store.ListActive(limit, offset)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, name string
		var active bool

		rows.Scan(&id, &name, &active)

		out = append(out, fiber.Map{
			"id":   id,
			"name": name,
		})
	}

	total, _ := h.store.CountActive()

	return c.JSON(fiber.Map{
		"data":  out,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}

//
// GET
//

func (h *Handler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	cid, name, active, err := h.store.Get(id)
	if err != nil {
		return c.Status(404).SendString("not found")
	}

	return c.JSON(fiber.Map{
		"id":        cid,
		"name":      name,
		"is_active": active,
	})
}

//
// UPDATE
//

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdateCategoryInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if err := h.store.Update(id, in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}

//
// SOFT DELETE
//

func (h *Handler) Deactivate(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.SetActive(id, false); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}

//
// RESTORE
//

func (h *Handler) Activate(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.SetActive(id, true); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}
