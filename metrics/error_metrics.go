package metrics

import "github.com/prometheus/client_golang/prometheus"

// ErrorMetrics groups error tracking metrics
type ErrorMetrics struct {
	// Core error tracking
	PanicsTotal *prometheus.CounterVec
	ErrorsTotal *prometheus.CounterVec

	// Component health
	ComponentHealth *prometheus.GaugeVec
}

// NewErrorMetrics creates and returns error tracking metrics
func NewErrorMetrics() *ErrorMetrics {
	return &ErrorMetrics{
		PanicsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "rollytics_panics_total",
				Help:        "Total number of panics occurred",
				ConstLabels: constLabels(),
			},
			[]string{"component"},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "rollytics_errors_total",
				Help:        "Total number of errors by component and type",
				ConstLabels: constLabels(),
			},
			[]string{"component", "error_type"},
		),
		ComponentHealth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name:        "rollytics_component_health",
				Help:        "Health status of components (1=healthy, 0=unhealthy)",
				ConstLabels: constLabels(),
			},
			[]string{"component"},
		),
	}
}

// Register registers all error metrics with the given registry
func (e *ErrorMetrics) Register(reg *prometheus.Registry) {
	reg.MustRegister(
		e.PanicsTotal,
		e.ErrorsTotal,
		e.ComponentHealth,
	)
}
