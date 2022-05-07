package srvfiber

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServerFiber(t *testing.T) {
	initializer := func(app *fiber.App) error {
		return nil
	}
	fiberSettings := &fiber.Config{}
	var config ServerFiberConfig

	t.Run("initializer", func(t *testing.T) {
		got := NewFiberServer(config, initializer)
		assert.Equal(t, fmt.Sprintf("%p", initializer), fmt.Sprintf("%p", got.initializer))
		assert.Equal(t, config, got.config)
		assert.Equal(t, defaultSettings, *got.fiberConfig)
	})

	t.Run("withName", func(t *testing.T) {
		wantName := "want name"
		got := NewFiberServer(config, initializer).WithName(wantName)
		assert.Equal(t, wantName, got.Name())
	})

	t.Run("withSettings", func(t *testing.T) {
		got := NewFiberServer(config, initializer).WithFiberConfig(fiberSettings)
		assert.Equal(t, fiberSettings, got.fiberConfig)
	})

	t.Run("with config", func(t *testing.T) {
		got := NewFiberServer(nil, initializer).WithConfig(config)
		assert.Equal(t, config, got.config)
	})

	t.Run("start server", func(t *testing.T) {
		ctx := context.TODO()

		got := NewFiberServer(nil, func(app *fiber.App) error {
			app.Get("/test", func(ctx *fiber.Ctx) error {
				return ctx.JSON(true)
			})
			return nil
		}).WithConfig(&PlatformConfig{
			BindAddress: ":18080",
		})

		go func() {
			time.Sleep(time.Second)
			resp, err := http.DefaultClient.Get("http://localhost:18080/test")

			require.NoError(t, err)
			defer func() {
				_ = resp.Body.Close()
			}()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			assert.Equal(t, "true", string(body))
			_ = got.Close(ctx)
		}()
		err := got.Listen(ctx)
		assert.NoError(t, err)
	})

	t.Run("intializer failed", func(t *testing.T) {
		ctx := context.TODO()

		wantErr := errors.New("random error")

		got := NewFiberServer(nil, func(app *fiber.App) error {
			return wantErr
		}).WithConfig(&PlatformConfig{
			BindAddress: ":18080",
		})

		err := got.Listen(ctx)
		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("fails starting server", func(t *testing.T) {
		ctx := context.TODO()

		bindAddr := ":18080"

		listen, err := net.Listen("tcp", bindAddr)
		require.NoError(t, err)
		defer func() {
			_ = listen.Close()
		}()

		got := NewFiberServer(nil, initializer).WithConfig(&PlatformConfig{
			BindAddress: bindAddr,
		})

		go func() {
			time.Sleep(time.Second)
			_ = got.Close(ctx)
		}()
		err = got.Listen(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "address already in use")
	})
}
