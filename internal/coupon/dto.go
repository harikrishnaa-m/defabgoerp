package coupon

import "time"

type CreateCouponInput struct {
	Code              string    `json:"code"`
	Description       string    `json:"description"`
	DiscountType      string    `json:"discount_type"` // FLAT | PERCENT
	DiscountValue     float64   `json:"discount_value"`
	MinOrderValue     float64   `json:"min_order_value"`
	MaxDiscountAmount float64   `json:"max_discount_amount"`
	StartDate         time.Time `json:"start_date"`
	EndDate           time.Time `json:"end_date"`
	UsageLimit        int       `json:"usage_limit"`
	UsagePerCustomer  int       `json:"usage_per_customer"`
}

type UpdateCouponInput struct {
	Description       *string    `json:"description"`
	DiscountType      *string    `json:"discount_type"`
	DiscountValue     *float64   `json:"discount_value"`
	MinOrderValue     *float64   `json:"min_order_value"`
	MaxDiscountAmount *float64   `json:"max_discount_amount"`
	StartDate         *time.Time `json:"start_date"`
	EndDate           *time.Time `json:"end_date"`
	UsageLimit        *int       `json:"usage_limit"`
	UsagePerCustomer  *int       `json:"usage_per_customer"`
}
