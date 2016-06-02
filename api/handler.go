package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogap/errors"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"golang.org/x/net/context"

	"github.com/micro/go-micro/metadata"
)

const (
	APIHeader            = "X-Api"
	MultiCallHeader      = "X-Api-Multi-Call"
	APICallTimeoutHeader = "X-Api-Call-Timeout"
)

type PostAPIResponse struct {
	api               string
	version           string
	isSpecificVersion bool
	Code              uint64      `json:"code"`
	Message           string      `json:"message,omitempty"`
	ErrID             string      `json:"err_id,omitempty"`
	ErrNamespace      string      `json:"err_namespace,omitempty"`
	Result            interface{} `json:"result"`
}

type PostAPIRequest struct {
	API               string
	Version           string
	IsSpecificVersion bool
	Content           map[string]interface{}
}

func (p *PostAPI) faviconICONHandle(c echo.Context) (err error) {
	return c.String(http.StatusNotFound, "")
}

func (p *PostAPI) pingHandle(c echo.Context) (err error) {
	return c.String(http.StatusOK, "pong")
}

func (p *PostAPI) getRequestTimeout(r engine.Request) time.Duration {
	strTimeout := r.Header().Get(APICallTimeoutHeader)
	if strTimeout == "" {
		return time.Second * 30
	}

	if i, e := strconv.Atoi(strTimeout); e == nil {
		return time.Duration(i) * time.Millisecond
	}

	return time.Second * 30
}

func (p *PostAPI) rpcHandle(c echo.Context) (err error) {

	badRequest := func(description string) {
		err = ErrBadRequest.New().Append(description)
		return
	}

	apiRequests := APIRequestsFromContext(c)

	if apiRequests == nil || apiRequests.Requests == nil {
		err = ErrBadRequest.New().Append("empty request")
		return
	}

	// response content type
	// w.Header().Set("Content-Type", "application/json")

	ct := c.Request().Header().Get("Content-Type")

	// Strip charset from Content-Type (like `application/json; charset=UTF-8`)
	if idx := strings.IndexRune(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}

	// create context
	ctx := requestToContext(c.Request(),
		p.Options.MicroHeaders, map[string]string{
			"Content-Type": ct,
			"Client-IP":    c.Request().RemoteAddress(),
			"Cookies":      jsonCookies(c.Request().Cookies()),
			"User-Agent":   c.Request().UserAgent(),
			"Request-Id":   c.Request().Header().Get("X-Request-Id"),
		})

	for _, req := range apiRequests.Requests {
		if _, exist := p.getService(req.API, req.Version); !exist {
			badRequest(fmt.Sprintf("api not exist, %s:%v", req.API, req.Version))
			return
		}
	}

	responsesChan := make(chan PostAPIResponse, len(apiRequests.Requests))

	for _, request := range apiRequests.Requests {
		go func(ctx context.Context, req PostAPIRequest, responsesChan chan PostAPIResponse) {
			var resp PostAPIResponse
			if srv, exist := p.getService(req.API, req.Version); !exist {
				err := ErrBadRequest.New().Append(fmt.Sprintf("api not exist, %s:%v", req.API, req.Version))

				resp = PostAPIResponse{
					ErrID:        err.Id(),
					ErrNamespace: err.Namespace(),
					Message:      err.Error(),
					Code:         err.Code(),
				}

			} else {
				resp = p.callMicroService(ctx, srv.Service, srv.Method, req.Content)
			}

			resp.api = req.API
			resp.version = req.Version
			resp.isSpecificVersion = req.IsSpecificVersion

			select {
			case responsesChan <- resp:
				{
				}
			default:
			}

		}(ctx, request, responsesChan)
	}

	apiResponses := map[string]PostAPIResponse{}

	callTimeout := p.getRequestTimeout(c.Request())

	isTimeout := false

responseFor:
	for {
		select {
		case resp := <-responsesChan:
			{
				api := resp.api
				if resp.isSpecificVersion {
					api += ":" + resp.version
				}
				apiResponses[api] = resp
			}
		case <-time.After(callTimeout):
			{
				isTimeout = true
				break responseFor
			}
		default:
			if len(apiResponses) == len(apiRequests.Requests) {
				break responseFor
			}
		}
	}

	for _, apiReq := range apiRequests.Requests {

		api := apiReq.API
		if apiReq.IsSpecificVersion {
			api += ":" + apiReq.Version
		}

		if _, exist := apiResponses[api]; !exist {
			var e errors.ErrCode

			if isTimeout {
				e = ErrRequestTimeout.New()
			} else {
				e = ErrInternalServerError.New().Append("response did not received")
			}

			apiResponses[api] = PostAPIResponse{
				api:          apiReq.API,
				version:      apiReq.Version,
				Code:         e.Code(),
				Message:      e.Error(),
				ErrID:        e.Id(),
				ErrNamespace: e.Namespace(),
				Result:       nil,
			}
		}
	}

	c.Set(apiRequestsKey, apiResponses)

	var finallyResp PostAPIResponse

	if apiRequests.IsMultiCall {
		finallyResp.Code = 0
		finallyResp.Message = ""
		finallyResp.Result = apiResponses
	} else {
		finallyResp = apiResponses[apiRequests.Requests[0].API]
	}

	c.JSON(http.StatusOK, finallyResp)

	return
}

func (p *PostAPI) errorHandle(err error, c echo.Context) {

	if c.Request().Method() == "POST" {
		var errCode errors.ErrCode

		if ec, ok := err.(errors.ErrCode); ok {
			errCode = ec
		} else {
			errCode = ErrInternalServerError.New().
				Append(err).
				WithContext("URI", c.Request().URI()).
				WithContext("Method", c.Request().Method())
		}

		resp := PostAPIResponse{
			Code:         errCode.Code(),
			Message:      errCode.Error(),
			ErrID:        errCode.Id(),
			ErrNamespace: errCode.Namespace(),
		}

		c.JSON(http.StatusOK, resp)

		if !c.Response().Committed() {
			c.JSON(http.StatusOK, resp)
		}
	}
}

func (p *PostAPI) callMicroService(ctx context.Context, service, method string, request map[string]interface{}) (response PostAPIResponse) {

	var resp map[string]interface{}
	req := p.Options.Client.NewJsonRequest(service, method, request)
	if err := p.Options.Client.Call(ctx, req, &resp); err != nil {
		if errCode, ok := err.(errors.ErrCode); !ok {
			err = ErrInternalServerError.New().Append(err)
		} else {
			response.Code = errCode.Code()
			response.ErrID = errCode.Id()
			response.ErrNamespace = errCode.Namespace()
			response.Message = errCode.Error()
		}

		return
	}

	response.Result = resp

	return
}

func requestToContext(r engine.Request, headerKeys []string, basicHeaders map[string]string) context.Context {
	ctx := context.Background()
	md := make(metadata.Metadata)

	for i := 0; i < len(headerKeys); i++ {
		v := r.Header().Get(headerKeys[i])
		if v != "" {
			md[headerKeys[i]] = r.Header().Get(headerKeys[i])
		}
	}

	if basicHeaders != nil {
		for k, v := range basicHeaders {
			if k != "" && v != "" {
				md[k] = v
			}
		}
	}

	return metadata.NewContext(ctx, md)
}

func jsonCookies(cookies []engine.Cookie) string {
	b, _ := json.Marshal(cookies)
	return string(b)
}
