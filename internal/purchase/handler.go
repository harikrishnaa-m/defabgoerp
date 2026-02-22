package purchase

import (
	"log"

	"defab-erp/internal/core/httperr"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// CREATE
func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreatePurchaseOrderInput

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON")
	}

	if in.SupplierID == "" || in.WarehouseID == "" || len(in.Items) == 0 {
		return httperr.BadRequest(c, "supplier, warehouse & items required")
	}

	id, err := h.store.Create(in)
	if err != nil {
		log.Println("po create error:", err)
		return httperr.Internal(c)
	}

	return c.Status(201).JSON(fiber.Map{
		"id":      id,
		"message": "Purchase order created",
	})
}

// LIST
func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	rows, err := h.store.List(limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, po, status, created string
		rows.Scan(&id, &po, &status, &created)

		out = append(out, fiber.Map{
			"id": id,
			"po_number": po,
			"status": status,
			"created_at": created,
		})
	}

	return c.JSON(out)
}

// GET
func (h *Handler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	row := h.store.Get(id)

	var poID, poNum, supplier, warehouse, status, created string
	var expected string

	if err := row.Scan(
		&poID, &poNum, &supplier,
		&warehouse, &status, &expected, &created,
	); err != nil {
		return httperr.NotFound(c, "Purchase order not found")
	}

	return c.JSON(fiber.Map{
		"id": poID,
		"po_number": poNum,
		"supplier_id": supplier,
		"warehouse_id": warehouse,
		"status": status,
		"expected_date": expected,
		"created_at": created,
	})
}

// UPDATE STATUS
func (h *Handler) UpdateStatus(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdatePOStatusInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON")
	}

	if err := h.store.UpdateStatus(id, in.Status); err != nil {
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"message": "PO status updated",
	})
}
