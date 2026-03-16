package supplier

import (
	"errors"
	"log"

	"defab-erp/internal/core/httperr"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// CREATE
func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateSupplierInput

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if in.Name == "" {
		return httperr.BadRequest(c, "Supplier name is required")
	}

	id, code, err := h.store.Create(in)
	if err != nil {
		log.Println("supplier create error:", err)

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return httperr.Conflict(
					c,
					"Supplier with this GST number already exists",
				)
			case "23503":
				return httperr.BadRequest(
					c,
					"Invalid reference data",
				)
			}
		}

		return httperr.Internal(c)
	}

	return c.Status(201).JSON(fiber.Map{
		"id":            id,
		"supplier_code": code,
		"message":       "Supplier created successfully",
	})
}

// LIST
func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	rows, err := h.store.List(limit, offset)
	if err != nil {
		log.Println("supplier list error:", err)
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, code, name, phone, email, address, gst string
		var created, updated string
		var active bool

		if err := rows.Scan(
			&id, &code, &name, &phone, &email, &address,
			&gst, &active, &created, &updated,
		); err != nil {
			log.Println("supplier scan error:", err)
			return httperr.Internal(c)
		}

		out = append(out, fiber.Map{
			"id":            id,
			"supplier_code": code,
			"name":          name,
			"phone":         phone,
			"email":         email,
			"address":       address,
			"gst_number":    gst,
			"is_active":     active,
			"created_at":    created,
			"updated_at":    updated,
		})
	}

	return c.JSON(out)
}

// GET
func (h *Handler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	row := h.store.Get(id)

	var sid, code, name, phone, email, address, gst, created, updated string
	var active bool

	if err := row.Scan(
		&sid, &code, &name, &phone, &email,
		&address, &gst, &active, &created, &updated,
	); err != nil {
		return httperr.NotFound(c, "Supplier not found")
	}

	return c.JSON(fiber.Map{
		"id":            sid,
		"supplier_code": code,
		"name":          name,
		"phone":         phone,
		"email":         email,
		"address":       address,
		"gst_number":    gst,
		"is_active":     active,
		"created_at":    created,
		"updated_at":    updated,
	})
}

// UPDATE
func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdateSupplierInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}

	if err := h.store.Update(id, in); err != nil {
		log.Println("supplier update error:", err)

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return httperr.Conflict(
				c,
				"GST number already used by another supplier",
			)
		}

		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"message": "Supplier updated successfully",
	})
}

// ACTIVATE / DEACTIVATE
func (h *Handler) Deactivate(c *fiber.Ctx) error {
	return h.toggle(c, false)
}

func (h *Handler) Activate(c *fiber.Ctx) error {
	return h.toggle(c, true)
}

func (h *Handler) toggle(c *fiber.Ctx, active bool) error {
	id := c.Params("id")

	if err := h.store.SetActive(id, active); err != nil {
		log.Println("supplier toggle error:", err)
		return httperr.Internal(c)
	}

	msg := "Supplier deactivated"
	if active {
		msg = "Supplier activated"
	}

	return c.JSON(fiber.Map{"message": msg})
}
