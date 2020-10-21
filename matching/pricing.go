package matching

const (
	PriceLevelDefault = -1.0
	PriceLevel0       = 0.0
	PriceLevel1       = 10.0
	PriceLevel2       = 30.0
	PriceLevel3       = 50.0
	PriceLevel4       = 100.0
)

func Pricing(priceLevel int) float64 {
	switch priceLevel {
	case 0:
		return PriceLevel0
	case 1:
		return PriceLevel1
	case 2:
		return PriceLevel2
	case 3:
		return PriceLevel3
	case 4:
		return PriceLevel4
	default:
		return PriceLevelDefault
	}
}
