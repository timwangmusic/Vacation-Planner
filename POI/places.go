package POI

import (
	"github.com/modern-go/reflect2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"googlemaps.github.io/maps"
)

type Weekday uint8

const (
	DateMonday Weekday = iota
	DateTuesday
	DateWednesday
	DateThursday
	DateFriday
	DateSaturday
	DateSunday
)

func (w Weekday) String() string {
	return strconv.Itoa(int(w))
}

func (w Weekday) Name() string {
	mapping := map[Weekday]string{
		DateMonday:    "Monday",
		DateTuesday:   "Tuesday",
		DateWednesday: "Wednesday",
		DateThursday:  "Thursday",
		DateFriday:    "Friday",
		DateSaturday:  "Saturday",
		DateSunday:    "Sunday",
	}
	return mapping[w]
}

type PlacePhoto struct {
	// reference from Google Images
	Reference string `bson:"reference"`
	// the maximum height of the image
	Height int `bson:"height"`
	// the maximum width of the image
	Width int `bson:"width"`
}

type BusinessStatus string

const (
	Operational        BusinessStatus = "OPERATIONAL"
	ClosedTemporarily  BusinessStatus = "CLOSED_TEMPORARILY"
	ClosedPermanently  BusinessStatus = "CLOSED_PERMANENTLY"
	StatusNotAvailable BusinessStatus = "STATUS_NOT_AVAILABLE"
)

type Place struct {
	ID               string         `bson:"_id"`
	Name             string         `bson:"name"`
	Status           BusinessStatus `bson:"status"`
	LocationType     LocationType   `bson:"location_type"`
	Address          Address        `bson:"address"`
	FormattedAddress string         `bson:"formatted_address"`
	Location         Location       `bson:"location"`
	PriceLevel       PriceLevel     `bson:"price_level"`
	Rating           float32        `bson:"rating"`
	Hours            [7]string      `bson:"hours"`
	URL              string         `bson:"url"`
	Photo            PlacePhoto     `bson:"photo"`
	UserRatingsTotal int            `bson:"user_ratings_total"`
	Summary          string         `bson:"summary"`
	LastUpdatedAt    string         `bson:"last_updated_at"`
}

type Location struct {
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	City              string  `json:"city"`              // name of the city where the location belongs to
	AdminAreaLevelOne string  `json:"adminAreaLevelOne"` // e.g. state names in the United States
	Country           string  `json:"country"`           // name of the country where the location belongs to
}

// String formalizes a location to a format with capitalized locality, followed by upper-cased admin area one and country names
func (l *Location) String() string {
	title := cases.Title(language.English)
	upper := cases.Upper(language.English)
	return strings.Join([]string{title.String(l.City), upper.String(l.AdminAreaLevelOne), upper.String(l.Country)}, ", ")
}

func (l *Location) Normalize() {
	title := cases.Title(language.English)
	upper := cases.Upper(language.English)
	l.City = title.String(l.City)
	l.AdminAreaLevelOne = upper.String(l.AdminAreaLevelOne)
	l.Country = upper.String(l.Country)
}

// Address in adr micro-format example:
// 665 3rd St.
// Suite 207
// San Francisco, CA 94107
// U.S.A.
type Address struct {
	POBox        string
	ExtendedAddr string
	StreetAddr   string
	Locality     string
	Region       string
	PostalCode   string
	Country      string
}

type PriceLevel int

type OpeningHours struct {
	Hours []string
}

const (
	PriceLevelZero    = 0
	PriceLevelOne     = 1
	PriceLevelTwo     = 2
	PriceLevelThree   = 3
	PriceLevelFour    = 4
	PriceLevelDefault = 2
)

func (place *Place) GetName() string {
	return place.Name
}

func (place *Place) GetType() LocationType {
	return place.LocationType
}

func (place *Place) GetStatus() BusinessStatus {
	return place.Status
}

func (place *Place) GetHour(day Weekday) string {
	return place.Hours[day]
}

func (place *Place) GetID() string {
	return place.ID
}

func (place *Place) GetAddress() Address {
	return place.Address
}

func (place *Place) GetFormattedAddress() string {
	return place.FormattedAddress
}

func (place *Place) GetLocation() Location {
	return place.Location
}

func (place *Place) GetPriceLevel() PriceLevel {
	return place.PriceLevel
}

func (place *Place) GetRating() float32 {
	return place.Rating
}

func (place *Place) GetURL() string {
	return place.URL
}

func (place *Place) GetUserRatingsTotal() int {
	return place.UserRatingsTotal
}

func (place *Place) GetSummary() string {
	return place.Summary
}

func (place *Place) GetPhoto() PlacePhoto {
	return place.Photo
}

func (place *Place) GetLastUpdatedAt() string {
	return place.LastUpdatedAt
}

func (place *Place) SetName(name string) {
	place.Name = name
}

