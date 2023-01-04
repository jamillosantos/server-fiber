package srvfiber

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
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

var (
	ErrNotReady = errors.New("service is not ready")
)

type opts struct {
	listener    net.Listener
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

// WithListener will set the listener to be used by the server. If no listener is set, the server will start a new one
// using the bind address.
func WithListener(l net.Listener) Option {
	return func(o *opts) {
		o.listener = l
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
	ready       atomic.Value
	serverWg    sync.WaitGroup
	listener    net.Listener
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

func (f *FiberServer) Name() string {
	return f.config.name
}

func (f *FiberServer) Listen(_ context.Context) error {
	config := defaultSettings
	f.config.apply(&config)

	f.app = fiber.New(config)

	if f.initializer != nil {
		err := f.initializer(f.app)
		if err != nil {
			return err
		}
	}

	l := f.config.listener

	if l == nil {
		newL, err := net.Listen("tcp", f.config.bindAddress)
		if err != nil {
			return err
		}
		l = newL
	}

	f.serverWg.Add(1)
	go func() {
		f.ready.Store(true)
		defer func() {
			f.serverWg.Done()
			f.ready.Store(false)
		}()

		f.listener = l
		_ = f.app.Listener(l)
	}()

	return nil
}

func (f *FiberServer) Close(_ context.Context) error {
	err := f.app.Shutdown()
	_ = f.listener.Close()
	f.serverWg.Wait()
	return err
}

// IsReady will return true if the service is ready to accept requests. This is compliant with the
// github.com/jamillosantos/application library.
func (f *FiberServer) IsReady(_ context.Context) error {
	if v := f.ready.Load(); v == nil || v == false {
		return ErrNotReady
	}
	return nil
}
