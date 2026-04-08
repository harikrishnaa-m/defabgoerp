package joborder

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

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
	var in CreateJobOrderInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.CustomerID == "" && in.CustomerPhone == "" {
		return httperr.BadRequest(c, "customer_id or customer_phone is required")
	}
	if in.JobType == "" {
		return httperr.BadRequest(c, "job_type is required")
	}

	user := c.Locals("user").(*model.User)
	branchID := ""
	if user.BranchID != nil {
		branchID = *user.BranchID
	}

	// Resolve warehouse from branch
	warehouseID := ""
	if branchID != "" {
		_ = h.store.db.QueryRow(`SELECT warehouse_id FROM branches WHERE id = $1`, branchID).Scan(&warehouseID)
	}

	id, err := h.store.CreateJobOrder(in, user.ID.String(), branchID, warehouseID)
	if err != nil {
		log.Println("create job order error:", err)
		return httperr.Internal(c)
	}

	result, err := h.store.GetByID(id)
	if err != nil {
		log.Println("fetch created job order error:", err)
		return c.Status(http.StatusCreated).JSON(fiber.Map{"job_order_id": id})
	}
	return c.Status(http.StatusCreated).JSON(result)
}

func (h *Handler) List(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	status := c.Query("status")
	jobType := c.Query("job_type")
	search := c.Query("search")

	user := c.Locals("user").(*model.User)
	var branchID *string
	if user.Role.Name == model.RoleStoreManager || user.Role.Name == model.RoleSalesPerson {
		branchID = user.BranchID
	} else if bid := c.Query("branch_id"); bid != "" {
		branchID = &bid
	}

	list, total, err := h.store.List(branchID, status, jobType, search, limit, offset)
	if err != nil {
		log.Println("list job orders error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"job_orders": list, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	result, err := h.store.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Job order not found")
		}
		log.Println("get job order error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(result)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var in UpdateJobOrderInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if err := h.store.UpdateJobOrder(id, in); err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Job order not found")
		}
		log.Println("update job order error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "updated"})
}

func (h *Handler) PushStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var in StatusUpdateInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.Status == "" {
		return httperr.BadRequest(c, "status is required")
	}
	user := c.Locals("user").(*model.User)
	if err := h.store.PushStatus(id, in, user.ID.String()); err != nil {
		log.Println("push job status error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "status updated"})
}

func (h *Handler) AddPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	var in PaymentInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.Amount <= 0 || in.PaymentMethod == "" {
		return httperr.BadRequest(c, "amount and payment_method are required")
	}
	if err := h.store.AddPayment(id, in); err != nil {
		if strings.Contains(err.Error(), "exceeds balance due") {
			return httperr.BadRequest(c, err.Error())
		}
		log.Println("add job payment error:", err)
		return httperr.Internal(c)
	}
	return c.Status(http.StatusCreated).JSON(fiber.Map{"message": "payment recorded"})
}

func (h *Handler) Cancel(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)
	if err := h.store.Cancel(id, user.ID.String()); err != nil {
		log.Println("cancel job order error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "cancelled"})
}
