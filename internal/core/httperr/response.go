package httperr

import "github.com/gofiber/fiber/v2"

func BadRequest(c *fiber.Ctx, msg string) error {
	return c.Status(400).JSON(fiber.Map{
		"error":   "BAD_REQUEST",
		"message": msg,
	})
}

func NotFound(c *fiber.Ctx, msg string) error {
	return c.Status(404).JSON(fiber.Map{
		"error":   "NOT_FOUND",
		"message": msg,
	})
}

func Forbidden(c *fiber.Ctx, msg string) error {
	return c.Status(403).JSON(fiber.Map{
		"error":   "FORBIDDEN",
		"message": msg,
	})
}

func Conflict(c *fiber.Ctx, msg string) error {
	return c.Status(409).JSON(fiber.Map{
		"error":   "CONFLICT",
		"message": msg,
	})
}

func Internal(c *fiber.Ctx) error {
	return c.Status(500).JSON(fiber.Map{
		"error":   "INTERNAL_ERROR",
		"message": "Something went wrong. Please try again.",
	})
}
