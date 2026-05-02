package ecomreturn

import (
	"fmt"
	"strconv"
	"strings"

	ecomMw "defab-erp/internal/ecom/middleware"
	ecomPayment "defab-erp/internal/ecom/payment"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// ── Customer endpoints ────────────────────────────────────────────────────────

// POST /ecom/returns
func (h *Handler) RequestReturn(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	var in struct {
		OrderID             string  `json:"order_id"`
		Reason              string  `json:"reason"`
		PayoutMethod        *string `json:"payout_method"`
		PayoutUPI           *string `json:"payout_upi"`
		PayoutAccountNumber *string `json:"payout_account_number"`
		PayoutIFSC          *string `json:"payout_ifsc"`
		PayoutAccountName   *string `json:"payout_account_name"`
	}
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if in.OrderID == "" || in.Reason == "" {
		return c.Status(400).JSON(fiber.Map{"error": "order_id and reason are required"})
	}

	ret, err := h.store.RequestReturn(
		cust.ID, in.OrderID, in.Reason,
		in.PayoutMethod, in.PayoutUPI, in.PayoutAccountNumber, in.PayoutIFSC, in.PayoutAccountName,
	)
	if err != nil {
		msg := err.Error()
		switch msg {
		case "order not found":
			return c.Status(404).JSON(fiber.Map{"error": msg})
		case "only delivered orders are eligible for return",
			"return window has expired (7 days from delivery)",
			"a return request already exists for this order",
			"delivery date not recorded; cannot process return":
			return c.Status(400).JSON(fiber.Map{"error": msg})
		default:
			if strings.HasPrefix(msg, "payout") {
				return c.Status(400).JSON(fiber.Map{"error": msg})
			}
			return c.Status(500).JSON(fiber.Map{"error": "failed to create return request"})
		}
	}

	return c.Status(201).JSON(ret)
}

// GET /ecom/returns/:id
func (h *Handler) GetReturn(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	returnID := c.Params("id")

	ret, err := h.store.GetReturn(cust.ID, returnID)
	if err != nil {
		if err.Error() == "return request not found" {
			return c.Status(404).JSON(fiber.Map{"error": "return request not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch return"})
	}
	return c.JSON(ret)
}

// GET /ecom/returns
func (h *Handler) ListReturns(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	list, err := h.store.ListReturns(cust.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch returns"})
	}
	if list == nil {
		list = []ReturnRequest{}
	}
	return c.JSON(list)
}

// ── Admin endpoints ───────────────────────────────────────────────────────────

// GET /admin/ecom-returns
func (h *Handler) AdminListReturns(c *fiber.Ctx) error {
	status := c.Query("status")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	list, err := h.store.AdminListReturns(status, page, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch returns"})
	}
	if list == nil {
		list = []ReturnRequest{}
	}
	return c.JSON(list)
}

// PATCH /admin/ecom-returns/:id/approve
// Approves the return and triggers refund (ONLINE) or payout (COD).
func (h *Handler) AdminApproveReturn(c *fiber.Ctx) error {
	returnID := c.Params("id")

	ret, info, err := h.store.AdminGetReturnWithOrder(returnID)
	if err != nil {
		if err.Error() == "return request not found" {
			return c.Status(404).JSON(fiber.Map{"error": "return request not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if ret.Status != "REQUESTED" {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("cannot approve a return in %s state", ret.Status)})
	}

	if err := h.store.AdminApproveReturn(returnID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Trigger refund or payout based on payment method
	if info.PaymentMethod == "ONLINE" && info.PaymentStatus == "PAID" {
		refundID := "RETURN-" + info.OrderNumber
		if err := ecomPayment.InitiateRefund(info.OrderNumber, refundID, info.GrandTotal); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "approved but refund initiation failed: " + err.Error()})
		}
		if err := h.store.MarkRefundInitiated(returnID, info.OrderNumber); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "refund initiated but status update failed"})
		}
		return c.JSON(fiber.Map{"message": "return approved and refund initiated"})
	}

	if info.PaymentMethod == "COD" {
		if ret.PayoutMethod == nil {
			return c.Status(400).JSON(fiber.Map{"error": "no payout method on file for this return"})
		}
		transferID := "POUT-" + info.OrderNumber
		bene := ecomPayment.PayoutBeneficiary{
			Name:  "Customer",
			Email: "noreply@defab.in",
			Phone: "9999999999",
		}
		if *ret.PayoutMethod == "UPI" && ret.PayoutUPI != nil {
			bene.BeneficiaryID = "CUST-" + info.CustomerID + "-UPI"
			bene.VPA = *ret.PayoutUPI
		} else if ret.PayoutAccountNumber != nil && ret.PayoutIFSC != nil {
			bene.BeneficiaryID = "CUST-" + info.CustomerID + "-BANK"
			bene.BankAccount = *ret.PayoutAccountNumber
			bene.IFSC = *ret.PayoutIFSC
			if ret.PayoutAccountName != nil {
				bene.Name = *ret.PayoutAccountName
			}
		} else {
			return c.Status(400).JSON(fiber.Map{"error": "incomplete payout details on file"})
		}

		if err := ecomPayment.InitiatePayout(transferID, info.GrandTotal, bene); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "approved but payout initiation failed: " + err.Error()})
		}
		if err := h.store.MarkPayoutInitiated(returnID, transferID, info.OrderNumber); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "payout initiated but status update failed"})
		}
		return c.JSON(fiber.Map{"message": "return approved and payout initiated"})
	}

	return c.JSON(fiber.Map{"message": "return approved"})
}

// PATCH /admin/ecom-returns/:id/reject
func (h *Handler) AdminRejectReturn(c *fiber.Ctx) error {
	returnID := c.Params("id")

	if err := h.store.AdminRejectReturn(returnID); err != nil {
		msg := err.Error()
		if msg == "return not found or not in REQUESTED state" {
			return c.Status(404).JSON(fiber.Map{"error": msg})
		}
		return c.Status(500).JSON(fiber.Map{"error": msg})
	}
	return c.JSON(fiber.Map{"message": "return rejected"})
}
