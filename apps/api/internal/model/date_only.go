package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const dateOnlyLayout = "2006-01-02"

// DateOnly is a calendar date without a time-of-day or timezone. It keeps the
// profile API and PostgreSQL DATE column on the same YYYY-MM-DD contract.
type DateOnly time.Time

func ParseDateOnly(value string) (DateOnly, error) {
	parsed, err := time.Parse(dateOnlyLayout, value)
	if err != nil {
		return DateOnly{}, fmt.Errorf("date must use YYYY-MM-DD: %w", err)
	}
	return DateOnly(parsed), nil
}

func (d DateOnly) Time() time.Time {
	return time.Time(d)
}

func (d DateOnly) String() string {
	return d.Time().Format(dateOnlyLayout)
}

func (d DateOnly) AgeAt(now time.Time) int {
	birthDate := d.Time()
	now = now.UTC()
	age := now.Year() - birthDate.Year()
	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}
	return age
}

func (d DateOnly) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *DateOnly) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("birth_date must be a YYYY-MM-DD string: %w", err)
	}
	parsed, err := ParseDateOnly(value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d *DateOnly) Scan(value any) error {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case time.Time:
		*d = DateOnly(time.Date(typed.Year(), typed.Month(), typed.Day(), 0, 0, 0, 0, time.UTC))
		return nil
	case string:
		parsed, err := ParseDateOnly(typed[:min(len(typed), len(dateOnlyLayout))])
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case []byte:
		return d.Scan(string(typed))
	default:
		return fmt.Errorf("cannot scan %T into DateOnly", value)
	}
}

func (d DateOnly) Value() (driver.Value, error) {
	if d.Time().IsZero() {
		return nil, nil
	}
	return d.String(), nil
}
