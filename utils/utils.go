package utils

import "math"

func RoundTo(n float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(n*pow) / pow
}
