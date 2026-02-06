package middleware

import (
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {

		u := c.Locals("user")
		if u == nil {
			return c.Status(401).SendString("not authenticated")
		}

		user := u.(*model.User)

		for _, r := range roles {
			if user.Role.Name == r {
				return c.Next()
			}
		}

		return c.Status(403).SendString("forbidden: insufficient role")
	}
}
