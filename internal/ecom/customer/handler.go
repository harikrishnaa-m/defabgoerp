package customer

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	ecomMw "defab-erp/internal/ecom/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func generateEcomJWT(customerID, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": customerID,
		"email":   email,
		"role":    "ecom_customer",
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

// POST /ecom/auth/register
func (h *Handler) Register(c *fiber.Ctx) error {
	var in RegisterInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}

	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	in.Name = strings.TrimSpace(in.Name)

	if in.Name == "" || in.Email == "" || in.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name, email, password are required"})
	}
	if len(in.Password) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "password must be at least 6 characters"})
	}

	if h.store.EmailExists(in.Email) {
		return c.Status(409).JSON(fiber.Map{"error": "email already registered"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}

	id, err := h.store.Create(in.Name, in.Email, in.Phone, string(hash))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "registration failed"})
	}

	token, err := generateEcomJWT(id, in.Email)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "token generation failed"})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "registered successfully",
		"token":   token,
		"customer": fiber.Map{
			"id":    id,
			"name":  in.Name,
			"email": in.Email,
		},
	})
}

// POST /ecom/auth/login
func (h *Handler) Login(c *fiber.Ctx) error {
	var in LoginInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}

	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if in.Email == "" || in.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "email and password are required"})
	}

	id, name, passwordHash, err := h.store.GetByEmail(in.Email)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid email or password"})
	}

	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(in.Password)) != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid email or password"})
	}

	token, err := generateEcomJWT(id, in.Email)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "token generation failed"})
	}

	return c.JSON(fiber.Map{
		"message": "login successful",
		"token":   token,
		"customer": fiber.Map{
			"id":    id,
			"name":  name,
			"email": in.Email,
		},
	})
}

// POST /ecom/auth/google
func (h *Handler) GoogleSignIn(c *fiber.Ctx) error {
	var in GoogleSignInInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if in.IDToken == "" {
		return c.Status(400).JSON(fiber.Map{"error": "id_token is required"})
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		return c.Status(500).JSON(fiber.Map{"error": "Google sign-in not configured"})
	}

	payload, err := idtoken.Validate(c.Context(), in.IDToken, clientID)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid Google token"})
	}

	googleID, _ := payload.Subject, payload.Issuer
	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)

	if email == "" {
		return c.Status(400).JSON(fiber.Map{"error": "email not available from Google"})
	}
	email = strings.ToLower(email)

	// 1. Try to find by google_id
	id, custName, custEmail, err := h.store.GetByGoogleID(googleID)
	if err == nil {
		token, err := generateEcomJWT(id, custEmail)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "token generation failed"})
		}
		return c.JSON(fiber.Map{
			"message": "login successful",
			"token":   token,
			"customer": fiber.Map{
				"id":    id,
				"name":  custName,
				"email": custEmail,
			},
		})
	}

	// 2. Try to find by email — link Google ID to existing account
	id, custName, _, err = h.store.GetByEmail(email)
	if err == nil {
		_ = h.store.LinkGoogleID(id, googleID)
		token, err := generateEcomJWT(id, email)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "token generation failed"})
		}
		return c.JSON(fiber.Map{
			"message": "login successful",
			"token":   token,
			"customer": fiber.Map{
				"id":    id,
				"name":  custName,
				"email": email,
			},
		})
	}

	// 3. New user — create account
	if err != sql.ErrNoRows && err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	id, err = h.store.CreateGoogleUser(name, email, googleID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "registration failed"})
	}

	token, err := generateEcomJWT(id, email)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "token generation failed"})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "registered successfully",
		"token":   token,
		"customer": fiber.Map{
			"id":    id,
			"name":  name,
			"email": email,
		},
	})
}

// POST /ecom/auth/forgot-password
func (h *Handler) ForgotPassword(c *fiber.Ctx) error {
	var in ForgotPasswordInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if in.Email == "" {
		return c.Status(400).JSON(fiber.Map{"error": "email is required"})
	}

	// Generate secure random token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}
	token := base64.URLEncoding.EncodeToString(b)

	if err := h.store.SetResetToken(in.Email, token); err != nil {
		// Don't reveal whether email exists
		return c.JSON(fiber.Map{"message": "if that email is registered, a reset link has been sent"})
	}

	if err := sendEcomResetEmail(in.Email, token); err != nil {
		log.Println("[SMTP ERROR]", err)
		return c.Status(500).JSON(fiber.Map{"error": "failed to send email"})
	}

	return c.JSON(fiber.Map{"message": "if that email is registered, a reset link has been sent"})
}

