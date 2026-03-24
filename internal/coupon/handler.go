package coupon

import (
	"defab-erp/internal/core/httperr"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// CREATE
func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateCouponInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid payload")
	}

	if in.Code == "" {
		return httperr.BadRequest(c, "coupon code required")
	}

	id, err := h.store.Create(in)
	if err != nil {
		log.Println("coupon create error:", err)
		return httperr.Internal(c)
	}

	return c.Status(201).JSON(fiber.Map{
		"id": id,
		"message": "Coupon created",
	})
}

// LIST
func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	rows, err := h.store.List(limit, offset)
	if err != nil {
		return httperr.Internal(c)
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, code, dtype, created string
		var value float64
		var active bool
		var start, end string

		rows.Scan(&id, &code, &dtype, &value, &start, &end, &active, &created)

		out = append(out, fiber.Map{
			"id": id,
			"code": code,
			"discount_type": dtype,
			"discount_value": value,
			"is_active": active,
			"start_date": start,
			"end_date": end,
		})
	}

	return c.JSON(out)
}

// GET
func (h *Handler) Get(c *fiber.Ctx) error {
	row := h.store.Get(c.Params("id"))

	var (
		id, code, description, discountType string
		startDate, endDate, createdAt       time.Time
		discountValue, minOrder, maxDiscount float64
		usageLimit, usagePerCustomer         int
		isActive                              bool
	)

	err := row.Scan(
		&id,
		&code,
		&description,
		&discountType,
		&discountValue,
		&minOrder,
		&maxDiscount,
		&startDate,
		&endDate,
		&usageLimit,
		&usagePerCustomer,
		&isActive,
		&createdAt,
	)

	if err != nil {
		return httperr.NotFound(c, "Coupon not found")
	}

	return c.JSON(fiber.Map{
		"id":                 id,
		"code":               code,
		"description":        description,
		"discount_type":      discountType,
		"discount_value":     discountValue,
		"min_order_value":    minOrder,
		"max_discount_amount": maxDiscount,
		"start_date":         startDate,
		"end_date":           endDate,
		"usage_limit":        usageLimit,
		"usage_per_customer": usagePerCustomer,
		"is_active":          isActive,
		"created_at":         createdAt,
	})
}

// UPDATE
func (h *Handler) Update(c *fiber.Ctx) error {
	var in UpdateCouponInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid payload")
	}

	if err := h.store.Update(c.Params("id"), in); err != nil {
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{"message": "Coupon updated"})
}

// ACTIVATE / DEACTIVATE
func (h *Handler) Activate(c *fiber.Ctx) error {
	return h.toggle(c, true)
}
func (h *Handler) Deactivate(c *fiber.Ctx) error {
	return h.toggle(c, false)
}

func (h *Handler) toggle(c *fiber.Ctx, active bool) error {
	if err := h.store.SetActive(c.Params("id"), active); err != nil {
		return httperr.Internal(c)
	}

	msg := "Coupon deactivated"
	if active {
		msg = "Coupon activated"
	}

	return c.JSON(fiber.Map{"message": msg})
}



//    Attach Coupon → Variants

func (h *Handler) AttachVariants(c *fiber.Ctx) error {
	couponID := c.Params("id")

	var in struct {
		VariantIDs []string `json:"variant_ids"`
	}

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "invalid payload")
	}

	if len(in.VariantIDs) == 0 {
		return httperr.BadRequest(c, "variant_ids required")
	}

	if err := h.store.AttachVariants(couponID, in.VariantIDs); err != nil {
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"message": "variants attached to coupon",
	})
}


// Attach Coupon Categories

func (h *Handler) AttachCategories(c *fiber.Ctx) error {
	couponID := c.Params("id")

	var in struct {
		CategoryIDs []string `json:"category_ids"`
	}

	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "invalid payload")
	}

	if len(in.CategoryIDs) == 0 {
		return httperr.BadRequest(c, "category_ids required")
	}

	if err := h.store.AttachCategories(couponID, in.CategoryIDs); err != nil {
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"message": "categories attached to coupon",
	})
}



// remove variant from coupon

func (h *Handler) RemoveVariant(c *fiber.Ctx) error {
	id := c.Params("mappingId")

	if err := h.store.RemoveVariant(id); err != nil {
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"message": "variant removed from coupon",
	})
}



//    Remove Category from Coupon

func (h *Handler) RemoveCategory(c *fiber.Ctx) error {
	id := c.Params("mappingId")

	if err := h.store.RemoveCategory(id); err != nil {
		return httperr.Internal(c)
	}

	return c.JSON(fiber.Map{
		"message": "category removed from coupon",
	})
}
