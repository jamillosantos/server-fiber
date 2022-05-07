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

// Initializer is a function that will receive the fiber.App instance with the objetive of initialize its middlewares
// and routes.
type Initializer func(app *fiber.App) error

// FiberServer represents the services.Server for fiber applications.
type FiberServer struct {
	name        string
	app         *fiber.App
	config      ServerFiberConfig
	fiberConfig *fiber.Config
	initializer Initializer
}

// NewFiberServer returns a new instance of FiberServer initialized.
func NewFiberServer(config ServerFiberConfig, initializer Initializer) *FiberServer {
	settings := defaultSettings
	return &FiberServer{
		config:      config,
		initializer: initializer,
		fiberConfig: &settings,
	}
}

// WithName will initialize the name of the service.
func (server *FiberServer) WithName(name string) *FiberServer {
	server.name = name
	return server
}

// WithConfig initializes the ServerFiberConfig that will be used to start the fiber.App.
func (server *FiberServer) WithConfig(config ServerFiberConfig) *FiberServer {
	server.config = config
	return server
}

func (server *FiberServer) WithFiberConfig(config *fiber.Config) *FiberServer {
	server.fiberConfig = config
	return server
}

func (server *FiberServer) Name() string {
	return server.name
}

func (server *FiberServer) Listen(_ context.Context) error {
	config := *server.fiberConfig

	server.app = fiber.New(config)

	if server.initializer != nil {
		err := server.initializer(server.app)
		if err != nil {
			return err
		}
	}

	return server.app.Listen(server.config.GetBindAddress())
}

func (server *FiberServer) Close(_ context.Context) error {
	return server.app.Shutdown()
}
