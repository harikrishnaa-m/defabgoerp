package variant

import (
	"database/sql"
	"fmt"
	"strings"
	// "github.com/google/uuid"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

//
// ================= CREATE =================
//

func (s *Store) Create(in CreateVariantInput) (string, string, int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", "", 0, err
	}
	defer tx.Rollback()

	// Auto-generate SKU: first 3 letters of brand + product name + attribute values
	var brand, productName string
	err = tx.QueryRow(`SELECT COALESCE(brand,''), name FROM products WHERE id = $1`, in.ProductID).Scan(&brand, &productName)
	if err != nil {
		return "", "", 0, fmt.Errorf("product not found: %w", err)
	}

	sku := strings.ToUpper(first3(brand) + " " + first3(productName))

	// Append first 3 letters of each attribute value
	for _, avid := range in.AttributeValueIDs {
		var val string
		err = tx.QueryRow(`SELECT value FROM attribute_values WHERE id = $1`, avid).Scan(&val)
		if err == nil {
			sku += " " + strings.ToUpper(first3(val))
		}
	}

	// Ensure uniqueness by appending a number if needed
	baseSku := sku
	var exists bool
	for i := 1; ; i++ {
		err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM variants WHERE sku = $1)`, sku).Scan(&exists)
		if err != nil {
			return "", "", 0, err
		}
		if !exists {
			break
		}
		sku = fmt.Sprintf("%s %d", baseSku, i)
	}

	var id string
	var variantCode int

	err = tx.QueryRow(`
	INSERT INTO variants
	(product_id,name,sku,price,cost_price,barcode)
	VALUES ($1,$2,$3,$4,$5,$6)
	RETURNING id, variant_code
	`,
		in.ProductID,
		in.Name,
		sku,
		in.Price,
		in.CostPrice,
		sku,
	).Scan(&id, &variantCode)

	if err != nil {
		return "", "", 0, err
	}

	for _, avid := range in.AttributeValueIDs {
		_, err := tx.Exec(`
			INSERT INTO variant_attribute_mapping
			(variant_id, attribute_value_id)
			VALUES ($1,$2)
		`, id, avid)
		if err != nil {
			return "", "", 0, err
		}
	}

	// Save images
	for _, imgPath := range in.ImagePaths {
		_, err := tx.Exec(`
			INSERT INTO variant_images
			(variant_id, image_url)
			VALUES ($1,$2)
		`, id, imgPath)
		if err != nil {
			return "", "", 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", "", 0, err
	}
	return id, sku, variantCode, nil
}

// first3 returns the first 3 letters of a string (uppercase, no spaces)
func first3(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	if len(s) >= 3 {
		return s[:3]
	}
	return s
}

//
// ================= LIST =================
//

func (s *Store) ListByProduct(pid string) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT id,variant_code,name,sku,price,cost_price,is_active
	FROM variants
	WHERE product_id=$1
	ORDER BY variant_code
	`, pid)
}

//
// ================= GET =================
//

func (s *Store) Get(id string) (*sql.Row, error) {
	return s.db.QueryRow(`
	SELECT id,product_id,variant_code,name,sku,price,cost_price,is_active
	FROM variants WHERE id=$1
	`, id), nil
}

func (s *Store) GetVariantAttributes(variantID string) ([]map[string]string, error) {
	rows, err := s.db.Query(`
		SELECT av.id, av.value, a.id, a.name
		FROM variant_attribute_mapping vam
		JOIN attribute_values av ON vam.attribute_value_id = av.id
		JOIN attributes a ON av.attribute_id = a.id
		WHERE vam.variant_id = $1
	`, variantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]string
	for rows.Next() {
		var avID, avValue, attID, attName string
		if err := rows.Scan(&avID, &avValue, &attID, &attName); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{
			"attribute_id":   attID,
			"attribute_name": attName,
			"value_id":       avID,
			"value_name":     avValue,
		})
	}
	return out, nil
}

//
// ================= UPDATE =================
//

