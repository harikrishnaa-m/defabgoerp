package product

import (
	"database/sql"
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

// 	name := c.FormValue("name")
// 	categoryID := c.FormValue("category_id")
// 	brand := c.FormValue("brand")

// 	if name == "" || categoryID == "" {
// 		return c.Status(400).SendString("name & category required")
// 	}

// 	file, err := c.FormFile("image")
// 	if err != nil {
// 		return c.Status(400).SendString("image required")
// 	}

// 	// ✅ process image
// 	data, filename, err := storage.ProcessImage(file)
// 	if err != nil {
// 		return c.Status(400).SendString(err.Error())
// 	}

// 	key := "products/" + filename

// 	url, err := storage.UploadFile(
// 		key,
// 		data,
// 		file.Header.Get("Content-Type"),
// 	)
// 	if err != nil {
// 		return c.Status(500).SendString(err.Error())
// 	}

// 	in := CreateProductInput{
// 		Name:       name,
// 		CategoryID: categoryID,
// 		Brand:      brand,
// 		ImageURL:   url,
// 	}

// 	if err := h.store.Create(in); err != nil {
// 		return c.Status(500).SendString(err.Error())
// 	}

// 	return c.Status(201).JSON(fiber.Map{
// 		"message": "product created",
// 		"image":   url,
// 	})
// }

func (h *Handler) Create(c *fiber.Ctx) error {

	name := c.FormValue("name")
	categoryID := c.FormValue("category_id")
	brand := c.FormValue("brand")

	description := c.FormValue("description")
	fabricComposition := c.FormValue("fabric_composition")
	pattern := c.FormValue("pattern")
	occasion := c.FormValue("occasion")
	careInstructions := c.FormValue("care_instructions")

	// MAIN IMAGE (optional)
	var mainURL string
	mainFile, err := c.FormFile("main_image")
	if err == nil {
		data, filename, pErr := storage.ProcessImage(mainFile)
		if pErr == nil {
			url, uErr := storage.UploadFile(
				"products/"+filename,
				data,
				mainFile.Header.Get("Content-Type"),
			)
			if uErr == nil {
				mainURL = url
			}
		}
	}

	in := CreateProductInput{
		Name:              name,
		CategoryID:        categoryID,
		Brand:             brand,
		Description:       description,
		FabricComposition: fabricComposition,
		Pattern:           pattern,
		Occasion:          occasion,
		CareInstructions:  careInstructions,
	}

	productID, err := h.store.CreateProduct(in, mainURL)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	form, _ := c.MultipartForm()
	if form != nil {
		files := form.File["gallery_images"]

		for _, f := range files {

			d, fname, err := storage.ProcessImage(f)
			if err != nil {
				continue
			}

			url, err := storage.UploadFile(
				"products/"+fname,
				d,
				f.Header.Get("Content-Type"),
			)
			if err != nil {
				continue
			}

			_ = h.store.InsertProductImage(productID, url)
		}
	}

	return c.Status(201).JSON(fiber.Map{
		"id":      productID,
		"message": "product created",
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
		var id, name, brand, mainImage, uom, created, description, fabricComposition, pattern, occasion, careInstructions string
		var web, stitched, active bool
		var cid, cname string

		rows.Scan(&id, &name, &brand, &mainImage, &web, &stitched, &uom, &created, &cid, &cname, &active, &description, &fabricComposition, &pattern, &occasion, &careInstructions)

		out = append(out, fiber.Map{
			"id":                 id,
			"name":               name,
			"brand":              brand,
			"main_image_url":     mainImage,
			"uom":                uom,
			"is_web_visible":     web,
			"is_stitched":        stitched,
			"is_active":          active,
			"created_at":         created,
			"description":        description,
			"fabric_composition": fabricComposition,
			"pattern":            pattern,
			"occasion":           occasion,
			"care_instructions":  careInstructions,
			"category": fiber.Map{
				"id":   cid,
				"name": cname,
			},
		})
	}

	total, _ := h.store.CountActive()

	return c.JSON(fiber.Map{
		"data":  out,
		"page":  page,
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

	var pid, name, uom, description, fabricComposition, pattern, occasion, careInstructions string
	var brand, mainImage, cid, cname sql.NullString
	var web, stitched, active bool

	err := row.Scan(
		&pid,
		&name,
		&brand,
		&mainImage,
		&web,
		&stitched,
		&uom,
		&active,
		&cid,
		&cname,
		&description,
		&fabricComposition,
		&pattern,
		&occasion,
		&careInstructions,
	)

	if err != nil {
		return c.Status(404).SendString(err.Error()) // show real error for now
	}

	// gallery
	// imgRows, err := h.store.ListImages(pid)
	// if err != nil {
	// 	return c.Status(500).SendString(err.Error())
	// }
	// defer imgRows.Close()

	// var gallery []string
	// for imgRows.Next() {
	// 	var url string
	// 	imgRows.Scan(&url)
	// 	gallery = append(gallery, url)
	// }

	imgRows, err := h.store.ListImages(pid)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer imgRows.Close()

	var gallery []fiber.Map

	for imgRows.Next() {
		var imgID, url string
		imgRows.Scan(&imgID, &url)

		gallery = append(gallery, fiber.Map{
			"id":  imgID,
			"url": url,
		})
	}

	return c.JSON(fiber.Map{
		"id":                 pid,
		"name":               name,
		"brand":              brand.String,
		"main_image_url":     mainImage.String,
		"gallery":            gallery,
		"uom":                uom,
		"is_active":          active,
		"is_web_visible":     web,
		"is_stitched":        stitched,
		"description":        description,
		"fabric_composition": fabricComposition,
		"pattern":            pattern,
		"occasion":           occasion,
		"care_instructions":  careInstructions,
		"category": fiber.Map{
			"id":   cid.String,
			"name": cname.String,
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

	return c.JSON(fiber.Map{
		"message": "product updated",
	})
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
