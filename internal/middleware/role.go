package middleware

import (
	"fmt"
	"log"

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

		log.Printf("[DEBUG] RequireRole: user=%s role='%s' allowed=%v path=%s",
			user.Email, user.Role.Name, roles, c.Path())

		for _, r := range roles {
			if user.Role.Name == r {
				return c.Next()
			}
		}

		return c.Status(403).SendString(fmt.Sprintf("forbidden: insufficient role (got '%s', need one of %v)", user.Role.Name, roles))
	}
}
