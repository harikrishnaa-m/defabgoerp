package user

import (
	"defab-erp/internal/auth"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

//
// ✅ CREATE USER (admin create staff)
//

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateUserInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return c.Status(500).SendString("hash error")
	}

	u := &model.User{
		Name:         in.Name,
		Email:        in.Email,
		PasswordHash: hash,
		RoleID:       in.RoleID,
		BranchID:     in.BranchID,
	}

	if err := h.store.Create(u); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	// ✅ reload with role join
	full, err := h.store.GetByID(u.ID.String())
	if err != nil {
		return c.Status(500).SendString("reload failed")
	}

	full.PasswordHash = ""
	return c.Status(201).JSON(full)
}


//
// ✅ LIST USERS (with role data)
//

func (h *Handler) List(c *fiber.Ctx) error {

	// defaults
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	rows, err := h.store.ListActive(limit, offset)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var out []model.User

	for rows.Next() {
		var u model.User

		err := rows.Scan(
			&u.ID,
			&u.Name,
			&u.Email,
			&u.RoleID,
			&u.Role.ID,
			&u.Role.Name,
			&u.Role.Permissions,
			&u.BranchID,
			&u.IsActive,
			&u.CreatedAt,
		)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		u.PasswordHash = ""
		out = append(out, u)
	}

	total, err := h.store.CountActive()
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.JSON(fiber.Map{
		"data":  out,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}


//
// ✅ GET USER BY ID (with role data)
//

func (h *Handler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	u, err := h.store.GetByID(id)
	if err != nil {
		return c.Status(404).SendString("not found")
	}

	u.PasswordHash = ""
	return c.JSON(u)
}

//
// ✅ UPDATE USER (partial update)
//

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdateUserInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if err := h.store.Update(id, in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}


func (h *Handler) Deactivate(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.SetActive(id, false); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}

func (h *Handler) Activate(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.SetActive(id, true); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}
