
package stock

import (
	"database/sql"
	"defab-erp/internal/core/httperr"
	"strconv"
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

	rows, err := h.store.ListByWarehouse(warehouseID, limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var variantID, product, variant, warehouse string
		var qty int

		rows.Scan(&variantID, &product, &variant, &warehouse, &qty)

		out = append(out, fiber.Map{
			"variant_id": variantID,
			"product":    product,
			"variant":    variant,
			"warehouse":  warehouse,
			"quantity":   qty,
		})
	}

	return c.JSON(out)
}

// GET /stocks/branch/:id
func (h *Handler) ByBranch(c *fiber.Ctx) error {
	branchID := c.Params("id")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

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
			"product_id": productID,
			"product_name": productName,
			"variant_id": variantID,
			"variant_name": variantName,
			"warehouse_id": warehouseID,
			"warehouse_name": warehouseName,
			"quantity": qty,
		})
	}
	return c.JSON(out)
}

// PATCH /stocks/:id
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
		var qty int
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
		var qty int

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
		"page":  page,
		"limit": limit,
		"data":  data,
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

	rows, err := h.store.GetMovements(limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var (
			id, variant, movement string
			qty                   decimal.Decimal
			fromWh, toWh          sql.NullString
			created               time.Time
		)

		if err := rows.Scan(
			&id,
			&variant,
			&movement,
			&qty,
			&fromWh,
			&toWh,
			&created,
		); err != nil {
			return httperr.Internal(c)
		}

		out = append(out, fiber.Map{
			"id":         id,
			"variant":    variant,
			"type":       movement,
			"quantity":   qty,
			"from":       nullOrValue(fromWh),
			"to":         nullOrValue(toWh),
			"created_at": created,
		})
	}

	return c.JSON(out)
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
