package billing

import (
	"database/sql"
	"log"
	"strconv"

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

// Create handles POST /billing — the main POS endpoint.
func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateBillInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.CustomerPhone == "" {
		return httperr.BadRequest(c, "customer_phone is required")
	}
	if in.CustomerName == "" {
		return httperr.BadRequest(c, "customer_name is required")
	}
	if in.WarehouseID == "" {
		return httperr.BadRequest(c, "warehouse_id is required")
	}
	if len(in.Items) == 0 {
		return httperr.BadRequest(c, "at least one item is required")
	}
	if len(in.Payments) == 0 {
		return httperr.BadRequest(c, "at least one payment is required")
	}

	for i, item := range in.Items {
		if item.VariantID == "" {
			return httperr.BadRequest(c, "items["+strconv.Itoa(i)+"].variant_id is required")
		}
		if item.Quantity <= 0 {
			return httperr.BadRequest(c, "items["+strconv.Itoa(i)+"].quantity must be > 0")
		}
		if item.UnitPrice <= 0 {
			return httperr.BadRequest(c, "items["+strconv.Itoa(i)+"].unit_price must be > 0")
		}
	}

	for i, p := range in.Payments {
		if p.Amount <= 0 {
			return httperr.BadRequest(c, "payments["+strconv.Itoa(i)+"].amount must be > 0")
		}
		if p.Method == "" {
			return httperr.BadRequest(c, "payments["+strconv.Itoa(i)+"].method is required")
		}
	}

	user := c.Locals("user").(*model.User)

	branchID := ""
	if user.BranchID != nil {
		branchID = *user.BranchID
	}

	result, err := h.store.CreateBill(in, user.ID.String(), branchID)
	if err != nil {
		log.Println("create bill error:", err)
		errMsg := err.Error()
		if len(errMsg) >= 18 && errMsg[:18] == "insufficient stock" {
			return httperr.BadRequest(c, errMsg)
		}
		return httperr.Internal(c)
	}

	return c.Status(201).JSON(result)
}

// GetByID handles GET /billing/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	result, err := h.store.GetByID(id)
	if err == sql.ErrNoRows {
		return httperr.NotFound(c, "Bill not found")
	}
	if err != nil {
		log.Println("get bill error:", err)
		return httperr.Internal(c)
	}

	// Branch check for StoreManager
	user := c.Locals("user").(*model.User)
	if user.Role.Name == model.RoleStoreManager {
		if user.BranchID != nil {
			billBranch, ok := result["branch_id"].(string)
			if ok && billBranch != *user.BranchID {
				return c.Status(403).JSON(fiber.Map{"error": "Access denied to this bill"})
			}
		}
	}

	return c.JSON(result)
}

// List handles GET /billing
func (h *Handler) List(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var branchID *string
	if user.Role.Name == model.RoleStoreManager || user.Role.Name == model.RoleSalesperson {
		branchID = user.BranchID
	}

	results, err := h.store.List(branchID, limit, offset)
	if err != nil {
		log.Println("list bills error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"bills":  results,
		"limit":  limit,
		"offset": offset,
	})
}
