package productdescription

import (
	"database/sql"
	"defab-erp/internal/core/storage"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}


func (h *Handler) Create(c *fiber.Ctx) error {

	productIDStr := c.FormValue("product_id")
	if productIDStr == "" {
		return c.Status(400).SendString("product_id required")
	}

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		return c.Status(400).SendString("invalid product_id")
	}

	// Optional text fields
	description := c.FormValue("description")
	fabric := c.FormValue("fabric_composition")
	pattern := c.FormValue("pattern")
	occasion := c.FormValue("occasion")
	care := c.FormValue("care_instructions")

	var (
		length, width, blouse float64
	)
	fmt.Sscan(c.FormValue("length"), &length)
	fmt.Sscan(c.FormValue("width"), &width)
	fmt.Sscan(c.FormValue("blouse_piece"), &blouse)

	// ✅ OPTIONAL IMAGE
	var sizeChartURL string

	file, err := c.FormFile("size_chart_image")
	if err == nil {
		data, filename, err := storage.ProcessImage(file)
		if err != nil {
			return c.Status(400).SendString(err.Error())
		}

		sizeChartURL, err = storage.UploadFile(
			"product-descriptions/"+filename,
			data,
			file.Header.Get("Content-Type"),
		)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
	}

	in := CreateProductDescriptionInput{
		ProductID:         productID,
		Description:       description,
		FabricComposition: fabric,
		Pattern:           pattern,
		Occasion:          occasion,
		CareInstructions:  care,
		Length:            length,
		Width:             width,
		BlousePiece:       blouse,
		SizeChartImage:    sizeChartURL,
	}

	if err := h.store.Create(in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(201)
}





func (h *Handler) Get(c *fiber.Ctx) error {
	productID, err := uuid.Parse(c.Params("productId"))
	if err != nil {
		return c.Status(400).SendString("invalid product id")
	}

	row, _ := h.store.Get(productID)

	var out map[string]any = make(map[string]any)

	var (
		id, pid uuid.UUID
		desc, fabric, pattern, occasion, care, img sql.NullString
		length, width, blouse sql.NullFloat64
		created, updated string
	)

	err = row.Scan(
		&id, &pid, &desc, &fabric,
		&pattern, &occasion, &care,
		&length, &width, &blouse, &img,
		&created, &updated,
	)

	if err == sql.ErrNoRows {
		return c.SendStatus(404)
	}
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	out["id"] = id
	out["product_id"] = pid
	out["description"] = desc.String
	out["fabric_composition"] = fabric.String
	out["pattern"] = pattern.String
	out["occasion"] = occasion.String
	out["care_instructions"] = care.String
	out["length"] = length.Float64
	out["width"] = width.Float64
	out["blouse_piece"] = blouse.Float64
	out["size_chart_image"] = img.String
	out["created_at"] = created
	out["updated_at"] = updated

	return c.JSON(out)
}

func (h *Handler) Update(c *fiber.Ctx) error {

	productID, err := uuid.Parse(c.Params("productId"))
	if err != nil {
		return c.Status(400).SendString("invalid product id")
	}

	var in UpdateProductDescriptionInput

	// text fields
	in.Description = ptrString(c.FormValue("description"))
	in.FabricComposition = ptrString(c.FormValue("fabric_composition"))
	in.Pattern = ptrString(c.FormValue("pattern"))
	in.Occasion = ptrString(c.FormValue("occasion"))
	in.CareInstructions = ptrString(c.FormValue("care_instructions"))

	in.Length = ptrFloat(c.FormValue("length"))
	in.Width = ptrFloat(c.FormValue("width"))
	in.BlousePiece = ptrFloat(c.FormValue("blouse_piece"))

	// ✅ OPTIONAL IMAGE UPDATE
	file, err := c.FormFile("size_chart_image")
	if err == nil {
		data, filename, err := storage.ProcessImage(file)
		if err != nil {
			return c.Status(400).SendString(err.Error())
		}

		url, err := storage.UploadFile(
			"product-descriptions/"+filename,
			data,
			file.Header.Get("Content-Type"),
		)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		in.SizeChartImage = &url
	}

	if err := h.store.Update(productID, in); err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.SendStatus(200)
}



func ptrString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func ptrFloat(v string) *float64 {
	if v == "" {
		return nil
	}
	var f float64
	if _, err := fmt.Sscan(v, &f); err != nil {
		return nil
	}
	return &f
}