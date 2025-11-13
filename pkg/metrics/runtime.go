package metrics

import (
	otelruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
)

// InitRuntimeMetrics initializes Go runtime metrics instrumentation.
// This includes CPU, memory, GC, and goroutine metrics.
// The runtime instrumentation automatically collects:
// - CPU usage (go_cpu_*)
// - Memory allocation and heap stats (go_memstats_*)
// - GC statistics (go_memstats_gc_*)
// - Goroutine count (go_goroutines)
func InitRuntimeMetrics() error {
	if !IsEnabled() {
		return nil
	}

	// Start OpenTelemetry runtime metrics collection
	// This automatically collects Go runtime metrics
	err := otelruntime.Start(otelruntime.WithMinimumReadMemStatsInterval(0))
	if err != nil {
		return err
	}

	return nil
}
