package srvfiber

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

var (
	defaultSettings = fiber.Config{
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
		DisableStartupMessage: true,
	}
)

type opts struct {
	bindAddress string
	appName     string
	name        string
}

func (o opts) apply(f *fiber.Config) {
	f.AppName = o.appName
}

func defaultOpts() opts {
	return opts{
		name:        "Fiber Server",
		appName:     "fiber-server",
		bindAddress: ":8080",
	}
}

func WithBindAddress(bindAddress string) Option {
	return func(o *opts) {
		o.bindAddress = bindAddress
	}
}

func WithAppName(appName string) Option {
	return func(o *opts) {
		o.appName = appName
	}
}

func WithName(name string) Option {
	return func(o *opts) {
		o.name = name
		if o.appName == "" {
			o.appName = name
		}
	}
}

// Initializer is a function that will receive the fiber.App instance with the objetive of initialize its middlewares
// and routes.
type Initializer func(app *fiber.App) error

// FiberServer represents the services.Server for fiber applications.
type FiberServer struct {
	app         *fiber.App
	config      opts
	initializer Initializer
}

type Option = func(cfg *opts)

// NewFiberServer returns a new instance of FiberServer initialized.
func NewFiberServer(initializer Initializer, opts ...Option) *FiberServer {
	o := defaultOpts()
	for _, opt := range opts {
		opt(&o)
	}
	return &FiberServer{
		config:      o,
		initializer: initializer,
	}
}

func (server *FiberServer) Name() string {
	return server.config.name
}

func (server *FiberServer) Listen(_ context.Context) error {
	config := defaultSettings
	server.config.apply(&config)

	server.app = fiber.New(config)

	if server.initializer != nil {
		err := server.initializer(server.app)
		if err != nil {
			return err
		}
	}

	return server.app.Listen(server.config.bindAddress)
}

func (server *FiberServer) Close(_ context.Context) error {
	return server.app.Shutdown()
}
