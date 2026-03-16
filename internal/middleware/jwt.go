package middleware

import (
	"errors"
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

		// 4️⃣ If token expired, try auto-refresh via cookie
		if err != nil && errors.Is(err, jwt.ErrTokenExpired) {
			refreshToken := c.Cookies("refresh_token")
			if refreshToken == "" {
				return c.Status(401).SendString("token expired, no refresh token")
			}

			u, lookupErr := store.GetUserByRefreshToken(refreshToken)
			if lookupErr != nil {
				return c.Status(401).SendString("token expired, invalid refresh token")
			}

			// Generate new access token
			newAccessToken, genErr := auth.GenerateJWT(u.ID.String(), u.Role.Name)
			if genErr != nil {
				return c.Status(500).SendString("failed to generate new token")
			}

			// Rotate refresh token
			newRefreshToken, refErr := auth.GenerateRefreshToken()
			if refErr != nil {
				return c.Status(500).SendString("failed to generate refresh token")
			}
			store.UpdateRefreshToken(u.ID, newRefreshToken)

			// Set new refresh cookie
			c.Cookie(&fiber.Cookie{
				Name:     "refresh_token",
				Value:    newRefreshToken,
				HTTPOnly: true,
				Secure:   true,
				SameSite: "Strict",
				Path:     "/",
				MaxAge:   60 * 60 * 24 * 7,
			})

			// Return new access token in response header so frontend can pick it up
			c.Set("X-New-Access-Token", newAccessToken)

			u.PasswordHash = ""
			c.Locals("user", u)
			return c.Next()
		}

		if err != nil || !token.Valid {
			return c.Status(401).SendString("invalid token")
		}

		// 5️⃣ Extract claims
		claims := token.Claims.(jwt.MapClaims)

		userID := claims["user_id"].(string)

		// 6️⃣ Load user from DB
		u, err := store.GetUserByID(userID)
		if err != nil {
			return c.Status(401).SendString("user not found")
		}

		// 7️⃣ Attach to context
		c.Locals("user", u)

		return c.Next()
	}
}