func (place *Place) SetStatus(status string) {
	switch status {
	case "OPERATIONAL":
		place.Status = Operational
	case "CLOSED_TEMPORARILY":
		place.Status = ClosedTemporarily
	case "CLOSED_PERMANENTLY":
		place.Status = ClosedPermanently
	default:
		place.Status = StatusNotAvailable
	}
}

// SetFormattedAddress sets a human-readable Address
func (place *Place) SetFormattedAddress(formattedAddress string) {
	place.FormattedAddress = formattedAddress
}

func (place *Place) SetType(t LocationType) {
	place.LocationType = t
}

func (place *Place) SetHour(day Weekday, hour string) {
	switch day {
	case DateSunday:
		place.Hours[day] = hour
	case DateMonday:
		place.Hours[day] = hour
	case DateTuesday:
		place.Hours[day] = hour
	case DateWednesday:
		place.Hours[day] = hour
	case DateThursday:
		place.Hours[day] = hour
	case DateFriday:
		place.Hours[day] = hour
	case DateSaturday:
		place.Hours[day] = hour
	default:
		log.Fatalf("day specified (%d) is not in range of 0-6", day)
	}
}

func (place *Place) SetID(id string) {
	place.ID = id
}

func (place *Place) SetAddress(addr string) {
	if addr == "" {
		return
	}
	p := regexp.MustCompile(`<.*?>.*?<`)
	pVal := regexp.MustCompile(`>.*<`)
	pFieldName := regexp.MustCompile(`".*"`)
	fields := p.FindAllString(addr, -1)
	for _, field := range fields {
		fieldName := pFieldName.FindString(field)
		value := pVal.FindString(field)
		val := value[1 : len(value)-1]
		switch fieldName {
		case `"post-office-box"`:
			place.Address.POBox = val
		case `"extended-address"`:
			place.Address.ExtendedAddr = val
		case `"street-address"`:
			place.Address.StreetAddr = val
		case `"locality"`:
			place.Address.Locality = val
		case `"region"`:
			place.Address.Region = val
		case `"postal-code"`:
			place.Address.PostalCode = val
		case `"country-name"`:
			place.Address.Country = val
		}
	}
}

func (place *Place) SetLocationCoordinates(coordinates [2]float64) {
	place.Location.Latitude = coordinates[0]
	place.Location.Longitude = coordinates[1]
}

func (place *Place) SetPriceLevel(priceRange int) {
	switch priceRange {
	case 0:
		place.PriceLevel = PriceLevelZero
	case 1:
		place.PriceLevel = PriceLevelOne
	case 2:
		place.PriceLevel = PriceLevelTwo
	case 3:
		place.PriceLevel = PriceLevelThree
	case 4:
		place.PriceLevel = PriceLevelFour
	default:
		place.PriceLevel = PriceLevelDefault
	}
}

func (place *Place) SetRating(rating float32) {
	place.Rating = rating
}

func (place *Place) SetURL(url string) {
	place.URL = url
}

func (place *Place) SetPhoto(photo *maps.Photo) {
	if !reflect2.IsNil(photo) {
		place.Photo.Reference = photo.PhotoReference
		place.Photo.Height = photo.Height
		place.Photo.Width = photo.Width
	}
}

func (place *Place) SetUserRatingsTotal(userRatingsTotal int) {
	place.UserRatingsTotal = userRatingsTotal
}

func (place *Place) SetSummary(summary string) {
	place.Summary = summary
}

func (place *Place) SetLastUpdatedAt(lastUpdateTimeStamp time.Time) {
	place.LastUpdatedAt = lastUpdateTimeStamp.Format(time.RFC3339)
}

func CreatePlace(name, addr, formattedAddr, businessStatus string, locationType LocationType, openingHours *OpeningHours, placeID string, priceLevel int, rating float32, url string, photo *maps.Photo, userRatingsTotal int, latitude, longitude float64, summary *string) (place Place) {
	place.SetType(locationType)
	place.SetName(name)
	place.SetID(placeID)
	var weekday Weekday
	if openingHours != nil && openingHours.Hours != nil {
		for weekday = DateMonday; weekday <= DateSunday; weekday++ {
			place.SetHour(weekday, openingHours.Hours[weekday])
		}
	}
	// set default
	for weekday = DateMonday; weekday <= DateSunday; weekday++ {
		if place.GetHour(weekday) == "" {
			place.SetHour(weekday, "8:30 am – 9:30 pm")
		}
	}

	place.SetStatus(businessStatus)
	place.SetLocationCoordinates([2]float64{latitude, longitude})
	place.SetAddress(addr)
	place.SetFormattedAddress(formattedAddr)
	place.SetPriceLevel(priceLevel)
	place.SetRating(rating)
	place.SetURL(url)
	place.SetPhoto(photo)
	place.SetUserRatingsTotal(userRatingsTotal)
	if summary != nil {
		place.SetSummary(*summary)
	}
	place.SetLastUpdatedAt(time.Now())
	return
}
