package payment

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func cashfreeBaseURL() string {
	if strings.ToLower(os.Getenv("CASHFREE_ENV")) == "production" {
		return "https://api.cashfree.com/pg"
	}
	return "https://sandbox.cashfree.com/pg"
}

type CashfreeOrderRequest struct {
	OrderID         string           `json:"order_id"`
	OrderAmount     float64          `json:"order_amount"`
	OrderCurrency   string           `json:"order_currency"`
	CustomerDetails CashfreeCustomer `json:"customer_details"`
	OrderMeta       CashfreeMeta     `json:"order_meta"`
	OrderNote       string           `json:"order_note,omitempty"`
}

type CashfreeCustomer struct {
	CustomerID    string `json:"customer_id"`
	CustomerEmail string `json:"customer_email"`
	CustomerPhone string `json:"customer_phone"`
	CustomerName  string `json:"customer_name"`
}

type CashfreeMeta struct {
	ReturnURL string `json:"return_url"`
	NotifyURL string `json:"notify_url,omitempty"`
}

type CashfreeOrderResponse struct {
	CFOrderID        string    `json:"cf_order_id"`
	OrderID          string    `json:"order_id"`
	OrderStatus      string    `json:"order_status"`
	PaymentSessionID string    `json:"payment_session_id"`
	OrderAmount      float64   `json:"order_amount"`
	CreatedAt        time.Time `json:"created_at"`
}

// CreateOrder creates a Cashfree payment order and returns the session ID.
func CreateOrder(req CashfreeOrderRequest) (*CashfreeOrderResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", cashfreeBaseURL()+"/orders", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-version", "2023-08-01")
	httpReq.Header.Set("x-client-id", os.Getenv("CASHFREE_APP_ID"))
	httpReq.Header.Set("x-client-secret", os.Getenv("CASHFREE_SECRET_KEY"))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cashfree request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("cashfree error %d: %s", resp.StatusCode, string(respBody))
	}

	var cfResp CashfreeOrderResponse
	if err := json.Unmarshal(respBody, &cfResp); err != nil {
		return nil, fmt.Errorf("failed to parse cashfree response: %w", err)
	}
	return &cfResp, nil
}

// VerifyWebhookSignature verifies the Cashfree webhook HMAC-SHA256 signature.
// Cashfree signs: timestamp + rawBody
func VerifyWebhookSignature(timestamp, signature string, rawBody []byte) bool {
	secret := os.Getenv("CASHFREE_SECRET_KEY")
	message := timestamp + string(rawBody)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
