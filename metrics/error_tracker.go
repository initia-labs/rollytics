package metrics

import (
	"fmt"
	"runtime"
)

// TrackPanic tracks panic occurrences
func TrackPanic(component string) {
	GetMetrics().Error.PanicsTotal.WithLabelValues(component).Inc()
}

// TrackError tracks errors by component and type
func TrackError(component, errorType string) {
	GetMetrics().Error.ErrorsTotal.WithLabelValues(component, errorType).Inc()
}

// SetComponentHealth sets the health status of a component
func SetComponentHealth(component string, healthy bool) {
	var status float64
	if healthy {
		status = 1
	}
	GetMetrics().Error.ComponentHealth.WithLabelValues(component).Set(status)
}

// RecoverFromPanic recovers from panics and tracks metrics
func RecoverFromPanic(component string) {
	if r := recover(); r != nil {
		// Get function name from stack
		pc, _, _, ok := runtime.Caller(1)
		functionName := "unknown"
		if ok {
			fn := runtime.FuncForPC(pc)
			if fn != nil {
				functionName = fn.Name()
			}
		}
		
		TrackPanic(component)
		TrackError(component, "panic")
		
		// Re-panic to maintain original behavior
		panic(fmt.Sprintf("recovered panic in %s.%s: %v", component, functionName, r))
	}
}