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
	hours            [7]string
	name             string
	locationType     string
	address          Address
	formattedAddress string
	location         [2]float64	// geolocation coordinates
	id               string
	priceLevel       int
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

//Sample Address in adr micro-format
//665 3rd St.
//Suite 207
//San Francisco, CA 94107
//U.S.A.
func (v *Place) GetAddress() Address {
	return v.address
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

// Set human-readable Address of this place
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
			v.address.PObox = val
		case `"extended-address"`:
			v.address.ExtendedAddr = val
		case `"street-address"`:
			v.address.StreetAddr = val
		case `"locality"`:
			v.address.Locality = val
		case `"region"`:
			v.address.Region = val
		case `"postal-code"`:
			v.address.PostalCode = val
		case `"country-name"`:
			v.address.Country = val
		}
	}
}

func (v *Place) SetLocation(location [2]float64){
	v.location = location
}

func (v *Place) SetPriceLevel(priceRange int){
	v.priceLevel = priceRange
}
