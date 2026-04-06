package migration

import (
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"strings"

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
