package matching

const (
	PRICE_LEVEL_DEFAULT = -1.0
	PRICE_LEVEL_0       = 0.0
	PRICE_LEVEL_1       = 10.0
	PRICE_LEVEL_2       = 30.0
	PRICE_LEVEL_3       = 50.0
	PRICE_LEVEL_4       = 100.0
)

func checkPrice(priceLevel int) float64 {
	switch priceLevel {
	case 0:
		return PRICE_LEVEL_0
	case 1:
		return PRICE_LEVEL_1
	case 2:
		return PRICE_LEVEL_2
	case 3:
		return PRICE_LEVEL_3
	case 4:
		return PRICE_LEVEL_4
	default:
		return PRICE_LEVEL_DEFAULT
	}
}
