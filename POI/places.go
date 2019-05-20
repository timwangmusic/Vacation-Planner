package POI

import (
	"log"
	"regexp"
)

type Weekday uint8

const(
	DATE_MONDAY Weekday= iota
	DATE_TUESDAY
	DATE_WEDNESDAY
	DATE_THURSAY
	DATE_FRIDAY
	DATE_SATURDAY
	DATE_SUNDAY
)

type Place struct {
	ID               string    `bson:"placeId"`
	Name             string    `bson:"name"`
	LocationType     string    `bson:"location_type"`
	Address          Address   `bson:"address"`
	FormattedAddress string    `bson:"formatted_address"`
	Location         Location  `bson:"location"`
	PriceLevel       int       `bson:"price_level"`
	Rating           float32   `bson:"rating"`
	Hours            [7]string `bson:"hours"`
}

type Location struct {
	Type string	`json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

type Address struct{
	PObox			string
	ExtendedAddr	string
	StreetAddr      string
	Locality        string
	Region 			string
	PostalCode      string
	Country			string
}

func (v *Place) GetName() string{
	return v.Name
}

func (v *Place) GetType() string{
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

func (v *Place) GetFormattedAddress() string{
	return v.FormattedAddress
}

func (v *Place) GetLocation() [2]float64{
	return v.Location.Coordinates
}

func (v *Place) GetPriceLevel() int{
	return v.PriceLevel
}

func (v *Place) GetRating() float32{
	return v.Rating
}

// Set name if POI name changed
func (v *Place) SetName(name string){
	v.Name = name
}

// Set human-readable Address of this place
func (v *Place) SetFormattedAddress(formattedAddress string){
	v.FormattedAddress = formattedAddress
}

// Set type if POI type changed
func (v *Place) SetType(t string){
	v.LocationType = t
}
// Set time if POI opening hour changed for some day in a week
func (v *Place) SetHour(day Weekday, hour string){
	switch day {
	case DATE_SUNDAY:
		v.Hours[day] = hour
	case DATE_MONDAY:
		v.Hours[day] = hour
	case DATE_TUESDAY:
		v.Hours[day] = hour
	case DATE_WEDNESDAY:
		v.Hours[day] = hour
	case DATE_THURSAY:
		v.Hours[day] = hour
	case DATE_FRIDAY:
		v.Hours[day] = hour
	case DATE_SATURDAY:
		v.Hours[day] = hour
	default:
		log.Fatalf("day specified (%d) is not in range of 0-6", day)
	}
}

func (v *Place) SetID(id string){
	v.ID = id
}

func (v *Place) SetAddress(addr string){
	if addr == ""{
		return
	}
	p := regexp.MustCompile(`<.*?>.*?<`)
	pVal := regexp.MustCompile(`>.*<`)
	pFieldName := regexp.MustCompile(`".*"`)
	fields := p.FindAllString(addr, -1)
	for _, field := range fields{
		fieldName := pFieldName.FindString(field)
		value := pVal.FindString(field)
		val := value[1:len(value)-1]
		switch fieldName{
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

func (v *Place) SetLocation(location [2]float64){
	v.Location.Coordinates = location
}

func (v *Place) SetPriceLevel(priceRange int){
	v.PriceLevel = priceRange
}

func (v *Place) SetRating(rating float32){
	v.Rating = rating
}