package supplierstatement

import (
	"log"
	"time"

	"defab-erp/internal/core/httperr"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// Get handles GET /supplier-statement
//
//	supplier_id  — required
//	date_from    — DD/MM/YYYY  (default: 01/01 of current year)
//	date_to      — DD/MM/YYYY  (default: today)
func (h *Handler) Get(c *fiber.Ctx) error {
	supplierID := c.Query("supplier_id")
	if supplierID == "" {
		return httperr.BadRequest(c, "supplier_id is required")
	}

	now := time.Now()
	defaultFrom := "01/01/" + now.Format("2006")
	defaultTo := now.Format("02/01/2006")

	rawFrom := c.Query("date_from", defaultFrom)
	rawTo := c.Query("date_to", defaultTo)

	// Parse DD/MM/YYYY → YYYY-MM-DD for Postgres
	fromDB, err := parseDMY(rawFrom)
	if err != nil {
		return httperr.BadRequest(c, "date_from must be DD/MM/YYYY")
	}
	toDB, err := parseDMY(rawTo)
	if err != nil {
		return httperr.BadRequest(c, "date_to must be DD/MM/YYYY")
	}

	result, err := h.store.Get(supplierID, fromDB, toDB)
	if err != nil {
		log.Println("supplier statement error:", err)
		if err.Error() == "supplier not found" {
			return httperr.NotFound(c, "Supplier not found")
		}
		return httperr.Internal(c)
	}

	// Override date fields in response with DD/MM/YYYY display format
	result.DateFrom = rawFrom
	result.DateTo = rawTo

	return c.JSON(result)
}

// parseDMY converts "DD/MM/YYYY" → "YYYY-MM-DD".
func parseDMY(s string) (string, error) {
	t, err := time.Parse("02/01/2006", s)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02"), nil
}
