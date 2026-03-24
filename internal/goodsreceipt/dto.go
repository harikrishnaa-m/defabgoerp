package goodsreceipt

type CreateGoodsReceiptInput struct {
	PurchaseOrderID string                   `json:"purchase_order_id" validate:"required"`
	Reference       string                   `json:"reference"` // Invoice / DC number
	Items           []CreateGoodsReceiptItem `json:"items" validate:"required,min=1"`
}

type CreateGoodsReceiptItem struct {
	PurchaseOrderItemID string  `json:"purchase_order_item_id" validate:"required"`
	ReceivedQty         float64 `json:"received_qty" validate:"required,gt=0"`
}

type GoodsReceiptResponse struct {
	ID              string                     `json:"id"`
	GRNNumber       string                     `json:"grn_number"`
	PurchaseOrderID string                     `json:"purchase_order_id"`
	PONumber        string                     `json:"po_number"`
	SupplierID      string                     `json:"supplier_id"`
	SupplierName    string                     `json:"supplier_name"`
	WarehouseID     string                     `json:"warehouse_id"`
	WarehouseName   string                     `json:"warehouse_name"`
	ReceivedBy      string                     `json:"received_by"`
	ReceivedByName  string                     `json:"received_by_name"`
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
	HSNCode             string  `json:"hsn_code"`
	Unit                string  `json:"unit"`
	OrderedQty          float64 `json:"ordered_qty"`
	ReceivedQty         float64 `json:"received_qty"`
}
