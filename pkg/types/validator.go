package types

import "github.com/liukeshao/echo-template/pkg/errors"

type Validator interface {
	Validate() []*errors.ErrorDetail
}
