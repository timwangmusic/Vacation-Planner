package solution

// PlaceView defines view of Place that front-end uses
type PlaceView struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	Rating       float32   `json:"rating"`
	RatingsCount int       `json:"ratings_count"`
	PriceLevel   int       `json:"price_level"`
	Hours        [7]string `json:"hours"`
}
