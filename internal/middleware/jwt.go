package middleware

import (
	"os"
	"strings"

	"defab-erp/internal/auth"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func JWTProtected(store *auth.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {

		// 1️⃣ Read header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).SendString("missing auth header")
		}

		// 2️⃣ Expect: Bearer TOKEN
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 {
			return c.Status(401).SendString("invalid auth header")
		}

		tokenStr := parts[1]

		// 3️⃣ Parse token
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			return c.Status(401).SendString("invalid token")
		}

		// 4️⃣ Extract claims
		claims := token.Claims.(jwt.MapClaims)

		userID := claims["user_id"].(string)

		// 5️⃣ Load user from DB
		u, err := store.GetUserByID(userID)
		if err != nil {
			return c.Status(401).SendString("user not found")
		}

		// 6️⃣ Attach to context
		c.Locals("user", u)

		return c.Next()
	}
}
