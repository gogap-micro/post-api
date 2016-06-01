package api

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogap/errors"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"

	"github.com/micro/go-micro/metadata"
)

const (
	APIHeader            = "X-Api"
	MultiCallHeader      = "X-Api-Multi-Call"
	APICallTimeoutHeader = "X-Api-Call-Timeout"
)

type postAPIResponse struct {
	api               string
	version           string
	isSpecificVersion bool
	Code              uint64      `json:"code"`
	Message           string      `json:"message,omitempty"`
	ErrID             string      `json:"err_id,omitempty"`
	ErrNamespace      string      `json:"err_namespace,omitempty"`
	Result            interface{} `json:"result"`
}

type postAPIRequest struct {
	API               string
	Version           string
	IsSpecificVersion bool
	Content           map[string]interface{}
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

func GetAPIRequests(c echo.Context) (apiRequests []postAPIRequest, isMultiCall bool, err error) {
	multiCall := false
	mcVal := strings.ToLower(c.Request().Header().Get(MultiCallHeader))
	if mcVal != "" {
		if mcVal == "on" ||
			mcVal == "1" ||
			mcVal == "true" {
			multiCall = true
		}
	}

	apiVersion := "v1"
	requestVer := c.Param("version")
	if requestVer != "" {
		apiVersion = requestVer
	}

	// multi api calls
	if multiCall {

		var multiRequest map[string]map[string]interface{}

		decoder := json.NewDecoder(c.Request().Body())
		decoder.UseNumber()
		if err = decoder.Decode(&multiRequest); err != nil {
			return
		}

		if multiRequest != nil {
			for tmpAPI, request := range multiRequest {

				api := ""
				ver := apiVersion
				isSpecificVersion := false

				apiV := strings.Split(tmpAPI, ":")
				if len(apiV) == 2 {
					ver = strings.TrimSpace(apiV[1])
					isSpecificVersion = true
				}

				api = strings.TrimSpace(apiV[0])

				if api == "" {
					err = ErrBadRequest.New().Append("API name is empty")
					return
				}

				apiRequests = append(apiRequests,
					postAPIRequest{
						API:               api,
						Content:           request,
						Version:           ver,
						IsSpecificVersion: isSpecificVersion,
					},
				)
			}
		}

		isMultiCall = true

		return
	}

	// singal api call
	api := c.Request().Header().Get(APIHeader)
	api = strings.TrimSpace(api)
	if api == "" {
		err = ErrBadRequest.New().Append("API name is empty")
		return
	}

	var request map[string]interface{}
	decoder := json.NewDecoder(c.Request().Body())
	decoder.UseNumber()
	if err = decoder.Decode(&request); err != nil {
		return
	}

	apiRequests = append(apiRequests, postAPIRequest{API: api, Content: request, Version: apiVersion})

	return
}

func (p *PostAPI) rpcHandle(c echo.Context) (err error) {

	badRequest := func(description string) {
		badErr := ErrBadRequest.New().Append(description)
		resp := postAPIResponse{
			Code:         badErr.Code(),
			Message:      badErr.Error(),
			ErrID:        badErr.Id(),
			ErrNamespace: badErr.Namespace(),
		}
		c.JSON(http.StatusOK, resp)
	}

	errResponse := func(e error) {
		var errCode errors.ErrCode

		if ec, ok := e.(errors.ErrCode); ok {
			errCode = ec
		} else {
			errCode = ErrInternalServerError.New().Append(e)
		}

		resp := postAPIResponse{
			Code:         errCode.Code(),
			Message:      errCode.Error(),
			ErrID:        errCode.Id(),
			ErrNamespace: errCode.Namespace(),
		}
		c.JSON(http.StatusOK, resp)
	}

	// response content type
	// w.Header().Set("Content-Type", "application/json")

	ct := c.Request().Header().Get("Content-Type")

	// Strip charset from Content-Type (like `application/json; charset=UTF-8`)
	if idx := strings.IndexRune(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}

	var apiRequests []postAPIRequest
	var isMultiCall bool

	if apiRequests, isMultiCall, err = GetAPIRequests(c); err != nil {
		errResponse(err)
		return
	}

	// create context
	ctx := requestToContext(c.Request())

	for _, apiRequest := range apiRequests {
		if _, exist := p.getService(apiRequest.API, apiRequest.Version); !exist {
			badRequest(fmt.Sprintf("api not exist, %s:%v", apiRequest.API, apiRequest.Version))
			return
		}
	}

	responsesChan := make(chan postAPIResponse, len(apiRequests))

	for _, apiRequest := range apiRequests {
		go func(ctx context.Context, req postAPIRequest, responsesChan chan postAPIResponse) {
			var resp postAPIResponse
			if srv, exist := p.getService(req.API, req.Version); !exist {
				err := ErrBadRequest.New().Append(fmt.Sprintf("api not exist, %s:%v", req.API, req.Version))

				resp = postAPIResponse{
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

		}(ctx, apiRequest, responsesChan)
	}

	apiResponses := map[string]postAPIResponse{}

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
			if len(apiResponses) == len(apiRequests) {
				break responseFor
			}
		}
	}

	for _, apiReq := range apiRequests {

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

			apiResponses[api] = postAPIResponse{
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

	var finallyResp postAPIResponse

	if isMultiCall {
		finallyResp.Code = 0
		finallyResp.Message = ""
		finallyResp.Result = apiResponses
	} else {
		finallyResp = apiResponses[apiRequests[0].API]
	}

	c.JSON(http.StatusOK, finallyResp)

	return
}

func (p *PostAPI) callMicroService(ctx context.Context, service, method string, request map[string]interface{}) (response postAPIResponse) {

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

func requestToContext(r engine.Request) context.Context {
	ctx := context.Background()
	md := make(metadata.Metadata)

	headerKeys := r.Header().Keys()

	for i := 0; i < len(headerKeys); i++ {
		md[headerKeys[i]] = r.Header().Get(headerKeys[i])
	}

	return metadata.NewContext(ctx, md)
}
