package supplier

type CreateSupplierInput struct {
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Address   string `json:"address"`
	GSTNumber string `json:"gst_number"`
}

type UpdateSupplierInput struct {
	Name      *string `json:"name"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"`
	Address   *string `json:"address"`
	GSTNumber *string `json:"gst_number"`
}
