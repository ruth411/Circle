package biztime

import (
	"fmt"
	"time"
)

const layout = "2006-01-02"

type BusinessDate string

func Parse(raw string) (BusinessDate, error) {
	if _, err := time.Parse(layout, raw); err != nil {
		return "", fmt.Errorf("invalid business date %q: %w", raw, err)
	}

	return BusinessDate(raw), nil
}

func FromTime(t time.Time) BusinessDate {
	year, month, day := t.Date()
	return BusinessDate(fmt.Sprintf("%04d-%02d-%02d", year, month, day))
}

func (d BusinessDate) String() string {
	return string(d)
}
