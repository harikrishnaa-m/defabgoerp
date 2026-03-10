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

func (s *Store) Create(in CreateVariantInput) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var id string

	err = tx.QueryRow(`
	INSERT INTO variants
	(product_id,name,sku,price,cost_price,barcode)
	VALUES ($1,$2,$3,$4,$5,$6)
	RETURNING id
	`,
		in.ProductID,
		in.Name,
		in.SKU,
		in.Price,
		in.CostPrice,
		in.SKU,
	).Scan(&id)

	if err != nil {
		return "", err
	}

	for _, avid := range in.AttributeValueIDs {
		_, err := tx.Exec(`
			INSERT INTO variant_attribute_mapping
			(variant_id, attribute_value_id)
			VALUES ($1,$2)
		`, id, avid)
		if err != nil {
			return "", err
		}
	}

	return id, tx.Commit()
}

//
// ================= LIST =================
//

func (s *Store) ListByProduct(pid string) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT id,name,sku,price,cost_price,is_active
	FROM variants
	WHERE product_id=$1
	ORDER BY name
	`, pid)
}

//
// ================= GET =================
//

func (s *Store) Get(id string) (*sql.Row, error) {
	return s.db.QueryRow(`
	SELECT id,product_id,name,sku,price,cost_price,is_active
	FROM variants WHERE id=$1
	`, id), nil
}

//
// ================= UPDATE =================
//

func (s *Store) Update(id string, in UpdateVariantInput) error {
	_, err := s.db.Exec(`
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
	return err
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
	prefix, _ := s.getProductPrefix(productID)

	fmt.Printf("GenerateWithAttrOrder: combinations = %v\n", combinations)
	for _, combo := range combinations {
		nameParts := []string{}
		for _, id := range combo {
			nameParts = append(nameParts, valMap[id])
		}
		name := strings.Join(nameParts, " ")
		sku := prefix + "-" + strings.ToUpper(strings.Join(nameParts, "-"))

		fmt.Printf("GenerateWithAttrOrder: Creating variant: name=%s, sku=%s, combo=%v\n", name, sku, combo)
		id, err := s.Create(CreateVariantInput{
			ProductID:         productID,
			Name:              name,
			SKU:               sku,
			Price:             basePrice,
			AttributeValueIDs: combo,
		})
		if err != nil {
			fmt.Printf("GenerateWithAttrOrder: Create error: %v\n", err)
		} else {
			fmt.Printf("GenerateWithAttrOrder: Created variant id=%s\n", id)
			created = append(created, id)
		}
	}
	return created, nil
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
