// Package weather provides a client for the external Weather API.
//
// Service-layer code must depend only on the [API] interface defined here,
// never on [Client] directly. This allows easy mocking in unit tests:
//
//	type mockWeather struct{}
//	func (m *mockWeather) GetCurrent(ctx context.Context, city string) (*CurrentWeather, error) { … }
//
// Template note: when adding a new third-party API, copy this package,
// rename it, and replace the method signatures with those of your target API.
package weather

import "context"

// API is the interface that service-layer code depends on.
// Keep it as narrow as possible — only the methods actually used.
type API interface {
	// GetCurrent returns the current weather conditions for city.
	GetCurrent(ctx context.Context, city string) (*CurrentWeather, error)
}

// CurrentWeather holds the response data returned by the external API.
// Extend with additional fields as the real API requires.
type CurrentWeather struct {
	City        string  `json:"city"`
	Description string  `json:"description"`
	TempCelsius float64 `json:"temp_celsius"`
	Humidity    int     `json:"humidity"`
}
