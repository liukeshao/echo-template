package api

import "github.com/liukeshao/go-rest-template/pkg/validator"

func (r *ChangePasswordRequest) validateExtra(v *validator.FieldViolations) {
	v.MustDiffer("new_password", r.NewPassword, "old_password", r.OldPassword)
}
