package stockrequest

import (
	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"
	"log"
	"math"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

//  create stock request

func (h *Handler) Create(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	var in struct {
		FromWarehouseID string  `json:"from_warehouse_id"`
		ToWarehouseID   string  `json:"to_warehouse_id"`
		Priority        string  `json:"priority"`
		ExpectedDate    *string `json:"expected_date"`
		Items           []struct {
			VariantID string `json:"variant_id"`
			Qty       int    `json:"qty"`
		} `json:"items"`
	}

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid payload")
	}

	// 🔍 VALIDATIONS
	if in.FromWarehouseID == "" || in.ToWarehouseID == "" {
		return httperr.BadRequest(c, "warehouse ids required")
	}

	if len(in.Items) == 0 {
		return httperr.BadRequest(c, "at least one item required")
	}

	log.Println("STOCK REQUEST PAYLOAD OK")
	log.Println("FROM:", in.FromWarehouseID)
	log.Println("TO:", in.ToWarehouseID)
	log.Println("USER:", user.ID)

	// 🔍 STEP 1: CREATE REQUEST
	reqID, err := h.store.CreateRequest(
		in.FromWarehouseID,
		in.ToWarehouseID,
		user.ID.String(),
		in.Priority,
		in.ExpectedDate,
	)
	if err != nil {
		log.Println("❌ CreateRequest failed:", err)
		return httperr.Internal(c)
	}

	log.Println("✅ Stock request created:", reqID)

	// 🔍 STEP 2: ADD ITEMS
	for _, it := range in.Items {
		log.Println("ADDING ITEM:", it.VariantID, "QTY:", it.Qty)

		if err := h.store.AddItem(reqID, it.VariantID, it.Qty); err != nil {
			log.Println("❌ AddItem failed:", err)
			return httperr.Internal(c)
		}
	}

	log.Println("✅ All items added")

	return c.Status(201).JSON(fiber.Map{
		"id":      reqID,
		"message": "Stock request created",
	})
}

// LIST STOCK REQUESTS
func (h *Handler) List(c *fiber.Ctx) error {
	// 🔹 Query params
	status := c.Query("status")
	fromDate := c.Query("from_date")
	toDate := c.Query("to_date")

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	offset := (page - 1) * limit

	// 🔹 Convert to nullable params
	var statusPtr, fromPtr, toPtr *string

	if status != "" {
		statusPtr = &status
	}
	if fromDate != "" {
		fromPtr = &fromDate
	}
	if toDate != "" {
		toPtr = &toDate
	}

	// 🔹 Get total count
	total, err := h.store.CountFiltered(
		statusPtr,
		fromPtr,
		toPtr,
	)
	if err != nil {
		return httperr.Internal(c)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	// 🔹 Get paginated data
	rows, err := h.store.ListFiltered(
		statusPtr,
		fromPtr,
		toPtr,
		limit,
		offset,
	)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var data []fiber.Map

	for rows.Next() {
		var id, status, priority string
		var fromWhID, fromWhName, toWhID, toWhName string
		var requestedByID, requestedByName, created string

		if err := rows.Scan(
			&id,
			&status,
			&priority,
			&fromWhID, &fromWhName,
			&toWhID, &toWhName,
			&requestedByID, &requestedByName,
			&created,
		); err != nil {
			return httperr.Internal(c)
		}

		data = append(data, fiber.Map{
			"id":                  id,
			"status":              status,
			"priority":            priority,
			"from_warehouse_id":   fromWhID,
			"from_warehouse_name": fromWhName,
			"to_warehouse_id":     toWhID,
			"to_warehouse_name":   toWhName,
			"requested_by":        requestedByID,
			"requested_by_name":   requestedByName,
			"created_at":          created,
		})
	}

	// 🔹 Final response
	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
		"data":        data,
	})
}

