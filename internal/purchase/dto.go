package purchase

type CreatePurchaseOrderInput struct {
	SupplierID   string                    `json:"supplier_id"`
	WarehouseID  string                    `json:"warehouse_id"`
	OrderDate    string                    `json:"order_date"`
	ExpectedDate string                    `json:"expected_date"`
	Items        []CreatePurchaseOrderItem `json:"items"`
}

type CreatePurchaseOrderItem struct {
	ItemName    string  `json:"item_name"`
	Description string  `json:"description"`
	HSNCode     string  `json:"hsn_code"`
	Unit        string  `json:"unit"` // METER, KG, PCS, ROLL
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	GSTPercent  float64 `json:"gst_percent"`
}

type UpdatePOStatusInput struct {
	Status string `json:"status"`
}

type UpdatePurchaseOrderInput struct {
	SupplierID   *string `json:"supplier_id"`
	WarehouseID  *string `json:"warehouse_id"`
	OrderDate    *string `json:"order_date"`
	ExpectedDate *string `json:"expected_date"`
}

type POListRow struct {
	ID           string  `json:"id"`
	PONumber     string  `json:"po_number"`
	SupplierID   string  `json:"supplier_id"`
	SupplierName string  `json:"supplier_name"`
	WarehouseID  string  `json:"warehouse_id"`
	Status       string  `json:"status"`
	OrderDate    string  `json:"order_date"`
	ExpectedDate string  `json:"expected_date"`
	TotalAmount  float64 `json:"total_amount"`
	TaxAmount    float64 `json:"tax_amount"`
	GrandTotal   float64 `json:"grand_total"`
	CreatedAt    string  `json:"created_at"`
}

type PODetailResponse struct {
	ID            string           `json:"id"`
	PONumber      string           `json:"po_number"`
	SupplierID    string           `json:"supplier_id"`
	SupplierName  string           `json:"supplier_name"`
	WarehouseID   string           `json:"warehouse_id"`
	WarehouseName string           `json:"warehouse_name"`
	Status        string           `json:"status"`
	OrderDate     string           `json:"order_date"`
	ExpectedDate  string           `json:"expected_date"`
	TotalAmount   float64          `json:"total_amount"`
	TaxAmount     float64          `json:"tax_amount"`
	GrandTotal    float64          `json:"grand_total"`
	CreatedAt     string           `json:"created_at"`
	Items         []POItemResponse `json:"items"`
}

type POItemResponse struct {
	ID          string  `json:"id"`
	ItemName    string  `json:"item_name"`
	Description string  `json:"description"`
	HSNCode     string  `json:"hsn_code"`
	Unit        string  `json:"unit"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	GSTPercent  float64 `json:"gst_percent"`
	GSTAmount   float64 `json:"gst_amount"`
	TotalPrice  float64 `json:"total_price"`
	ReceivedQty float64 `json:"received_qty"`
}

type AddPOItemInput struct {
	ItemName    string  `json:"item_name"`
	Description string  `json:"description"`
	HSNCode     string  `json:"hsn_code"`
	Unit        string  `json:"unit"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	GSTPercent  float64 `json:"gst_percent"`
}

type UpdatePOItemInput struct {
	ItemName    *string  `json:"item_name"`
	Description *string  `json:"description"`
	HSNCode     *string  `json:"hsn_code"`
	Unit        *string  `json:"unit"`
	Quantity    *float64 `json:"quantity"`
	UnitPrice   *float64 `json:"unit_price"`
	GSTPercent  *float64 `json:"gst_percent"`
}
