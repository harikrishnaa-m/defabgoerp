package auth

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *AuthHandler) {
	auth := r.Group("/auth")
	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)
}
