package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-micro/transport"
)

var internalAllowHeaders = []string{
	"Origin",
	"Content-Type",
	"Authorization",
	"Accept",
	"Accept-Encoding",
	"X-Requested-With",
	"X-Forwarded-Payload",
	"X-CSRF-Token",
	APIHeader,
	MultiCallHeader,
	APICallTimeoutHeader,
}

type XDomainOptions struct {
	Path    string            `json:"path"`
	LibPath string            `json:"lib_path"`
	LibUrl  string            `json:"lib_url"`
	Masters map[string]string `json:"masters"`
}

type CORSOptions struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

type Option func(*Options)

type Options struct {
	Address        string
	CORS           CORSOptions
	Path           string
	ResponseHeader http.Header
	BodyLimit      string

	Client    client.Client
	Transport transport.Transport
	Registry  registry.Registry
	Broker    broker.Broker

	BeforeHandlers []echo.MiddlewareFunc
	AfterHandlers  []echo.MiddlewareFunc
}

func Address(address string) Option {
	return func(o *Options) {
		o.Address = address
	}
}

func ResponseHeader(key, val string) Option {
	return func(o *Options) {
		if key != "" {
			if o.ResponseHeader == nil {
				o.ResponseHeader = make(http.Header)
			}
			o.ResponseHeader.Set(key, val)
		}
	}
}

func CORS(cors CORSOptions) Option {
	return func(o *Options) {

		allowHeaders := distinctString(append(internalAllowHeaders, cors.AllowHeaders...))
		allowMethods := distinctString(append(cors.AllowMethods, "POST"))
		exposeHeaders := distinctString(cors.ExposeHeaders)

		o.CORS.AllowHeaders = allowHeaders
		o.CORS.AllowMethods = allowMethods
		o.CORS.ExposeHeaders = exposeHeaders
		o.CORS.AllowOrigins = cors.AllowOrigins
		o.CORS.AllowCredentials = cors.AllowCredentials
		o.CORS.MaxAge = cors.MaxAge

	}
}

func Path(path string) Option {
	return func(o *Options) {
		o.Path = path
	}
}

func BodyLimit(size string) Option {
	return func(o *Options) {
		if size == "" {
			o.BodyLimit = "2M"
		}
	}
}

func BeforeHandler(middlewares ...echo.MiddlewareFunc) Option {
	return func(o *Options) {
		o.BeforeHandlers = append(o.BeforeHandlers, middlewares...)
	}
}

func AfterHandler(middlewares ...echo.MiddlewareFunc) Option {
	return func(o *Options) {
		o.AfterHandlers = append(o.AfterHandlers, middlewares...)
	}
}

func MicroClient(c client.Client) Option {
	return func(o *Options) {
		if c != nil {
			o.Client = c
		}
	}
}

func MicroTransport(t transport.Transport) Option {
	return func(o *Options) {
		if t != nil {
			o.Transport = t
		}
	}
}

func MicroRegistry(r registry.Registry) Option {
	return func(o *Options) {
		o.Registry = r
	}
}

func MicroBroker(b broker.Broker) Option {
	return func(o *Options) {
		o.Broker = b
	}
}

func distinctString(values []string) []string {
	if values == nil {
		return nil
	}

	distinctCache := map[string]string{}

	for _, v := range values {
		distinctCache[strings.ToLower(v)] = v
	}

	newValues := []string{}

	for _, v := range distinctCache {
		newValues = append(newValues, v)
	}

	return newValues
}
