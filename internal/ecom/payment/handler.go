package payment

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	ecomMw "defab-erp/internal/ecom/middleware"

	"github.com/gofiber/fiber/v2"
)

// ReturnStoreInterface is the subset of ecom/return Store used by the payment handler.
type ReturnStoreInterface interface {
	MarkPayoutCompleted(transferID string) error
}

type Handler struct {
	store       *Store
	db          *sql.DB
	returnStore ReturnStoreInterface
}

func NewHandler(s *Store, db *sql.DB) *Handler {
	return &Handler{store: s, db: db}
}

func (h *Handler) SetReturnStore(rs ReturnStoreInterface) {
	h.returnStore = rs
}

// POST /ecom/payments/initiate
// Called after checkout with payment_method=ONLINE. Creates Cashfree order.
func (h *Handler) Initiate(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	var in struct {
		OrderID string `json:"order_id"`
	}
	if err := c.BodyParser(&in); err != nil || in.OrderID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "order_id is required"})
	}

	// Load order
	var orderNumber, custName, custPhone string
	var grandTotal float64
	var payMethod, payStatus string
	var custPhone_ sql.NullString

	err := h.db.QueryRow(`
		SELECT o.order_number, o.grand_total, o.payment_method, o.payment_status,
		       ec.name, ec.phone
		FROM ecom_orders o
		JOIN ecom_customers ec ON ec.id = o.customer_id
		WHERE o.id = $1 AND o.customer_id = $2
	`, in.OrderID, cust.ID).Scan(&orderNumber, &grandTotal, &payMethod, &payStatus, &custName, &custPhone_)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "order not found"})
	}
	if payMethod != "ONLINE" {
		return c.Status(400).JSON(fiber.Map{"error": "order is not an online payment order"})
	}
	if payStatus == "PAID" {
		return c.Status(400).JSON(fiber.Map{"error": "order already paid"})
	}
	if custPhone_.Valid {
		custPhone = custPhone_.String
	}
	if custPhone == "" {
		custPhone = "9999999999" // Cashfree requires a phone number
	}

	returnURL := os.Getenv("CASHFREE_RETURN_URL")
	if returnURL == "" {
		returnURL = "https://yoursite.com/payment/result"
	}
	notifyURL := os.Getenv("CASHFREE_WEBHOOK_URL")

	cfReq := CashfreeOrderRequest{
		OrderID:       orderNumber,
		OrderAmount:   grandTotal,
		OrderCurrency: "INR",
		CustomerDetails: CashfreeCustomer{
			CustomerID:    cust.ID,
			CustomerEmail: cust.Email,
			CustomerPhone: custPhone,
			CustomerName:  custName,
		},
		OrderMeta: CashfreeMeta{
			ReturnURL: returnURL + "?order_id=" + in.OrderID,
			NotifyURL: notifyURL,
		},
	}

	cfResp, err := CreateOrder(cfReq)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "payment gateway error: " + err.Error()})
	}

	if err := h.store.SaveCashfreeOrder(in.OrderID, cfResp.CFOrderID, cfResp.PaymentSessionID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to save payment session"})
	}

	return c.JSON(fiber.Map{
		"order_id":           in.OrderID,
		"cf_order_id":        cfResp.CFOrderID,
		"payment_session_id": cfResp.PaymentSessionID,
		"amount":             grandTotal,
	})
}

// POST /ecom/payments/webhook  (public — no auth, verified by signature)
func (h *Handler) Webhook(c *fiber.Ctx) error {
	timestamp := c.Get("x-webhook-timestamp")
	signature := c.Get("x-webhook-signature")
	rawBody := c.Body()

	log.Printf("[WEBHOOK] timestamp=%q", timestamp)
	log.Printf("[WEBHOOK] received signature=%q", signature)
	log.Printf("[WEBHOOK] body (first 300)=%s", string(rawBody[:min(300, len(rawBody))]))
	log.Printf("[WEBHOOK] computed signature=%s", DebugSignature(timestamp, rawBody))

	if !VerifyWebhookSignature(timestamp, signature, rawBody) {
		log.Printf("[WEBHOOK] signature MISMATCH — returning 401")
		return c.Status(401).SendString("invalid signature")
	}
	log.Printf("[WEBHOOK] signature OK")

	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return c.Status(400).SendString("invalid JSON")
	}

	// Extract event type
	eventType, _ := payload["type"].(string)
	log.Printf("[WEBHOOK] event type=%q", eventType)

	switch eventType {
	case "PAYMENT_SUCCESS_WEBHOOK":
		data, _ := payload["data"].(map[string]interface{})
		if data == nil {
			return c.SendStatus(200)
		}
		order, _ := data["order"].(map[string]interface{})
		payment, _ := data["payment"].(map[string]interface{})
		if order == nil || payment == nil {
			return c.SendStatus(200)
		}
		orderNumber, _ := order["order_id"].(string)
		cfPaymentID, _ := payment["cf_payment_id"].(string)
		paymentStatus, _ := payment["payment_status"].(string)
		if orderNumber == "" || paymentStatus != "SUCCESS" {
			return c.SendStatus(200)
		}
		payRef := fmt.Sprintf("CF-%s", cfPaymentID)
		if err := h.store.MarkOrderPaid(orderNumber, payRef); err != nil {
			log.Printf("[WEBHOOK] MarkOrderPaid failed: %v", err)
		}

	case "REFUND_SUCCESS_WEBHOOK", "REFUND_STATUS_WEBHOOK":
		data, _ := payload["data"].(map[string]interface{})
		if data == nil {
			return c.SendStatus(200)
		}
		refund, _ := data["refund"].(map[string]interface{})
		if refund == nil {
			return c.SendStatus(200)
		}
		orderID, _ := refund["order_id"].(string) // our order_number
		refundStatus, _ := refund["refund_status"].(string)
		if orderID == "" || refundStatus != "SUCCESS" {
			return c.SendStatus(200)
		}
		if err := h.store.MarkRefunded(orderID); err != nil {
			log.Printf("[WEBHOOK] MarkRefunded failed: %v", err)
		}
	}

	return c.SendStatus(200)
}

