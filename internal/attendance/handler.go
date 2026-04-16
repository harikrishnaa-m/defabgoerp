package attendance

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

func (h *Handler) PunchIn(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	var in PunchInput
	c.BodyParser(&in)

	// Branch from request body, else user's default branch
	branchID := ""
	if in.BranchID != nil && *in.BranchID != "" {
		branchID = *in.BranchID
	} else if user.BranchID != nil {
		branchID = *user.BranchID
	}

	result, err := h.store.PunchIn(user.ID.String(), branchID, in.Notes)
	if err != nil {
		if err.Error() == "already punched in today" {
			return httperr.BadRequest(c, err.Error())
		}
		log.Println("punch in error:", err)
		return httperr.Internal(c)
	}
	return c.Status(201).JSON(result)
}

func (h *Handler) PunchOut(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	var in PunchInput
	c.BodyParser(&in)

	result, err := h.store.PunchOut(user.ID.String(), in.Notes)
	if err != nil {
		msg := err.Error()
		if msg == "no punch-in found for today" {
			return httperr.BadRequest(c, msg)
		}
		log.Println("punch out error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(result)
}

func (h *Handler) List(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	search := c.Query("search")

	user := c.Locals("user").(*model.User)

	var userID *string
	var branchID *string

	switch user.Role.Name {
	case model.RoleEmployee, model.RoleSalesPerson:
		// Employees and SalesPersons can only see their own status
		uid := user.ID.String()
		userID = &uid
	case model.RoleStoreManager:
		// StoreManagers see all users in their branch
		if user.BranchID != nil {
			branchID = user.BranchID
		}
	default:
		// SuperAdmin / AccountsManager — can filter by branch
		if bid := c.Query("branch_id"); bid != "" {
			branchID = &bid
		}
	}

	list, total, err := h.store.List(userID, branchID, search, limit, offset)
	if err != nil {
		log.Println("list attendance error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"attendance": list, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	targetUserID := c.Params("id")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	user := c.Locals("user").(*model.User)

	switch user.Role.Name {
	case model.RoleEmployee, model.RoleSalesPerson:
		// Can only view their own
		if targetUserID != user.ID.String() {
			return httperr.Forbidden(c, "you can only view your own attendance")
		}
	case model.RoleStoreManager:
		// Can view users in their branch — verify target is in same branch
		if user.BranchID != nil {
			var targetBranch sql.NullString
			h.store.db.QueryRow(`SELECT branch_id FROM users WHERE id = $1`, targetUserID).Scan(&targetBranch)
			if !targetBranch.Valid || targetBranch.String != *user.BranchID {
				return httperr.Forbidden(c, "user not in your branch")
			}
		}
	}

	result, err := h.store.GetByID(targetUserID, dateFrom, dateTo)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "user not found")
		}
		log.Println("get attendance by id error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(result)
}

// UploadExcel handles branch-manager Excel attendance uploads.
// POST /attendance/upload  (multipart: file + optional branch_id)
func (h *Handler) UploadExcel(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	// Only StoreManager and SuperAdmin may upload
	if user.Role.Name != model.RoleSuperAdmin && user.Role.Name != model.RoleStoreManager {
		return httperr.Forbidden(c, "only store managers and admins can upload attendance")
	}

	// Determine target branch
	var branchID string
	if user.Role.Name == model.RoleStoreManager {
		if user.BranchID == nil {
			return httperr.BadRequest(c, "your account has no branch assigned")
		}
		branchID = *user.BranchID
	} else {
		branchID = c.FormValue("branch_id")
		if branchID == "" {
			return httperr.BadRequest(c, "branch_id is required")
		}
	}

	// Read uploaded file
	fh, err := c.FormFile("file")
	if err != nil {
		return httperr.BadRequest(c, "file is required")
	}

	// Parse the biometric attendance Excel
	records, err := parseAttendanceExcel(fh)
	if err != nil {
		return httperr.BadRequest(c, err.Error())
	}
	if len(records) == 0 {
		return httperr.BadRequest(c, "no attendance records found in the file")
	}

	// Bulk-create attendance rows (and auto-create missing employees)
	result, err := h.store.BulkCreateFromUpload(records, branchID)
	if err != nil {
		log.Println("upload attendance error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}
