package joborder

import "time"

type JobOrder struct {
	ID                   string     `json:"id"`
	JobNumber            string     `json:"job_number"`
	CustomerID           string     `json:"customer_id"`
	BranchID             *string    `json:"branch_id"`
	WarehouseID          *string    `json:"warehouse_id"`
	JobType              string     `json:"job_type"`
	MaterialSource       string     `json:"material_source"`
	Status               string     `json:"status"`
	PaymentStatus        string     `json:"payment_status"`
	ReceivedDate         time.Time  `json:"received_date"`
	ExpectedDeliveryDate *string    `json:"expected_delivery_date"`
	ActualDeliveryDate   *time.Time `json:"actual_delivery_date"`
	SubAmount            float64    `json:"sub_amount"`
	DiscountAmount       float64    `json:"discount_amount"`
	GSTAmount            float64    `json:"gst_amount"`
	NetAmount            float64    `json:"net_amount"`
	Notes                string     `json:"notes"`
	CreatedBy            string     `json:"created_by"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type JobOrderItem struct {
	ID          string  `json:"id"`
	JobOrderID  string  `json:"job_order_id"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Discount    float64 `json:"discount"`
	TaxPercent  float64 `json:"tax_percent"`
	CGST        float64 `json:"cgst"`
	SGST        float64 `json:"sgst"`
	TotalPrice  float64 `json:"total_price"`
}

type JobOrderMaterial struct {
	ID                 string  `json:"id"`
	JobOrderID         string  `json:"job_order_id"`
	RawMaterialStockID string  `json:"raw_material_stock_id"`
	QuantityUsed       float64 `json:"quantity_used"`
}

type JobOrderStatusEntry struct {
	ID         string    `json:"id"`
	JobOrderID string    `json:"job_order_id"`
	Status     string    `json:"status"`
	Notes      string    `json:"notes"`
	UpdatedBy  string    `json:"updated_by"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type JobOrderPayment struct {
	ID            string    `json:"id"`
	JobOrderID    string    `json:"job_order_id"`
	Amount        float64   `json:"amount"`
	PaymentMethod string    `json:"payment_method"`
	Reference     string    `json:"reference"`
	PaidAt        time.Time `json:"paid_at"`
}

const (
	MaterialSourceCustomer = "CUSTOMER"
	MaterialSourceStore    = "STORE"
)
