package productdescription

import (
	"database/sql"

	"github.com/google/uuid"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

/* CREATE (one-time insert) */
func (s *Store) Create(in CreateProductDescriptionInput) error {
	_, err := s.db.Exec(`
		INSERT INTO product_descriptions (
			product_id, description, fabric_composition,
			pattern, occasion, care_instructions,
			length, width, blouse_piece, size_chart_image
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`,
		in.ProductID,
		in.Description,
		in.FabricComposition,
		in.Pattern,
		in.Occasion,
		in.CareInstructions,
		in.Length,
		in.Width,
		in.BlousePiece,
		in.SizeChartImage,
	)

	return err
}

/* LIST / GET by product */
func (s *Store) Get(productID uuid.UUID) (*sql.Row, error) {
	return s.db.QueryRow(`
		SELECT
			id, product_id, description, fabric_composition,
			pattern, occasion, care_instructions,
			length, width, blouse_piece, size_chart_image,
			created_at, updated_at
		FROM product_descriptions
		WHERE product_id = $1
	`, productID), nil
}

/* UPDATE (PATCH style like Branch) */
func (s *Store) Update(productID uuid.UUID, in UpdateProductDescriptionInput) error {
	_, err := s.db.Exec(`
		UPDATE product_descriptions
		SET
			description = COALESCE($1, description),
			fabric_composition = COALESCE($2, fabric_composition),
			pattern = COALESCE($3, pattern),
			occasion = COALESCE($4, occasion),
			care_instructions = COALESCE($5, care_instructions),
			length = COALESCE($6, length),
			width = COALESCE($7, width),
			blouse_piece = COALESCE($8, blouse_piece),
			size_chart_image = COALESCE($9, size_chart_image),
			updated_at = CURRENT_TIMESTAMP
		WHERE product_id = $10
	`,
		in.Description,
		in.FabricComposition,
		in.Pattern,
		in.Occasion,
		in.CareInstructions,
		in.Length,
		in.Width,
		in.BlousePiece,
		in.SizeChartImage,
		productID,
	)

	return err
}