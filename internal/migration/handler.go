package migration

import (
	"fmt"
	"io"
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

// UpsertXlsx handles POST /api/migration/upsert-xlsx
// Same params as import-xlsx but with upsert semantics:
//   - Existing variant (product + code match) → price/cost_price updated to MRP
//   - New variant → created with MRP as price
//   - Stock quantity is added for existing rows, created for new ones
func (h *Handler) UpsertXlsx(c *fiber.Ctx) error {
	folder := c.Query("folder")
	branch := c.Query("branch")
	if folder == "" || branch == "" {
		return c.Status(400).JSON(fiber.Map{"error": "query params required: folder, branch"})
	}

	basePath := filepath.Join("internal", "migration", folder)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("folder not found: %s", basePath)})
	}

	result, err := h.store.UpsertFolderMRP(basePath, branch)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"message": "upsert completed",
		"result":  result,
	})
}

// UpsertDryRun handles GET /api/migration/upsert-dry-run
// Same params as UpsertXlsx (folder + branch not required — read-only).
// Shows how many variants would be updated vs inserted with no DB writes.
func (h *Handler) UpsertDryRun(c *fiber.Ctx) error {
	folder := c.Query("folder")
	if folder == "" {
		return c.Status(400).JSON(fiber.Map{"error": "query param required: folder"})
	}

	basePath := filepath.Join("internal", "migration", folder)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("folder not found: %s", basePath)})
	}

	result, err := h.store.DryRunUpsertMRP(basePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"dry_run": true,
		"folder":  folder,
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

// RepriceFromXlsx handles POST /api/migration/reprice-from-xlsx
// Re-reads the xlsx folder and updates variants.price to GST-inclusive MRP.
// Query params: folder (same as import-xlsx)
func (h *Handler) RepriceFromXlsx(c *fiber.Ctx) error {
	folder := c.Query("folder")
	if folder == "" {
		return c.Status(400).JSON(fiber.Map{"error": "query param required: folder"})
	}

	basePath := filepath.Join("internal", "migration", folder)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("folder not found: %s", basePath)})
	}

	updated, err := h.store.RepriceFolderToMRP(basePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"message":          "reprice completed",
		"variants_updated": updated,
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

// ImportVyttilaStock handles POST /api/migration/import-vyttila-stock
//
// Accepts a multipart file upload (field name: "file") of the Vyttila stock xlsx.
// Query params:
//
//	branch   – branch name to target (default: "DEFAB Vyttila"); created if not found
//	dry_run  – "true" to preview counts without writing to DB
func (h *Handler) ImportVyttilaStock(c *fiber.Ctx) error {
	branch := strings.TrimSpace(c.Query("branch"))
	if branch == "" {
		branch = "DEFAB Vyttila"
	}
	dryRun := c.Query("dry_run") == "true"

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "multipart field 'file' is required",
		})
	}

	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to open uploaded file: " + err.Error(),
		})
	}
	defer f.Close()

	fileBytes, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to read uploaded file: " + err.Error(),
		})
	}

	result, err := h.store.ImportVyttilaStock(fileBytes, branch, dryRun)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "import failed",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "vyttila stock import completed",
		"branch":  branch,
		"dry_run": dryRun,
		"result":  result,
	})
}

// ensure model import is used (imported for auth in ImportSales)
var _ = (*model.User)(nil)

// MapHSNFromXlsx handles POST /api/migration/map-hsn-from-xlsx
// Accepts a multipart file upload (field: "file") with two columns: CODE, HSN CODE.
// Updates hsn_code on ALL variants whose variant_code matches each CODE row.
func (h *Handler) MapHSNFromXlsx(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "multipart field 'file' is required",
		})
	}

	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to open uploaded file: " + err.Error(),
		})
	}
	defer f.Close()

	fileBytes, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to read uploaded file: " + err.Error(),
		})
	}

	codesProcessed, variantsUpdated, err := h.store.BulkMapHSNCodes(fileBytes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "hsn mapping failed",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":          "hsn mapping completed",
		"codes_processed":  codesProcessed,
		"variants_updated": variantsUpdated,
	})
}
