// Package metrics owns the service's Prometheus registry. A private registry
// keeps tests isolated and avoids global collector registration races.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry                   *prometheus.Registry
	HTTPRequests               *prometheus.CounterVec
	HTTPRequestDuration        *prometheus.HistogramVec
	OrdersCreated              prometheus.Counter
	OrdersPaid                 prometheus.Counter
	OrdersExpired              prometheus.Counter
	InventoryReservationFailed prometheus.Counter
}

func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total", Help: "Total HTTP requests by method, route and status.",
		}, []string{"method", "route", "status"}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "http_request_duration_seconds", Help: "HTTP request duration by method and route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		OrdersCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "orders_created_total", Help: "Orders committed successfully.",
		}),
		OrdersPaid: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "orders_paid_total", Help: "Orders paid successfully.",
		}),
		OrdersExpired: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "orders_expired_total", Help: "Pending orders expired by the worker.",
		}),
		InventoryReservationFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "inventory_reservation_failed_total", Help: "Inventory reservations rejected or failed.",
		}),
	}
	m.registry.MustRegister(
		m.HTTPRequests, m.HTTPRequestDuration, m.OrdersCreated, m.OrdersPaid,
		m.OrdersExpired, m.InventoryReservationFailed,
	)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
