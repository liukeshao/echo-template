package validator

// Validator 是可自我校验的请求类型接口。
// 各 Request/Input struct 实现此接口后，handler 层可统一调用 Validate()，
// 错误由 error_handler 识别 *FieldViolations 并转为 AIP-193 BadRequest detail。
type Validator interface {
	Validate() error
}
