package stocktransfer

import (
	"log"

	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) GetBranchVariantStock(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	if user.BranchID == nil || *user.BranchID == "" {
		return httperr.BadRequest(c, "user has no branch assigned")
	}

	variantCode := c.Params("variant_code")
	if variantCode == "" {
		return httperr.BadRequest(c, "variant_code is required")
	}

	stock, err := h.store.GetBranchVariantStock(*user.BranchID, variantCode)
	if err != nil {
		log.Println("branch variant stock lookup error:", err)
		if err.Error() == "variant not found" {
			return httperr.NotFound(c, err.Error())
		}
		if err.Error() == "warehouse not found for branch" {
			return httperr.NotFound(c, err.Error())
		}
		return httperr.Internal(c)
	}

	return c.JSON(stock)
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

func (h *Handler) InterWarehouse(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	var in InterWarehouseTransferInput

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

	for _, item := range in.Items {
		if item.VariantID == "" || item.Quantity.LessThanOrEqual(decimal.Zero) {
			return httperr.BadRequest(c, "each item requires a positive quantity and variant_id")
		}
	}

	managerBranchID := ""
	if user.Role.Name == model.RoleStoreManager {
		if user.BranchID == nil || *user.BranchID == "" {
			return httperr.BadRequest(c, "Your account is not associated with a branch, so a source warehouse cannot be selected")
		}
		managerBranchID = *user.BranchID
	} else if user.BranchID != nil {
		managerBranchID = *user.BranchID
	}

	if err := h.store.CreateInterWarehouseTransfer(in, managerBranchID); err != nil {
		log.Println("inter-warehouse transfer error:", err)

		switch err.Error() {
		case "source or destination warehouse not found":
			return httperr.NotFound(c, err.Error())
		case "source warehouse is not associated with your branch":
			return httperr.BadRequest(c, "The selected source warehouse is not associated with your branch")
		case "source and destination warehouses must belong to different branches":
			return httperr.BadRequest(c, err.Error())
		case "insufficient stock":
			return httperr.Conflict(c, "Insufficient stock in source warehouse")
		case "stock not found in source warehouse":
			return httperr.NotFound(c, err.Error())
		default:
			return httperr.Internal(c)
		}
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "Inter-warehouse stock transferred successfully",
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
