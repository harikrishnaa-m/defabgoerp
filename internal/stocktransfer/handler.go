package stocktransfer

import (
	"log"

	"defab-erp/internal/core/httperr"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateStockTransferInput

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.FromWarehouseID == "" || in.ToWarehouseID == "" {
		return httperr.BadRequest(c, "from_warehouse_id and to_warehouse_id required")
	}

	if in.FromWarehouseID == in.ToWarehouseID {
		return httperr.BadRequest(c, "source and destination warehouse cannot be same")
	}

	if len(in.Items) == 0 {
		return httperr.BadRequest(c, "at least one item required")
	}

	if err := h.store.Create(in); err != nil {
		log.Println("stock transfer error:", err)

		switch err.Error() {
		case "insufficient stock":
			return httperr.Conflict(c, "Insufficient stock in source warehouse")
		case "stock not found in source warehouse":
			return httperr.NotFound(c, err.Error())
		default:
			return httperr.Internal(c)
		}
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "Stock transferred successfully",
	})
}


// dispatched stock recieve handler

func (h *Handler) Receive(c *fiber.Ctx) error {
	movementID := c.Params("id")

	var in struct {
		ReceivedQty string `json:"received_qty"`
		Remarks     string `json:"remarks"`
	}

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid payload")
	}

	qty, err := decimal.NewFromString(in.ReceivedQty)
	if err != nil || qty.LessThanOrEqual(decimal.Zero) {
		return httperr.BadRequest(c, "Invalid received_qty")
	}

	if err := h.store.ReceiveTransfer(movementID, qty, in.Remarks); err != nil {
		log.Println("❌ receive error:", err)

		if err.Error() == "movement not in transit" {
			return httperr.BadRequest(c, "Transfer already received or invalid")
		}
		if err.Error() == "destination warehouse missing" {
			return httperr.BadRequest(c, "Invalid transfer record")
		}

		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"message": "Stock received successfully",
	})
}
