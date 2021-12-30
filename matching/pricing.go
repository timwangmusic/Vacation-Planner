package matching

import "github.com/weihesdlegend/Vacation-planner/POI"

const (
	PriceZeroMean    = 0.0
	PriceOneMean     = 10.0
	PriceTwoMean     = 30.0
	PriceThreeMean   = 50.0
	PriceFourMean    = 100.0
	PriceDefaultMean = PriceTwoMean
)

// AveragePricing returns expected price of the price level
func AveragePricing(priceLevel POI.PriceLevel) float64 {
	switch priceLevel {
	case POI.PriceLevelZero:
		return PriceZeroMean
	case POI.PriceLevelOne:
		return PriceOneMean
	case POI.PriceLevelTwo:
		return PriceTwoMean
	case POI.PriceLevelThree:
		return PriceThreeMean
	case POI.PriceLevelFour:
		return PriceFourMean
	default:
		return PriceDefaultMean
	}
}
