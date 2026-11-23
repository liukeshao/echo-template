package api

import "github.com/liukeshao/echo-template/pkg/validator"

// validateExtra 追加跨字段校验规则（新密码不能与旧密码相同），
// 由生成的 ChangePasswordRequest.Validate() 自动调用。
func (r *ChangePasswordRequest) validateExtra(v *validator.FieldViolations) {
	if r.OldPassword != "" && r.OldPassword == r.NewPassword {
		v.Add("new_password", validator.KeyMustDiffer, map[string]string{
			"field": "new_password", "other": "old_password",
		})
	}
}
