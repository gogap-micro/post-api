package api

import (
	"sync"

	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-micro/transport"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/fasthttp"
	"github.com/labstack/echo/engine/standard"
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

			Client:    client.DefaultClient,
			Transport: transport.DefaultTransport,
			Registry:  registry.DefaultRegistry,
			Broker:    broker.DefaultBroker,

			RequestTopic:  DefaultRequestTopic,
			ResponseTopic: DefaultResponseTopic,
		},
		httpSrv:    nil,
		apiService: make(map[string]map[string]microService),
		stopedChan: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(&postAPI.Options)
	}

	httpSrv := echo.New()

	groupRoot := httpSrv.Group("")
	groupRoot.Get("/ping", postAPI.pingHandle)
	groupRoot.Get("/favicon.ico", postAPI.faviconICONHandle)

	groupAPI := groupRoot.Group(postAPI.Options.Path,
		middleware.BodyLimit(postAPI.Options.BodyLimit),
		postAPI.writeBasicHeaders,
		postAPI.cors)

	beforeMiddlewares := append([]echo.MiddlewareFunc{postAPI.parseAPIRequests, postAPI.onRequestEvent}, postAPI.Options.BeforeHandlers...)

	afterMiddlerwres := append([]echo.MiddlewareFunc{postAPI.onResponseEvent}, postAPI.Options.AfterHandlers...)

	groupAPI.Use(beforeMiddlewares...)
	groupAPI.Post("/:version", postAPI.rpcHandle)
	groupAPI.Use(afterMiddlerwres...)

	httpSrv.SetHTTPErrorHandler(postAPI.errorHandle)
	httpSrv.SetLogger(wrapperLogger(postAPI.Options.Logger))

	postAPI.httpSrv = httpSrv

	srv = &postAPI

	return
}

func (p *PostAPI) Run() (err error) {

	conf := engine.Config{
		Address:     p.Options.Address,
		TLSCertfile: p.Options.TLSCertFile,
		TLSKeyfile:  p.Options.TLSKeyFile,
	}

	var echoEngine engine.Server

	if p.Options.Engine == Fasthttp {
		echoEngine = fasthttp.WithConfig(conf)

	} else {
		echoEngine = standard.WithConfig(conf)
	}

	go p.httpSrv.Run(echoEngine)

	if p.Options.Broker != nil {
		if err = p.Options.Broker.Connect(); err != nil {
			return
		}
	}

	var regWatcher registry.Watcher
	if regWatcher, err = p.Options.Registry.Watch(); err != nil {
		return
	}

	if err = p.watch(regWatcher); err != nil {
		return
	}

	close(p.stopedChan)

	return
}
