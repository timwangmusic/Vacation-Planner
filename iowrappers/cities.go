package iowrappers

type City struct {
	ID         string  `json:"id"`
	GeonameID  int64   `json:"geonameId"`
	Name       string  `json:"name"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lng"`
	Population int64   `json:"population"`
	AdminArea1 string  `json:"adminArea1"`
	Country    string  `json:"country"`
}
