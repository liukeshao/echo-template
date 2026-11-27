package api

import "github.com/liukeshao/echo-template/pkg/validator"

func (r *ChangePasswordRequest) validateExtra(v *validator.FieldViolations) {
	v.MustDiffer("new_password", r.NewPassword, "old_password", r.OldPassword)
}
