package auth

import (
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

// Injecting the Store interface is a good practice for testing
type AuthHandler struct {
	store *Store
}

func NewHandler(s *Store) *AuthHandler {
	return &AuthHandler{store: s}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	input := new(RegisterInput)

	if err := c.BodyParser(input); err != nil {
		return c.Status(400).SendString("Invalid input")
	}

	hash, err := HashPassword(input.Password)
	if err != nil {
		return c.Status(500).SendString("hash error")
	}

	u := &model.User{
		Name:         input.Name,
		Email:        input.Email,
		PasswordHash: hash,
		RoleID:       input.RoleID,
		BranchID:     input.BranchID,
	}

	if err := h.store.CreateUser(u); err != nil {
	return c.Status(500).SendString(err.Error())
	}

	fullUser, err := h.store.GetUserByEmail(u.Email)
	if err != nil {
	return c.Status(500).SendString(err.Error())
	}

	fullUser.PasswordHash = ""
	return c.JSON(fullUser)

}


func (h *AuthHandler) Login(c *fiber.Ctx) error {
	input := new(LoginInput)

	if err := c.BodyParser(input); err != nil {
		return c.Status(400).SendString("invalid input")
	}

	// find user
	u, err := h.store.GetUserByEmail(input.Email)
	if err != nil {
		return c.Status(401).SendString("invalid credentials")
	}

	// check password
	if !CheckPassword(u.PasswordHash, input.Password) {
		return c.Status(401).SendString("invalid credentials")
	}

	// generate token
	token, err := GenerateJWT(u.ID.String(), u.Role.Name)
	if err != nil {
		return c.Status(500).SendString("token error")
	}

	u.PasswordHash = ""

	return c.JSON(fiber.Map{
		"token": token,
		"user":  u,
	})
}

