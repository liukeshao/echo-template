package weather

import "context"

type API interface {
	GetCurrent(ctx context.Context, city string) (*CurrentWeather, error)
}

type CurrentWeather struct {
	City        string  `json:"city"`
	Description string  `json:"description"`
	TempCelsius float64 `json:"temp_celsius"`
	Humidity    int     `json:"humidity"`
}
