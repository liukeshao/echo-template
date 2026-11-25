package codes

import (
	"net/http"

	"github.com/liukeshao/echo-template/pkg/rest"
)

// 通用 HTTP 传输层与协议错误码，与 pkg/rest 保持单一事实来源（SSOT），
// 统一注册后由 internal/i18n 的 [errors] 段提供国际化翻译与启动覆盖率检查。
const (
	BadRequest            Code = rest.CodeBadRequest
	NotFound              Code = rest.CodeNotFound
	MethodNotAllowed      Code = rest.CodeMethodNotAllowed
	RequestEntityTooLarge Code = rest.CodeRequestEntityTooLarge
	GatewayTimeout        Code = rest.CodeGatewayTimeout
	ClientClosedRequest   Code = rest.CodeClientClosedRequest
	InternalError         Code = rest.CodeInternalError
)

func init() {
	Register(BadRequest, http.StatusBadRequest)
	Register(NotFound, http.StatusNotFound)
	Register(MethodNotAllowed, http.StatusMethodNotAllowed)
	Register(RequestEntityTooLarge, http.StatusRequestEntityTooLarge)
	Register(GatewayTimeout, http.StatusGatewayTimeout)
	Register(ClientClosedRequest, rest.StatusClientClosedRequest)
	Register(InternalError, http.StatusInternalServerError)
}
