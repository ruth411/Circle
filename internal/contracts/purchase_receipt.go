package contracts

import (
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
)

const PurchaseReceiptSourceType = "purchase_receipt"

type PurchaseReceipt struct {
	ReceiptID  string
	LocationID string
	OccurredAt time.Time
	SourceType string
	SourceID   string
	Lines      []PurchaseReceiptLine
}

type PurchaseReceiptLine struct {
	IngredientID      string
	QuantityBaseUnits float64
	Unit              ingredient.Unit
}
