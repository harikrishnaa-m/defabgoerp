package auth

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *AuthHandler) {
	auth := r.Group("/auth")
	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)
	auth.Post("/refresh", h.Refresh)
	auth.Post("/logout", h.Logout)
	auth.Post("/forgot-password", h.ForgotPassword)
	auth.Post("/reset-password", h.ResetPassword)
}
