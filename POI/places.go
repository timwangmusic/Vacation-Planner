package POI

import (
	"log"
	"strings"
)

type Place struct {
	hours [7]string
	name string
	locationType string
	address address
	location [2]float64
	id string
}

type address struct{
	street string
	city string
	country string
	countryCode int
	zipCode string
}

func (v *Place) GetName() string{
	return v.name
}

func (v *Place) GetType() string{
	return v.locationType
}

func (v *Place) GetHour(day int) string{
	return v.hours[day]
}

func (v *Place) GetID() string {
	return v.id
}

func (v *Place) GetAddress() string{
	addr := v.address
	return strings.Join([]string{addr.street, addr.city, addr.country, addr.zipCode}, ", ")
}

func (v *Place) GetLocation() [2]float64{
	return v.location
}

// Set name if POI name changed
func (v *Place) SetName(name string){
	v.name = name
}
// Set type if POI type changed
func (v *Place) SetType(t string){
	v.locationType = t
}
// Set time if POI opening hour changed for some day in a week
func (v *Place) SetHour(day int, hour string){
	v.hours[day] = hour
}

func (v *Place) SetID(id string){
	v.id = id
}

func (v *Place) SetAddress(addr string){
	// expected addr format: "street, city, country, zipCode"
	fields := strings.Split(addr, ", ")
	if len(fields) != 4{
		log.Fatalf("Wrong address format, expected 4 fields, got %d fields", len(fields))
	}
	v.address.street = fields[0]
	v.address.city = fields[1]
	v.address.country = fields[2]
	v.address.zipCode = fields[3]
}

func (v *Place) SetLocation(location [2]float64){
	v.location = location
}