// POST /ecom/payouts/webhook  (public — Cashfree Payouts webhook)
func (h *Handler) PayoutWebhook(c *fiber.Ctx) error {
	rawBody := c.Body()
	log.Printf("[PAYOUT WEBHOOK] raw body: %s", string(rawBody))

	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		log.Printf("[PAYOUT WEBHOOK] JSON parse error: %v", err)
		return c.Status(400).SendString("invalid JSON")
	}

	// Try both V1 camelCase and V2 snake_case field names
	event, _ := payload["event"].(string)
	if event == "" {
		event, _ = payload["type"].(string)
	}

	// V2: fields are nested under "data"
	transferID, _ := payload["transferId"].(string)
	status, _ := payload["status"].(string)
	if data, ok := payload["data"].(map[string]interface{}); ok {
		if transferID == "" {
			transferID, _ = data["transfer_id"].(string)
		}
		if status == "" {
			status, _ = data["status"].(string)
		}
	}

	log.Printf("[PAYOUT WEBHOOK] event=%q transferId=%q status=%q", event, transferID, status)

	if transferID == "" {
		return c.SendStatus(200)
	}

	if event == "TRANSFER_SUCCESS" || status == "SUCCESS" || status == "TRANSFER_SUCCESS" {
		if err := h.returnStore.MarkPayoutCompleted(transferID); err != nil {
			log.Printf("[PAYOUT WEBHOOK] MarkPayoutCompleted failed: %v", err)
		}
	}

	return c.SendStatus(200)
}

// GET /ecom/payments/:order_id/verify
// Called by frontend after redirect to check if payment was captured.
func (h *Handler) Verify(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	orderID := c.Params("order_id")

	// Fetch order_number and payment_status; order_number is what we sent to Cashfree
	var orderNumber, payStatus string
	err := h.db.QueryRow(`
		SELECT order_number, payment_status FROM ecom_orders
		WHERE id = $1 AND customer_id = $2
	`, orderID, cust.ID).Scan(&orderNumber, &payStatus)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "order not found"})
	}

	// Already marked paid (e.g. via webhook)
	if payStatus == "PAID" {
		return c.JSON(fiber.Map{"payment_status": "PAID", "order_id": orderID})
	}

	// Poll Cashfree using our order_number (Cashfree's GET /orders/{order_id} uses our order_id)
	cfStatus, err := GetOrderStatus(orderNumber)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "could not verify with payment gateway"})
	}

	if cfStatus.OrderStatus == "PAID" {
		if err := h.store.MarkOrderPaidByOrderNumber(orderNumber); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to update order"})
		}
		return c.JSON(fiber.Map{"payment_status": "PAID", "order_id": orderID})
	}

	return c.JSON(fiber.Map{"payment_status": cfStatus.OrderStatus, "order_id": orderID})
}

// GET /ecom/payments/:order_id/refund-status
// Polls Cashfree for the refund status and syncs the DB if refund is confirmed.
func (h *Handler) RefundStatus(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	orderID := c.Params("order_id")

	var orderNumber, payStatus string
	err := h.db.QueryRow(`
		SELECT order_number, payment_status FROM ecom_orders
		WHERE id = $1 AND customer_id = $2
	`, orderID, cust.ID).Scan(&orderNumber, &payStatus)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "order not found"})
	}

	if payStatus == "REFUNDED" {
		return c.JSON(fiber.Map{"refund_status": "SUCCESS", "order_id": orderID})
	}

	refundID := fmt.Sprintf("REF-%s", orderNumber)
	status, err := GetRefundStatus(orderNumber, refundID)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "could not check refund status"})
	}

	if status == "SUCCESS" {
		_ = h.store.MarkRefunded(orderNumber)
		return c.JSON(fiber.Map{"refund_status": "SUCCESS", "order_id": orderID})
	}

	return c.JSON(fiber.Map{"refund_status": status, "order_id": orderID})
}
