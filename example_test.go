package promx_test

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/powerman/promx"
)

// This example shows the typical lifecycle of a long-lived entity
// (e.g. a WebSocket connection) tracked with a State gauge.
func ExampleState_basic() {
	// Create a gauge that tracks connections per state,
	// registered with a local registry.
	reg := prometheus.NewRegistry()
	stream := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stream",
		Help: "Current streams per state.",
	}, []string{promx.StateLabel})
	reg.MustRegister(stream)

	// Pre-initialise all possible states to zero so they appear in /metrics immediately,
	// even before any connection reaches that state.
	states := []promx.LabelState{"connecting", "working", "idle", "closed"}
	for _, s := range states {
		stream.With(prometheus.Labels{
			promx.StateLabel: string(s),
		})
	}

	// Start tracking a stream in "connecting" state.
	state := promx.NewState(stream, "connecting", nil)

	// Transition through the lifecycle.
	state.Set("working")
	state.Set("idle")
	state.Set("working")
	state.Set("closed")

	// When the stream is gone, remove it so the gauge is properly zeroed.
	state.Remove()

	fmt.Println("lifecycle complete")
	// Output: lifecycle complete
}

// This example shows how to attach constant labels (like a service or component name)
// to every observation of a State gauge.
func ExampleState_withLabels() {
	reg := prometheus.NewRegistry()
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "component_state",
		Help: "Components per state and kind.",
	}, []string{"kind", promx.StateLabel})
	reg.MustRegister(gauge)

	// Pre-initialise all combinations of known label values.
	states := []promx.LabelState{"starting", "running", "stopped"}
	for _, s := range states {
		gauge.With(prometheus.Labels{
			"kind":           "worker",
			promx.StateLabel: string(s),
		})
	}

	// Pass extra labels to NewState — they are merged into every observation.
	state := promx.NewState(gauge, "starting", prometheus.Labels{
		"kind": "worker",
	})

	state.Set("running")
	state.Remove()

	fmt.Println("tracked with labels")
	// Output: tracked with labels
}
