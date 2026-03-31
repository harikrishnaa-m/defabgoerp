package customer

import "github.com/gofiber/fiber/v2"

// RegisterPublicRoutes registers unauthenticated routes (register, login).
func RegisterPublicRoutes(r fiber.Router, h *Handler) {
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/google", h.GoogleSignIn)
}

// RegisterProtectedRoutes registers authenticated customer routes.
func RegisterProtectedRoutes(r fiber.Router, h *Handler) {
	// Profile
	r.Get("/profile", h.GetProfile)
	r.Patch("/profile", h.UpdateProfile)

	// Addresses
	r.Post("/addresses", h.AddAddress)
	r.Get("/addresses", h.ListAddresses)
	r.Put("/addresses/:id", h.UpdateAddress)
	r.Delete("/addresses/:id", h.DeleteAddress)
}
