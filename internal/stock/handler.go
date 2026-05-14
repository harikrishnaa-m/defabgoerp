package stock

import (
	"database/sql"
	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/shopspring/decimal"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// POST /stocks
func (h *Handler) Create(c *fiber.Ctx) error {
	var in StockCreateInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if in.VariantID == "" || in.WarehouseID == "" || in.Quantity.IsZero() {
		return c.Status(400).SendString("variant_id, warehouse_id, quantity required")
	}
	if in.StockType == "" {
		in.StockType = "PRODUCT"
	}

	id, err := h.store.Create(in)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"message": "stock created", "id": id})
}
func (h *Handler) ByWarehouse(c *fiber.Ctx) error {
	warehouseID := c.Params("id")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	total, err := h.store.CountByWarehouse(warehouseID)
	if err != nil {
		return httperr.Internal(c)
	}

	rows, err := h.store.ListByWarehouse(warehouseID, limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var variantID, product, variant, warehouse string
		var variantCode int
		var qty decimal.Decimal

		rows.Scan(&variantID, &variantCode, &product, &variant, &warehouse, &qty)

		out = append(out, fiber.Map{
			"variant_id":   variantID,
			"variant_code": variantCode,
			"product":      product,
			"variant":      variant,
			"warehouse":    warehouse,
			"quantity":     qty,
		})
	}

	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		"data":        out,
	})
}

// GET /stocks/branch/:id
func (h *Handler) ByBranch(c *fiber.Ctx) error {
	branchID := c.Params("id")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	total, err := h.store.CountByBranch(branchID)
	if err != nil {
		return httperr.Internal(c)
	}

	rows, err := h.store.ListByBranch(branchID, limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map
	for rows.Next() {
		var productID, productName, variantID, variantName, warehouseID, warehouseName string
		var variantCode int
		var qty decimal.Decimal
		if err := rows.Scan(&productID, &productName, &variantID, &variantCode, &variantName, &warehouseID, &warehouseName, &qty); err != nil {
			return httperr.Internal(c)
		}
		out = append(out, fiber.Map{
			"product_id":     productID,
			"product_name":   productName,
			"variant_id":     variantID,
			"variant_code":   variantCode,
			"variant_name":   variantName,
			"warehouse_id":   warehouseID,
			"warehouse_name": warehouseName,
			"quantity":       qty,
		})
	}
	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		"data":        out,
	})
}

// PATCH /stocks/:id — raw update (backward compat)
func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var in StockUpdateInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}
	if in.VariantID == "" || in.WarehouseID == "" || in.Quantity.IsZero() {
		return c.Status(400).SendString("variant_id, warehouse_id, quantity required")
	}
	err := h.store.Update(id, in)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"message": "stock updated", "id": id})
}

// POST /stocks/:id/adjust — audited stock adjustment
func (h *Handler) Adjust(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	id := c.Params("id")

	var in StockAdjustInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "invalid payload")
	}
	if in.NewQuantity.IsNegative() {
		return httperr.BadRequest(c, "new_quantity cannot be negative")
	}
	if in.Reason == "" {
		return httperr.BadRequest(c, "reason is required")
	}

	_ = user // userID available for future per-user audit
	if err := h.store.Adjust(id, in.NewQuantity, in.Reason, user.ID.String()); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(404).JSON(fiber.Map{"error": "stock not found"})
		}
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{"message": "stock adjusted", "id": id})
}

// GET /stocks/:id — single stock detail
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	row, _ := h.store.GetByID(id)

	var (
		stockID, variantID, variantName, sku string
		variantCode                          int
		productID, productName               string
		warehouseID, warehouseName           string
		qty                                  decimal.Decimal
		stockType                            string
		updatedAt                            time.Time
	)

	if err := row.Scan(
		&stockID, &variantID, &variantCode, &variantName, &sku,
		&productID, &productName,
		&warehouseID, &warehouseName,
		&qty, &stockType, &updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "stock not found"})
		}
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"id":             stockID,
		"variant_id":     variantID,
		"variant_code":   variantCode,
		"variant_name":   variantName,
		"sku":            sku,
		"product_id":     productID,
		"product_name":   productName,
		"warehouse_id":   warehouseID,
		"warehouse_name": warehouseName,
		"quantity":       qty,
		"stock_type":     stockType,
		"updated_at":     updatedAt,
	})
}

// DELETE /stocks/:id
func (h *Handler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.store.Delete(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(404).JSON(fiber.Map{"error": "stock not found"})
		}
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "stock deleted"})
}

// GET /stocks/variant/:id
func (h *Handler) ByVariant(c *fiber.Ctx) error {
	variantID := c.Params("id")

	rows, err := h.store.ListByVariant(variantID)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var warehouse string
		var qty decimal.Decimal
		rows.Scan(&warehouse, &qty)

		out = append(out, fiber.Map{
			"warehouse": warehouse,
			"quantity":  qty,
		})
	}

	return c.JSON(out)
}

