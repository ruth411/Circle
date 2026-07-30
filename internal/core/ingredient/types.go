package ingredient

import (
	"fmt"
	"math"
	"time"
)

const costScale int64 = 10000

type Unit string

const (
	UnitGram       Unit = "g"
	UnitMilliliter Unit = "ml"
	UnitEach       Unit = "each"
)

type Provenance string

const (
	ProvenanceManual             Provenance = "manual"
	ProvenanceUSDA               Provenance = "usda"
	ProvenanceRestaurantOfficial Provenance = "restaurant_official"
)

type VerificationStatus string

const (
	VerificationVerified   VerificationStatus = "verified"
	VerificationUnverified VerificationStatus = "unverified"
)

type MacroValues struct {
	Calories     float64
	ProteinGrams float64
	CarbsGrams   float64
	FatGrams     float64
}

type CostPerBaseUnit int64

func NewCostPerBaseUnit(value float64) CostPerBaseUnit {
	return CostPerBaseUnit(roundToScale(value, costScale))
}

func MustCostPerBaseUnit(value float64) CostPerBaseUnit {
	return NewCostPerBaseUnit(value)
}

func (c CostPerBaseUnit) Float64() float64 {
	return float64(c) / float64(costScale)
}

func (c CostPerBaseUnit) ScaledForQuantity(quantity float64) float64 {
	return float64(c) * quantity
}

func (c CostPerBaseUnit) MinorForQuantity(quantity float64) int64 {
	return roundToInt64(c.ScaledForQuantity(quantity) / float64(costScale))
}

func (m MacroValues) Add(other MacroValues) MacroValues {
	return MacroValues{
		Calories:     m.Calories + other.Calories,
		ProteinGrams: m.ProteinGrams + other.ProteinGrams,
		CarbsGrams:   m.CarbsGrams + other.CarbsGrams,
		FatGrams:     m.FatGrams + other.FatGrams,
	}
}

func (m MacroValues) Scale(multiplier float64) MacroValues {
	return MacroValues{
		Calories:     m.Calories * multiplier,
		ProteinGrams: m.ProteinGrams * multiplier,
		CarbsGrams:   m.CarbsGrams * multiplier,
		FatGrams:     m.FatGrams * multiplier,
	}
}

type Ingredient struct {
	ID                     string
	LocationID             string
	SourceItemID           string
	Name                   string
	Category               string
	BaseUnit               Unit
	AlternateUnits         map[Unit]float64
	MacrosPerBaseUnit      MacroValues
	CurrentCostPerBaseUnit CostPerBaseUnit
	Currency               string
	OnHandBaseUnits        float64
	ParLevelBaseUnits      float64
	Provenance             Provenance
	VerificationStatus     VerificationStatus
	ServingSizeQuantity    float64
	ServingSizeUnit        string
	YieldFactors           map[string]float64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (i Ingredient) ToBaseUnit(quantity float64, unit Unit) (float64, error) {
	if quantity < 0 {
		return 0, fmt.Errorf("quantity must be non-negative")
	}

	if unit == i.BaseUnit {
		return quantity, nil
	}

	factor, ok := i.AlternateUnits[unit]
	if !ok || factor <= 0 {
		return 0, fmt.Errorf("missing conversion from %s to %s", unit, i.BaseUnit)
	}

	return quantity * factor, nil
}

func (i Ingredient) YieldFactor(method string) (float64, bool) {
	if method == "" {
		return 1, true
	}

	factor, ok := i.YieldFactors[method]
	if !ok || factor <= 0 {
		return 0, false
	}

	return factor, true
}

func roundToScale(value float64, scale int64) int64 {
	return roundToInt64(value * float64(scale))
}

func roundToInt64(value float64) int64 {
	if value < 0 {
		return int64(math.Ceil(value - 0.5))
	}
	return int64(math.Floor(value + 0.5))
}
