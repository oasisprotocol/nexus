package metrics

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

// Registers the collector with Prometheus. If an identical collector is already
// registered, returns the existing collector, otherwise returns the provided collector.
// Panics if the collector cannot be registered.
func registerOnce(collector prometheus.Collector) prometheus.Collector {
	if err := prometheus.Register(collector); err != nil {
		are := &prometheus.AlreadyRegisteredError{}
		if errors.As(err, are) {
			// Use the old collector from now on.
			return are.ExistingCollector
		}
		// Something else went wrong.
		panic(err)
	}
	return collector
}
