package rawmaterial

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"defab-erp/internal/core/model"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) ListAll(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	hsnCode := c.Query("hsn_code")
	search := c.Query("search")

	var rows []RawMaterialStockRow
	var total int
	var err error

	// StoreManager sees only their branch
	if user.Role.Name != model.RoleSuperAdmin && user.BranchID != nil {
		rows, total, err = h.store.ListByBranch(*user.BranchID, hsnCode, search, limit, offset)
	} else {
		rows, total, err = h.store.ListAll(hsnCode, search, limit, offset)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	result := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		result = append(result, stockRowToMap(r))
	}
	return c.JSON(fiber.Map{"data": result, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) ListByWarehouse(c *fiber.Ctx) error {
	warehouseID := c.Params("warehouseId")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	rows, err := h.store.ListByWarehouse(warehouseID, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	result := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		result = append(result, stockRowToMap(r))
	}
	return c.JSON(fiber.Map{"data": result})
}

func (h *Handler) ListMovements(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	// StoreManager sees only their branch movements
	if user.Role.Name != model.RoleSuperAdmin && user.BranchID != nil {
		return h.listMovementsByBranchID(c, *user.BranchID)
	}

	stockID := c.Query("stock_id")
	itemName := c.Query("item_name")
	warehouseID := c.Query("warehouse_id")

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	var rows []RawMaterialMovementRow
	var err error

	if stockID != "" {
		rows, err = h.store.ListMovementsByStockID(stockID, limit, offset)
	} else if itemName != "" && warehouseID != "" {
		rows, err = h.store.ListMovements(itemName, warehouseID, limit, offset)
	} else {
		rows, err = h.store.ListAllMovements(limit, offset)
	}

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	result := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		m := fiber.Map{
			"id":             r.ID,
			"item_name":      r.ItemName,
			"warehouse_id":   r.WarehouseID,
			"warehouse_name": r.WarehouseName,
			"quantity":       r.Quantity,
			"movement_type":  r.MovementType,
			"created_at":     r.CreatedAt,
		}
		if r.GoodsReceiptID.Valid {
			m["goods_receipt_id"] = r.GoodsReceiptID.String
		}
		if r.GRNNumber.Valid {
			m["grn_number"] = r.GRNNumber.String
		}
		if r.PurchaseOrderID.Valid {
			m["purchase_order_id"] = r.PurchaseOrderID.String
		}
		if r.PONumber.Valid {
			m["po_number"] = r.PONumber.String
		}
		if r.Reference.Valid {
			m["reference"] = r.Reference.String
		}
		result = append(result, m)
	}
	return c.JSON(fiber.Map{"data": result})
}

func (h *Handler) listMovementsByBranchID(c *fiber.Ctx, branchID string) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	rows, err := h.store.ListMovementsByBranch(branchID, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": movementRowsToMaps(rows)})
}

func (h *Handler) MovementsByBranch(c *fiber.Ctx) error {
	branchID := c.Query("branch_id")
	if branchID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "branch_id query param is required"})
	}
	return h.listMovementsByBranchID(c, branchID)
}

func (h *Handler) StocksByBranch(c *fiber.Ctx) error {
	branchID := c.Query("branch_id")
	if branchID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "branch_id query param is required"})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	rows, total, err := h.store.ListByBranch(branchID, "", "", limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	result := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		result = append(result, stockRowToMap(r))
	}
	return c.JSON(fiber.Map{"data": result, "total": total, "limit": limit, "offset": offset})
}

func movementRowsToMaps(rows []RawMaterialMovementRow) []fiber.Map {
	result := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		m := fiber.Map{
			"id":             r.ID,
			"item_name":      r.ItemName,
			"warehouse_id":   r.WarehouseID,
			"warehouse_name": r.WarehouseName,
			"quantity":       r.Quantity,
			"movement_type":  r.MovementType,
			"created_at":     r.CreatedAt,
		}
		if r.GoodsReceiptID.Valid {
			m["goods_receipt_id"] = r.GoodsReceiptID.String
		}
		if r.GRNNumber.Valid {
			m["grn_number"] = r.GRNNumber.String
		}
		if r.PurchaseOrderID.Valid {
			m["purchase_order_id"] = r.PurchaseOrderID.String
		}
		if r.PONumber.Valid {
			m["po_number"] = r.PONumber.String
		}
		if r.Reference.Valid {
			m["reference"] = r.Reference.String
		}
		result = append(result, m)
	}
	return result
}

func stockRowToMap(r RawMaterialStockRow) fiber.Map {
	m := fiber.Map{
		"id":             r.ID,
		"item_name":      r.ItemName,
		"warehouse_id":   r.WarehouseID,
		"warehouse_name": r.WarehouseName,
		"quantity":       r.Quantity,
		"updated_at":     r.UpdatedAt,
	}
	if r.HSNCode.Valid {
		m["hsn_code"] = r.HSNCode.String
	}
	if r.Unit.Valid {
		m["unit"] = r.Unit.String
	}
	return m
}

func (h *Handler) MovementByID(c *fiber.Ctx) error {
	id := c.Params("id")

	r, err := h.store.GetMovementByID(id)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return c.Status(404).JSON(fiber.Map{"error": "movement not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	m := fiber.Map{
		"id":             r.ID,
		"item_name":      r.ItemName,
		"warehouse_id":   r.WarehouseID,
		"warehouse_name": r.WarehouseName,
		"quantity":       r.Quantity,
		"movement_type":  r.MovementType,
		"created_at":     r.CreatedAt,
	}
	if r.GoodsReceiptID.Valid {
		m["goods_receipt_id"] = r.GoodsReceiptID.String
	}
	if r.GRNNumber.Valid {
		m["grn_number"] = r.GRNNumber.String
	}
	if r.PurchaseOrderID.Valid {
		m["purchase_order_id"] = r.PurchaseOrderID.String
	}
	if r.PONumber.Valid {
		m["po_number"] = r.PONumber.String
	}
	if r.Reference.Valid {
		m["reference"] = r.Reference.String
	}
	return c.JSON(m)
}

func (h *Handler) AdjustStock(c *fiber.Ctx) error {
	var in AdjustStockInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON body"})
	}

	if in.StockID == "" || in.Quantity <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "stock_id and quantity (>0) are required"})
	}

	if in.Type != "OUT" && in.Type != "ADJUSTMENT" {
		return c.Status(400).JSON(fiber.Map{"error": "type must be OUT or ADJUSTMENT"})
	}

	if err := h.store.AdjustStock(in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Stock adjusted successfully"})
}
