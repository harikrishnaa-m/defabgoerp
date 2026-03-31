package middleware

import (
	"database/sql"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// EcomCustomer is the lightweight struct stored in c.Locals("ecom_customer").
type EcomCustomer struct {
	ID    string
	Email string
}

// EcomJWTProtected validates an ecom customer JWT token.
func EcomJWTProtected(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{"error": "missing auth header"})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(401).JSON(fiber.Map{"error": "invalid auth header"})
		}

		token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "invalid claims"})
		}

		// Ecom tokens have role = "ecom_customer"
		role, _ := claims["role"].(string)
		if role != "ecom_customer" {
			return c.Status(403).JSON(fiber.Map{"error": "not an ecom customer token"})
		}

		customerID, _ := claims["user_id"].(string)
		email, _ := claims["email"].(string)

		// Verify customer still exists and is active
		var isActive bool
		err = db.QueryRow(`SELECT is_active FROM ecom_customers WHERE id = $1`, customerID).Scan(&isActive)
		if err != nil || !isActive {
			return c.Status(401).JSON(fiber.Map{"error": "account not found or deactivated"})
		}

		c.Locals("ecom_customer", &EcomCustomer{
			ID:    customerID,
			Email: email,
		})

		return c.Next()
	}
}
