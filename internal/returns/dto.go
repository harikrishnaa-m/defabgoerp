package returns

type CreateReturnItemInput struct {
	SalesInvoiceItemID string `json:"sales_invoice_item_id"`
	Quantity           int    `json:"quantity"`
	Reason             string `json:"reason"`
}

type CreateReturnOrderInput struct {
	SalesInvoiceID  string                  `json:"sales_invoice_id"`
	Items           []CreateReturnItemInput `json:"items"`
	Notes           string                  `json:"notes"`
	RefundType      string                  `json:"refund_type"`   // CASH or CREDIT
	RefundMethod    string                  `json:"refund_method"` // CASH, BANK_TRANSFER, UPI, CARD
	RefundReference string                  `json:"refund_reference"`
}

type CompleteReturnInput struct {
	RefundType      string `json:"refund_type"`   // CASH or CREDIT
	RefundMethod    string `json:"refund_method"` // CASH, BANK_TRANSFER, UPI, CARD
	RefundReference string `json:"refund_reference"`
}

type ReturnListFilter struct {
	BranchID *string `query:"branch_id"`
	Status   string  `query:"status"`
	Search   string  `query:"search"`
	Limit    int     `query:"limit"`
	Offset   int     `query:"offset"`
}
