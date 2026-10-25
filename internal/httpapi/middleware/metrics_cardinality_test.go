package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fonghehe/vue-h5-template-business-service/internal/metrics"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func TestPrometheusBoundsUnknownMethodLabels(t *testing.T) {
	m := metrics.New()
	router := gin.New()
	router.Use(Prometheus(m))
	router.NoRoute(func(c *gin.Context) { c.Status(http.StatusNotFound) })
	// Real HTTP requests confirm that valid extension methods reach Gin.
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	for i := range 25 {
		req, err := http.NewRequest(fmt.Sprintf("EXTENSION%d", i), server.URL+"/missing", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, readErr := io.Copy(io.Discard, resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("extension method response read=%v close=%v", readErr, closeErr)
		}
	}
	// Inspect only the two application metric families, not runtime collectors.
	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, family := range []string{"http_requests_total", "http_request_duration_seconds_count"} {
		count := 0
		for _, line := range strings.Split(recorder.Body.String(), "\n") {
			if strings.HasPrefix(line, family+"{") {
				count++
				if !strings.Contains(line, `method="OTHER"`) {
					t.Errorf("%s extension method label = %s, want OTHER", family, line)
				}
			}
		}
		if count != 1 {
			t.Errorf("%s extension method series = %d, want 1 for 25 distinct methods", family, count)
		}
	}
}

func TestPrometheusPreservesStandardMethodLabels(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace} {
		t.Run(method, func(t *testing.T) {
			m := metrics.New()
			router := gin.New()
			router.Use(Prometheus(m))
			router.NoRoute(func(c *gin.Context) { c.Status(http.StatusNotFound) })
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, "/missing", nil))
			recorder := httptest.NewRecorder()
			m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
			if !strings.Contains(recorder.Body.String(), `http_requests_total{method="`+method+`",route="unmatched",status="404"} 1`) {
				t.Errorf("Prometheus(%s) missing original standard method label", method)
			}
		})
	}
}

// Each operation processes 100 distinct extension methods with a fresh
// registry. This measures allocation growth, not HTTP/network throughput.
func BenchmarkPrometheusExtensionMethods(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	requests := make([]*http.Request, 100)
	for i := range requests {
		requests[i] = httptest.NewRequest(fmt.Sprintf("EXTENSION%d", i), "/missing", nil)
	}
	b.ReportAllocs()
	for b.Loop() {
		m := metrics.New()
		router := gin.New()
		router.Use(Prometheus(m))
		router.NoRoute(func(c *gin.Context) { c.Status(http.StatusNotFound) })
		for _, request := range requests {
			router.ServeHTTP(httptest.NewRecorder(), request)
		}
		// Collect cannot exceed one counter child per supplied method.
		collected := make(chan prometheus.Metric, len(requests))
		m.HTTPRequests.Collect(collected)
		close(collected)
		b.ReportMetric(float64(len(collected)), "method-series/op")
	}
}
