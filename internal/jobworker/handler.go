package jobworker

import (
	"database/sql"
	"log"
	"net/http"

	"defab-erp/internal/core/httperr"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateJobOrderWorkerInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.Name == "" {
		return httperr.BadRequest(c, "name is required")
	}
	if !IsValidRole(in.Role) {
		return httperr.BadRequest(c, "role must be one of DESIGNER, CUTTER, STITCHER, HAND_WORKER, or omitted")
	}

	w, err := h.store.Create(in)
	if err != nil {
		log.Println("create job order worker error:", err)
		return httperr.Internal(c)
	}
	return c.Status(http.StatusCreated).JSON(w)
}

func (h *Handler) List(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 0)
	offset := c.QueryInt("offset", 0)
	role := c.Query("role")
	search := c.Query("search")
	activeOnly := c.QueryBool("active_only", false)

	list, total, err := h.store.List(role, search, activeOnly, limit, offset)
	if err != nil {
		log.Println("list job order workers error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"workers": list, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	w, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Worker not found")
		}
		log.Println("get job order worker error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(w)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var in UpdateJobOrderWorkerInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.Role != nil && !IsValidRole(*in.Role) {
		return httperr.BadRequest(c, "role must be one of DESIGNER, CUTTER, STITCHER, HAND_WORKER, or omitted")
	}

	if err := h.store.Update(id, in); err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Worker not found")
		}
		log.Println("update job order worker error:", err)
		return httperr.Internal(c)
	}

	w, err := h.store.GetByID(id)
	if err != nil {
		return c.JSON(fiber.Map{"message": "updated"})
	}
	return c.JSON(w)
}
