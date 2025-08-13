package metrics

// TrackPanic tracks panic occurrences
func TrackPanic(component string) {
	GetMetrics().Error.PanicsTotal.WithLabelValues(component).Inc()
}

// TrackError tracks errors by component and type
func TrackError(component, errorType string) {
	GetMetrics().Error.ErrorsTotal.WithLabelValues(component, errorType).Inc()
}
