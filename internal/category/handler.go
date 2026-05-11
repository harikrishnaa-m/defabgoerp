package category

import "github.com/gofiber/fiber/v2"

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

//
// CREATE
//

func (h *Handler) Create(c *fiber.Ctx) error {
	var in CreateCategoryInput

	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if in.Name == "" {
		return c.Status(400).SendString("name required")
	}

	if err := h.store.Create(in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(201)
}

//
// LIST ACTIVE ONLY
//

func (h *Handler) List(c *fiber.Ctx) error {
	search := c.Query("search")
	pageStr := c.Query("page")
	limitStr := c.Query("limit")

	// If page or limit is provided, use paginated response (preserves old frontend behaviour).
	if pageStr != "" || limitStr != "" {
		page := c.QueryInt("page", 1)
		limit := c.QueryInt("limit", 20)
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}
		offset := (page - 1) * limit

		rows, err := h.store.ListActivePaged(search, limit, offset)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		defer rows.Close()

		var out []fiber.Map
		for rows.Next() {
			var id, name, imageURL string
			var active bool
			var productsCount int
			rows.Scan(&id, &name, &active, &productsCount, &imageURL)
			out = append(out, fiber.Map{
				"id":             id,
				"name":           name,
				"is_active":      active,
				"products_count": productsCount,
				"image_url":      imageURL,
			})
		}
		total, _ := h.store.CountActive(search)
		return c.JSON(fiber.Map{"data": out, "page": page, "limit": limit, "total": total})
	}

	// No pagination params — return all.
	rows, err := h.store.ListActive(search)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var out []fiber.Map
	for rows.Next() {
		var id, name, imageURL string
		var active bool
		var productsCount int
		rows.Scan(&id, &name, &active, &productsCount, &imageURL)
		out = append(out, fiber.Map{
			"id":             id,
			"name":           name,
			"is_active":      active,
			"products_count": productsCount,
			"image_url":      imageURL,
		})
	}
	return c.JSON(out)
}

//
// LIST PRODUCTS WITHIN A CATEGORY
//

func (h *Handler) ListProducts(c *fiber.Ctx) error {
	categoryID := c.Params("id")
	search := c.Query("search")
	pageStr := c.Query("page")
	limitStr := c.Query("limit")

	// If page or limit is provided, use paginated response (preserves old frontend behaviour).
	if pageStr != "" || limitStr != "" {
		page := c.QueryInt("page", 1)
		limit := c.QueryInt("limit", 20)
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}
		offset := (page - 1) * limit

		rows, err := h.store.ListProductsByCategoryPaged(categoryID, search, limit, offset)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		defer rows.Close()

		var out []fiber.Map
		for rows.Next() {
			var id, name, brand, mainImage string
			var active bool
			rows.Scan(&id, &name, &brand, &mainImage, &active)
			out = append(out, fiber.Map{
				"id":             id,
				"name":           name,
				"brand":          brand,
				"main_image_url": mainImage,
				"is_active":      active,
			})
		}
		total, _ := h.store.CountProductsByCategory(categoryID, search)
		return c.JSON(fiber.Map{"data": out, "page": page, "limit": limit, "total": total})
	}

	// No pagination params — return all.
	rows, err := h.store.ListProductsByCategory(categoryID, search)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var out []fiber.Map
	for rows.Next() {
		var id, name, brand, mainImage string
		var active bool
		rows.Scan(&id, &name, &brand, &mainImage, &active)
		out = append(out, fiber.Map{
			"id":             id,
			"name":           name,
			"brand":          brand,
			"main_image_url": mainImage,
			"is_active":      active,
		})
	}
	return c.JSON(out)
}

//
// GET
//

func (h *Handler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	cid, name, active, productsCount, imageURL, err := h.store.Get(id)
	if err != nil {
		return c.Status(404).SendString("not found")
	}

	return c.JSON(fiber.Map{
		"id":             cid,
		"name":           name,
		"is_active":      active,
		"products_count": productsCount,
		"image_url":      imageURL,
	})
}

//
// UPDATE
//

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdateCategoryInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if err := h.store.Update(id, in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}

//
// SOFT DELETE
//

func (h *Handler) Deactivate(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.SetActive(id, false); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}

//
// RESTORE
//

func (h *Handler) Activate(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.SetActive(id, true); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}
