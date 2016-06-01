package api

import (
	"github.com/labstack/echo/engine"
	"sync"

	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-micro/transport"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/fasthttp"
	"github.com/labstack/echo/middleware"
)

type microService struct {
	Service string
	Method  string
}

type PostAPI struct {
	Options Options

	httpSrv    *echo.Echo
	stopedChan chan struct{}

	apiService map[string]map[string]microService

	reglocker sync.Mutex
}

func NewPostAPI(opts ...Option) (srv *PostAPI, err error) {
	postAPI := PostAPI{
		Options: Options{
			Address:   ":8088",
			Path:      "/",
			BodyLimit: "2M",
		},
		httpSrv:    nil,
		apiService: make(map[string]map[string]microService),
		stopedChan: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(&postAPI.Options)
	}

	httpSrv := echo.New()

	groupRoot := httpSrv.Group("/")
	groupRoot.Get("ping", postAPI.pingHandle)
	groupRoot.Get("favicon.ico", postAPI.faviconICONHandle)

	groupAPI := httpSrv.Group(postAPI.Options.Path,
		middleware.BodyLimit(postAPI.Options.BodyLimit),
		postAPI.writeBasicHeaders,
		postAPI.cors)

	handlers := append([]echo.MiddlewareFunc{postAPI.parseAPIRequests}, postAPI.Options.BeforeHandlers...)

	groupAPI.Post("/:version", postAPI.rpcHandle, handlers...)
	groupAPI.Use(postAPI.Options.AfterHandlers...)

	httpSrv.SetHTTPErrorHandler(postAPI.errorHandle)

	postAPI.httpSrv = httpSrv

	if postAPI.Options.Client == nil {
		postAPI.Options.Client = client.DefaultClient
	}

	if postAPI.Options.Transport == nil {
		postAPI.Options.Transport = transport.DefaultTransport
	}

	if postAPI.Options.Registry == nil {
		postAPI.Options.Registry = registry.DefaultRegistry
	}

	if postAPI.Options.Broker == nil {
		postAPI.Options.Broker = broker.DefaultBroker
	}

	srv = &postAPI

	return
}

func (p *PostAPI) Run() (err error) {

	var regWatcher registry.Watcher
	if regWatcher, err = p.Options.Registry.Watch(); err != nil {
		return
	}

	conf := engine.Config{
		Address:     p.Options.Address,
		TLSCertfile: p.Options.TLSCertFile,
		TLSKeyfile:  p.Options.TLSKeyFile,
	}

	httpSrvEngine := fasthttp.WithConfig(conf)

	go p.httpSrv.Run(httpSrvEngine)

	if err = p.watch(regWatcher); err != nil {
		return
	}

	close(p.stopedChan)

	return
}
