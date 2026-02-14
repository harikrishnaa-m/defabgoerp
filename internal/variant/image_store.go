package variant

import "database/sql"

func (s *Store) InsertImage(variantID, url string) error {
	_, err := s.db.Exec(`
	INSERT INTO variant_images (variant_id, image_url)
	VALUES ($1,$2)
	`, variantID, url)
	return err
}

func (s *Store) ListImages(variantID string) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT id, image_url, created_at
	FROM variant_images
	WHERE variant_id=$1
	ORDER BY created_at
	`, variantID)
}

func (s *Store) GetImage(imageID string) (string, error) {
	var url string
	err := s.db.QueryRow(`
	SELECT image_url FROM variant_images
	WHERE id=$1
	`, imageID).Scan(&url)
	return url, err
}

func (s *Store) DeleteImage(imageID string) error {
	_, err := s.db.Exec(`
	DELETE FROM variant_images WHERE id=$1
	`, imageID)
	return err
}
