package role

import (
	"defab-erp/internal/core/model"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var in struct {
		Name        string `json:"name"`
		Permissions string `json:"permissions"`
	}

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad input"})
	}

	if in.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name required"})
	}

	if !model.ValidRoles[in.Name] {
		return c.Status(400).JSON(fiber.Map{
			"error": fmt.Sprintf("invalid role name '%s'. allowed: SuperAdmin, StoreManager, SalesPerson, AccountsManager", in.Name),
		})
	}

	exists, err := h.store.ExistsByName(in.Name)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if exists {
		return c.Status(409).JSON(fiber.Map{"error": fmt.Sprintf("role '%s' already exists", in.Name)})
	}

	if err := h.store.Create(in.Name, in.Permissions); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{"message": fmt.Sprintf("role '%s' created", in.Name)})
}

func (h *Handler) List(c *fiber.Ctx) error {
	roles, err := h.store.List()
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.JSON(roles)
}
