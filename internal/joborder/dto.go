package joborder

// WorkEntry describes a single unit of work on a garment piece.
// type values: DYING | STITCHING | MACHINE | LINING | OTHER
// Fields used per type:
//
//	DYING    → color_code (required), notes (optional)
//	STITCHING→ notes (optional)
//	MACHINE  → notes (optional)
//	LINING   → material (required), notes (optional)
//	OTHER    → description (required)
type WorkEntry struct {
	Type        string `json:"type"`
	ColorCode   string `json:"color_code,omitempty"`
	Material    string `json:"material,omitempty"`
	Notes       string `json:"notes,omitempty"`
	Description string `json:"description,omitempty"`
}

// PieceEntry describes one component of a garment item.
// For sets (Churidar, Salwar etc.) — include one entry per piece (TOP, BOTTOM, PALAZZO).
// For single-piece garments (Blouse, Kurthi) — include exactly one entry with piece_type = "".
type PieceEntry struct {
	PieceType  string      `json:"piece_type"` // "TOP" | "BOTTOM" | "PALAZZO" | "" for single pieces
	WithLining bool        `json:"with_lining"`
	Works      []WorkEntry `json:"works"`
}

// CreateJobOrderItemInput represents one garment item in a job order.
// Sets (Churidar, Salwar etc.) are expressed as a single item with multiple pieces.
// sub_category is optional — send empty string if not applicable.
type CreateJobOrderItemInput struct {
	// Garment structure
	Category    string       `json:"category"`     // e.g. "CHURIDAR SET", "BLOUSE", "KURTHI"
	SubCategory string       `json:"sub_category"` // e.g. "NORMAL with L", "PRINCESS CUT without L" (optional)
	Pieces      []PieceEntry `json:"pieces"`       // one per component; single-piece → one entry, piece_type=""

	// Pricing — fully decided by client/frontend
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	Discount   float64 `json:"discount"`
	TaxPercent float64 `json:"tax_percent"`
	CGST       float64 `json:"cgst"`
	SGST       float64 `json:"sgst"`
	TotalPrice float64 `json:"total_price"`
}

type CreateJobOrderMaterialInput struct {
	RawMaterialStockID string  `json:"raw_material_stock_id"`
	VariantID          string  `json:"variant_id"`
	WarehouseID        string  `json:"warehouse_id"`
	QuantityUsed       float64 `json:"quantity_used"`
}

type CreateJobOrderInput struct {
	CustomerID            string                        `json:"customer_id"`
	CustomerPhone         string                        `json:"customer_phone"`
	CustomerName          string                        `json:"customer_name"`
	CustomerEmail         string                        `json:"customer_email"`
	JobType               string                        `json:"job_type"`
	MaterialSource        string                        `json:"material_source"` // CUSTOMER or STORE
	ExpectedDeliveryDate  *string                       `json:"expected_delivery_date"`
	Notes                 string                        `json:"notes"`
	SampleProvided        bool                          `json:"sample_provided"`
	SampleDescription     string                        `json:"sample_description"`
	MeasurementBillNumber string                        `json:"measurement_bill_number"`
	ImageURL              string                        `json:"image_url"`
	DesignImageURL        string                        `json:"design_image_url"`
	SubAmount             float64                       `json:"sub_amount"`
	DiscountAmount        float64                       `json:"discount_amount"`
	GSTAmount             float64                       `json:"gst_amount"`
	NetAmount             float64                       `json:"net_amount"`
	Items                 []CreateJobOrderItemInput     `json:"items"`
	Materials             []CreateJobOrderMaterialInput `json:"materials"`
	Payments              []PaymentInput                `json:"payments"`
}

type UpdateJobOrderInput struct {
	CustomerID            *string                       `json:"customer_id"`
	CustomerPhone         string                        `json:"customer_phone"`
	CustomerName          string                        `json:"customer_name"`
	CustomerEmail         string                        `json:"customer_email"`
	JobType               *string                       `json:"job_type"`
	MaterialSource        *string                       `json:"material_source"`
	ExpectedDeliveryDate  *string                       `json:"expected_delivery_date"`
	Notes                 *string                       `json:"notes"`
	SampleProvided        *bool                         `json:"sample_provided"`
	SampleDescription     *string                       `json:"sample_description"`
	MeasurementBillNumber *string                       `json:"measurement_bill_number"`
	ImageURL              *string                       `json:"image_url"`
	DesignImageURL        *string                       `json:"design_image_url"`
	SubAmount             *float64                      `json:"sub_amount"`
	DiscountAmount        *float64                      `json:"discount_amount"`
	GSTAmount             *float64                      `json:"gst_amount"`
	NetAmount             *float64                      `json:"net_amount"`
	Items                 []CreateJobOrderItemInput     `json:"items"`
	Materials             []CreateJobOrderMaterialInput `json:"materials"`
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
