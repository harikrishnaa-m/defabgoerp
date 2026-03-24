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
	// Generate warehouse_code: WHxxx
	code, err := h.store.NextWarehouseCode()
	if err != nil {
		return c.Status(500).SendString("warehouse code generation failed")
	}
	in.WarehouseCode = code
	if err := h.store.Create(in); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.SendStatus(201)
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	row := h.store.GetByID(id)

	var warehouse struct {
		ID            string `json:"id"`
		BranchID      string `json:"branch_id"`
		BranchName    string `json:"branch_name"`
		Name          string `json:"name"`
		Type          string `json:"type"`
		CreatedAt     string `json:"created_at"`
		WarehouseCode string `json:"warehouse_code"`
		BranchCity    string `json:"branch_city"`
		BranchState   string `json:"branch_state"`
		BranchPhone   string `json:"branch_phone"`
	}

	err := row.Scan(
		&warehouse.ID,
		&warehouse.BranchID,
		&warehouse.BranchName,
		&warehouse.Name,
		&warehouse.Type,
		&warehouse.CreatedAt,
		&warehouse.WarehouseCode,
		&warehouse.BranchCity,
		&warehouse.BranchState,
		&warehouse.BranchPhone,
	)

	if err != nil {
		return c.Status(404).SendString("warehouse not found")
	}

	return c.JSON(warehouse)
}

func (h *Handler) List(c *fiber.Ctx) error {
	rows, err := h.store.List()
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var out []map[string]any

	for rows.Next() {
		var id, branchID, branchName, name, typ, created, warehouseCode, city, state, phoneNumber string

		rows.Scan(&id, &branchID, &branchName, &name, &typ, &created, &warehouseCode, &city, &state, &phoneNumber)

		out = append(out, map[string]any{
			"id":             id,
			"branch_id":      branchID,
			"branch_name":    branchName,
			"name":           name,
			"type":           typ,
			"created_at":     created,
			"warehouse_code": warehouseCode,
			"branch_city":    city,
			"branch_state":   state,
			"branch_phone":   phoneNumber,
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

func (h *Handler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.Delete(id); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.JSON(fiber.Map{
		"message": "warehouse deleted",
	})
}
