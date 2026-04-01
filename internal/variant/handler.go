package variant

import (
	"defab-erp/internal/core/storage"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Create(c *fiber.Ctx) error {

	productID := c.FormValue("product_id")
	name := c.FormValue("name")

	priceStr := c.FormValue("price")
	costPriceStr := c.FormValue("cost_price")

	price, _ := strconv.ParseFloat(priceStr, 64)
	costPrice, _ := strconv.ParseFloat(costPriceStr, 64)

	form, _ := c.MultipartForm()
	if form == nil {
		// no multipart form — continue with empty collections
		in := CreateVariantInput{
			ProductID: productID,
			Name:      name,
			Price:     price,
			CostPrice: costPrice,
		}
		id, sku, variantCode, err := h.store.Create(in)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(201).JSON(fiber.Map{
			"message":      "variant created",
			"id":           id,
			"sku":          sku,
			"variant_code": variantCode,
		})
	}

	attrIDs := form.Value["attribute_value_ids[]"]

	var cleanAttrIDs []string
	for _, id := range attrIDs {
		if id != "" {
			cleanAttrIDs = append(cleanAttrIDs, id)
		}
	}

	files := form.File["images"]

	var imagePaths []string

	for _, file := range files {

		data, fname, err := storage.ProcessImage(file)
		if err != nil {
			fmt.Println("image processing error:", err)
			continue
		}

		url, err := storage.UploadFile(
			"variants/"+fname,
			data,
			file.Header.Get("Content-Type"),
		)
		if err != nil {
			fmt.Println("upload error:", err)
			continue
		}

		imagePaths = append(imagePaths, url)
	}

	fmt.Println("Attribute IDs:", cleanAttrIDs)
	fmt.Println("Files count:", len(files))
	fmt.Println("Image paths:", imagePaths)

	in := CreateVariantInput{
		ProductID:         productID,
		Name:              name,
		Price:             price,
		CostPrice:         costPrice,
		AttributeValueIDs: cleanAttrIDs,
		ImagePaths:        imagePaths,
	}

	id, sku, variantCode, err := h.store.Create(in)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message":      "variant created",
		"id":           id,
		"sku":          sku,
		"variant_code": variantCode,
	})
}

func (h *Handler) Generate(c *fiber.Ctx) error {
	var in GenerateVariantsInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "bad input"})
	}

	// Convert map to ordered groups for cartesian product
	var groups [][]string
	var attrOrder []string
	for attrID, valueIDs := range in.AttributeValues {
		groups = append(groups, valueIDs)
		attrOrder = append(attrOrder, attrID)
	}
	// Debug output
	fmt.Printf("Generate handler: attrOrder = %v\n", attrOrder)
	fmt.Printf("Generate handler: groups = %v\n", groups)

	ids, err := h.store.GenerateWithAttrOrder(in.ProductID, in.BasePrice, attrOrder, groups)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "variants generated",
		"count":   len(ids),
	})
}

func (h *Handler) ListByProduct(c *fiber.Ctx) error {
	rows, err := h.store.ListByProduct(c.Params("productId"))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var out []fiber.Map

	for rows.Next() {
		var id, name, sku string
		var variantCode int
		var price, cost float64
		var active bool

		rows.Scan(&id, &variantCode, &name, &sku, &price, &cost, &active)

		out = append(out, fiber.Map{
			"id":           id,
			"variant_code": variantCode,
			"name":         name,
			"sku":          sku,
			"price":        price,
			"cost_price":   cost,
			"is_active":    active,
		})
	}

	return c.JSON(out)
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	row, _ := h.store.Get(id)
	var variantID, productID, name, sku string
	var variantCode int
	var price, costPrice float64
	var isActive bool
	if err := row.Scan(&variantID, &productID, &variantCode, &name, &sku, &price, &costPrice, &isActive); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "variant not found"})
	}

	// images
	imgRows, err := h.store.ListImages(variantID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer imgRows.Close()
	var images []fiber.Map
	for imgRows.Next() {
		var imgID, url, created string
		imgRows.Scan(&imgID, &url, &created)
		images = append(images, fiber.Map{"id": imgID, "url": url})
	}

	// attributes
	attributes, err := h.store.GetVariantAttributes(variantID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"id":           variantID,
		"product_id":   productID,
		"variant_code": variantCode,
		"name":         name,
		"sku":          sku,
		"price":        price,
		"cost_price":   costPrice,
		"is_active":    isActive,
		"images":       images,
		"attributes":   attributes,
	})
}

func (h *Handler) Update(c *fiber.Ctx) error {
	variantID := c.Params("id")

	contentType := string(c.Request().Header.ContentType())

	var in UpdateVariantInput

	// Support both JSON and multipart form
	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid form"})
		}

		if v := c.FormValue("name"); v != "" {
			in.Name = &v
		}
		if v := c.FormValue("price"); v != "" {
			p, err := strconv.ParseFloat(v, 64)
			if err == nil {
				in.Price = &p
			}
		}
		if v := c.FormValue("cost_price"); v != "" {
			cp, err := strconv.ParseFloat(v, 64)
			if err == nil {
				in.CostPrice = &cp
			}
		}

		for _, id := range form.Value["attribute_value_ids[]"] {
			if id != "" {
				in.AttributeValueIDs = append(in.AttributeValueIDs, id)
			}
		}
		// fallback: try without []
		if len(in.AttributeValueIDs) == 0 {
			for _, id := range form.Value["attribute_value_ids"] {
				if id != "" {
					in.AttributeValueIDs = append(in.AttributeValueIDs, id)
				}
			}
		}
		fmt.Println("Update: attribute_value_ids =", in.AttributeValueIDs)

		// Handle new image uploads
		files := form.File["images"]
		fmt.Println("Update: image files count =", len(files))
		for _, file := range files {
			data, fname, err := storage.ProcessImage(file)
			if err != nil {
				fmt.Println("Update: image process error:", err)
				continue
			}
			url, err := storage.UploadFile("variants/"+fname, data, file.Header.Get("Content-Type"))
			if err != nil {
				fmt.Println("Update: image upload error:", err)
				continue
			}
			fmt.Println("Update: inserted image url =", url)
			_ = h.store.InsertImage(variantID, url)
		}
	} else {
		if err := c.BodyParser(&in); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "bad input"})
		}
	}

	if err := h.store.Update(variantID, in); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "variant updated"})
}

func (h *Handler) Deactivate(c *fiber.Ctx) error {
	return h.toggle(c, false)
}

func (h *Handler) Activate(c *fiber.Ctx) error {
	return h.toggle(c, true)
}

func (h *Handler) toggle(c *fiber.Ctx, active bool) error {
	if err := h.store.SetActive(c.Params("id"), active); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "status updated"})
}

func (h *Handler) BackfillVariantCodes(c *fiber.Ctx) error {
	count, err := h.store.BackfillVariantCodes()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("assigned variant_code to %d variants", count),
		"count":   count,
	})
}
