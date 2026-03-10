package auth

import (
	"crypto/rand"
	"defab-erp/internal/core/model"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"os"

	"github.com/gofiber/fiber/v2"
)

// GenerateRefreshToken creates a secure random refresh token
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Injecting the Store interface is a good practice for testing
type AuthHandler struct {
	store *Store
}

func NewHandler(s *Store) *AuthHandler {
	return &AuthHandler{store: s}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	input := new(RegisterInput)

	if err := c.BodyParser(input); err != nil {
		return c.Status(400).SendString("Invalid input")
	}

	hash, err := HashPassword(input.Password)
	if err != nil {
		return c.Status(500).SendString("hash error")
	}

	u := &model.User{
		Name:         input.Name,
		Email:        input.Email,
		PasswordHash: hash,
		RoleID:       input.RoleID,
		BranchID:     input.BranchID,
	}

	if err := h.store.CreateUser(u); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	fullUser, err := h.store.GetUserByEmail(u.Email)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	fullUser.PasswordHash = ""
	return c.JSON(fullUser)

}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	input := new(LoginInput)

	if err := c.BodyParser(input); err != nil {
		return c.Status(400).SendString("invalid input")
	}

	// find user
	u, err := h.store.GetUserByEmail(input.Email)
	if err != nil {
		return c.Status(401).SendString("invalid credentials")
	}

	// check password
	if !CheckPassword(u.PasswordHash, input.Password) {
		return c.Status(401).SendString("invalid credentials")
	}

	// generate access token
	token, err := GenerateJWT(u.ID.String(), u.Role.Name)
	if err != nil {
		return c.Status(500).SendString("token error")
	}

	// generate refresh token (simple random string for now)
	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return c.Status(500).SendString("refresh token error")
	}

	// Store refresh token in DB
	if err := h.store.UpdateRefreshToken(u.ID, refreshToken); err != nil {
		return c.Status(500).SendString("failed to store refresh token")
	}

	u.PasswordHash = ""

	// Set refresh token as httpOnly, secure cookie
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 7, // 7 days
	})

	return c.JSON(fiber.Map{
		"token": token,
		"user":  u,
	})
}

// Forgot Password: Request password reset
func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	type inputStruct struct {
		Email string `json:"email"`
	}
	input := new(inputStruct)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).SendString("Invalid input")
	}
	user, err := h.store.GetUserByEmail(input.Email)
	if err != nil {
		return c.Status(404).SendString("User not found")
	}
	// Generate secure token (for demo, random string)
	token, err := GenerateRefreshToken() // Use a dedicated function in production
	if err != nil {
		return c.Status(500).SendString("Token generation error")
	}
	// Store token in DB (add a reset_token column or use cache)
	if err := h.store.UpdateResetToken(user.ID, token); err != nil {
		return c.Status(500).SendString("Failed to store reset token")
	}
	// Send email with reset link
	if err := sendResetEmail(user.Email, token); err != nil {
		return c.Status(500).SendString("Failed to send email")
	}
	return c.SendString("Password reset link sent to your email.")
}

// Reset Password: Use token to set new password
func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	type inputStruct struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	input := new(inputStruct)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).SendString("Invalid input")
	}
	user, err := h.store.GetUserByResetToken(input.Token)
	if err != nil {
		return c.Status(400).SendString("Invalid or expired token")
	}
	hash, err := HashPassword(input.NewPassword)
	if err != nil {
		return c.Status(500).SendString("Password hash error")
	}
	if err := h.store.UpdatePassword(user.ID, hash); err != nil {
		return c.Status(500).SendString("Failed to update password")
	}
	// Invalidate token
	h.store.UpdateResetToken(user.ID, "")
	return c.SendString("Password reset successful.")
}

func sendResetEmail(toEmail, token string) error {
	from := os.Getenv("GMAIL_USER")
	pass := os.Getenv("GMAIL_PASS")
	frontendURL := os.Getenv("FRONTEND_RESET_URL_BASE") + token
	msg := fmt.Sprintf("Subject: Password Reset Request\r\n\r\nClick the link below to reset your password:\r\n%s", frontendURL)
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"
	auth := smtp.PlainAuth("", from, pass, smtpHost)
	return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{toEmail}, []byte(msg))
}
