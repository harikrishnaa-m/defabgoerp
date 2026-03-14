package goodsreceipt

type CreateGoodsReceiptInput struct {
	PurchaseOrderID string                   `json:"purchase_order_id" validate:"required"`
	SupplierID      string                   `json:"supplier_id" validate:"required"`
	WarehouseID     string                   `json:"warehouse_id" validate:"required"`
	Reference       string                   `json:"reference"` // Invoice / DC number
	Items           []CreateGoodsReceiptItem `json:"items" validate:"required,min=1"`
}

type CreateGoodsReceiptItem struct {
	PurchaseOrderItemID string  `json:"purchase_order_item_id" validate:"required"`
	OrderedQty          float64 `json:"ordered_qty"`
	ReceivedQty         float64 `json:"received_qty" validate:"required,gt=0"`
}

type GoodsReceiptResponse struct {
	ID              string                     `json:"id"`
	GRNNumber       string                     `json:"grn_number"`
	PurchaseOrderID string                     `json:"purchase_order_id"`
	SupplierID      string                     `json:"supplier_id"`
	WarehouseID     string                     `json:"warehouse_id"`
	ReceivedBy      string                     `json:"received_by"`
	ReceivedDate    string                     `json:"received_date"`
	Reference       string                     `json:"reference"`
	Status          string                     `json:"status"`
	CreatedAt       string                     `json:"created_at"`
	Items           []GoodsReceiptItemResponse `json:"items"`
}

type GoodsReceiptItemResponse struct {
	ID                  string  `json:"id"`
	PurchaseOrderItemID string  `json:"purchase_order_item_id"`
	ItemName            string  `json:"item_name"`
	OrderedQty          float64 `json:"ordered_qty"`
	ReceivedQty         float64 `json:"received_qty"`
}
