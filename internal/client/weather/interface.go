// Package weather 提供外部天气 API 客户端示例。
//
// 领域服务仅依赖此处定义的 API 接口，不得直接依赖 *Client，便于在单元测试中替换 mock。
package weather

import "context"

// API 是领域服务依赖的外部天气接口（保持最小接口原则）。
type API interface {
	// GetCurrent 查询指定城市的实时天气。
	GetCurrent(ctx context.Context, city string) (*CurrentWeather, error)
}

// CurrentWeather 表示外部天气 API 返回的领域数据结构。
type CurrentWeather struct {
	City        string  `json:"city"`
	Description string  `json:"description"`
	TempCelsius float64 `json:"temp_celsius"`
	Humidity    int     `json:"humidity"`
}