// POST /ecom/auth/reset-password
func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	var in ResetPasswordInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if in.Token == "" || in.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{"error": "token and new_password are required"})
	}
	if len(in.NewPassword) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "password must be at least 6 characters"})
	}

	customerID, err := h.store.GetByResetToken(in.Token)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid or expired reset token"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), 12)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}

	if err := h.store.UpdatePassword(customerID, string(hash)); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to update password"})
	}

	return c.JSON(fiber.Map{"message": "password reset successful"})
}

func sendEcomResetEmail(toEmail, token string) error {
	from := os.Getenv("GMAIL_USER")
	pass := os.Getenv("GMAIL_PASS")
	frontendURL := os.Getenv("ECOM_RESET_URL_BASE")
	if frontendURL == "" {
		frontendURL = os.Getenv("FRONTEND_RESET_URL_BASE")
	}
	resetLink := frontendURL + token
	msg := fmt.Sprintf("Subject: Password Reset Request\r\n\r\nClick the link below to reset your password:\r\n%s", resetLink)
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"
	auth := smtp.PlainAuth("", from, pass, smtpHost)
	return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{toEmail}, []byte(msg))
}

// GET /ecom/profile
func (h *Handler) GetProfile(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	profile, err := h.store.GetProfile(cust.ID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "profile not found"})
	}
	return c.JSON(profile)
}

// POST /ecom/change-password
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	var in ChangePasswordInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if in.OldPassword == "" || in.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{"error": "old_password and new_password are required"})
	}
	if len(in.NewPassword) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "new password must be at least 6 characters"})
	}

	currentHash, err := h.store.GetPasswordHash(cust.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}
	if currentHash == "" {
		return c.Status(400).JSON(fiber.Map{"error": "account uses Google sign-in, no password set"})
	}

	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(in.OldPassword)) != nil {
		return c.Status(401).JSON(fiber.Map{"error": "old password is incorrect"})
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), 12)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "internal error"})
	}

	if err := h.store.UpdatePassword(cust.ID, string(newHash)); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to update password"})
	}

	return c.JSON(fiber.Map{"message": "password changed successfully"})
}

// PATCH /ecom/profile
func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	var in UpdateProfileInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}

	if err := h.store.UpdateProfile(cust.ID, in); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "update failed"})
	}

	return c.JSON(fiber.Map{"message": "profile updated"})
}

// ── Addresses ───────────────────────────────────────────────

// POST /ecom/addresses
func (h *Handler) AddAddress(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	var in AddressInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if in.FullName == "" || in.Phone == "" || in.AddressLine1 == "" || in.City == "" || in.State == "" || in.Pincode == "" {
		return c.Status(400).JSON(fiber.Map{"error": "full_name, phone, address_line1, city, state, pincode are required"})
	}
	if in.Label == "" {
		in.Label = "Home"
	}

	id, err := h.store.AddAddress(cust.ID, in)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to add address"})
	}
	return c.Status(201).JSON(fiber.Map{"message": "address added", "id": id})
}

// GET /ecom/addresses
func (h *Handler) ListAddresses(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	addrs, err := h.store.ListAddresses(cust.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch addresses"})
	}
	if addrs == nil {
		addrs = []Address{}
	}
	return c.JSON(fiber.Map{"addresses": addrs})
}

// PUT /ecom/addresses/:id
func (h *Handler) UpdateAddress(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	addrID := c.Params("id")

	var in AddressInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}

	if err := h.store.UpdateAddress(cust.ID, addrID, in); err != nil {
		if err.Error() == "address not found" {
			return c.Status(404).JSON(fiber.Map{"error": "address not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "update failed"})
	}
	return c.JSON(fiber.Map{"message": "address updated"})
}

// DELETE /ecom/addresses/:id
func (h *Handler) DeleteAddress(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	addrID := c.Params("id")

	if err := h.store.DeleteAddress(cust.ID, addrID); err != nil {
		if err.Error() == "address not found" {
			return c.Status(404).JSON(fiber.Map{"error": "address not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "delete failed"})
	}
	return c.JSON(fiber.Map{"message": "address deleted"})
}