// APPROVE / PARTIAL / REJECT REQUEST
func (h *Handler) Approve(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	id := c.Params("id")

	var in struct {
		Status  string `json:"status"` // APPROVED / PARTIAL / REJECTED
		Remarks string `json:"remarks"`
	}

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid payload")
	}

	status := strings.ToUpper(in.Status)

	switch status {
	case "APPROVED", "PARTIAL", "REJECTED", "CANCELLED":
		// valid
	default:
		return httperr.BadRequest(c, "invalid status value")
	}

	if err := h.store.UpdateStatus(
		id,
		status,
		user.ID.String(),
		in.Remarks,
	); err != nil {

		if strings.Contains(err.Error(), "closed") {
			return httperr.BadRequest(c, err.Error())
		}

		if strings.Contains(err.Error(), "invalid status") {
			return httperr.BadRequest(c, err.Error())
		}

		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"message": "Stock request updated successfully",
	})
}

// func (h *Handler) Dispatch(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*model.User)
// 	requestID := c.Params("id")

// 	var in struct {
//     Items   []DispatchItem `json:"items"`
//     Remarks string         `json:"remarks"`
// }

// 	if err := c.BodyParser(&in); err != nil {
// 		return httperr.BadRequest(c, "Invalid payload")
// 	}

// 	if len(in.Items) == 0 {
// 		return httperr.BadRequest(c, "No items to dispatch")
// 	}

// 	// get central warehouse id
// 	fromWarehouseID, err := h.store.GetFromWarehouse(requestID)
// 	if err != nil {
// 		return httperr.Internal(c)
// 	}

// 	err = h.store.Dispatch(
// 		requestID,
// 		fromWarehouseID,
// 		user.ID.String(),
// 		in.Items,
// 		in.Remarks,
// 	)
// 	if err != nil {
// 		return httperr.BadRequest(c, err.Error())
// 	}

// 	return c.JSON(fiber.Map{
// 		"message": "Stock dispatched successfully",
// 	})
// }

func (h *Handler) Dispatch(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	requestID := c.Params("id")

	if requestID == "" {
		return httperr.BadRequest(c, "request id is required")
	}

	var in struct {
		FromWarehouseID string `json:"from_warehouse_id"`
		Items           []struct {
			VariantID string `json:"variant_id"`
			Qty       int    `json:"dispatch_qty"`
		} `json:"items"`
		Remarks string `json:"remarks"`
	}

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid payload")
	}

	if in.FromWarehouseID == "" {
		return httperr.BadRequest(c, "from_warehouse_id is required")
	}

	if len(in.Items) == 0 {
		return httperr.BadRequest(c, "No items to dispatch")
	}

	items := make([]DispatchItem, 0)

	for _, it := range in.Items {
		if it.VariantID == "" {
			return httperr.BadRequest(c, "variant_id is required")
		}
		if it.Qty <= 0 {
			return httperr.BadRequest(c, "dispatch_qty must be greater than 0")
		}

		items = append(items, DispatchItem{
			VariantID: it.VariantID,
			Qty:       it.Qty,
		})
	}

	if err := h.store.Dispatch(
		requestID,
		in.FromWarehouseID,
		user.ID.String(),
		items,
		in.Remarks,
	); err != nil {
		return httperr.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"message": "Stock dispatched successfully",
	})
}

// GET /:id — single stock request detail
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return httperr.BadRequest(c, "id is required")
	}

	result, err := h.store.GetByID(id)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return c.Status(404).JSON(fiber.Map{"error": "stock request not found"})
		}
		return httperr.Internal(c)
	}

	return c.JSON(result)
}

// DELETE /:id — cancel a stock request
func (h *Handler) Cancel(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	id := c.Params("id")
	if id == "" {
		return httperr.BadRequest(c, "id is required")
	}

	if err := h.store.UpdateStatus(id, "CANCELLED", user.ID.String(), "Cancelled by user"); err != nil {
		if strings.Contains(err.Error(), "closed") {
			return httperr.BadRequest(c, err.Error())
		}
		if strings.Contains(err.Error(), "invalid status") {
			return httperr.BadRequest(c, err.Error())
		}
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{"message": "Stock request cancelled"})
}
