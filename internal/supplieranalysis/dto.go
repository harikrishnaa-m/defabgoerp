package supplieranalysis

// SupplierAnalysisRow is one item line in the report.
type SupplierAnalysisRow struct {
	SupplierID   string  `json:"supplier_id"`
	SupplierName string  `json:"supplier_name"`
	ItemName     string  `json:"item_name"`
	Quantity     float64 `json:"quantity"`
	UOM          string  `json:"uom"`
	Rate         float64 `json:"rate"`
	PONumber     string  `json:"po_number"`
	PODate       string  `json:"po_date"`
	Discount     float64 `json:"discount"`
	Location     string  `json:"location"`
	WarehouseID  string  `json:"warehouse_id"`
}

// SupplierAnalysisResponse is the full response.
type SupplierAnalysisResponse struct {
	SupplierID   string                `json:"supplier_id"`   // empty when search_by_item=true
	SupplierName string                `json:"supplier_name"` // empty when search_by_item=true
	Month        int                   `json:"month"`
	Year         int                   `json:"year"`
	Items        []SupplierAnalysisRow `json:"items"`
}

// Filter holds query parameters.
type Filter struct {
	SupplierID   string // required unless SearchByItem=true
	Month        int    // 1-12, required
	Year         int    // default: current year
	WarehouseID  string // optional — filter to one location
	SearchItem   string // item code/name (partial match)
	SearchByItem bool   // when true: search item across all suppliers
}
