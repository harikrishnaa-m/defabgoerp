package salesperson

import (
	"database/sql"
	"log"

	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// Create handles POST /salespersons
func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateSalesPersonInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.Name == "" {
		return httperr.BadRequest(c, "name is required")
	}
	if in.Email == "" {
		return httperr.BadRequest(c, "email is required")
	}
	if in.Password == "" {
		return httperr.BadRequest(c, "password is required")
	}

	user := c.Locals("user").(*model.User)

	// StoreManager can only create for their own branch
	if user.Role.Name == model.RoleStoreManager {
		if user.BranchID == nil {
			return c.Status(403).JSON(fiber.Map{"error": "Your account has no branch assigned"})
		}
		in.BranchID = user.BranchID
	} else {
		// SuperAdmin must specify branch_id
		if in.BranchID == nil {
			return httperr.BadRequest(c, "branch_id is required")
		}
	}

	id, code, err := h.store.Create(in)
	if err != nil {
		log.Println("create salesperson error:", err)
		return httperr.Internal(c)
	}

	return c.Status(201).JSON(fiber.Map{
		"id":            id,
		"employee_code": code,
		"message":       "Salesperson created successfully",
	})
}

// List handles GET /salespersons
func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	user := c.Locals("user").(*model.User)

	var branchID *string
	// StoreManager sees only their branch
	if user.Role.Name == model.RoleStoreManager {
		branchID = user.BranchID
	} else {
		// SuperAdmin can optionally filter by branch
		if bid := c.Query("branch_id"); bid != "" {
			branchID = &bid
		}
	}

	list, err := h.store.List(branchID, limit, offset)
	if err != nil {
		log.Println("list salespersons error:", err)
		return httperr.Internal(c)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}

	return c.JSON(list)
}

// GetByID handles GET /salespersons/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)

	// StoreManager can only view salespersons in their branch
	if user.Role.Name == model.RoleStoreManager {
		brID, err := h.store.GetBranchID(id)
		if err != nil {
			if err == sql.ErrNoRows {
				return httperr.NotFound(c, "Salesperson not found")
			}
			log.Println("get salesperson branch error:", err)
			return httperr.Internal(c)
		}
		if brID == nil || user.BranchID == nil || *brID != *user.BranchID {
			return c.Status(403).JSON(fiber.Map{"error": "You can only view salespersons in your branch"})
		}
	}

	result, err := h.store.GetByID(id, SalesFilter{
		From:       c.Query("from"),
		To:         c.Query("to"),
		CategoryID: c.Query("category_id"),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Salesperson not found")
		}
		log.Println("get salesperson error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}

// Update handles PATCH /salespersons/:id
func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)

	// StoreManager can only edit salespersons in their branch
	if user.Role.Name == model.RoleStoreManager {
		brID, err := h.store.GetBranchID(id)
		if err != nil {
			if err == sql.ErrNoRows {
				return httperr.NotFound(c, "Salesperson not found")
			}
			log.Println("get salesperson branch error:", err)
			return httperr.Internal(c)
		}
		if brID == nil || user.BranchID == nil || *brID != *user.BranchID {
			return c.Status(403).JSON(fiber.Map{"error": "You can only edit salespersons in your branch"})
		}
	}

	var in UpdateSalesPersonInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	// StoreManager cannot change branch_id
	if user.Role.Name == model.RoleStoreManager {
		in.BranchID = nil
	}

	if err := h.store.Update(id, in); err != nil {
		log.Println("update salesperson error:", err)
		return httperr.Internal(c)
	}

	result, err := h.store.GetByID(id, SalesFilter{})
	if err != nil {
		log.Println("fetch salesperson after update error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}

// Activate handles PATCH /salespersons/:id/activate
func (h *Handler) Activate(c *fiber.Ctx) error {
	return h.setActive(c, true)
}

// Deactivate handles PATCH /salespersons/:id/deactivate
func (h *Handler) Deactivate(c *fiber.Ctx) error {
	return h.setActive(c, false)
}

func (h *Handler) setActive(c *fiber.Ctx, active bool) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)

	// StoreManager can only manage salespersons in their branch
	if user.Role.Name == model.RoleStoreManager {
		brID, err := h.store.GetBranchID(id)
		if err != nil {
			if err == sql.ErrNoRows {
				return httperr.NotFound(c, "Salesperson not found")
			}
			log.Println("get salesperson branch error:", err)
			return httperr.Internal(c)
		}
		if brID == nil || user.BranchID == nil || *brID != *user.BranchID {
			return c.Status(403).JSON(fiber.Map{"error": "You can only manage salespersons in your branch"})
		}
	}

	if err := h.store.SetActive(id, active); err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Salesperson not found")
		}
		log.Println("set salesperson active error:", err)
		return httperr.Internal(c)
	}

	action := "activated"
	if !active {
		action = "deactivated"
	}
	return c.JSON(fiber.Map{"message": "Salesperson " + action})
}
