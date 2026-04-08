package joborder

type CreateJobOrderItemInput struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Discount    float64 `json:"discount"`
	TaxPercent  float64 `json:"tax_percent"`
	CGST        float64 `json:"cgst"`
	SGST        float64 `json:"sgst"`
	TotalPrice  float64 `json:"total_price"`
}

type CreateJobOrderMaterialInput struct {
	RawMaterialStockID string  `json:"raw_material_stock_id"`
	QuantityUsed       float64 `json:"quantity_used"`
}

type CreateJobOrderInput struct {
	CustomerID           string                        `json:"customer_id"`
	CustomerPhone        string                        `json:"customer_phone"`
	CustomerName         string                        `json:"customer_name"`
	CustomerEmail        string                        `json:"customer_email"`
	JobType              string                        `json:"job_type"`
	MaterialSource       string                        `json:"material_source"` // CUSTOMER or STORE
	ExpectedDeliveryDate *string                       `json:"expected_delivery_date"`
	Notes                string                        `json:"notes"`
	SubAmount            float64                       `json:"sub_amount"`
	DiscountAmount       float64                       `json:"discount_amount"`
	GSTAmount            float64                       `json:"gst_amount"`
	NetAmount            float64                       `json:"net_amount"`
	Items                []CreateJobOrderItemInput     `json:"items"`
	Materials            []CreateJobOrderMaterialInput `json:"materials"`
	Payments             []PaymentInput                `json:"payments"`
}

type UpdateJobOrderInput struct {
	CustomerID           *string                       `json:"customer_id"`
	CustomerPhone        string                        `json:"customer_phone"`
	CustomerName         string                        `json:"customer_name"`
	CustomerEmail        string                        `json:"customer_email"`
	JobType              *string                       `json:"job_type"`
	MaterialSource       *string                       `json:"material_source"`
	ExpectedDeliveryDate *string                       `json:"expected_delivery_date"`
	Notes                *string                       `json:"notes"`
	SubAmount            *float64                      `json:"sub_amount"`
	DiscountAmount       *float64                      `json:"discount_amount"`
	GSTAmount            *float64                      `json:"gst_amount"`
	NetAmount            *float64                      `json:"net_amount"`
	Items                []CreateJobOrderItemInput     `json:"items"`
	Materials            []CreateJobOrderMaterialInput `json:"materials"`
}

type StatusUpdateInput struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type PaymentInput struct {
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	Reference     string  `json:"reference"`
}
