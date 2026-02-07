package model

import (
	"time"

	"github.com/google/uuid"
)

// Customer represents the Customer table.
type Customer struct {
	ID          uuid.UUID    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string       `gorm:"size:200;not null" json:"name"`
	Phone       string       `gorm:"size:30" json:"phone"`
	Email       string       `gorm:"size:150" json:"email"`
	Address     string       `gorm:"type:text" json:"address"`
	SalesOrders []SalesOrder `gorm:"foreignKey:CustomerID" json:"sales_orders"`
}

// SalesPerson represents the SalesPerson table.
type SalesPerson struct {
	ID           uuid.UUID    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name         string       `gorm:"size:200;not null" json:"name"`
	EmployeeCode string       `gorm:"size:50" json:"employee_code"`
	Phone        string       `gorm:"size:30" json:"phone"`
	Email        string       `gorm:"size:150" json:"email"`
	SalesOrders  []SalesOrder `gorm:"foreignKey:SalesPersonID" json:"sales_orders"`
}

// SalesOrder represents the SalesOrder table.
type SalesOrder struct {
	ID            uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SONumber      string           `gorm:"size:50;not null" json:"so_number"`
	CustomerID    uuid.UUID        `gorm:"type:uuid;not null;index" json:"customer_id"`
	Customer      Customer         `gorm:"foreignKey:CustomerID;references:ID" json:"customer"`
	SalesPersonID uuid.UUID        `gorm:"type:uuid;not null;index" json:"salesperson_id"`
	SalesPerson   SalesPerson      `gorm:"foreignKey:SalesPersonID;references:ID" json:"salesperson"`
	WarehouseID   uuid.UUID        `gorm:"type:uuid;not null;index" json:"warehouse_id"`
	Warehouse     Warehouse        `gorm:"foreignKey:WarehouseID;references:ID" json:"warehouse"`
	Subtotal      float64          `gorm:"type:numeric(12,2)" json:"subtotal"`
	DiscountTotal float64          `gorm:"type:numeric(12,2)" json:"discount_total"`
	GrandTotal    float64          `gorm:"type:numeric(12,2)" json:"grand_total"`
	Status        string           `gorm:"size:30" json:"status"`
	CreatedAt     time.Time        `gorm:"autoCreateTime" json:"created_at"`
	Items         []SalesOrderItem `gorm:"foreignKey:SalesOrderID" json:"items"`
}

// SalesOrderItem represents the SalesOrderItem table.
type SalesOrderItem struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SalesOrderID uuid.UUID  `gorm:"type:uuid;not null;index" json:"sales_order_id"`
	SalesOrder   SalesOrder `gorm:"foreignKey:SalesOrderID;references:ID" json:"sales_order"`
	VariantID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"variant_id"`
	Variant      Variant    `gorm:"foreignKey:VariantID;references:ID" json:"variant"`
	Quantity     int        `gorm:"not null" json:"quantity"`
	UnitPrice    float64    `gorm:"type:numeric(12,2)" json:"unit_price"`
}
