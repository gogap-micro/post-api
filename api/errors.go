package api

import (
	"github.com/gogap/errors"
)

const (
	ErrNamespace = "POST-API"
)

var (
	ErrBadRequest          = errors.TN(ErrNamespace, 400, "")
	ErrInternalServerError = errors.TN(ErrNamespace, 500, "")
	ErrRequestTimeout      = errors.TN(ErrNamespace, 408, "request timeout")
)