// GET /stocks/low
func (h *Handler) LowStock(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	t := c.Query("threshold", "10")
	threshold, _ := strconv.Atoi(t)

	var rows *sql.Rows
	var err error

	if user.Role.Name == model.RoleSuperAdmin {
		rows, err = h.store.LowStock(threshold)
	} else {
		if user.BranchID == nil {
			return c.JSON([]fiber.Map{})
		}
		rows, err = h.store.LowStockByBranch(threshold, *user.BranchID)
	}
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var product, variant, warehouse string
		var variantCode int
		var qty decimal.Decimal
		var hsnCode string

		rows.Scan(&product, &variant, &variantCode, &warehouse, &qty, &hsnCode)

		out = append(out, fiber.Map{
			"product":      product,
			"variant":      variant,
			"variant_code": variantCode,
			"warehouse":    warehouse,
			"quantity":     qty,
			"hsn_code":     hsnCode,
		})
	}

	return c.JSON(out)
}

// gat all stocks

func (h *Handler) All(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	limitStr := c.Query("limit")
	pageStr := c.Query("page")

	page := 1
	limit := 0
	offset := 0

	if limitStr != "" || pageStr != "" {
		page = c.QueryInt("page", 1)
		limit = c.QueryInt("limit", 20)
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 20
		}
		offset = (page - 1) * limit
	}

	variantCode := c.Query("variant_code")

	var total int
	var rows *sql.Rows
	var err error

	if user.Role.Name == model.RoleSuperAdmin {
		total, err = h.store.CountAll(variantCode)
		if err != nil {
			return httperr.Internal(c)
		}
		rows, err = h.store.GetAll(variantCode, limit, offset)
	} else {
		if user.BranchID == nil {
			return c.JSON(fiber.Map{"page": page, "limit": limit, "total": 0, "total_pages": 0, "data": []fiber.Map{}})
		}
		total, err = h.store.CountAllByBranch(*user.BranchID, variantCode)
		if err != nil {
			return httperr.Internal(c)
		}
		rows, err = h.store.GetAllByBranch(*user.BranchID, variantCode, limit, offset)
	}
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var data []fiber.Map

	for rows.Next() {
		var (
			stockID     string
			pid, pname  string
			vid         string
			variantCode int
			vname, sku  string
			wid, wname  string
			qty         decimal.Decimal
			hsnCode     string
		)

		if err := rows.Scan(
			&stockID,
			&pid, &pname,
			&vid, &variantCode, &vname, &sku,
			&wid, &wname,
			&qty,
			&hsnCode,
		); err != nil {
			return httperr.Internal(c)
		}

		data = append(data, fiber.Map{
			"id":        stockID,
			"product":   fiber.Map{"id": pid, "name": pname},
			"variant":   fiber.Map{"id": vid, "variant_code": variantCode, "name": vname, "sku": sku, "hsn_code": hsnCode},
			"warehouse": fiber.Map{"id": wid, "name": wname},
			"quantity":  qty,
		})
	}

	totalPages := 1
	if limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}

	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
		"data":        data,
	})
}

// GET /stocks/available — stocks outside the user's branch (central + other branches)
func (h *Handler) Available(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	// SuperAdmin sees everything — use All instead
	if user.Role.Name == model.RoleSuperAdmin || user.BranchID == nil {
		return h.All(c)
	}

	total, err := h.store.CountAvailable(*user.BranchID)
	if err != nil {
		return httperr.Internal(c)
	}

	rows, err := h.store.GetAvailable(*user.BranchID, limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var data []fiber.Map
	for rows.Next() {
		var (
			stockID     string
			pid, pname  string
			vid         string
			variantCode int
			vname, sku  string
			wid, wname  string
			qty         decimal.Decimal
		)
		if err := rows.Scan(
			&stockID,
			&pid, &pname,
			&vid, &variantCode, &vname, &sku,
			&wid, &wname,
			&qty,
		); err != nil {
			return httperr.Internal(c)
		}
		data = append(data, fiber.Map{
			"id":        stockID,
			"product":   fiber.Map{"id": pid, "name": pname},
			"variant":   fiber.Map{"id": vid, "variant_code": variantCode, "name": vname, "sku": sku},
			"warehouse": fiber.Map{"id": wid, "name": wname},
			"quantity":  qty,
		})
	}

	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		"data":        data,
	})
}

