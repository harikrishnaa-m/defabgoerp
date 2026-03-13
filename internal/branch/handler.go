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
	// Generate branch_code: BRxxx
	code, err := h.store.NextBranchCode()
	if err != nil {
		return c.Status(500).SendString("branch code generation failed")
	}
	if err := h.store.Create(in.Name, in.Address, in.ManagerID, code, in.City, in.State, in.PhoneNumber); err != nil {
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
		var id, name, address, managerName, created, branchCode, phoneNumber, city, state string
		var managerID *string

		rows.Scan(&id, &name, &address, &managerID, &managerName, &created, &branchCode, &phoneNumber, &city, &state)

		out = append(out, map[string]any{
			"id":           id,
			"name":         name,
			"address":      address,
			"manager_id":   managerID,
			"manager_name": managerName,
			"created_at":   created,
			"branch_code":  branchCode,
			"phone_number": phoneNumber,
			"city":         city,
			"state":        state,
		})
	}

	return c.JSON(out)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdateBranchInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if err := h.store.Update(id, in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.JSON(fiber.Map{"message": "branch updated"})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	row := h.store.GetByID(id)
	var branch struct {
		ID          string
		Name        string
		Address     string
		ManagerID   *string
		ManagerName string
		CreatedAt   string
		BranchCode  string
		PhoneNumber string
		City        string
		State       string
	}
	err := row.Scan(&branch.ID, &branch.Name, &branch.Address, &branch.ManagerID, &branch.ManagerName, &branch.CreatedAt,
		&branch.BranchCode, &branch.PhoneNumber, &branch.City, &branch.State)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return c.Status(404).SendString("branch not found")
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(branch)
}
