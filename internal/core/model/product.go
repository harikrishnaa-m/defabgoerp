package model

import (
	"time"

	"github.com/google/uuid"
)

// ================== CATEGORY ==================

type Category struct {
	ID   uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name string    `gorm:"size:150;not null;uniqueIndex" json:"name"`
}

// ================== PRODUCT ==================

type Product struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name       string    `gorm:"size:200;not null;index" json:"name"`
	CategoryID uuid.UUID `gorm:"type:uuid;not null;index" json:"category_id"`
	Category   Category  `gorm:"foreignKey:CategoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`

	Brand string `gorm:"size:150" json:"brand"`

	MainImageURL string `gorm:"size:500" json:"main_image_url"` // ✅ cover image

	IsWebVisible bool   `gorm:"default:true;index" json:"is_web_visible"`
	IsStitched   bool   `gorm:"default:false" json:"is_stitched"`
	UOM          string `gorm:"size:20;default:'Unit'" json:"uom"`

	Variants []Variant      `gorm:"foreignKey:ProductID" json:"variants"`
	Images   []ProductImage `gorm:"foreignKey:ProductID" json:"images"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProductImage struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProductID uuid.UUID `gorm:"type:uuid;not null;index"`
	Product   Product   `gorm:"foreignKey:ProductID;references:ID;constraint:OnDelete:CASCADE"`

	ImageURL string `gorm:"size:500;not null"`

	CreatedAt time.Time
}

// ================== VARIANT ==================

type Variant struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ProductID uuid.UUID `gorm:"type:uuid;not null;index" json:"product_id"`
	Product   Product   `gorm:"foreignKey:ProductID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`

	Name string `gorm:"size:150;not null" json:"name"`
	SKU  string `gorm:"size:100;not null;uniqueIndex" json:"sku"`

	Price     float64 `gorm:"not null" json:"price"`
	CostPrice float64 `json:"cost_price"`

	Barcode string         `gorm:"size:100;uniqueIndex"`
	Images  []VariantImage `gorm:"foreignKey:VariantID" json:"images"` // Added for images
}

type VariantImage struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	VariantID uuid.UUID `gorm:"type:uuid;not null;index"`
	Variant   Variant   `gorm:"foreignKey:VariantID;references:ID;constraint:OnDelete:CASCADE"`

	ImageURL string `gorm:"size:500;not null"`

	CreatedAt time.Time
}

// ================== VARIANT ATTRIBUTE MAP ==================

type VariantAttributeMapping struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	VariantID uuid.UUID `gorm:"type:uuid;not null;index:idx_variant_attr,unique"`
	Variant   Variant   `gorm:"foreignKey:VariantID;references:ID;constraint:OnDelete:CASCADE"`

	AttributeValueID uuid.UUID       `gorm:"type:uuid;not null;index:idx_variant_attr,unique"`
	AttributeValue   *AttributeValue `gorm:"foreignKey:AttributeValueID;references:ID;constraint:OnDelete:RESTRICT"`
}

// ================== BARCODE ==================

// type Barcode struct {
// 	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
// 	VariantID   uuid.UUID `gorm:"type:uuid;not null;index" json:"variant_id"`
// 	Code        string    `gorm:"size:100;not null;uniqueIndex" json:"code"`
// 	GeneratedAt time.Time `json:"generated_at"`
// }

// ================== PRODUCT DESCRIPTION ==================

type ProductDescription struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProductID uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	Product   Product   `gorm:"foreignKey:ProductID;references:ID;constraint:OnDelete:CASCADE"`

	Description       string `gorm:"type:text"`
	FabricComposition string `gorm:"size:200"`
	Pattern           string `gorm:"size:100"`
	Occasion          string `gorm:"size:100"`
	CareInstructions  string `gorm:"size:200"`
	Length            float64
	Width             float64
	BlousePiece       float64
	SizeChartImage    string `gorm:"size:500"` // stores URL or file path
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
