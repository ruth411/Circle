package ingredient

import "fmt"

type Unit string

const (
	UnitGram       Unit = "g"
	UnitMilliliter Unit = "ml"
	UnitEach       Unit = "each"
)

type Provenance string

const (
	ProvenanceManual Provenance = "manual"
	ProvenanceUSDA   Provenance = "usda"
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
	ID                 string
	LocationID         string
	Name               string
	Category           string
	BaseUnit           Unit
	AlternateUnits     map[Unit]float64
	MacrosPerBaseUnit  MacroValues
	CurrentCostMinor   int64
	Currency           string
	OnHandBaseUnits    float64
	ParLevelBaseUnits  float64
	Provenance         Provenance
	VerificationStatus VerificationStatus
	YieldFactors       map[string]float64
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
