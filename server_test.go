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

	"github.com/gofiber/fiber/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "srvfiber")
}

var _ = Describe("FiberServer", func() {
	var (
		ctx         = context.Background()
		initializer = func(app *fiber.App) error { return nil }
	)

	Describe("NewFiberServer", func() {
		It("stores the initializer", func() {
			got := NewFiberServer(initializer)
			Expect(fmt.Sprintf("%p", got.initializer)).To(Equal(fmt.Sprintf("%p", initializer)))
		})

		It("sets the name via WithName", func() {
			got := NewFiberServer(initializer, WithName("want name"))
			Expect(got.Name()).To(Equal("want name"))
		})
	})

	Describe("Listen", func() {
		It("starts the server and serves requests", func() {
			got := NewFiberServer(func(app *fiber.App) error {
				app.Get("/test", func(c fiber.Ctx) error {
					return c.JSON(true)
				})
				return nil
			}, WithBindAddress(":18080"))

			Expect(got.Listen(ctx)).To(Succeed())
			DeferCleanup(got.Close, ctx)

			var resp *http.Response
			Eventually(func() error {
				var err error
				resp, err = http.DefaultClient.Get("http://localhost:18080/test")
				return err
			}, time.Second, 50*time.Millisecond).Should(Succeed())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(resp.Body)
			Expect(string(body)).To(Equal("true"))
		})

		It("returns an error when the initializer fails", func() {
			wantErr := errors.New("random error")
			got := NewFiberServer(func(app *fiber.App) error {
				return wantErr
			}, WithBindAddress(":18080"))

			Expect(got.Listen(ctx)).To(MatchError(wantErr))
		})

		It("returns an error when the address is already in use", func() {
			existing, err := net.Listen("tcp", ":18080")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(existing.Close)

			got := NewFiberServer(initializer, WithBindAddress(":18080"))
			Expect(got.Listen(ctx)).To(MatchError(ContainSubstring("address already in use")))
		})
	})

	Describe("IsReady", func() {
		It("returns ErrNotReady before Listen is called", func() {
			got := NewFiberServer(initializer)
			Expect(got.IsReady(ctx)).To(MatchError(ErrNotReady))
		})

		It("returns nil once the server is accepting connections", func() {
			got := NewFiberServer(func(app *fiber.App) error {
				time.Sleep(500 * time.Millisecond)
				return nil
			}, WithBindAddress(":18080"))
			go func() {
				defer GinkgoRecover()

				Expect(got.Listen(ctx)).To(Succeed())
				DeferCleanup(got.Close, ctx)
			}()

			now := time.Now()
			Eventually(func() error {
				return got.IsReady(ctx)
			}).
				WithTimeout(time.Second).
				WithPolling(time.Millisecond).
				Should(Succeed())
			Expect(time.Since(now)).To(BeNumerically("~", 500*time.Millisecond, 10*time.Millisecond))
		})
	})
})
