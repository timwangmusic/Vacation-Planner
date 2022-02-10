package planner

import (
	"errors"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"regexp"
	"strconv"
	"time"
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

func toWeekday(date string) POI.Weekday {
	datePattern := regexp.MustCompile(`(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`)
	dateFields := datePattern.FindStringSubmatch(date)
	year, _ := strconv.Atoi(dateFields[1])
	month, _ := strconv.Atoi(dateFields[2])
	day, _ := strconv.Atoi(dateFields[3])
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return POI.Weekday(t.Weekday())
}

func toPriceLevel(priceLevel string) POI.PriceLevel {
	price, _ := strconv.Atoi(priceLevel)
	return POI.PriceLevel(price)
}

// validate location is in the format of city,country
func validateLocation(location string) error {
	if len(location) == 0 {
		return errors.New("location cannot be empty")
	}

	locationPattern := `[a-zA-Z\s]+,\s[a-zA-Z\s]+,\s[a-zA-Z\s]+|[a-zA-Z\s]+,\s[a-zA-Z\s]+`
	if matched, _ := regexp.Match(locationPattern, []byte(location)); !matched {
		return errors.New("location format must be city, region, country or city, country")
	}
	return nil
}
