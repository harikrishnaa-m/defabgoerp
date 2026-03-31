package customer

import "time"

type RegisterInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type GoogleSignInInput struct {
	IDToken string `json:"id_token"`
}

type UpdateProfileInput struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type AddressInput struct {
	Label        string `json:"label"`
	FullName     string `json:"full_name"`
	Phone        string `json:"phone"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2"`
	City         string `json:"city"`
	State        string `json:"state"`
	Pincode      string `json:"pincode"`
	IsDefault    bool   `json:"is_default"`
}

type Address struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	FullName     string    `json:"full_name"`
	Phone        string    `json:"phone"`
	AddressLine1 string    `json:"address_line1"`
	AddressLine2 string    `json:"address_line2"`
	City         string    `json:"city"`
	State        string    `json:"state"`
	Pincode      string    `json:"pincode"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
}
