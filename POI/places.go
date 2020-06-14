package POI

import (
	"googlemaps.github.io/maps"
	"log"
	"reflect"
	"regexp"
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

type PlacePhoto struct {
	// reference from Google Images
	Reference string `bson:"reference"`
	// the maximum height of the image
	Height int `bson:"height"`
	// the maximum width of the image
	Width int `bson:"width"`
}

type Place struct {
	ID               string       `bson:"_id"`
	Name             string       `bson:"name"`
	LocationType     LocationType `bson:"location_type"`
	Address          Address      `bson:"address"`
	FormattedAddress string       `bson:"formatted_address"`
	Location         Location     `bson:"location"`
	PriceLevel       int          `bson:"price_level"`
	Rating           float32      `bson:"rating"`
	Hours            [7]string    `bson:"hours"`
	URL              string       `bson:"url"`
	Photo            PlacePhoto   `bson:"photo"`
}

type Location struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

type Address struct {
	PObox        string
	ExtendedAddr string
	StreetAddr   string
	Locality     string
	Region       string
	PostalCode   string
	Country      string
}

func (v *Place) GetName() string {
	return v.Name
}

func (v *Place) GetType() LocationType {
	return v.LocationType
}

func (v *Place) GetHour(day Weekday) string {
	return v.Hours[day]
}

func (v *Place) GetID() string {
	return v.ID
}

//Sample Address in adr micro-format
//665 3rd St.
//Suite 207
//San Francisco, CA 94107
//U.S.A.
func (v *Place) GetAddress() Address {
	return v.Address
}

func (v *Place) GetFormattedAddress() string {
	return v.FormattedAddress
}

func (v *Place) GetLocation() [2]float64 {
	return v.Location.Coordinates
}

func (v *Place) GetPriceLevel() int {
	return v.PriceLevel
}

func (v *Place) GetRating() float32 {
	return v.Rating
}

// Set name if POI name changed
func (v *Place) SetName(name string) {
	v.Name = name
}

// Set human-readable Address of this place
func (v *Place) SetFormattedAddress(formattedAddress string) {
	v.FormattedAddress = formattedAddress
}

// Set type if POI type changed
func (v *Place) SetType(t LocationType) {
	v.LocationType = t
}

// Set time if POI opening hour changed for some day in a week
func (v *Place) SetHour(day Weekday, hour string) {
	switch day {
	case DateSunday:
		v.Hours[day] = hour
	case DateMonday:
		v.Hours[day] = hour
	case DateTuesday:
		v.Hours[day] = hour
	case DateWednesday:
		v.Hours[day] = hour
	case DateThursday:
		v.Hours[day] = hour
	case DateFriday:
		v.Hours[day] = hour
	case DateSaturday:
		v.Hours[day] = hour
	default:
		log.Fatalf("day specified (%d) is not in range of 0-6", day)
	}
}

func (v *Place) SetID(id string) {
	v.ID = id
}

func (v *Place) SetAddress(addr string) {
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
			v.Address.PObox = val
		case `"extended-address"`:
			v.Address.ExtendedAddr = val
		case `"street-address"`:
			v.Address.StreetAddr = val
		case `"locality"`:
			v.Address.Locality = val
		case `"region"`:
			v.Address.Region = val
		case `"postal-code"`:
			v.Address.PostalCode = val
		case `"country-name"`:
			v.Address.Country = val
		}
	}
}

func (v *Place) SetLocation(location [2]float64) {
	v.Location.Coordinates = location
	v.Location.Type = "Point"
}

func (v *Place) SetPriceLevel(priceRange int) {
	v.PriceLevel = priceRange
}

func (v *Place) SetRating(rating float32) {
	v.Rating = rating
}

func (v *Place) SetURL(url string) {
	v.URL = url
}

func (v *Place) SetPhoto(photo *maps.Photo) {
	if val := reflect.ValueOf(photo); !val.IsNil() {
		v.Photo.Reference = photo.PhotoReference
		v.Photo.Height = photo.Height
		v.Photo.Width = photo.Width
	}
}
