# go-server-fiber

A Go library that wraps [Fiber v3](https://github.com/gofiber/fiber) to implement the server lifecycle interface from [`github.com/jamillosantos/application`](https://github.com/jamillosantos/application).

## Installation

```bash
go get github.com/jamillosantos/server-fiber/v2
```

## Usage

```go
import srvfiber "github.com/jamillosantos/server-fiber/v2"

server := srvfiber.NewFiberServer(func(app *fiber.App) error {
    app.Get("/health", func(c fiber.Ctx) error {
        return c.SendStatus(fiber.StatusOK)
    })
    return nil
}, srvfiber.WithBindAddress(":8080"))

// Start (non-blocking)
if err := server.Listen(ctx); err != nil {
    log.Fatal(err)
}

// Graceful shutdown
if err := server.Close(ctx); err != nil {
    log.Fatal(err)
}
```

## Options

| Option | Description |
|--------|-------------|
| `WithBindAddress(addr string)` | TCP address to listen on (default: `:8080`) |
| `WithName(name string)` | Server name, also sets AppName if not already set |
| `WithAppName(name string)` | Fiber app name |
| `WithListener(l net.Listener)` | Use a custom `net.Listener` instead of creating one |
| `WithErrorHandler(h fiber.ErrorHandler)` | Custom Fiber error handler |

## Readiness

`FiberServer` implements a readiness check compatible with `github.com/jamillosantos/application`:

```go
err := server.IsReady(ctx) // returns ErrNotReady if not yet listening
```
