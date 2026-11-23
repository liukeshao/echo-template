package codes

import "net/http"

// Weather 领域错误码（第三方天气 API 客户端）。
//
// 命名规范：<领域>_<错误语义>，使用 UPPER_SNAKE_CASE。
// 新增外部 API 时，在此文件旁创建 pkg/codes/<api>.go 并遵循同样模式。
const (
	// WeatherUnavailable 表示外部天气 API 暂时不可达（网络故障、超时等）。
	WeatherUnavailable Code = "WEATHER_UNAVAILABLE"

	// WeatherRateLimited 表示已触发天气 API 的速率限制（HTTP 429）。
	WeatherRateLimited Code = "WEATHER_RATE_LIMITED"

	// WeatherInternal 表示天气客户端遇到未预期的内部错误。
	WeatherInternal Code = "WEATHER_INTERNAL_ERROR"
)

func init() {
	Register(WeatherUnavailable, http.StatusServiceUnavailable)
	Register(WeatherRateLimited, http.StatusTooManyRequests)
	Register(WeatherInternal, http.StatusInternalServerError)
}