func (h *Handler) AvailableNew(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	if user.BranchID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user has no branch assigned"})
	}

	total, err := h.store.CountAvailableNew(*user.BranchID)
	if err != nil {
		return httperr.Internal(c)
	}

	rows, err := h.store.GetAvailableNew(*user.BranchID, limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var data []fiber.Map
	for rows.Next() {
		var (
			stockID     string
			pid, pname  string
			vid         string
			variantCode int
			vname, sku  string
			wid, wname  string
			qty         decimal.Decimal
		)
		if err := rows.Scan(
			&stockID,
			&pid, &pname,
			&vid, &variantCode, &vname, &sku,
			&wid, &wname,
			&qty,
		); err != nil {
			return httperr.Internal(c)
		}
		data = append(data, fiber.Map{
			"id":        stockID,
			"product":   fiber.Map{"id": pid, "name": pname},
			"variant":   fiber.Map{"id": vid, "variant_code": variantCode, "name": vname, "sku": sku},
			"warehouse": fiber.Map{"id": wid, "name": wname},
			"quantity":  qty,
		})
	}

	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		"data":        data,
	})
}

func (h *Handler) ByProduct(c *fiber.Ctx) error {
	productID := c.Params("id")

	rows, err := h.store.GetByProduct(productID)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, name, sku string
		var variantCode int
		var qty decimal.Decimal

		if err := rows.Scan(&id, &variantCode, &name, &sku, &qty); err != nil {
			return httperr.Internal(c)
		}

		out = append(out, fiber.Map{
			"id":           id,
			"variant_code": variantCode,
			"name":         name,
			"sku":          sku,
			"total_qty":    qty,
		})
	}

	return c.JSON(fiber.Map{
		"product_id": productID,
		"variants":   out,
	})
}

func nullOrValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func (h *Handler) Movements(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)

	// StoreManagers see only their branch movements (branch from token)
	if user.Role.Name != model.RoleSuperAdmin && user.BranchID != nil {
		return h.movementsByBranchID(c, *user.BranchID)
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	// Optional filters
	var variantID, warehouseID, movementType, fromDate, toDate *string
	if v := c.Query("variant_id"); v != "" {
		variantID = &v
	}
	if v := c.Query("warehouse_id"); v != "" {
		warehouseID = &v
	}
	if v := c.Query("type"); v != "" {
		up := strings.ToUpper(v)
		movementType = &up
	}
	if v := c.Query("from_date"); v != "" {
		fromDate = &v
	}
	if v := c.Query("to_date"); v != "" {
		toDate = &v
	}

	total, err := h.store.CountMovements(variantID, warehouseID, movementType, fromDate, toDate)
	if err != nil {
		return httperr.Internal(c)
	}

	rows, err := h.store.GetMovements(variantID, warehouseID, movementType, fromDate, toDate, limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var (
			id                                     string
			varID, varName                         string
			variantCode                            int
			pid, pname                             string
			movement                               string
			qty                                    decimal.Decimal
			fromWhID, fromWhName, toWhID, toWhName sql.NullString
			reference, status                      string
			created                                time.Time
		)

		if err := rows.Scan(
			&id,
			&varID, &variantCode, &varName,
			&pid, &pname,
			&movement,
			&qty,
			&fromWhID, &fromWhName,
			&toWhID, &toWhName,
			&reference, &status,
			&created,
		); err != nil {
			return httperr.Internal(c)
		}

		out = append(out, fiber.Map{
			"id":                  id,
			"variant_id":          varID,
			"variant_code":        variantCode,
			"variant_name":        varName,
			"product_id":          pid,
			"product_name":        pname,
			"type":                movement,
			"quantity":            qty,
			"from_warehouse_id":   nullOrValue(fromWhID),
			"from_warehouse_name": nullOrValue(fromWhName),
			"to_warehouse_id":     nullOrValue(toWhID),
			"to_warehouse_name":   nullOrValue(toWhName),
			"reference":           reference,
			"status":              status,
			"created_at":          created,
		})
	}

	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		"data":        out,
	})
}

