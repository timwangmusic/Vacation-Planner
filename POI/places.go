package POI

import (
	"log"
	"strings"
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
	hours        [7]string
	name         string
	locationType string
	address      address
	formattedAddress string
	location     [2]float64	// geolocation coordinates
	id           string
	priceLevel   int
}

type address struct{
	street1     string
	street2     string
	city        string
	country     string
	countryCode int
	zipCode     string
}

func (v *Place) GetName() string{
	return v.name
}

func (v *Place) GetType() string{
	return v.locationType
}

func (v *Place) GetHour(day Weekday) string {
	return v.hours[day]
}

func (v *Place) GetID() string {
	return v.id
}

func (v *Place) GetAddress() string{
	addr := v.address
	return strings.Join([]string{addr.street1, addr.city, addr.country, addr.zipCode}, ", ")
}

func (v *Place) GetFormattedAddress() string{
	return v.formattedAddress
}

func (v *Place) GetLocation() [2]float64{
	return v.location
}

func (v *Place) GetPriceLevel() int{
	return v.priceLevel
}

// Set name if POI name changed
func (v *Place) SetName(name string){
	v.name = name
}

// Set human-readable address of this place
func (v *Place) SetFormattedAddress(formattedAddress string){
	v.formattedAddress = formattedAddress
}

// Set type if POI type changed
func (v *Place) SetType(t string){
	v.locationType = t
}
// Set time if POI opening hour changed for some day in a week
func (v *Place) SetHour(day Weekday, hour string){
	switch day {
	case DATE_SUNDAY:
		v.hours[day] = hour
	case DATE_MONDAY:
		v.hours[day] = hour
	case DATE_TUESDAY:
		v.hours[day] = hour
	case DATE_WEDNESDAY:
		v.hours[day] = hour
	case DATE_THURSAY:
		v.hours[day] = hour
	case DATE_FRIDAY:
		v.hours[day] = hour
	case DATE_SATURDAY:
		v.hours[day] = hour
	default:
		log.Fatalf("day specified (%d) is not in range of 0-6", day)
	}
}

func (v *Place) SetID(id string){
	v.id = id
}

func (v *Place) SetAddress(addr string){
	// expected addr format: "street1, city, country, zipCode"
	fields := strings.Split(addr, ", ")
	if len(fields) != 4{
		v.address.street1 = addr
		return
		//log.Fatalf("Wrong address format, expected 4 fields, got %d fields", len(fields))
	}
	v.address.street1 = fields[0]
	v.address.city = fields[1]
	v.address.country = fields[2]
	v.address.zipCode = fields[3]
}

func (v *Place) SetLocation(location [2]float64){
	v.location = location
}

func (v *Place) SetPriceLevel(priceRange int){
	v.priceLevel = priceRange
}
