package helper

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"github.com/micro/go-micro/server"
)

const (
	APIMetadataKey    = "post_api"
	APIVerMetadataKey = "post_api_ver"
)

const (
	matchFuncNameExpr   = `\(\*{0,1}[a-zA-Z0-9_]+\)\.\w+`
	replaceFuncNameExpr = `[\(\)\*]`
)

var (
	nilHandlerOption = func(o *server.HandlerOptions) {}
)

func ToHandlerOption(fn interface{}, ver, api string, alias ...string) server.HandlerOption {
	if fn == nil {
		return nilHandlerOption
	}

	api = strings.TrimSpace(api)

	if api == "" {
		return nilHandlerOption
	}

	apis := []string{api}

	if len(alias) > 0 {
		apis = append(apis, alias...)
	}

	strAPIs := strings.Join(apis, ",")

	if name, err := FuncName(fn); err != nil {
		return nilHandlerOption
	} else {
		return func(o *server.HandlerOptions) {
			o.Metadata[name] = map[string]string{APIMetadataKey: strAPIs, APIVerMetadataKey: ver}
			fmt.Println(o.Metadata)
		}
	}

}

func FuncName(v interface{}) (name string, err error) {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Func {
		err = errors.New("value is not a func")
		return
	}

	fullName := runtime.FuncForPC(val.Pointer()).Name()

	var r *regexp.Regexp
	if r, err = regexp.Compile(matchFuncNameExpr); err != nil {
		return
	}

	tmpName := r.FindString(fullName)

	if r, err = regexp.Compile(replaceFuncNameExpr); err != nil {
		return
	}

	name = r.ReplaceAllString(tmpName, "")

	return
}
