package employee

import (
	"database/sql"
	"log"
	"net/http"

	"defab-erp/internal/auth"
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

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateEmployeeInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.Name == "" || in.Email == "" || in.Password == "" {
		return httperr.BadRequest(c, "name, email and password are required")
	}

	user := c.Locals("user").(*model.User)

	// StoreManager can only create employees in their own branch
	if user.Role.Name == model.RoleStoreManager {
		if user.BranchID == nil {
			return httperr.BadRequest(c, "your account has no branch assigned")
		}
		in.BranchID = user.BranchID
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		log.Println("hash password error:", err)
		return httperr.Internal(c)
	}

	result, err := h.store.Create(in, hash)
	if err != nil {
		log.Println("create employee error:", err)
		return httperr.Internal(c)
	}
	return c.Status(http.StatusCreated).JSON(result)
}

func (h *Handler) List(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	search := c.Query("search")

	user := c.Locals("user").(*model.User)
	var branchID *string
	if user.Role.Name == model.RoleStoreManager {
		if user.BranchID != nil {
			branchID = user.BranchID
		}
	} else if bid := c.Query("branch_id"); bid != "" {
		branchID = &bid
	}

	list, total, err := h.store.List(branchID, search, limit, offset)
	if err != nil {
		log.Println("list employees error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"employees": list, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	result, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Employee not found")
		}
		log.Println("get employee error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(result)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var in UpdateEmployeeInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	user := c.Locals("user").(*model.User)

	// StoreManager can only update employees in their branch
	if user.Role.Name == model.RoleStoreManager {
		if user.BranchID == nil {
			return httperr.BadRequest(c, "your account has no branch assigned")
		}
		// Force branch to own branch
		in.BranchID = user.BranchID
	}

	if err := h.store.Update(id, in); err != nil {
		log.Println("update employee error:", err)
		return httperr.Internal(c)
	}

	result, _ := h.store.GetByID(id)
	return c.JSON(result)
}
