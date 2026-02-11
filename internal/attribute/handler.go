package attribute

import "github.com/gofiber/fiber/v2"

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

//
// ================= ATTRIBUTE =================
//

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateAttributeInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad input"})
	}

	if in.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name required"})
	}

	if err := h.store.Create(in.Name); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "attribute created",
	})
}

func (h *Handler) List(c *fiber.Ctx) error {
	rows, err := h.store.List(50, 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, name string
		rows.Scan(&id, &name)

		out = append(out, fiber.Map{
			"id":   id,
			"name": name,
		})
	}

	return c.JSON(out)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdateAttributeInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad input"})
	}

	if err := h.store.Update(id, in.Name); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "attribute updated",
	})
}

func (h *Handler) Deactivate(c *fiber.Ctx) error {
	if err := h.store.SetActive(c.Params("id"), false); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "attribute deactivated",
	})
}

func (h *Handler) Activate(c *fiber.Ctx) error {
	if err := h.store.SetActive(c.Params("id"), true); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "attribute activated",
	})
}

//
// ================= ATTRIBUTE VALUES =================
//

func (h *Handler) CreateValue(c *fiber.Ctx) error {
	var in CreateValueInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad input"})
	}

	if in.AttributeID == "" || in.Value == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "attribute_id and value required",
		})
	}

	if err := h.store.CreateValue(in.AttributeID, in.Value); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "attribute value created",
	})
}

func (h *Handler) ListValues(c *fiber.Ctx) error {
	rows, err := h.store.ListValues(c.Params("id"))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, value string
		rows.Scan(&id, &value)

		out = append(out, fiber.Map{
			"id":    id,
			"value": value,
		})
	}

	return c.JSON(out)
}

func (h *Handler) UpdateValue(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdateValueInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad input"})
	}

	if err := h.store.UpdateValue(id, in.Value); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "attribute value updated",
	})
}

func (h *Handler) DeactivateValue(c *fiber.Ctx) error {
	if err := h.store.SetValueActive(c.Params("id"), false); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "attribute value deactivated",
	})
}

func (h *Handler) ActivateValue(c *fiber.Ctx) error {
	if err := h.store.SetValueActive(c.Params("id"), true); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "attribute value activated",
	})
}
