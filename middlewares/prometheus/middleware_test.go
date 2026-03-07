package prometheus

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	fiberrecover "github.com/gofiber/fiber/v3/middleware/recover"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "prometheus middleware")
}

// nsSeq generates unique namespaces per test so each test gets its own set of
// metrics and never conflicts with the default Prometheus registry.
var nsSeq int

func nextNS() string {
	nsSeq++
	return fmt.Sprintf("testns%d", nsSeq)
}

func doRequest(app *fiber.App, method, path string, body []byte) *http.Response {
	var req *http.Request
	if len(body) > 0 {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	resp, err := app.Test(req)
	Expect(err).NotTo(HaveOccurred())
	return resp
}

// gatherHistogram returns the histogram sample for the given metric name and label set.
func gatherHistogram(metricName string, labels prometheus.Labels) *dto.Histogram {
	mfs, _ := prometheus.DefaultGatherer.Gather()
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelsMatch(m.GetLabel(), labels) {
				return m.GetHistogram()
			}
		}
	}
	return nil
}

func labelsMatch(pairs []*dto.LabelPair, want prometheus.Labels) bool {
	got := make(prometheus.Labels, len(pairs))
	for _, p := range pairs {
		got[p.GetName()] = p.GetValue()
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

var _ = Describe("Middleware", func() {
	var (
		ns  string
		app *fiber.App
	)

	BeforeEach(func() {
		ns = nextNS()
		app = fiber.New()
		app.Use(Middleware(WithNamespace(ns)))
		app.Get("/test", func(c fiber.Ctx) error {
			return c.SendString("hello")
		})
	})

	Describe("requests_total", func() {
		It("is incremented after a completed request", func() {
			doRequest(app, http.MethodGet, "/test", nil)

			counter := mapMetrics[ns].requestsTotalCounter.WithLabelValues("GET", "200", "/test")
			Expect(testutil.ToFloat64(counter)).To(Equal(1.0))
		})

		It("uses the status code set by the handler", func() {
			app.Get("/not-found", func(c fiber.Ctx) error {
				return c.SendStatus(http.StatusNotFound)
			})

			doRequest(app, http.MethodGet, "/not-found", nil)

			counter := mapMetrics[ns].requestsTotalCounter.WithLabelValues("GET", "404", "/not-found")
			Expect(testutil.ToFloat64(counter)).To(Equal(1.0))
		})
	})

	Describe("requests_in_flight", func() {
		It("is 1 while a request is in progress", func() {
			handlerReached := make(chan struct{})
			proceed := make(chan struct{})

			app.Get("/slow", func(c fiber.Ctx) error {
				close(handlerReached)
				<-proceed
				return nil
			})

			go func() {
				defer GinkgoRecover()
				doRequest(app, http.MethodGet, "/slow", nil)
			}()

			<-handlerReached
			gauge := mapMetrics[ns].requestsInFlightTotal.WithLabelValues("GET", "/slow")
			Expect(testutil.ToFloat64(gauge)).To(Equal(1.0))
			close(proceed)
		})

		It("is 0 after the request completes", func() {
			doRequest(app, http.MethodGet, "/test", nil)

			gauge := mapMetrics[ns].requestsInFlightTotal.WithLabelValues("GET", "/test")
			Expect(testutil.ToFloat64(gauge)).To(Equal(0.0))
		})
	})

	Describe("request_duration_seconds", func() {
		It("records one sample per request", func() {
			doRequest(app, http.MethodGet, "/test", nil)

			h := gatherHistogram(ns+"_request_duration_seconds", prometheus.Labels{
				"method": "GET", "code": "200", "path": "/test",
			})
			Expect(h).NotTo(BeNil())
			Expect(h.GetSampleCount()).To(BeEquivalentTo(1))
		})
	})

	Describe("request_size_bytes", func() {
		It("records the request body size", func() {
			postNS := nextNS()
			postApp := fiber.New()
			postApp.Use(Middleware(WithNamespace(postNS)))
			postApp.Post("/upload", func(c fiber.Ctx) error { return nil })

			body := []byte("hello world")
			doRequest(postApp, http.MethodPost, "/upload", body)

			h := gatherHistogram(postNS+"_request_size_bytes", prometheus.Labels{
				"method": "POST", "code": "200", "path": "/upload",
			})
			Expect(h).NotTo(BeNil())
			Expect(h.GetSampleCount()).To(BeEquivalentTo(1))
			Expect(h.GetSampleSum()).To(BeEquivalentTo(float64(len(body))))
		})
	})

	Describe("response_size_bytes", func() {
		It("records the response body size", func() {
			doRequest(app, http.MethodGet, "/test", nil)

			h := gatherHistogram(ns+"_response_size_bytes", prometheus.Labels{
				"method": "GET", "code": "200", "path": "/test",
			})
			Expect(h).NotTo(BeNil())
			Expect(h.GetSampleCount()).To(BeEquivalentTo(1))
			Expect(h.GetSampleSum()).To(BeEquivalentTo(float64(len("hello"))))
		})
	})

	Describe("panics_total", func() {
		It("is incremented when a handler panics and the panic is re-raised", func() {
			panicNS := nextNS()
			panicApp := fiber.New()
			panicApp.Use(fiberrecover.New())
			panicApp.Use(Middleware(WithNamespace(panicNS)))
			panicApp.Get("/panic", func(c fiber.Ctx) error {
				panic("something went wrong")
			})

			resp := doRequest(panicApp, http.MethodGet, "/panic", nil)
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			counter := mapMetrics[panicNS].panicsTotalCounter.WithLabelValues("GET", "/panic")
			Expect(testutil.ToFloat64(counter)).To(Equal(1.0))
		})
	})
})
