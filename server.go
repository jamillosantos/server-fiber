package srvfiber

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

var (
	defaultSettings = fiber.Config{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
)

var (
	// ErrNotReady is returned by IsReady when the server goroutine has not started yet or has already shut down.
	ErrNotReady = errors.New("service is not ready")
)

type opts struct {
	listener     net.Listener
	bindAddress  string
	appName      string
	name         string
	errorHandler fiber.ErrorHandler
}

func (o opts) apply(f *fiber.Config) {
	f.AppName = o.appName
	if o.errorHandler != nil {
		f.ErrorHandler = o.errorHandler
	}
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

// WithBindAddress sets the TCP address the server will listen on (e.g. ":8080"). Defaults to ":8080".
// Has no effect when a custom listener is set via WithListener.
func WithBindAddress(bindAddress string) Option {
	return func(o *opts) {
		o.bindAddress = bindAddress
	}
}

// WithAppName sets the Fiber application name, exposed in the Server HTTP response header.
func WithAppName(appName string) Option {
	return func(o *opts) {
		o.appName = appName
	}
}

// WithName sets the server's display name (returned by Name()) and also sets the Fiber app name
// if it has not been set already.
func WithName(name string) Option {
	return func(o *opts) {
		o.name = name
		if o.appName == "" {
			o.appName = name
		}
	}
}

// WithErrorHandler sets a custom Fiber error handler that is invoked when a route returns an error.
func WithErrorHandler(handler fiber.ErrorHandler) Option {
	return func(o *opts) {
		o.errorHandler = handler
	}
}

// Initializer is a function that will receive the fiber.App instance with the objetive of initialize its middlewares
// and routes.
type Initializer func(app *fiber.App) error

// FiberServer implements a Fiber-based HTTP server with a managed lifecycle compatible with
// github.com/jamillosantos/application. Use NewFiberServer to create an instance.
type FiberServer struct {
	app         *fiber.App
	config      opts
	initializer Initializer
	ready       atomic.Value
	serverWg    sync.WaitGroup
	listener    net.Listener
}

// Option is a functional option for configuring a FiberServer.
type Option = func(cfg *opts)

// NewFiberServer creates a new FiberServer. The initializer is called during Listen to register
// routes and middleware. Pass functional options to override defaults (bind address, name, etc.).
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

// Name returns the server's display name as configured by WithName.
func (f *FiberServer) Name() string {
	return f.config.name
}

// Listen initializes the Fiber app, runs the Initializer to register routes, binds a TCP listener,
// and starts serving requests in a background goroutine. It returns immediately after the goroutine
// is spawned; use IsReady to confirm the server is accepting connections.
//
// Returns an error if the Initializer fails or if the TCP listener cannot be created.
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
		f.listener = l
		_ = f.app.Listener(l)
	}()

	return nil
}

// Close gracefully shuts down the Fiber app, closes the listener, and waits for the server
// goroutine to finish.
func (f *FiberServer) Close(_ context.Context) error {
	err := f.app.Shutdown()
	_ = f.listener.Close()
	f.serverWg.Wait()
	return err
}

// IsReady returns nil when the server goroutine is running and accepting connections.
// Returns ErrNotReady if Listen has not been called yet or if the server has shut down.
// Implements the readiness interface from github.com/jamillosantos/application.
func (f *FiberServer) IsReady(_ context.Context) error {
	if v := f.ready.Load(); v == nil || v == false {
		return ErrNotReady
	}
	return nil
}
