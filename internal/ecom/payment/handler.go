package payment

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	ecomMw "defab-erp/internal/ecom/middleware"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
	db    *sql.DB
}

func NewHandler(s *Store, db *sql.DB) *Handler {
	return &Handler{store: s, db: db}
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

	if !VerifyWebhookSignature(timestamp, signature, rawBody) {
		return c.Status(401).SendString("invalid signature")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return c.Status(400).SendString("invalid JSON")
	}

	// Extract event type
	eventType, _ := payload["type"].(string)
	if eventType != "PAYMENT_SUCCESS_WEBHOOK" {
		// Acknowledge other events without processing
		return c.SendStatus(200)
	}

	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return c.SendStatus(200)
	}

	order, _ := data["order"].(map[string]interface{})
	payment, _ := data["payment"].(map[string]interface{})
	if order == nil || payment == nil {
		return c.SendStatus(200)
	}

	cfOrderID, _ := order["order_id"].(string) // Cashfree's order_id (our order_number)
	cfPaymentID, _ := payment["cf_payment_id"].(string)
	paymentStatus, _ := payment["payment_status"].(string)

	if cfOrderID == "" || paymentStatus != "SUCCESS" {
		return c.SendStatus(200)
	}

	payRef := fmt.Sprintf("CF-%s", cfPaymentID)
	if err := h.store.MarkOrderPaid(cfOrderID, payRef); err != nil {
		// Log but still return 200 so Cashfree doesn't retry
		return c.SendStatus(200)
	}

	return c.SendStatus(200)
}
