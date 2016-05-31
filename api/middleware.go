package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type MiddlewareFunc func(next http.Handler) http.Handler

func (p *PostAPI) writeBasicHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		if p.Options.ResponseHeader != nil {
			for key, values := range p.Options.ResponseHeader {
				value := strings.Join(values, ";")
				c.Response().Header().Set(key, value)
			}
		}
		return next(c)
	}
}