func (s *Store) Update(id string, in UpdateVariantInput) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
	UPDATE variants SET
	name = COALESCE($1,name),
	price = COALESCE($2,price),
	cost_price = COALESCE($3,cost_price)
	WHERE id=$4
	`,
		in.Name,
		in.Price,
		in.CostPrice,
		id,
	)
	if err != nil {
		return err
	}

	if len(in.AttributeValueIDs) > 0 {
		_, err = tx.Exec(`DELETE FROM variant_attribute_mapping WHERE variant_id=$1`, id)
		if err != nil {
			return err
		}
		for _, avid := range in.AttributeValueIDs {
			_, err = tx.Exec(`INSERT INTO variant_attribute_mapping (variant_id, attribute_value_id) VALUES ($1,$2)`, id, avid)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

//
// ================= SOFT DELETE =================
//

func (s *Store) SetActive(id string, active bool) error {
	_, err := s.db.Exec(
		`UPDATE variants SET is_active=$1 WHERE id=$2`,
		active, id,
	)
	return err
}

//
// ================= ATTRIBUTE LOOKUP =================
//

func (s *Store) GetAttributeValues(ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no attribute value IDs provided")
	}
	query := fmt.Sprintf(`SELECT id, value FROM attribute_values WHERE id IN (%s)`, placeholders(len(ids)))
	args := make([]interface{}, len(ids))
	for i, v := range ids {
		args[i] = v
	}
	// Debug: print query and args
	fmt.Printf("GetAttributeValues query: %s\n", query)
	fmt.Printf("GetAttributeValues args: %v\n", args)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("attribute_values query error: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, val string
		if err := rows.Scan(&id, &val); err != nil {
			return nil, fmt.Errorf("attribute_values scan error: %w", err)
		}
		out[id] = val
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no attribute values found for provided IDs")
	}
	return out, nil
}

//
// ================= GENERATOR =================
//

// GenerateWithAttrOrder generates variants using attribute IDs and value IDs
func (s *Store) GenerateWithAttrOrder(productID string, basePrice float64, attrOrder []string, groups [][]string) ([]string, error) {
	combinations := cartesian(groups)

	valMap, err := s.GetAttributeValues(flatten(groups))
	if err != nil {
		fmt.Printf("GenerateWithAttrOrder: GetAttributeValues error: %v\n", err)
		return nil, err
	}

	var created []string

	fmt.Printf("GenerateWithAttrOrder: combinations = %v\n", combinations)
	for _, combo := range combinations {
		nameParts := []string{}
		for _, id := range combo {
			nameParts = append(nameParts, valMap[id])
		}
		name := strings.Join(nameParts, " ")

		fmt.Printf("GenerateWithAttrOrder: Creating variant: name=%s, combo=%v\n", name, combo)
		id, sku, _, err := s.Create(CreateVariantInput{
			ProductID:         productID,
			Name:              name,
			Price:             basePrice,
			AttributeValueIDs: combo,
		})
		if err != nil {
			fmt.Printf("GenerateWithAttrOrder: Create error: %v\n", err)
		} else {
			fmt.Printf("GenerateWithAttrOrder: Created variant id=%s, sku=%s\n", id, sku)
			created = append(created, id)
		}
	}
	return created, nil
}

//
//
// ================= BACKFILL VARIANT CODES =================
//

func (s *Store) BackfillVariantCodes() (int, error) {
	res, err := s.db.Exec(`
		UPDATE variants
		SET variant_code = nextval('variant_code_seq')
		WHERE variant_code IS NULL OR variant_code = 0
	`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

//
// ================= HELPERS =================
//

func placeholders(n int) string {
	p := make([]string, n)
	for i := range p {
		p[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(p, ",")
}

func flatten(g [][]string) []string {
	var out []string
	for _, a := range g {
		out = append(out, a...)
	}
	return out
}

func cartesian(groups [][]string) [][]string {
	if len(groups) == 0 {
		return [][]string{}
	}

	result := [][]string{{}}

	for _, group := range groups {
		var next [][]string
		for _, r := range result {
			for _, g := range group {
				n := append([]string{}, r...)
				n = append(n, g)
				next = append(next, n)
			}
		}
		result = next
	}
	return result
}

func (s *Store) getProductPrefix(productID string) (string, error) {
	var name string
	err := s.db.QueryRow(
		`SELECT name FROM products WHERE id=$1`,
		productID,
	).Scan(&name)

	if err != nil {
		return "", err
	}

	// simple prefix rule
	name = strings.ToUpper(name)
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "_", "")

	return name, nil
}
