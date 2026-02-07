package model

import (
	"time"

	"github.com/google/uuid"
)

// Coupon represents the Coupon table.
type Coupon struct {
	ID                uuid.UUID          `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Code              string             `gorm:"size:50;not null" json:"code"`
	Description       string             `gorm:"type:text" json:"description"`
	DiscountType      string             `gorm:"size:20" json:"discount_type"`
	DiscountValue     float64            `gorm:"type:numeric(12,2)" json:"discount_value"`
	MinOrderValue     float64            `gorm:"type:numeric(12,2)" json:"min_order_value"`
	MaxDiscountAmount float64            `gorm:"type:numeric(12,2)" json:"max_discount_amount"`
	StartDate         time.Time          `json:"start_date"`
	EndDate           time.Time          `json:"end_date"`
	UsageLimit        int                `json:"usage_limit"`
	UsagePerCustomer  int                `json:"usage_per_customer"`
	IsActive          bool               `json:"is_active"`
	CreatedAt         time.Time          `gorm:"autoCreateTime" json:"created_at"`
	SalesOrderCoupons []SalesOrderCoupon `gorm:"foreignKey:CouponID" json:"sales_order_coupons"`
	CouponUsages      []CouponUsage      `gorm:"foreignKey:CouponID" json:"coupon_usages"`
	CouponVariants    []CouponVariant    `gorm:"foreignKey:CouponID" json:"coupon_variants"`
	CouponCategories  []CouponCategory   `gorm:"foreignKey:CouponID" json:"coupon_categories"`
}

// SalesOrderCoupon represents the SalesOrderCoupon table.
type SalesOrderCoupon struct {
	ID             uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SalesOrderID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"sales_order_id"`
	SalesOrder     SalesOrder `gorm:"foreignKey:SalesOrderID;references:ID" json:"sales_order"`
	CouponID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"coupon_id"`
	Coupon         Coupon     `gorm:"foreignKey:CouponID;references:ID" json:"coupon"`
	DiscountAmount float64    `gorm:"type:numeric(12,2)" json:"discount_amount"`
}

// CouponUsage represents the CouponUsage table.
type CouponUsage struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CouponID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"coupon_id"`
	Coupon       Coupon     `gorm:"foreignKey:CouponID;references:ID" json:"coupon"`
	CustomerID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"customer_id"`
	Customer     Customer   `gorm:"foreignKey:CustomerID;references:ID" json:"customer"`
	SalesOrderID uuid.UUID  `gorm:"type:uuid;not null;index" json:"sales_order_id"`
	SalesOrder   SalesOrder `gorm:"foreignKey:SalesOrderID;references:ID" json:"sales_order"`
	UsedAt       time.Time  `json:"used_at"`
}

// CouponVariant represents the CouponVariant table.
type CouponVariant struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CouponID  uuid.UUID `gorm:"type:uuid;not null;index" json:"coupon_id"`
	Coupon    Coupon    `gorm:"foreignKey:CouponID;references:ID" json:"coupon"`
	VariantID uuid.UUID `gorm:"type:uuid;not null;index" json:"variant_id"`
	Variant   Variant   `gorm:"foreignKey:VariantID;references:ID" json:"variant"`
}

// CouponCategory represents the CouponCategory table.
type CouponCategory struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CouponID   uuid.UUID `gorm:"type:uuid;not null;index" json:"coupon_id"`
	Coupon     Coupon    `gorm:"foreignKey:CouponID;references:ID" json:"coupon"`
	CategoryID uuid.UUID `gorm:"type:uuid;not null;index" json:"category_id"`
	Category   Category  `gorm:"foreignKey:CategoryID;references:ID" json:"category"`
}
