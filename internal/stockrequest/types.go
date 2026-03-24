package stockrequest

type DispatchItem struct {
	VariantID string `json:"variant_id"`
	Qty       int    `json:"dispatch_qty"`
}
