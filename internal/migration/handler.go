package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// ImportXlsx handles POST /api/migration/import-xlsx
//
//	Query params:
//	  folder – subfolder name inside internal/migration/ (e.g. "Defab Thrippunithura")
//	  branch – branch name: if it exists, uses its warehouse; if not, creates both
func (h *Handler) ImportXlsx(c *fiber.Ctx) error {
	folder := c.Query("folder")
	branch := c.Query("branch")

	if folder == "" || branch == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "query params required: folder, branch",
		})
	}

	// Resolve folder path relative to the running binary's working dir
	basePath := filepath.Join("internal", "migration", folder)

	// Check folder exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return c.Status(400).JSON(fiber.Map{
			"error": fmt.Sprintf("folder not found: %s", basePath),
		})
	}

	result, err := h.store.ImportFolder(basePath, branch)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "import failed",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "import completed",
		"result":  result,
	})
}

// DryRun handles GET /api/migration/dry-run
// Same params as ImportXlsx but only parses and summarises — no DB writes.
func (h *Handler) DryRun(c *fiber.Ctx) error {
	folder := c.Query("folder")
	if folder == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "query param required: folder",
		})
	}

	basePath := filepath.Join("internal", "migration", folder)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return c.Status(400).JSON(fiber.Map{
			"error": fmt.Sprintf("folder not found: %s", basePath),
		})
	}

	files, err := filepath.Glob(filepath.Join(basePath, "*.xlsx"))
	if err != nil || len(files) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "no xlsx files found"})
	}

	type fileSummary struct {
		FileName    string   `json:"file_name"`
		Category    string   `json:"category"`
		TotalRows   int      `json:"total_rows"`
		UniqueCodes int      `json:"unique_codes"`
		UniqueItems int      `json:"unique_items"`
		SampleItems []string `json:"sample_items"`
	}

	var summaries []fileSummary
	totalRows := 0

	for _, file := range files {
		catName := categoryNameFromFile(file)
		rows, err := parseXlsx(file, catName)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": fmt.Sprintf("parse error in %s: %s", filepath.Base(file), err.Error()),
			})
		}

		codes := make(map[int]bool)
		items := make(map[string]bool)
		for _, r := range rows {
			codes[r.Code] = true
			items[r.ItemName] = true
		}

		samples := make([]string, 0, 5)
		count := 0
		for item := range items {
			if count >= 5 {
				break
			}
			samples = append(samples, item)
			count++
		}

		summaries = append(summaries, fileSummary{
			FileName:    filepath.Base(file),
			Category:    catName,
			TotalRows:   len(rows),
			UniqueCodes: len(codes),
			UniqueItems: len(items),
			SampleItems: samples,
		})
		totalRows += len(rows)
	}

	return c.JSON(fiber.Map{
		"files":      summaries,
		"total_rows": totalRows,
		"file_count": len(files),
	})
}

// ImportSales handles POST /api/migration/import-sales?branch=<branch name>
// Looks up the branch by name, finds its warehouse, then migrates sales invoices.
func (h *Handler) ImportSales(c *fiber.Ctx) error {
	branchName := strings.TrimSpace(c.Query("branch"))
	if branchName == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "query param required: branch",
		})
	}

	user := c.Locals("user").(*model.User)

	result, err := h.store.ImportSales(user.ID.String(), branchName)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "sales migration failed",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "sales migration completed",
		"result":  result,
	})
}
