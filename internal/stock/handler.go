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
		var qty decimal.Decimal

		rows.Scan(&variantID, &product, &variant, &warehouse, &qty)

		out = append(out, fiber.Map{
			"variant_id": variantID,
			"product":    product,
			"variant":    variant,
			"warehouse":  warehouse,
			"quantity":   qty,
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
		var qty decimal.Decimal
		if err := rows.Scan(&productID, &productName, &variantID, &variantName, &warehouseID, &warehouseName, &qty); err != nil {
			return httperr.Internal(c)
		}
		out = append(out, fiber.Map{
			"product_id":     productID,
			"product_name":   productName,
			"variant_id":     variantID,
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
		productID, productName               string
		warehouseID, warehouseName           string
		qty                                  decimal.Decimal
		stockType                            string
		updatedAt                            time.Time
	)

	if err := row.Scan(
		&stockID, &variantID, &variantName, &sku,
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
	t := c.Query("threshold", "10")
	threshold, _ := strconv.Atoi(t)

	rows, err := h.store.LowStock(threshold)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var product, variant, warehouse string
		var qty decimal.Decimal

		rows.Scan(&product, &variant, &warehouse, &qty)

		out = append(out, fiber.Map{
			"product":   product,
			"variant":   variant,
			"warehouse": warehouse,
			"quantity":  qty,
		})
	}

	return c.JSON(out)
}

// gat all stocks

func (h *Handler) All(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	total, err := h.store.CountAll()
	if err != nil {
		return httperr.Internal(c)
	}

	rows, err := h.store.GetAll(limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var data []fiber.Map

	for rows.Next() {
		var (
			pid, pname      string
			vid, vname, sku string
			wid, wname      string
			qty             decimal.Decimal
		)

		if err := rows.Scan(
			&pid, &pname,
			&vid, &vname, &sku,
			&wid, &wname,
			&qty,
		); err != nil {
			return httperr.Internal(c)
		}

		data = append(data, fiber.Map{
			"product":   fiber.Map{"id": pid, "name": pname},
			"variant":   fiber.Map{"id": vid, "name": vname, "sku": sku},
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
		var qty decimal.Decimal

		if err := rows.Scan(&id, &name, &sku, &qty); err != nil {
			return httperr.Internal(c)
		}

		out = append(out, fiber.Map{
			"id":        id,
			"name":      name,
			"sku":       sku,
			"total_qty": qty,
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
			movement                               string
			qty                                    decimal.Decimal
			fromWhID, fromWhName, toWhID, toWhName sql.NullString
			reference, status                      string
			created                                time.Time
		)

		if err := rows.Scan(
			&id,
			&varID, &varName,
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
			"variant_name":        varName,
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
