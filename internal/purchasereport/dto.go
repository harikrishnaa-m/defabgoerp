package purchasereport

// PurchaseReportRow is one row in the report.
type PurchaseReportRow struct {
	ID            string  `json:"id"`
	InvoiceNumber string  `json:"invoice_number"`
	BillDate      string  `json:"bill_date"`
	SupplierID    string  `json:"supplier_id"`
	PartyName     string  `json:"party_name"`
	PurchaseCost  float64 `json:"purchase_cost"`
	RoundOff      float64 `json:"round_off"`
	OtherCharges  float64 `json:"other_charges"`
	Tax           float64 `json:"tax"`
	Discount      float64 `json:"discount"`
	NetAmount     float64 `json:"net_amount"`
	WarehouseID   string  `json:"warehouse_id"`
	Location      string  `json:"location"`
}

// Totals holds column sums for the current page and for all filtered rows.
// The UI displays them as "page_total OF all_total" per column.
type Totals struct {
	PurchaseCostPage float64 `json:"purchase_cost_page"`
	PurchaseCostAll  float64 `json:"purchase_cost_all"`
	RoundOffPage     float64 `json:"round_off_page"`
	RoundOffAll      float64 `json:"round_off_all"`
	OtherChargesPage float64 `json:"other_charges_page"`
	OtherChargesAll  float64 `json:"other_charges_all"`
	TaxPage          float64 `json:"tax_page"`
	TaxAll           float64 `json:"tax_all"`
	DiscountPage     float64 `json:"discount_page"`
	DiscountAll      float64 `json:"discount_all"`
	NetAmountPage    float64 `json:"net_amount_page"`
	NetAmountAll     float64 `json:"net_amount_all"`
}

// ReportResult is the full API response.
type ReportResult struct {
	Data       []PurchaseReportRow `json:"data"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	Limit      int                 `json:"limit"`
	TotalPages int                 `json:"total_pages"`
	Totals     Totals              `json:"totals"`
}

// Filter holds all query parameters for the report.
type Filter struct {
	// Top-level filters (dropdown)
	WarehouseID string
	SupplierID  string

	// Column-level search (text partial match)
	SearchInvoice      string
	SearchDate         string
	SearchParty        string
	SearchPurchaseCost string
	SearchRoundOff     string
	SearchOtherCharges string
	SearchTax          string
	SearchDiscount     string
	SearchNetAmount    string
	SearchLocation     string

	// Pagination
	Page  int
	Limit int
}
