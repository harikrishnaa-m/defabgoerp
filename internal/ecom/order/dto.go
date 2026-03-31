package order

type CheckoutInput struct {
	AddressID      string  `json:"address_id"`
	PaymentMethod  string  `json:"payment_method"` // COD, RAZORPAY, UPI
	Notes          string  `json:"notes"`
	CouponCode     string  `json:"coupon_code"`
	ShippingCharge float64 `json:"shipping_charge"`
}

type UpdateStatusInput struct {
	Status string `json:"status"`
}

type UpdatePaymentInput struct {
	PaymentStatus string `json:"payment_status"`
	PaymentRef    string `json:"payment_ref"`
}
