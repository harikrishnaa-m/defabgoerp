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
	LastNMonths  int                   `json:"last_n_months"` // number of months shown
	DateFrom     string                `json:"date_from"`     // first day of earliest month
	DateTo       string                `json:"date_to"`       // today
	Items        []SupplierAnalysisRow `json:"items"`
}

// Filter holds query parameters.
type Filter struct {
	SupplierID   string // required unless SearchByItem=true
	LastNMonths  int    // 1 = current month only, 2 = current + previous, etc.
	WarehouseID  string // optional — filter to one location
	SearchItem   string // item code/name (partial match)
	SearchByItem bool   // when true: search item across all suppliers
}
