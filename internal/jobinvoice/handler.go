package jobinvoice

import (
	"database/sql"
	"log"
	"net/http"

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

func (h *Handler) List(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	search := c.Query("search")

	user := c.Locals("user").(*model.User)
	var branchID *string
	if user.Role.Name == model.RoleStoreManager || user.Role.Name == model.RoleSalesPerson {
		if user.BranchID != nil {
			branchID = user.BranchID
		}
	} else if bid := c.Query("branch_id"); bid != "" {
		branchID = &bid
	}

	list, total, err := h.store.List(branchID, search, limit, offset)
	if err != nil {
		log.Println("list job invoices error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"job_invoices": list, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	result, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Job invoice not found")
		}
		log.Println("get job invoice error:", err)
		return httperr.Internal(c)
	}
	return c.Status(http.StatusOK).JSON(result)
}

func (h *Handler) Backfill(c *fiber.Ctx) error {
	count, err := h.store.Backfill()
	if err != nil {
		log.Println("backfill job invoices error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "backfill complete", "invoices_created": count})
}
