package variant

import (
	"defab-erp/internal/core/storage"

	"github.com/gofiber/fiber/v2"
)

//
// POST /variants/:id/images
//

func (h *Handler) AddImages(c *fiber.Ctx) error {

	variantID := c.Params("id")

	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(400).SendString("invalid form")
	}

	files := form.File["images"]
	if len(files) == 0 {
		return c.Status(400).SendString("no images provided")
	}

	var urls []string

	for _, f := range files {

		data, fname, err := storage.ProcessImage(f)
		if err != nil {
			continue
		}

		url, err := storage.UploadFile(
			"variants/"+fname,
			data,
			f.Header.Get("Content-Type"),
		)
		if err != nil {
			continue
		}

		_ = h.store.InsertImage(variantID, url)
		urls = append(urls, url)
	}

	return c.JSON(fiber.Map{
		"message": "variant images added",
		"count":   len(urls),
		"urls":    urls,
	})
}

//
// GET /variants/:id/images
//

func (h *Handler) ListImages(c *fiber.Ctx) error {

	rows, err := h.store.ListImages(c.Params("id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, url, created string
		rows.Scan(&id, &url, &created)

		out = append(out, fiber.Map{
			"id":  id,
			"url": url,
		})
	}

	return c.JSON(out)
}

//
// DELETE /variants/images/:imageId
//

func (h *Handler) DeleteImage(c *fiber.Ctx) error {

	imageID := c.Params("imageId")

	url, err := h.store.GetImage(imageID)
	if err != nil {
		return c.Status(404).SendString("image not found")
	}

	key := storage.ExtractKey(url)

	_ = storage.DeleteFile(key) // ignore error — DB delete still proceeds

	if err := h.store.DeleteImage(imageID); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.JSON(fiber.Map{
		"message": "variant image deleted",
	})
}
