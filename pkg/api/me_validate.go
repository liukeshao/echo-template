package api

import "github.com/liukeshao/echo-template/pkg/validator"

// validateExtra 追加跨字段校验规则（新密码不能与旧密码相同），
// 由生成的 ChangePasswordRequest.Validate() 自动调用。
func (r *ChangePasswordRequest) validateExtra(v *validator.FieldViolations) {
	v.MustDiffer("new_password", r.NewPassword, "old_password", r.OldPassword)
}
