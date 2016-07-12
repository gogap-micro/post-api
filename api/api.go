package api

import (
	"github.com/micro/go-micro/selector"
	"sync"
	"time"

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

	timerPool sync.Pool
}

func NewPostAPI(opts ...Option) (srv *PostAPI, err error) {
	postAPI := PostAPI{
		Options: Options{
			Address:   ":8088",
			Path:      "/",
			BodyLimit: "1M",

			Client:    client.DefaultClient,
			Transport: transport.DefaultTransport,
			Registry:  registry.DefaultRegistry,
			Broker:    broker.DefaultBroker,
			Selector:  selector.DefaultSelector,

			RequestTopic:  DefaultRequestTopic,
			ResponseTopic: DefaultResponseTopic,
		},
		httpSrv:    nil,
		apiService: make(map[string]map[string]microService),
		stopedChan: make(chan struct{}),

		timerPool: sync.Pool{New: func() interface{} { t := time.NewTimer(time.Second * 30); t.Stop(); return t }},
	}

	for _, opt := range opts {
		opt(&postAPI.Options)
	}

	httpSrv := echo.New()

	httpSrv.Use(middleware.BodyLimit(postAPI.Options.BodyLimit))

	groupRoot := httpSrv.Group("")
	groupRoot.Get("/ping", postAPI.pingHandle)
	groupRoot.Get("/favicon.ico", postAPI.faviconICONHandle)

	groupAPI := groupRoot.Group(
		postAPI.Options.Path,
	)

	middlewares := append([]echo.MiddlewareFunc{postAPI.cors, postAPI.writeBasicHeaders, postAPI.parseAPIRequests, postAPI.onRequestEvent}, postAPI.Options.Middlewares...)

	groupAPI.Post("/:version", postAPI.rpcHandle, middlewares...)
	groupAPI.Options("/:version", nil, postAPI.cors, postAPI.writeBasicHeaders)

	httpSrv.SetHTTPErrorHandler(postAPI.errorHandle)
	httpSrv.SetLogger(wrapperLogger(postAPI.Options.Logger))

	postAPI.httpSrv = httpSrv

	srv = &postAPI

	return
}

func (p *PostAPI) Run() (err error) {

	if err = p.Options.Client.Init(client.Transport(p.Options.Transport)); err != nil {
		return
	}

	if err = p.Options.Client.Init(client.Registry(p.Options.Registry)); err != nil {
		return
	}

	if err = p.Options.Client.Init(client.Selector(p.Options.Selector)); err != nil {
		return
	}

	if err = p.Options.Broker.Init(broker.Registry(p.Options.Registry)); err != nil {
		return
	}

	conf := engine.Config{
		Address:     p.Options.Address,
		TLSCertFile: p.Options.TLSCertFile,
		TLSKeyFile:  p.Options.TLSKeyFile,
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
