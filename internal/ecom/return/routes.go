package ecomreturn

import "github.com/gofiber/fiber/v2"

// RegisterCustomerRoutes registers customer-facing return routes on ecomProtected.
func RegisterCustomerRoutes(ecomProtected fiber.Router, h *Handler) {
	ecomProtected.Post("/returns", h.RequestReturn)
	ecomProtected.Get("/returns", h.ListReturns)
	ecomProtected.Get("/returns/:id", h.GetReturn)
}

// RegisterAdminRoutes registers admin routes on the admin group.
func RegisterAdminRoutes(adminGroup fiber.Router, h *Handler) {
	adminGroup.Get("", h.AdminListReturns)
	adminGroup.Patch("/:id/approve", h.AdminApproveReturn)
	adminGroup.Patch("/:id/reject", h.AdminRejectReturn)
}
