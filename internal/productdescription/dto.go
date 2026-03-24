package productdescription

import "github.com/google/uuid"

type CreateProductDescriptionInput struct {
	ProductID         uuid.UUID `json:"product_id"`
	Description       string    `json:"description"`
	FabricComposition string    `json:"fabric_composition"`
	Pattern           string    `json:"pattern"`
	Occasion          string    `json:"occasion"`
	CareInstructions  string    `json:"care_instructions"`
	Length            float64   `json:"length"`
	Width             float64   `json:"width"`
	BlousePiece       float64   `json:"blouse_piece"`
	SizeChartImage    string    `json:"size_chart_image"`
}

type UpdateProductDescriptionInput struct {
	Description       *string  `json:"description"`
	FabricComposition *string  `json:"fabric_composition"`
	Pattern           *string  `json:"pattern"`
	Occasion          *string  `json:"occasion"`
	CareInstructions  *string  `json:"care_instructions"`
	Length            *float64 `json:"length"`
	Width             *float64 `json:"width"`
	BlousePiece       *float64 `json:"blouse_piece"`
	SizeChartImage    *string  `json:"size_chart_image"`
}