package hsnreport

import (
	"log"
	"strings"

	"defab-erp/internal/core/httperr"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// ListSales handles GET /hsn-report/sales
//
// Query params:
//
//	hsn_code  — required; the HSN code to filter by (case-insensitive)
//	from_date — optional; YYYY-MM-DD
//	to_date   — optional; YYYY-MM-DD
func (h *Handler) ListSales(c *fiber.Ctx) error {
	hsnCode := strings.TrimSpace(c.Query("hsn_code"))
	if hsnCode == "" {
		return httperr.BadRequest(c, "hsn_code is required")
	}

	f := Filter{
		HSNCode:  hsnCode,
		FromDate: c.Query("from_date"),
		ToDate:   c.Query("to_date"),
	}

	result, err := h.store.ListSales(f)
	if err != nil {
		log.Println("hsnreport sales error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}

// ListJobOrders handles GET /hsn-report/job-orders
//
// Query params:
//
//	hsn_code  — required; the HSN code to filter by (case-insensitive)
//	from_date — optional; YYYY-MM-DD
//	to_date   — optional; YYYY-MM-DD
func (h *Handler) ListJobOrders(c *fiber.Ctx) error {
	hsnCode := strings.TrimSpace(c.Query("hsn_code"))
	if hsnCode == "" {
		return httperr.BadRequest(c, "hsn_code is required")
	}

	f := Filter{
		HSNCode:  hsnCode,
		FromDate: c.Query("from_date"),
		ToDate:   c.Query("to_date"),
	}

	result, err := h.store.ListJobOrders(f)
	if err != nil {
		log.Println("hsnreport job-orders error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}

// ListPurchase handles GET /hsn-report/purchase
//
// Query params:
//
//	hsn_code  — required; the HSN code to filter by (case-insensitive)
//	from_date — optional; YYYY-MM-DD
//	to_date   — optional; YYYY-MM-DD
func (h *Handler) ListPurchase(c *fiber.Ctx) error {
	hsnCode := strings.TrimSpace(c.Query("hsn_code"))
	if hsnCode == "" {
		return httperr.BadRequest(c, "hsn_code is required")
	}

	f := Filter{
		HSNCode:  hsnCode,
		FromDate: c.Query("from_date"),
		ToDate:   c.Query("to_date"),
	}

	result, err := h.store.ListPurchase(f)
	if err != nil {
		log.Println("hsnreport purchase error:", err)
		return httperr.Internal(c)
	}

	return c.JSON(result)
}
