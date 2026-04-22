package migration

import (
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// ── Types ────────────────────────────────────────────────────

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// XlsxRow is one parsed row from an xlsx sheet.
type XlsxRow struct {
	ItemName    string
	Code        int
	MRP         float64
	Qty         float64 // 0 when cell is empty
	CategoryKey string  // derived from file name
}

// ImportResult is returned to the caller after a full import.
type ImportResult struct {
	CategoriesCreated int              `json:"categories_created"`
	ProductsCreated   int              `json:"products_created"`
	VariantsCreated   int              `json:"variants_created"`
	StockRowsCreated  int              `json:"stock_rows_created"`
	TotalQtyImported  float64          `json:"total_qty_imported"`
	PerCategory       []CategoryResult `json:"per_category"`
}

type CategoryResult struct {
	Name     string  `json:"name"`
	Products int     `json:"products"`
	Variants int     `json:"variants"`
	TotalQty float64 `json:"total_qty"`
}

// ── Public entry point ───────────────────────────────────────

// ImportFolder reads every .xlsx inside folderPath, maps:
//
//	file name → category
//	item name → product (grouped within the same category)
//	code      → variant  (grouped by item name + code + MRP within a category)
//
// branchName / warehouseName are looked up (or created) so stock rows land in
// the correct warehouse.
func (s *Store) ImportFolder(folderPath, branchName string) (*ImportResult, error) {
	// 1. Resolve branch + warehouse
	branchID, warehouseID, err := s.resolveBranchAndWarehouse(branchName)
	if err != nil {
		return nil, fmt.Errorf("branch/warehouse: %w", err)
	}
	_ = branchID

	// 2. Discover all .xlsx files
	files, err := filepath.Glob(filepath.Join(folderPath, "*.xlsx"))
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .xlsx files found in %s", folderPath)
	}

	// 3. Parse ALL files first (no DB hits)
	type parsedFile struct {
		catName string
		rows    []XlsxRow
	}
	var parsed []parsedFile
	for _, file := range files {
		catName := categoryNameFromFile(file)
		rows, err := parseXlsx(file, catName)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filepath.Base(file), err)
		}
		parsed = append(parsed, parsedFile{catName: catName, rows: rows})
	}

	// 4. Flatten all data into bulk structures
	type variantKey struct {
		catName  string
		itemName string
		code     int
	}
	type variantData struct {
		mrp float64
		qty float64
	}

	catNames := make(map[string]bool)
	type productKey struct {
		catName  string
		itemName string
	}
	prodKeys := make(map[productKey]bool)
	variantMap := make(map[variantKey]*variantData)

	for _, pf := range parsed {
		catNames[pf.catName] = true
		for _, r := range pf.rows {
			prodKeys[productKey{catName: pf.catName, itemName: r.ItemName}] = true
			vk := variantKey{catName: pf.catName, itemName: r.ItemName, code: r.Code}
			if vd, ok := variantMap[vk]; ok {
				vd.qty += r.Qty
				vd.mrp = r.MRP // take latest MRP
			} else {
				variantMap[vk] = &variantData{mrp: r.MRP, qty: r.Qty}
			}
		}
	}

	// 5. Single transaction with bulk operations
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// ── Bulk insert categories ──
	catIDs := make(map[string]uuid.UUID)
	for catName := range catNames {
		id := uuid.New()
		err := tx.QueryRow(
			`INSERT INTO categories (id, name, is_active) VALUES ($1, $2, true)
			 ON CONFLICT (name) DO UPDATE SET name = categories.name
			 RETURNING id`,
			id, catName,
		).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("category %s: %w", catName, err)
		}
		catIDs[catName] = id
	}

	// ── Bulk insert products (multi-row VALUES, batches of 500) ──
	type prodInfo struct {
		catName  string
		itemName string
		id       uuid.UUID
		catID    uuid.UUID
	}
	var allProds []prodInfo
	for pk := range prodKeys {
		allProds = append(allProds, prodInfo{
			catName:  pk.catName,
			itemName: pk.itemName,
			id:       uuid.New(),
			catID:    catIDs[pk.catName],
		})
	}

	const prodBatchSize = 500
	for i := 0; i < len(allProds); i += prodBatchSize {
		end := i + prodBatchSize
		if end > len(allProds) {
			end = len(allProds)
		}
		batch := allProds[i:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO products (id, name, category_id, is_active, is_web_visible, uom) VALUES `)
		args := make([]interface{}, 0, len(batch)*3)
		for j, p := range batch {
			if j > 0 {
				sb.WriteString(", ")
			}
			base := j * 3
			fmt.Fprintf(&sb, "($%d, $%d, $%d, true, true, 'Unit')", base+1, base+2, base+3)
			args = append(args, p.id, p.itemName, p.catID)
		}
		sb.WriteString(` ON CONFLICT DO NOTHING`)

		if _, err := tx.Exec(sb.String(), args...); err != nil {
			return nil, fmt.Errorf("bulk product insert: %w", err)
		}
	}

	// ── Bulk SELECT all product IDs for our categories ──
	catNameByID := make(map[uuid.UUID]string)
	catIDSlice := make([]interface{}, 0, len(catIDs))
	for name, id := range catIDs {
		catNameByID[id] = name
		catIDSlice = append(catIDSlice, id)
	}

	prodIDs := make(map[string]uuid.UUID) // "catName|lowerItemName" → id
	{
		var sb strings.Builder
		sb.WriteString(`SELECT id, LOWER(name), category_id FROM products WHERE category_id IN (`)
		for i := range catIDSlice {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "$%d", i+1)
		}
		sb.WriteString(`)`)

		rows, err := tx.Query(sb.String(), catIDSlice...)
		if err != nil {
			return nil, fmt.Errorf("product lookup: %w", err)
		}
		for rows.Next() {
			var id, catID uuid.UUID
			var lowerName string
			if err := rows.Scan(&id, &lowerName, &catID); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan product: %w", err)
			}
			prodIDs[catNameByID[catID]+"|"+lowerName] = id
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("product rows: %w", err)
		}
		rows.Close()
	}

	// ── Pre-generate all variant + stock data ──
	type variantRow struct {
		id        uuid.UUID
		productID uuid.UUID
		code      int
		name      string
		sku       string
		barcode   string
		mrp       float64
		qty       float64
		catName   string
		itemName  string
	}
	allVariants := make([]variantRow, 0, len(variantMap))
	for vk, vd := range variantMap {
		prodKey := vk.catName + "|" + strings.ToLower(vk.itemName)
		productID := prodIDs[prodKey]
		allVariants = append(allVariants, variantRow{
			id:        uuid.New(),
			productID: productID,
			code:      vk.code,
			name:      fmt.Sprintf("%s - %d", vk.itemName, vk.code),
			sku:       generateSKU(vk.catName, vk.code),
			barcode:   generateBarcode(),
			mrp:       vd.mrp,
			qty:       math.Round(vd.qty*100) / 100,
			catName:   vk.catName,
			itemName:  vk.itemName,
		})
	}

	// ── Bulk insert variants (multi-row VALUES, batches of 500) ──
	// 7 unique params per row: id, product_id, variant_code, name, sku, barcode, price
	const varBatchSize = 500
	for i := 0; i < len(allVariants); i += varBatchSize {
		end := i + varBatchSize
		if end > len(allVariants) {
			end = len(allVariants)
		}
		batch := allVariants[i:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO variants (id, product_id, variant_code, name, sku, barcode, price, cost_price, is_active) VALUES `)
		args := make([]interface{}, 0, len(batch)*7)
		for j, v := range batch {
			if j > 0 {
				sb.WriteString(", ")
			}
			base := j * 7
			fmt.Fprintf(&sb, "($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, true)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+7)
			args = append(args, v.id, v.productID, v.code, v.name, v.sku, v.barcode, v.mrp)
		}

		if _, err := tx.Exec(sb.String(), args...); err != nil {
			return nil, fmt.Errorf("bulk variant insert: %w", err)
		}
	}

	// ── Bulk upsert stock (multi-row VALUES, batches of 500) ──
	// 3 params per row: variant_id, warehouse_id, quantity
	const stockBatchSize = 500
	for i := 0; i < len(allVariants); i += stockBatchSize {
		end := i + stockBatchSize
		if end > len(allVariants) {
			end = len(allVariants)
		}
		batch := allVariants[i:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO stocks (id, variant_id, warehouse_id, quantity, stock_type) VALUES `)
		args := make([]interface{}, 0, len(batch)*3)
		for j, v := range batch {
			if j > 0 {
				sb.WriteString(", ")
			}
			base := j * 3
			fmt.Fprintf(&sb, "(uuid_generate_v4(), $%d, $%d, $%d, 'PRODUCT')", base+1, base+2, base+3)
			args = append(args, v.id, warehouseID, v.qty)
		}
		sb.WriteString(` ON CONFLICT (variant_id, warehouse_id)
		 DO UPDATE SET quantity = stocks.quantity + EXCLUDED.quantity, updated_at = NOW()`)

		if _, err := tx.Exec(sb.String(), args...); err != nil {
			return nil, fmt.Errorf("bulk stock upsert: %w", err)
		}
	}

	// ── Gather results ──
	result := &ImportResult{}
	catVariantCount := make(map[string]int)
	catQty := make(map[string]float64)
	catProducts := make(map[string]map[string]bool)

	for _, v := range allVariants {
		catVariantCount[v.catName]++
		catQty[v.catName] += v.qty
		if catProducts[v.catName] == nil {
			catProducts[v.catName] = make(map[string]bool)
		}
		catProducts[v.catName][v.itemName] = true
	}

	// ── Update category counts ──
	for catName, catID := range catIDs {
		if _, err := tx.Exec(
			`UPDATE categories SET products_count = (
				SELECT COUNT(*) FROM products WHERE category_id = $1
			 ) WHERE id = $1`, catID,
		); err != nil {
			return nil, err
		}

		cr := CategoryResult{
			Name:     catName,
			Products: len(catProducts[catName]),
			Variants: catVariantCount[catName],
			TotalQty: catQty[catName],
		}
		result.CategoriesCreated++
		result.ProductsCreated += cr.Products
		result.VariantsCreated += cr.Variants
		result.StockRowsCreated += cr.Variants
		result.TotalQtyImported += cr.TotalQty
		result.PerCategory = append(result.PerCategory, cr)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return result, nil
}

// ── Parse xlsx ───────────────────────────────────────────────

func categoryNameFromFile(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	// Title-case & clean up
	name = strings.TrimSpace(name)
	return name
}

// parseXlsx reads every sheet of a workbook and returns flat rows.
func parseXlsx(path, catKey string) ([]XlsxRow, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rows []XlsxRow

	for _, sheet := range f.GetSheetList() {
		allRows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		if len(allRows) == 0 {
			continue
		}

		// Detect column layout from header row
		colMap := detectColumns(allRows)
		if colMap == nil {
			continue // can't figure out columns
		}

		for i, row := range allRows {
			if i <= colMap.headerRow {
				continue // skip header and rows before it
			}
			if len(row) == 0 {
				continue
			}

			item := safeCol(row, colMap.itemIdx)
			codeStr := safeCol(row, colMap.codeIdx)
			// Prefer MRP excluding GST column over plain MRP
			var mrpStr string
			if colMap.mrpExclGstIdx >= 0 {
				mrpStr = safeCol(row, colMap.mrpExclGstIdx)
			}
			if mrpStr == "" {
				mrpStr = safeCol(row, colMap.mrpIdx)
			}
			qtyStr := safeCol(row, colMap.qtyIdx)

			if codeStr == "" {
				continue // no code → skip
			}

			code := parseFloat(codeStr)
			if code == 0 {
				continue
			}

			mrp := parseFloat(mrpStr)
			qty := parseFloat(qtyStr) // returns 0 for empty/nil

			// If item is empty, use category key as fallback
			if item == "" {
				item = catKey
			}

			rows = append(rows, XlsxRow{
				ItemName:    cleanItemName(item),
				Code:        int(code),
				MRP:         mrp,
				Qty:         qty,
				CategoryKey: catKey,
			})
		}
	}
	return rows, nil
}

// colLayout stores detected column indices for a sheet.
type colLayout struct {
	headerRow     int
	itemIdx       int
	codeIdx       int
	mrpIdx        int
	mrpExclGstIdx int
	qtyIdx        int
}

func detectColumns(allRows [][]string) *colLayout {
	// Look in first 3 rows for a header-like row
	for i := 0; i < len(allRows) && i < 3; i++ {
		row := allRows[i]
		layout := &colLayout{headerRow: i, itemIdx: -1, codeIdx: -1, mrpIdx: -1, mrpExclGstIdx: -1, qtyIdx: -1}

		for j, cell := range row {
			upper := strings.ToUpper(strings.TrimSpace(cell))
			switch {
			case strings.Contains(upper, "ITEM") || strings.Contains(upper, "ITEAM"):
				layout.itemIdx = j
			case upper == "CODE":
				layout.codeIdx = j
			// "MRP EXCLUDING GST", "NEW MRP EXCLUDING GST", "MRP EXCLUSING GST" (typo)
			case strings.Contains(upper, "MRP") && strings.Contains(upper, "EXCL"):
				layout.mrpExclGstIdx = j
			case strings.HasPrefix(upper, "MRP") && !strings.Contains(upper, "EXCL"):
				layout.mrpIdx = j
			case strings.Contains(upper, "MRP") && !strings.Contains(upper, "EXCL") && !strings.HasPrefix(upper, "MRP"):
				// e.g. "Mrp" not starting with MRP after uppercasing — already covered above
				layout.mrpIdx = j
			case upper == "QTY":
				layout.qtyIdx = j
			}
		}

		// Need at least code + (mrp or mrpExclGst)
		if layout.codeIdx >= 0 && (layout.mrpIdx >= 0 || layout.mrpExclGstIdx >= 0) {
			// If item col not found, guess it's next to code (DYBL MTRL pattern: SL, Code, Items, Qty, Mrp)
			if layout.itemIdx < 0 {
				// Try column 2 for items in DYBL MTRL layout
				if layout.codeIdx == 1 {
					layout.itemIdx = 2
				} else {
					layout.itemIdx = 1
				}
			}
			if layout.qtyIdx < 0 {
				// Qty is typically the last column
				layout.qtyIdx = len(row) - 1
				if layout.qtyIdx == layout.mrpIdx {
					layout.qtyIdx = -1 // no separate qty column
				}
			}
			return layout
		}
	}
	return nil
}

func safeCol(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func cleanItemName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Title case the first letter, keep rest as-is
	return strings.ToUpper(s[:1]) + s[1:]
}

// ── Import one category (runs inside a transaction) ──────────

func importCategoryTx(
	tx *sql.Tx,
	catName string,
	rows []XlsxRow,
	warehouseID uuid.UUID,
	catCache map[string]uuid.UUID,
	prodCache map[string]uuid.UUID,
	varCache map[string]uuid.UUID,
) (*CategoryResult, error) {
	if len(rows) == 0 {
		return &CategoryResult{Name: catName}, nil
	}

	// 1. Get or create category (cached)
	catID, err := getOrCreateCategoryTx(tx, catName, catCache)
	if err != nil {
		return nil, err
	}

	// 2. Aggregate: (item_name + code + mrp) → sum of qty
	type variantKey struct {
		itemName string
		code     int
		mrp      float64
	}
	variantQty := make(map[variantKey]float64)

	for _, r := range rows {
		k := variantKey{itemName: r.ItemName, code: r.Code, mrp: r.MRP}
		variantQty[k] += r.Qty
	}

	// 3. Group by product
	type variantData struct {
		code int
		mrp  float64
		qty  float64
	}
	productVariants := make(map[string][]variantData)
	for k, qty := range variantQty {
		productVariants[k.itemName] = append(productVariants[k.itemName], variantData{
			code: k.code, mrp: k.mrp, qty: qty,
		})
	}

	cr := &CategoryResult{Name: catName}

	for productName, variants := range productVariants {
		productID, err := getOrCreateProductTx(tx, productName, catID, prodCache)
		if err != nil {
			return nil, fmt.Errorf("product %s: %w", productName, err)
		}
		cr.Products++

		for _, v := range variants {
			variantID, created, err := getOrCreateVariantTx(tx, productID, productName, catName, v.code, v.mrp, varCache)
			if err != nil {
				return nil, fmt.Errorf("variant %s code %d: %w", productName, v.code, err)
			}
			if created {
				cr.Variants++
			}

			qty := math.Round(v.qty*100) / 100
			if err := upsertStockTx(tx, variantID, warehouseID, qty); err != nil {
				return nil, fmt.Errorf("stock for code %d: %w", v.code, err)
			}
			cr.TotalQty += qty
		}
	}

	// Update category product count
	if _, err := tx.Exec(
		`UPDATE categories SET products_count = (
			SELECT COUNT(*) FROM products WHERE category_id = $1
		 ) WHERE id = $1`, catID,
	); err != nil {
		return nil, err
	}

	return cr, nil
}

// ── DB helpers ───────────────────────────────────────────────

// resolveBranchAndWarehouse:
//   - If a branch with the given name exists → find its first warehouse
//   - If neither exists → create both with the same name
func (s *Store) resolveBranchAndWarehouse(branchName string) (uuid.UUID, uuid.UUID, error) {
	var branchID, warehouseID uuid.UUID

	// Try to find existing branch
	err := s.db.QueryRow(
		`SELECT id FROM branches WHERE LOWER(name) = LOWER($1)`, branchName,
	).Scan(&branchID)

	if err == nil {
		// Branch exists → find its warehouse
		err = s.db.QueryRow(
			`SELECT id FROM warehouses WHERE branch_id = $1 ORDER BY created_at LIMIT 1`,
			branchID,
		).Scan(&warehouseID)
		if err == nil {
			return branchID, warehouseID, nil
		}
		if err != sql.ErrNoRows {
			return branchID, warehouseID, err
		}
		// Branch exists but no warehouse → create warehouse with same name
		warehouseID = uuid.New()
		_, err = s.db.Exec(
			`INSERT INTO warehouses (id, name, branch_id, type) VALUES ($1, $2, $3, 'STORE')`,
			warehouseID, branchName, branchID,
		)
		return branchID, warehouseID, err
	}
	if err != sql.ErrNoRows {
		return branchID, warehouseID, err
	}

	// Branch doesn't exist → create both
	branchID = uuid.New()
	_, err = s.db.Exec(`INSERT INTO branches (id, name) VALUES ($1, $2)`, branchID, branchName)
	if err != nil {
		return branchID, warehouseID, err
	}

	warehouseID = uuid.New()
	_, err = s.db.Exec(
		`INSERT INTO warehouses (id, name, branch_id, type) VALUES ($1, $2, $3, 'STORE')`,
		warehouseID, branchName, branchID,
	)
	return branchID, warehouseID, err
}

// ── Transaction-based DB helpers with in-memory caching ──────

// generateSKU creates a unique SKU like "SAR-5217-a3f2"
func generateSKU(catName string, code int) string {
	prefix := strings.ToUpper(catName)
	if len(prefix) > 3 {
		prefix = prefix[:3]
	}
	prefix = strings.ReplaceAll(prefix, " ", "")
	short := uuid.New().String()[:4]
	return fmt.Sprintf("%s-%d-%s", prefix, code, short)
}

// generateBarcode creates a unique 13-digit EAN-like barcode
func generateBarcode() string {
	u := uuid.New()
	// Take first 12 hex chars, convert to digits
	hex := strings.ReplaceAll(u.String(), "-", "")[:12]
	var digits []byte
	for _, c := range hex {
		if c >= '0' && c <= '9' {
			digits = append(digits, byte(c))
		} else {
			// a=1, b=2, ..., f=6
			digits = append(digits, byte('0'+c-'a'+1))
		}
	}
	// Pad to 12 if needed, then add check digit
	for len(digits) < 12 {
		digits = append(digits, '0')
	}
	sum := 0
	for i, d := range digits[:12] {
		v := int(d - '0')
		if i%2 == 0 {
			sum += v
		} else {
			sum += v * 3
		}
	}
	check := (10 - sum%10) % 10
	return string(digits[:12]) + fmt.Sprintf("%d", check)
}

func getOrCreateCategoryTx(tx *sql.Tx, name string, cache map[string]uuid.UUID) (uuid.UUID, error) {
	key := strings.ToLower(name)
	if id, ok := cache[key]; ok {
		return id, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(`SELECT id FROM categories WHERE LOWER(name) = LOWER($1)`, name).Scan(&id)
	if err == nil {
		cache[key] = id
		return id, nil
	}
	if err != sql.ErrNoRows {
		return id, err
	}
	id = uuid.New()
	_, err = tx.Exec(`INSERT INTO categories (id, name, is_active) VALUES ($1, $2, true)`, id, name)
	if err != nil {
		return id, err
	}
	cache[key] = id
	return id, nil
}

func getOrCreateProductTx(tx *sql.Tx, name string, categoryID uuid.UUID, cache map[string]uuid.UUID) (uuid.UUID, error) {
	key := categoryID.String() + "|" + strings.ToLower(name)
	if id, ok := cache[key]; ok {
		return id, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(
		`SELECT id FROM products WHERE LOWER(name) = LOWER($1) AND category_id = $2`,
		name, categoryID,
	).Scan(&id)
	if err == nil {
		cache[key] = id
		return id, nil
	}
	if err != sql.ErrNoRows {
		return id, err
	}
	id = uuid.New()
	_, err = tx.Exec(
		`INSERT INTO products (id, name, category_id, is_active, is_web_visible, uom)
		 VALUES ($1, $2, $3, true, true, 'Unit')`,
		id, name, categoryID,
	)
	if err != nil {
		return id, err
	}
	cache[key] = id
	return id, nil
}

func getOrCreateVariantTx(tx *sql.Tx, productID uuid.UUID, productName, catName string, code int, price float64, cache map[string]uuid.UUID) (uuid.UUID, bool, error) {
	key := fmt.Sprintf("%s|%d|%.2f", productID, code, price)
	if id, ok := cache[key]; ok {
		return id, false, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(
		`SELECT id FROM variants WHERE product_id = $1 AND variant_code = $2 AND price = $3`,
		productID, code, price,
	).Scan(&id)
	if err == nil {
		cache[key] = id
		return id, false, nil
	}
	if err != sql.ErrNoRows {
		return id, false, err
	}

	sku := generateSKU(catName, code)
	barcode := generateBarcode()
	variantName := fmt.Sprintf("%s - %d", productName, code)

	id = uuid.New()
	_, err = tx.Exec(
		`INSERT INTO variants (id, product_id, variant_code, name, sku, barcode, price, cost_price, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7, true)`,
		id, productID, code, variantName, sku, barcode, price,
	)
	if err != nil {
		return id, false, err
	}
	cache[key] = id
	return id, true, nil
}

func upsertStockTx(tx *sql.Tx, variantID, warehouseID uuid.UUID, qty float64) error {
	_, err := tx.Exec(
		`INSERT INTO stocks (id, variant_id, warehouse_id, quantity, stock_type)
		 VALUES (uuid_generate_v4(), $1, $2, $3, 'PRODUCT')
		 ON CONFLICT (variant_id, warehouse_id)
		 DO UPDATE SET quantity = stocks.quantity + $3, updated_at = NOW()`,
		variantID, warehouseID, qty,
	)
	return err
}

// ── Sales Invoice Migration ───────────────────────────────────

const salesMigXLSXPath = "internal/migration/SalesMigration/Sales Report List.xlsx"

// SalesMigrationResult is returned after running the sales import.
type SalesMigrationResult struct {
	TotalGroups int      `json:"total_groups"`
	Created     int      `json:"created"`
	Skipped     int      `json:"skipped"`
	Errored     int      `json:"errored"`
	Errors      []string `json:"errors,omitempty"`
}

// salesInvGroup holds one logical invoice (one or more item rows + one payment row).
type salesInvGroup struct {
	InvoiceNumber     string
	CustomerName      string
	DateStr           string
	Items             []salesMigItem
	RoundOff          float64
	SalesReturnAdjust float64
	AdvAdjust         float64
	Cheque            float64
	Online            float64
	Cash              float64
	DebitCard         float64
	CreditCard        float64
}

// salesMigItem is one line item from the Excel.
type salesMigItem struct {
	Description string
	Rate        float64
	Qty         float64
	Taxable     float64
	TaxRate     float64
	CGST        float64
	SGST        float64
	IGST        float64
	InvVal      float64
}

// ImportSales parses the sales migration Excel and inserts invoice records.
// branchName must match an existing branch in the DB; its first warehouse is used.
// userID (from JWT) becomes created_by on all inserted rows.
func (s *Store) ImportSales(userID, branchName string) (*SalesMigrationResult, error) {
	// Resolve branch
	var branchID string
	if err := s.db.QueryRow(
		`SELECT id FROM branches WHERE LOWER(TRIM(name)) = LOWER($1) LIMIT 1`, branchName,
	).Scan(&branchID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("branch %q not found", branchName)
		}
		return nil, fmt.Errorf("lookup branch: %w", err)
	}

	// Resolve warehouse
	var warehouseID string
	if err := s.db.QueryRow(
		`SELECT id FROM warehouses WHERE branch_id = $1 ORDER BY created_at LIMIT 1`, branchID,
	).Scan(&warehouseID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no warehouse found for branch %q", branchName)
		}
		return nil, fmt.Errorf("lookup warehouse: %w", err)
	}

	groups, err := parseSalesXLSX(salesMigXLSXPath)
	if err != nil {
		return nil, fmt.Errorf("parse xlsx: %w", err)
	}

	result := &SalesMigrationResult{TotalGroups: len(groups)}
	for _, g := range groups {
		imported, err := s.importOneInvoice(g, userID, branchID, warehouseID)
		if err != nil {
			result.Errored++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", g.InvoiceNumber, err))
		} else if imported {
			result.Created++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

func (s *Store) importOneInvoice(g salesInvGroup, userID, branchID, warehouseID string) (created bool, err error) {
	// Idempotency check
	var exists bool
	if err := s.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM sales_invoices WHERE invoice_number = $1)`,
		g.InvoiceNumber,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check exists: %w", err)
	}
	if exists {
		return false, nil
	}

	invoiceDate, err := parseSalesDate(g.DateStr)
	if err != nil {
		return false, fmt.Errorf("parse date %q: %w", g.DateStr, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Find or create customer by name
	customerName := strings.TrimSpace(g.CustomerName)
	if customerName == "" {
		customerName = "Walk-in Customer"
	}
	var customerID string
	err = tx.QueryRow(
		`SELECT id FROM customers WHERE LOWER(TRIM(name)) = LOWER($1) LIMIT 1`,
		customerName,
	).Scan(&customerID)
	if err == sql.ErrNoRows {
		var maxCode sql.NullString
		tx.QueryRow(`SELECT MAX(customer_code) FROM customers WHERE customer_code LIKE 'CUS%'`).Scan(&maxCode)
		next := 1
		if maxCode.Valid && len(maxCode.String) > 3 {
			fmt.Sscanf(maxCode.String[3:], "%d", &next)
			next++
		}
		code := fmt.Sprintf("CUS%04d", next)
		if err = tx.QueryRow(
			`INSERT INTO customers (customer_code, name, phone, email)
			 VALUES ($1, $2, NULL, NULL) RETURNING id`,
			code, customerName,
		).Scan(&customerID); err != nil {
			return false, fmt.Errorf("create customer: %w", err)
		}
	} else if err != nil {
		return false, fmt.Errorf("find customer: %w", err)
	}

	// 2. Compute totals
	var subTotal, gstTotal, netTotal float64
	for _, item := range g.Items {
		subTotal += item.Taxable
		if item.IGST > 0 {
			gstTotal += item.IGST
		} else {
			gstTotal += item.CGST + item.SGST
		}
		netTotal += item.InvVal
	}
	subTotal = salesRound2(subTotal)
	gstTotal = salesRound2(gstTotal)
	netTotal = salesRound2(netTotal)
	roundOff := g.RoundOff
	netAmount := salesRound2(netTotal + roundOff)

	// paid = all cash receipts + return / advance adjustments
	paidAmount := salesRound2(g.Cheque + g.Online + g.Cash + g.DebitCard + g.CreditCard +
		g.SalesReturnAdjust + g.AdvAdjust)
	paymentStatus := "UNPAID"
	if paidAmount >= netAmount {
		paymentStatus = "PAID"
	} else if paidAmount > 0 {
		paymentStatus = "PARTIAL"
	}

	// 3. Create sales_order
	soNumber := "MIG-" + g.InvoiceNumber
	var salesOrderID string
	if err = tx.QueryRow(`
		INSERT INTO sales_orders
			(so_number, channel, branch_id, customer_id, salesperson_id,
			 warehouse_id, created_by, order_date,
			 subtotal, tax_total, discount_total, bill_discount, grand_total,
			 status, payment_status, notes)
		VALUES ($1,'STORE',$2,$3,NULL,$4,$5,$6,$7,$8,0,0,$9,'CONFIRMED',$10,'Migrated from POS')
		RETURNING id`,
		soNumber, branchID, customerID, warehouseID,
		userID, invoiceDate,
		subTotal, gstTotal, netAmount, paymentStatus,
	).Scan(&salesOrderID); err != nil {
		return false, fmt.Errorf("create sales order: %w", err)
	}

	// 4. Create sales_invoice
	var salesInvoiceID string
	if err = tx.QueryRow(`
		INSERT INTO sales_invoices
			(sales_order_id, customer_id, warehouse_id, channel, branch_id,
			 invoice_number, invoice_date,
			 sub_amount, discount_amount, bill_discount, gst_amount, round_off,
			 net_amount, paid_amount, status, created_by)
		VALUES ($1,$2,$3,'STORE',$4,$5,$6,$7,0,0,$8,$9,$10,$11,$12,$13)
		RETURNING id`,
		salesOrderID, customerID, warehouseID, branchID,
		g.InvoiceNumber, invoiceDate,
		subTotal, gstTotal, roundOff,
		netAmount, paidAmount, paymentStatus, userID,
	).Scan(&salesInvoiceID); err != nil {
		return false, fmt.Errorf("create sales invoice: %w", err)
	}

	// 5. Create sales_order_items + sales_invoice_items (no variant — migrated data)
	for _, item := range g.Items {
		taxAmt := item.CGST + item.SGST
		if item.IGST > 0 {
			taxAmt = item.IGST
		}
		if _, err = tx.Exec(`
			INSERT INTO sales_order_items
				(sales_order_id, variant_id, item_description,
				 quantity, unit_price, discount, tax_percent, tax_amount, total_price)
			VALUES ($1, NULL, $2, $3, $4, 0, $5, $6, $7)`,
			salesOrderID, item.Description, item.Qty, item.Rate,
			item.TaxRate, salesRound2(taxAmt), salesRound2(item.InvVal),
		); err != nil {
			return false, fmt.Errorf("create order item: %w", err)
		}
		if _, err = tx.Exec(`
			INSERT INTO sales_invoice_items
				(sales_invoice_id, variant_id, item_description,
				 quantity, unit_price, discount, tax_percent, tax_amount, total_price)
			VALUES ($1, NULL, $2, $3, $4, 0, $5, $6, $7)`,
			salesInvoiceID, item.Description, item.Qty, item.Rate,
			item.TaxRate, salesRound2(taxAmt), salesRound2(item.InvVal),
		); err != nil {
			return false, fmt.Errorf("create invoice item: %w", err)
		}
	}

	// 6. Create sales_payments
	type pmEntry struct {
		method string
		amount float64
	}
	pmList := []pmEntry{
		{"CASH", g.Cash},
		{"ONLINE", g.Online},
		{"CHEQUE", g.Cheque},
		{"DEBITCARD", g.DebitCard},
		{"CREDITCARD", g.CreditCard},
		{"RETURN_ADJUST", g.SalesReturnAdjust},
		{"ADV_ADJUST", g.AdvAdjust},
	}
	for _, p := range pmList {
		if p.amount <= 0 {
			continue
		}
		if _, err = tx.Exec(`
			INSERT INTO sales_payments (sales_invoice_id, amount, payment_method, reference, paid_at)
			VALUES ($1, $2, $3, 'Migrated', $4)`,
			salesInvoiceID, salesRound2(p.amount), p.method, invoiceDate,
		); err != nil {
			return false, fmt.Errorf("create payment %s: %w", p.method, err)
		}
	}

	// 7. Update customer total_purchases
	if _, err = tx.Exec(
		`UPDATE customers SET total_purchases = total_purchases + $1, updated_at = NOW() WHERE id = $2`,
		netAmount, customerID,
	); err != nil {
		return false, fmt.Errorf("update customer purchases: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

// parseSalesXLSX reads the Sales Report List.xlsx and groups rows into invoices.
//
// Column layout (0-based indices in each row slice):
//
//	0=SlNo, 1=CUSTOMER, 2=INVNO, 3=Date, 4=Item, 5=HSN,
//	6=Rate, 7=Qty, 8=Taxable, 9=Tax Rate, 10=CGST, 11=SGST,
//	12=IGST, 13=TOTAL GST, 14=INV VAL, 15=RoundOff,
//	16=SALESRETURN ADJUST, 17=ADV ADJUST,
//	18=Cheque, 19=Online, 20=Cash, 21=Debitcard, 22=CreditCard
func parseSalesXLSX(path string) ([]salesInvGroup, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in workbook")
	}

	allRows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	if len(allRows) < 2 {
		return nil, fmt.Errorf("no data rows found")
	}

	var groups []salesInvGroup
	var current *salesInvGroup

	for i := 1; i < len(allRows); i++ { // skip header at index 0
		row := allRows[i]
		invNo := strings.TrimSpace(safeCol(row, 2))

		if invNo != "" {
			// Invoice item row
			if current == nil || current.InvoiceNumber != invNo {
				// New invoice starts; carry forward any unfinished group
				if current != nil && len(current.Items) > 0 {
					// Missing payment row — store what we have
					groups = append(groups, *current)
				}
				current = &salesInvGroup{
					InvoiceNumber: invNo,
					CustomerName:  strings.TrimSpace(safeCol(row, 1)),
					DateStr:       strings.TrimSpace(safeCol(row, 3)),
				}
			}
			current.Items = append(current.Items, salesMigItem{
				Description: strings.TrimSpace(safeCol(row, 4)),
				Rate:        parseFloat(safeCol(row, 6)),
				Qty:         parseFloat(safeCol(row, 7)),
				Taxable:     parseFloat(safeCol(row, 8)),
				TaxRate:     parseFloat(safeCol(row, 9)),
				CGST:        parseFloat(safeCol(row, 10)),
				SGST:        parseFloat(safeCol(row, 11)),
				IGST:        parseFloat(safeCol(row, 12)),
				InvVal:      parseFloat(safeCol(row, 14)),
			})
		} else {
			// Payment row (INVNO is empty)
			if current != nil && len(current.Items) > 0 {
				current.RoundOff = parseFloat(safeCol(row, 15))
				current.SalesReturnAdjust = parseFloat(safeCol(row, 16))
				current.AdvAdjust = parseFloat(safeCol(row, 17))
				current.Cheque = parseFloat(safeCol(row, 18))
				current.Online = parseFloat(safeCol(row, 19))
				current.Cash = parseFloat(safeCol(row, 20))
				current.DebitCard = parseFloat(safeCol(row, 21))
				current.CreditCard = parseFloat(safeCol(row, 22))
				groups = append(groups, *current)
				current = nil
			}
			// else: orphan row (e.g. the totals summary at the end) — skip
		}
	}

	return groups, nil
}

// parseSalesDate tries several common date layouts.
func parseSalesDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"02/01/2006", "2/1/2006", "2006-01-02", "01/02/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	// Last-resort: Excel date serial number (days since 1899-12-30)
	if n := parseFloat(s); n > 100 {
		t := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(
			time.Duration(int(n)) * 24 * time.Hour)
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognised date %q", s)
}

func salesRound2(v float64) float64 {
	return math.Round(v*100) / 100
}
