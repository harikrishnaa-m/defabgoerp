package purchase

import (
	"database/sql"
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

	list, err := h.store.List(limit, offset)
	if err != nil {
		log.Println("po list error:", err)
		return httperr.Internal(c)
	}

	if list == nil {
		list = []POListRow{}
	}

	return c.JSON(list)
}

// GET
func (h *Handler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	po, err := h.store.Get(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Purchase order not found")
		}
		log.Println("po get error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(po)
}

// UPDATE STATUS
func (h *Handler) UpdateStatus(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdatePOStatusInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON")
	}

	if err := h.store.UpdateStatus(id, in.Status); err != nil {
		log.Println("po status error:", err)
		return httperr.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"message": "PO status updated",
	})
}

// ADD ITEM to PO
func (h *Handler) AddItem(c *fiber.Ctx) error {
	poID := c.Params("id")

	var in AddPOItemInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON")
	}

	if in.ItemName == "" || in.Unit == "" || in.Quantity <= 0 || in.UnitPrice <= 0 {
		return httperr.BadRequest(c, "item_name, unit, quantity and unit_price required")
	}

	itemID, err := h.store.AddItem(poID, in)
	if err != nil {
		log.Println("po add item error:", err)
		return httperr.Internal(c)
	}

	return c.Status(201).JSON(fiber.Map{
		"id":      itemID,
		"message": "Item added to purchase order",
	})
}

// UPDATE ITEM in PO
func (h *Handler) UpdateItem(c *fiber.Ctx) error {
	poID := c.Params("id")
	itemID := c.Params("itemId")

	var in UpdatePOItemInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON")
	}

	if err := h.store.UpdateItem(poID, itemID, in); err != nil {
		if err == sql.ErrNoRows {
			return httperr.NotFound(c, "Item not found")
		}
		log.Println("po update item error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"message": "Item updated",
	})
}

// DELETE ITEM from PO
func (h *Handler) DeleteItem(c *fiber.Ctx) error {
	poID := c.Params("id")
	itemID := c.Params("itemId")

	if err := h.store.DeleteItem(poID, itemID); err != nil {
		log.Println("po delete item error:", err)
		return httperr.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"message": "Item removed from purchase order",
	})
}
