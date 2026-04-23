package product

import (
	"database/sql"
	"math"
	"strconv"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ListProducts returns paginated products visible on web.
func (s *Store) ListProducts(categoryID string, search string, page, limit int) ([]map[string]interface{}, int, error) {
	offset := (page - 1) * limit

	// Build dynamic WHERE
	where := "WHERE p.is_active = true AND p.is_web_visible = true"
	args := []interface{}{}
	idx := 1

	if categoryID != "" {
		where += " AND p.category_id = $" + strconv.Itoa(idx)
		args = append(args, categoryID)
		idx++
	}
	if search != "" {
		where += " AND (p.name ILIKE $" + strconv.Itoa(idx) + " OR p.brand ILIKE $" + strconv.Itoa(idx) + ")"
		args = append(args, "%"+search+"%")
		idx++
	}

	// Count
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := s.db.QueryRow("SELECT COUNT(*) FROM products p "+where, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Fetch products
	query := `
		SELECT p.id, p.name, COALESCE(p.description, ''), COALESCE(p.brand, ''),
		       COALESCE(p.main_image_url, ''), COALESCE(c.name, '') AS category,
		       p.uom,
		       (SELECT MIN(v.price) FROM variants v WHERE v.product_id = p.id AND v.is_active = true) AS min_price,
		       (SELECT MAX(v.price) FROM variants v WHERE v.product_id = p.id AND v.is_active = true) AS max_price
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		` + where + `
		ORDER BY p.created_at DESC
		LIMIT $` + strconv.Itoa(idx) + ` OFFSET $` + strconv.Itoa(idx+1)

	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []map[string]interface{}
	for rows.Next() {
		var id, name, desc, brand, image, category, uom string
		var minPrice, maxPrice sql.NullFloat64

		if err := rows.Scan(&id, &name, &desc, &brand, &image, &category, &uom, &minPrice, &maxPrice); err != nil {
			return nil, 0, err
		}

		p := map[string]interface{}{
			"id":          id,
			"name":        name,
			"description": desc,
			"brand":       brand,
			"image":       image,
			"category":    category,
			"uom":         uom,
		}
		if minPrice.Valid {
			p["min_price"] = minPrice.Float64
		}
		if maxPrice.Valid {
			p["max_price"] = maxPrice.Float64
		}

		products = append(products, p)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	_ = totalPages

	return products, total, nil
}

// GetProductDetail returns a product with its variants, images, and stock info.
func (s *Store) GetProductDetail(productID string) (map[string]interface{}, error) {
	// Product base
	var id, name, desc, brand, image, category, uom string
	var fabricComp, pattern, occasion, care sql.NullString

	err := s.db.QueryRow(`
		SELECT p.id, p.name, COALESCE(p.description, ''), COALESCE(p.brand, ''),
		       COALESCE(p.main_image_url, ''), COALESCE(c.name, '') AS category,
		       p.uom, p.fabric_composition, p.pattern, p.occasion, p.care_instructions
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.id = $1 AND p.is_active = true AND p.is_web_visible = true
	`, productID).Scan(&id, &name, &desc, &brand, &image, &category, &uom,
		&fabricComp, &pattern, &occasion, &care)
	if err != nil {
		return nil, err
	}

	product := map[string]interface{}{
		"id":                 id,
		"name":               name,
		"description":        desc,
		"brand":              brand,
		"image":              image,
		"category":           category,
		"uom":                uom,
		"fabric_composition": nullStr(fabricComp),
		"pattern":            nullStr(pattern),
		"occasion":           nullStr(occasion),
		"care_instructions":  nullStr(care),
	}

	// Product images
	imgRows, err := s.db.Query(`
		SELECT image_url FROM product_images WHERE product_id = $1
	`, productID)
	if err == nil {
		defer imgRows.Close()
		var images []string
		for imgRows.Next() {
			var url string
			imgRows.Scan(&url)
			images = append(images, url)
		}
		product["images"] = images
	}

	// Variants with stock
	varRows, err := s.db.Query(`
		SELECT v.id, v.variant_code, v.name, v.sku, v.price, COALESCE(v.barcode, ''),
		       COALESCE(SUM(st.quantity), 0) AS total_stock
		FROM variants v
		LEFT JOIN stocks st ON st.variant_id = v.id
		WHERE v.product_id = $1 AND v.is_active = true
		GROUP BY v.id, v.variant_code, v.name, v.sku, v.price, v.barcode
		ORDER BY v.variant_code
	`, productID)
	if err != nil {
		return product, nil
	}
	defer varRows.Close()

	var variants []map[string]interface{}
	for varRows.Next() {
		var vid, vname, sku, barcode string
		var variantCode int
		var price, stock float64

		if err := varRows.Scan(&vid, &variantCode, &vname, &sku, &price, &barcode, &stock); err != nil {
			continue
		}

		v := map[string]interface{}{
			"id":           vid,
			"variant_code": variantCode,
			"name":         vname,
			"sku":          sku,
			"price":        price,
			"barcode":      barcode,
			"in_stock":     stock > 0,
		}

		// Variant images
		viRows, err := s.db.Query(`SELECT image_url FROM variant_images WHERE variant_id = $1`, vid)
		if err == nil {
			var vimages []string
			for viRows.Next() {
				var url string
				viRows.Scan(&url)
				vimages = append(vimages, url)
			}
			viRows.Close()
			if len(vimages) > 0 {
				v["images"] = vimages
			}
		}

		// Variant attributes
		attrRows, err := s.db.Query(`
			SELECT a.name, av.value
			FROM variant_attribute_mapping vam
			JOIN attribute_values av ON av.id = vam.attribute_value_id
			JOIN attributes a ON a.id = av.attribute_id
			WHERE vam.variant_id = $1
		`, vid)
		if err == nil {
			attrs := map[string]string{}
			for attrRows.Next() {
				var aName, aVal string
				attrRows.Scan(&aName, &aVal)
				attrs[aName] = aVal
			}
			attrRows.Close()
			if len(attrs) > 0 {
				v["attributes"] = attrs
			}
		}

		variants = append(variants, v)
	}
	product["variants"] = variants

	return product, nil
}

// ListCategories returns active categories.
func (s *Store) ListCategories() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT id, name, products_count, COALESCE(image_url, '') FROM categories WHERE is_active = true ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []map[string]interface{}
	for rows.Next() {
		var id, name, imageURL string
		var count int
		rows.Scan(&id, &name, &count, &imageURL)
		cats = append(cats, map[string]interface{}{
			"id":             id,
			"name":           name,
			"products_count": count,
			"image_url":      imageURL,
		})
	}
	return cats, nil
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
