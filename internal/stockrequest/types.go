package stockrequest

type DispatchItem struct {
	VariantID string  `json:"variant_id"`
	Qty       float64 `json:"dispatch_qty"`
}
