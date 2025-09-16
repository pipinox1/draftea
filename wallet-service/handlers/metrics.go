package handlers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewMetricsHandler creates a new Prometheus metrics handler
func NewMetricsHandler() http.Handler {
	return promhttp.Handler()
}