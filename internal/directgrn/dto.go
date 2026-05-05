package directgrn

// DirectGRNItem represents one line item in a Direct GRN.
type DirectGRNItem struct {
	ItemName             string  `json:"item_name"`
	Description          string  `json:"description"`
	ProductCode          string  `json:"product_code"`
	Category             string  `json:"category"`
	HSNCode              string  `json:"hsn_code"`
	Unit                 string  `json:"unit"`
	Quantity             float64 `json:"quantity"`
	FreeQty              float64 `json:"free_qty"`
	UnitPrice            float64 `json:"unit_price"`
	GSTPercent           float64 `json:"gst_percent"`
	AdditionalWork       string  `json:"additional_work"`
	AdditionalWorkAmount float64 `json:"additional_work_amount"`
	PaidByUserID         string  `json:"paid_by_user_id"`     // references users.id (internal staff)
	PaidToSupplierID     string  `json:"paid_to_supplier_id"` // references suppliers.id
	CashAmount           float64 `json:"cash_amount"`
	CreditAmount         float64 `json:"credit_amount"`
}

// DirectGRNCharge represents an extra charge (freight, coolie, handling, etc.).
type DirectGRNCharge struct {
	ChargeType string  `json:"charge_type"`
	Amount     float64 `json:"amount"`
}

// DirectGRNInput is the payload for POST /api/direct-grn.
type DirectGRNInput struct {
	SupplierID          string            `json:"supplier_id"`
	WarehouseID         string            `json:"warehouse_id"`
	PurchaseType        string            `json:"purchase_type"`
	OrderDate           string            `json:"order_date"`
	TransportSupplierID string            `json:"transport_supplier_id"` // references suppliers.id
	LRNumber            string            `json:"lr_number"`
	InvoiceNumber       string            `json:"invoice_number"`
	InvoiceDate         string            `json:"invoice_date"`
	DiscountAmount      float64           `json:"discount_amount"`
	RoundOff            float64           `json:"round_off"`
	Notes               string            `json:"notes"`
	Reference           string            `json:"reference"`
	PaymentMethod       string            `json:"payment_method"`
	PaymentAmount       float64           `json:"payment_amount"`
	Items               []DirectGRNItem   `json:"items"`
	Charges             []DirectGRNCharge `json:"charges"`
}

// DirectGRNResult is returned after a successful Direct GRN creation.
type DirectGRNResult struct {
	POID          string  `json:"po_id"`
	PONumber      string  `json:"po_number"`
	GRNID         string  `json:"grn_id"`
	GRNNumber     string  `json:"grn_number"`
	InvoiceID     string  `json:"invoice_id"`
	InvoiceNumber string  `json:"invoice_number"`
	NetAmount     float64 `json:"net_amount"`
}

// ListFilter holds optional query filters for the list endpoint.
type ListFilter struct {
	GRNNumber    string // partial, case-insensitive
	SupplierName string // partial, case-insensitive
	DateFrom     string // YYYY-MM-DD, filters received_date >=
	DateTo       string // YYYY-MM-DD, filters received_date <=
}

// DirectGRNListRow is one row in the list view.
type DirectGRNListRow struct {
	GRNID                 string  `json:"grn_id"`
	GRNNumber             string  `json:"grn_number"`
	POID                  string  `json:"po_id"`
	PONumber              string  `json:"po_number"`
	PurchaseType          string  `json:"purchase_type"`
	SupplierID            string  `json:"supplier_id"`
	SupplierName          string  `json:"supplier_name"`
	WarehouseID           string  `json:"warehouse_id"`
	WarehouseName         string  `json:"warehouse_name"`
	TransportSupplierID   string  `json:"transport_supplier_id"`
	TransportSupplierName string  `json:"transport_supplier_name"`
	LRNumber              string  `json:"lr_number"`
	ReceivedDate          string  `json:"received_date"`
	ExpectedDate          string  `json:"expected_date"`
	InvoiceID             string  `json:"invoice_id"`
	InvoiceNumber         string  `json:"invoice_number"`
	InvoiceDate           string  `json:"invoice_date"`
	NetAmount             float64 `json:"net_amount"`
	AdditionalCharges     float64 `json:"additional_charges"`
}

// DirectGRNDetailItem is one item in the detail view.
type DirectGRNDetailItem struct {
	ID                   string  `json:"id"`
	ItemName             string  `json:"item_name"`
	Description          string  `json:"description"`
	ProductCode          string  `json:"product_code"`
	Category             string  `json:"category"`
	HSNCode              string  `json:"hsn_code"`
	Unit                 string  `json:"unit"`
	Quantity             float64 `json:"quantity"`
	FreeQty              float64 `json:"free_qty"`
	UnitPrice            float64 `json:"unit_price"`
	GSTPercent           float64 `json:"gst_percent"`
	GSTAmount            float64 `json:"gst_amount"`
	TotalPrice           float64 `json:"total_price"`
	AdditionalWork       string  `json:"additional_work"`
	AdditionalWorkAmount float64 `json:"additional_work_amount"`
	PaidByUserID         string  `json:"paid_by_user_id"`
	PaidByUserName       string  `json:"paid_by_user_name"`
	PaidToSupplierID     string  `json:"paid_to_supplier_id"`
	PaidToSupplierName   string  `json:"paid_to_supplier_name"`
	CashAmount           float64 `json:"cash_amount"`
	CreditAmount         float64 `json:"credit_amount"`
}

// DirectGRNDetailCharge is one charge row in the detail view.
type DirectGRNDetailCharge struct {
	ID         string  `json:"id"`
	ChargeType string  `json:"charge_type"`
	Amount     float64 `json:"amount"`
}

// DirectGRNDetail is the full detail response for GET /direct-grn/:id.
type DirectGRNDetail struct {
	GRNID                 string                  `json:"grn_id"`
	GRNNumber             string                  `json:"grn_number"`
	POID                  string                  `json:"po_id"`
	PONumber              string                  `json:"po_number"`
	PurchaseType          string                  `json:"purchase_type"`
	SupplierID            string                  `json:"supplier_id"`
	SupplierName          string                  `json:"supplier_name"`
	WarehouseID           string                  `json:"warehouse_id"`
	WarehouseName         string                  `json:"warehouse_name"`
	TransportSupplierID   string                  `json:"transport_supplier_id"`
	TransportSupplierName string                  `json:"transport_supplier_name"`
	LRNumber              string                  `json:"lr_number"`
	ReceivedDate          string                  `json:"received_date"`
	ExpectedDate          string                  `json:"expected_date"`
	InvoiceID             string                  `json:"invoice_id"`
	InvoiceNumber         string                  `json:"invoice_number"`
	InvoiceDate           string                  `json:"invoice_date"`
	SubAmount             float64                 `json:"sub_amount"`
	DiscountAmount        float64                 `json:"discount_amount"`
	GSTAmount             float64                 `json:"gst_amount"`
	RoundOff              float64                 `json:"round_off"`
	NetAmount             float64                 `json:"net_amount"`
	PaidAmount            float64                 `json:"paid_amount"`
	Status                string                  `json:"status"`
	Notes                 string                  `json:"notes"`
	Reference             string                  `json:"reference"`
	Items                 []DirectGRNDetailItem   `json:"items"`
	Charges               []DirectGRNDetailCharge `json:"charges"`
}
