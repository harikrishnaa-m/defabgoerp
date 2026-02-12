package product

import (
	"defab-erp/internal/core/storage"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

//
// CREATE
//

// func (h *Handler) Create(c *fiber.Ctx) error {
// 	var in CreateProductInput

// 	if err := c.BodyParser(&in); err != nil {
// 		return c.Status(400).SendString("bad input")
// 	}

// 	if in.Name == "" || in.CategoryID == "" {
// 		return c.Status(400).SendString("name & category required")
// 	}

// 	if err := h.store.Create(in); err != nil {
// 		return c.Status(500).SendString(err.Error())
// 	}

// 	return c.SendStatus(201)
// }



func (h *Handler) Create(c *fiber.Ctx) error {

	name := c.FormValue("name")
	categoryID := c.FormValue("category_id")
	brand := c.FormValue("brand")

	if name == "" || categoryID == "" {
		return c.Status(400).SendString("name & category required")
	}

	file, err := c.FormFile("image")
	if err != nil {
		return c.Status(400).SendString("image required")
	}

	// ✅ process image
	data, filename, err := storage.ProcessImage(file)
	if err != nil {
		return c.Status(400).SendString(err.Error())
	}

	key := "products/" + filename

	url, err := storage.UploadFile(
		key,
		data,
		file.Header.Get("Content-Type"),
	)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	in := CreateProductInput{
		Name:       name,
		CategoryID: categoryID,
		Brand:      brand,
		ImageURL:   url,
	}

	if err := h.store.Create(in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "product created",
		"image":   url,
	})
}



//
// LIST
//

func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	rows, err := h.store.List(limit, offset)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, name, brand, image, uom, created string
		var web, stitched bool
		var cid, cname string

		rows.Scan(&id, &name, &brand, &image, &web, &stitched, &uom, &created, &cid, &cname)

		out = append(out, fiber.Map{
			"id": id,
			"name": name,
			"brand": brand,
			"image": image,
			"uom": uom,
			"is_web_visible": web,
			"is_stitched": stitched,
			"category": fiber.Map{
				"id": cid,
				"name": cname,
			},
		})
	}

	total, _ := h.store.CountActive()

	return c.JSON(fiber.Map{
		"data": out,
		"page": page,
		"limit": limit,
		"total": total,
	})
}

//
// GET
//

func (h *Handler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	row := h.store.Get(id)

	var pid, name, brand, image, uom, cid, cname string
	var web, stitched, active bool

	err := row.Scan(&pid, &name, &brand, &image, &web, &stitched, &uom, &active, &cid, &cname)
	if err != nil {
		return c.Status(404).SendString("not found")
	}

	return c.JSON(fiber.Map{
		"id": pid,
		"name": name,
		"brand": brand,
		"image": image,
		"uom": uom,
		"is_active": active,
		"is_web_visible": web,
		"is_stitched": stitched,
		"category": fiber.Map{
			"id": cid,
			"name": cname,
		},
	})
}

//
// UPDATE
//

func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var in UpdateProductInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).SendString("bad input")
	}

	if err := h.store.Update(id, in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}

//
// SOFT DELETE / RESTORE
//

func (h *Handler) Deactivate(c *fiber.Ctx) error {
	return h.toggle(c, false)
}

func (h *Handler) Activate(c *fiber.Ctx) error {
	return h.toggle(c, true)
}

func (h *Handler) toggle(c *fiber.Ctx, active bool) error {
	id := c.Params("id")

	if err := h.store.SetActive(id, active); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}
