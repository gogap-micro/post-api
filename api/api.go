package api

import (
	"path/filepath"
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

	httpSrv.Get("/ping", postAPI.pingHandle)

	corsMiddleware := middleware.CORSWithConfig(
		middleware.CORSConfig{
			AllowOrigins:     postAPI.Options.CORS.AllowOrigins,
			AllowMethods:     postAPI.Options.CORS.AllowMethods,
			AllowHeaders:     postAPI.Options.CORS.AllowHeaders,
			AllowCredentials: postAPI.Options.CORS.AllowCredentials,
			ExposeHeaders:    postAPI.Options.CORS.ExposeHeaders,
			MaxAge:           postAPI.Options.CORS.MaxAge,
		})

	httpSrv.Use(
		middleware.BodyLimit(postAPI.Options.BodyLimit),
		postAPI.writeBasicHeaders,
		corsMiddleware,
	)

	httpSrv.Use(postAPI.Options.BeforeHandlers...)
	httpSrv.Use(postAPI.Options.AfterHandlers...)

	path := filepath.Join(postAPI.Options.Path, ":version")
	httpSrv.Post(path, postAPI.rpcHandle)

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

	postAPI.httpSrv = httpSrv

	srv = &postAPI

	return
}

func (p *PostAPI) Run() (err error) {

	var regWatcher registry.Watcher
	if regWatcher, err = p.Options.Registry.Watch(); err != nil {
		return
	}

	httpSrvEngine := fasthttp.New(p.Options.Address)

	go p.httpSrv.Run(httpSrvEngine)

	if err = p.watch(regWatcher); err != nil {
		return
	}

	close(p.stopedChan)

	return
}