func (h *Handler) MovementByID(c *fiber.Ctx) error {
	id := c.Params("id")

	var (
		movID                    string
		vid, vname, vsku         string
		variantCode              int
		pid, pname               string
		movType                  string
		qty                      decimal.Decimal
		fromWhID, fromWhName     sql.NullString
		toWhID, toWhName         sql.NullString
		reference, status        string
		stockReqID, poID, suppID string
		saleOrderID              string
		createdAt, updatedAt     time.Time
	)

	err := h.store.GetMovementByID(id).Scan(
		&movID,
		&vid, &variantCode, &vname, &vsku,
		&pid, &pname,
		&movType,
		&qty,
		&fromWhID, &fromWhName,
		&toWhID, &toWhName,
		&reference,
		&status,
		&stockReqID, &poID, &suppID, &saleOrderID,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "movement not found"})
	}
	if err != nil {
		return httperr.Internal(c)
	}

	result := fiber.Map{
		"id":       movID,
		"variant":  fiber.Map{"id": vid, "variant_code": variantCode, "name": vname, "sku": vsku},
		"product":  fiber.Map{"id": pid, "name": pname},
		"type":     movType,
		"quantity": qty,
		"from_warehouse": fiber.Map{
			"id":   nullOrValue(fromWhID),
			"name": nullOrValue(fromWhName),
		},
		"to_warehouse": fiber.Map{
			"id":   nullOrValue(toWhID),
			"name": nullOrValue(toWhName),
		},
		"reference":  reference,
		"status":     status,
		"created_at": createdAt,
		"updated_at": updatedAt,
	}

	if stockReqID != "" {
		result["stock_request_id"] = stockReqID
	}
	if poID != "" {
		result["purchase_order_id"] = poID
	}
	if suppID != "" {
		result["supplier_id"] = suppID
	}
	if saleOrderID != "" {
		result["sale_order_id"] = saleOrderID
	}

	return c.JSON(result)
}

func (h *Handler) MovementsByBranch(c *fiber.Ctx) error {
	branchID := c.Query("branch_id")
	if branchID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "branch_id query param is required"})
	}
	return h.movementsByBranchID(c, branchID)
}

func (h *Handler) movementsByBranchID(c *fiber.Ctx, branchID string) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	var movementType, fromDate, toDate *string
	if v := c.Query("type"); v != "" {
		up := strings.ToUpper(v)
		movementType = &up
	}
	if v := c.Query("from_date"); v != "" {
		fromDate = &v
	}
	if v := c.Query("to_date"); v != "" {
		toDate = &v
	}

	total, err := h.store.CountMovementsByBranch(branchID, movementType, fromDate, toDate)
	if err != nil {
		return httperr.Internal(c)
	}

	rows, err := h.store.GetMovementsByBranch(branchID, movementType, fromDate, toDate, limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map
	for rows.Next() {
		var (
			id                                     string
			varID, varName                         string
			variantCode                            int
			pid, pname                             string
			movement                               string
			qty                                    decimal.Decimal
			fromWhID, fromWhName, toWhID, toWhName sql.NullString
			reference, status                      string
			created                                time.Time
		)

		if err := rows.Scan(
			&id,
			&varID, &variantCode, &varName,
			&pid, &pname,
			&movement,
			&qty,
			&fromWhID, &fromWhName,
			&toWhID, &toWhName,
			&reference, &status,
			&created,
		); err != nil {
			return httperr.Internal(c)
		}
		out = append(out, fiber.Map{
			"id":                  id,
			"variant_id":          varID,
			"variant_code":        variantCode,
			"variant_name":        varName,
			"product_id":          pid,
			"product_name":        pname,
			"type":                movement,
			"quantity":            qty,
			"from_warehouse_id":   nullOrValue(fromWhID),
			"from_warehouse_name": nullOrValue(fromWhName),
			"to_warehouse_id":     nullOrValue(toWhID),
			"to_warehouse_name":   nullOrValue(toWhName),
			"reference":           reference,
			"status":              status,
			"created_at":          created,
		})
	}

	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		"data":        out,
	})
}

func (h *Handler) ByWarehouseProductSummary(c *fiber.Ctx) error {
	warehouseID := c.Params("id")

	rows, err := h.store.GetWarehouseProductSummary(warehouseID)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, name string
		var qty decimal.Decimal

		if err := rows.Scan(&id, &name, &qty); err != nil {
			return httperr.Internal(c)
		}

		out = append(out, fiber.Map{
			"id":        id,
			"name":      name,
			"total_qty": qty,
		})
	}

	return c.JSON(out)
}

// POST /stocks/quick-add
// Creates a product (if needed), a variant, and stock in one transaction.
func (h *Handler) QuickAdd(c *fiber.Ctx) error {
	var in QuickAddInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "invalid request body")
	}

	if in.VariantCode == 0 {
		return httperr.BadRequest(c, "variant_code is required")
	}
	if in.VariantName == "" {
		return httperr.BadRequest(c, "variant_name is required")
	}
	if in.Price <= 0 {
		return httperr.BadRequest(c, "price is required")
	}
	if in.WarehouseID == "" {
		return httperr.BadRequest(c, "warehouse_id is required")
	}
	if in.Quantity.IsZero() {
		return httperr.BadRequest(c, "quantity is required")
	}
	if in.ProductID == "" && (in.ProductName == "" || in.CategoryID == "") {
		return httperr.BadRequest(c, "provide product_id or product_name + category_id")
	}

	result, err := h.store.QuickAdd(in)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message":      "product, variant and stock created",
		"product_id":   result.ProductID,
		"variant_id":   result.VariantID,
		"variant_code": result.VariantCode,
		"stock_id":     result.StockID,
	})
}
