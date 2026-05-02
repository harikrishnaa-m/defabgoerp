package payment

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
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

type CashfreePaymentStatus struct {
	OrderID     string `json:"order_id"`
	OrderStatus string `json:"order_status"` // PAID, ACTIVE, EXPIRED, etc.
}

// GetOrderStatus fetches the payment status of a Cashfree order by cf_order_id.
func GetOrderStatus(cfOrderID string) (*CashfreePaymentStatus, error) {
	httpReq, err := http.NewRequest("GET", cashfreeBaseURL()+"/orders/"+cfOrderID, nil)
	if err != nil {
		return nil, err
	}
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

	var status CashfreePaymentStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return nil, fmt.Errorf("failed to parse cashfree response: %w", err)
	}
	return &status, nil
}

// InitiateRefund creates a refund for a Cashfree order.
// orderID is our order_number (e.g. ECOM-00018), refundID must be unique per refund.
func InitiateRefund(orderID, refundID string, amount float64) error {
	body, err := json.Marshal(map[string]interface{}{
		"refund_amount": amount,
		"refund_id":     refundID,
		"refund_note":   "Customer cancellation",
	})
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest("POST",
		cashfreeBaseURL()+"/orders/"+orderID+"/refunds",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-version", "2023-08-01")
	httpReq.Header.Set("x-client-id", os.Getenv("CASHFREE_APP_ID"))
	httpReq.Header.Set("x-client-secret", os.Getenv("CASHFREE_SECRET_KEY"))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("cashfree refund request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("cashfree refund error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// GetRefundStatus fetches the status of a specific refund from Cashfree.
// Returns "SUCCESS", "PENDING", "CANCELLED", etc.
func GetRefundStatus(orderID, refundID string) (string, error) {
	httpReq, err := http.NewRequest("GET",
		cashfreeBaseURL()+"/orders/"+orderID+"/refunds/"+refundID, nil)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("x-api-version", "2023-08-01")
	httpReq.Header.Set("x-client-id", os.Getenv("CASHFREE_APP_ID"))
	httpReq.Header.Set("x-client-secret", os.Getenv("CASHFREE_SECRET_KEY"))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("cashfree request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("cashfree error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		RefundStatus string `json:"refund_status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	return result.RefundStatus, nil
}

// generatePayoutSignature creates the x-cf-signature header value.
// Payload is "clientId.unixTimestamp", encrypted with RSA OAEP (SHA1) to match OpenSSL default.
// Reads key from CASHFREE_PAYOUT_PUBLIC_KEY env var (Render) or CASHFREE_PAYOUT_PUBLIC_KEY_FILE (local).
func generatePayoutSignature() (string, error) {
	var pubKeyBytes []byte

	if keyEnv := os.Getenv("CASHFREE_PAYOUT_PUBLIC_KEY"); keyEnv != "" {
		pubKeyBytes = []byte(strings.ReplaceAll(keyEnv, `\n`, "\n"))
	} else if keyFile := os.Getenv("CASHFREE_PAYOUT_PUBLIC_KEY_FILE"); keyFile != "" {
		var err error
		pubKeyBytes, err = os.ReadFile(keyFile)
		if err != nil {
			return "", fmt.Errorf("read public key: %w", err)
		}
	} else {
		return "", nil // no key configured — IP whitelist mode
	}

	block, _ := pem.Decode(pubKeyBytes)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse public key: %w", err)
	}
	pubKey := pubInterface.(*rsa.PublicKey)
	data := fmt.Sprintf("%s.%d", os.Getenv("CASHFREE_PAYOUT_CLIENT_ID"), time.Now().Unix())
	encrypted, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, pubKey, []byte(data), nil)
	if err != nil {
		return "", fmt.Errorf("encrypt signature: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// setPayoutHeaders sets auth headers on a payout request, including x-cf-signature if public key is configured.
func setPayoutHeaders(req *http.Request) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-client-id", os.Getenv("CASHFREE_PAYOUT_CLIENT_ID"))
	req.Header.Set("x-client-secret", os.Getenv("CASHFREE_PAYOUT_CLIENT_SECRET"))
	req.Header.Set("x-api-version", "2024-01-01")
	sig, err := generatePayoutSignature()
	if err != nil {
		return err
	}
	if sig != "" {
		req.Header.Set("x-cf-signature", sig)
	}
	return nil
}

// cashfreePayoutBaseURL returns the Cashfree Payouts API base URL.
func cashfreePayoutBaseURL() string {
	if strings.ToLower(os.Getenv("CASHFREE_ENV")) == "production" {
		return "https://api.cashfree.com/payout"
	}
	return "https://sandbox.cashfree.com/payout"
}

type PayoutBeneficiary struct {
	BeneficiaryID string `json:"beneId"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	BankAccount   string `json:"bankAccount,omitempty"`
	IFSC          string `json:"ifsc,omitempty"`
	VPA           string `json:"vpa,omitempty"` // UPI ID
	Address       string `json:"address,omitempty"`
}

// sanitizeBeneID strips non-alphanumeric characters (Cashfree requirement).
func sanitizeBeneID(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, id)
}

// registerBeneficiary creates the beneficiary via POST /payout/beneficiary.
// A 409 means already exists — treated as success.
func registerBeneficiary(beneID string, b PayoutBeneficiary) error {
	instrumentDetails := map[string]interface{}{}
	if b.VPA != "" {
		instrumentDetails["vpa"] = b.VPA
	} else {
		instrumentDetails["bank_account_number"] = b.BankAccount
		instrumentDetails["bank_ifsc"] = b.IFSC
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"beneficiary_id":                 beneID,
		"beneficiary_name":               b.Name,
		"beneficiary_instrument_details": instrumentDetails,
		"beneficiary_contact_details": map[string]interface{}{
			"beneficiary_email": b.Email,
			"beneficiary_phone": b.Phone,
		},
	})

	url := cashfreePayoutBaseURL() + "/beneficiary"
	log.Printf("[PAYOUT BENE] registering at %s body: %s", url, string(payload))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err := setPayoutHeaders(req); err != nil {
		return fmt.Errorf("payout signature error: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("beneficiary registration failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[PAYOUT BENE] status=%d body=%s", resp.StatusCode, string(respBody))

	if resp.StatusCode == 409 {
		// Already registered — fine to continue.
		return nil
	}
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("beneficiary registration error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// InitiatePayout sends money to a bank account or UPI via Cashfree Payouts V2.
// Step 1: register beneficiary. Step 2: transfer using beneficiary_id.
func InitiatePayout(transferID string, amount float64, b PayoutBeneficiary) error {
	beneID := sanitizeBeneID(b.BeneficiaryID)

	if err := registerBeneficiary(beneID, b); err != nil {
		return err
	}

	transferMode := "banktransfer"
	if b.VPA != "" {
		transferMode = "upi"
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"transfer_id":      transferID,
		"transfer_amount":  amount,
		"transfer_mode":    transferMode,
		"transfer_remarks": "Return refund",
		"beneficiary_details": map[string]interface{}{
			"beneficiary_id": beneID,
		},
	})

	url := cashfreePayoutBaseURL() + "/transfers"
	log.Printf("[PAYOUT V2] calling URL: %s body: %s", url, string(payload))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err := setPayoutHeaders(req); err != nil {
		return fmt.Errorf("payout signature error: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("payout transfer failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[PAYOUT V2] status=%d body=%s", resp.StatusCode, string(respBody))

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("payout error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// VerifyWebhookSignature verifies the Cashfree webhook HMAC-SHA256 signature.
func VerifyWebhookSignature(timestamp, signature string, rawBody []byte) bool {
	secret := os.Getenv("CASHFREE_SECRET_KEY")
	message := timestamp + string(rawBody)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// DebugSignature returns the computed signature for logging purposes.
func DebugSignature(timestamp string, rawBody []byte) string {
	secret := os.Getenv("CASHFREE_SECRET_KEY")
	message := timestamp + string(rawBody)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
