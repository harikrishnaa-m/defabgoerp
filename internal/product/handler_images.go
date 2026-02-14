package product

import (
	"defab-erp/internal/core/storage"

	"github.com/gofiber/fiber/v2"
)


func (h *Handler) ReplaceMainImage(c *fiber.Ctx) error {

	id := c.Params("id")

	file, err := c.FormFile("main_image")
	if err != nil {
		return c.Status(400).SendString("main_image required")
	}

	data, fname, err := storage.ProcessImage(file)
	if err != nil {
		return c.Status(400).SendString(err.Error())
	}

	url, err := storage.UploadFile(
		"products/"+fname,
		data,
		file.Header.Get("Content-Type"),
	)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	// get old image
	oldURL, _ := h.store.GetMainImage(id)

	// update db
	if err := h.store.UpdateMainImage(id, url); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	// delete old file
	if oldURL != "" {
		key := extractKey(oldURL)
		_ = storage.DeleteFile(key)
	}

	return c.JSON(fiber.Map{
		"message": "main image replaced",
		"url":     url,
	})
}


func (h *Handler) AddGalleryImages(c *fiber.Ctx) error {

	id := c.Params("id")

	form, err := c.MultipartForm()
	if err != nil || form == nil {
		return c.Status(400).SendString("no files")
	}

	files := form.File["gallery_images"]

	var added []string

	for _, f := range files {

		data, fname, err := storage.ProcessImage(f)
		if err != nil {
			continue
		}

		url, err := storage.UploadFile(
			"products/"+fname,
			data,
			f.Header.Get("Content-Type"),
		)
		if err != nil {
			continue
		}

		if err := h.store.InsertProductImage(id, url); err == nil {
			added = append(added, url)
		}
	}

	return c.JSON(fiber.Map{
		"message": "gallery images added",
		"count":   len(added),
		"urls":    added,
	})
}


func (h *Handler) DeleteGalleryImage(c *fiber.Ctx) error {

	imgID := c.Params("imageId")

	url, err := h.store.GetProductImage(imgID)
	if err != nil {
		return c.Status(404).SendString("image not found")
	}

	if err := h.store.DeleteProductImage(imgID); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	key := extractKey(url)
	_ = storage.DeleteFile(key)

	return c.JSON(fiber.Map{
		"message": "gallery image deleted",
	})
}
