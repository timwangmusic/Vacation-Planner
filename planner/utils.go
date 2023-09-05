package planner

import (
	"errors"
	"regexp"
)

// validate date is in the format of yyyy-mm-dd
func validateDate(date string) error {
	if len(date) == 0 {
		return errors.New("date cannot be empty")
	}

	datePattern := `(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`
	if matched, _ := regexp.Match(datePattern, []byte(date)); !matched {
		return errors.New("date format must be yyyy-mm-dd")
	}
	return nil
}

// validate location is in the format of city,country
func validateLocation(location string, precise bool) error {
	if precise {
		return nil
	}
	if len(location) == 0 {
		return errors.New("location cannot be empty")
	}

	locationPattern := `[a-zA-Z\s]+,\s[a-zA-Z\s]+,\s[a-zA-Z\s]+|[a-zA-Z\s]+,\s[a-zA-Z\s]+`
	if matched, _ := regexp.Match(locationPattern, []byte(location)); !matched {
		return errors.New("location format must be city, region, country or city, country")
	}
	return nil
}

// MapSlice is a generic function for mapping a slice to another
func MapSlice[T, V any](ts []T, fn func(t T) V) []V {
	result := make([]V, len(ts))
	for idx, t := range ts {
		result[idx] = fn(t)
	}
	return result
}
