package supplieraging

// SupplierAgingRow is one supplier's aging breakdown.
type SupplierAgingRow struct {
	SlNo          int     `json:"sl_no"`
	SupplierID    string  `json:"supplier_id"`
	SupplierName  string  `json:"supplier_name"`
	SupplierRefNo string  `json:"supplier_ref_no"` // PO number
	Location      string  `json:"location"`        // branch name
	Opening       float64 `json:"opening"`         // sum of net_amount in range
	Pending       float64 `json:"pending"`         // sum of (net_amount - paid_amount)
	Under7Days    float64 `json:"under_7_days"`    // pending aged < 7 days from To date
	Days7To15     float64 `json:"days_7_to_15"`
	Days15To31    float64 `json:"days_15_to_31"`
	Over31Days    float64 `json:"over_31_days"`
}

// SupplierAgingTotals holds footer aggregate values.
type SupplierAgingTotals struct {
	Opening         float64 `json:"opening"`
	TotalOpening    float64 `json:"total_opening"` // sum of all invoices (no date filter)
	Pending         float64 `json:"pending"`
	TotalPending    float64 `json:"total_pending"` // total outstanding across all time
	Under7Days      float64 `json:"under_7_days"`
	TotalUnder7     float64 `json:"total_under_7"`
	Days7To15       float64 `json:"days_7_to_15"`
	TotalDays7To15  float64 `json:"total_days_7_to_15"`
	Days15To31      float64 `json:"days_15_to_31"`
	Days15To31Total float64 `json:"days_15_to_31_total"`
	Over31Days      float64 `json:"over_31_days"`
	Over31DaysTotal float64 `json:"over_31_days_total"`
}

// SupplierAgingResponse is the full response.
type SupplierAgingResponse struct {
	DateFrom string              `json:"date_from"`
	DateTo   string              `json:"date_to"`
	Rows     []SupplierAgingRow  `json:"rows"`
	Totals   SupplierAgingTotals `json:"totals"`
}
