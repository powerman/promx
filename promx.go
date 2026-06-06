// Package promx provides helpers extending Prometheus client metrics.
//
// # State gauge
//
// The [State] type tracks a finite-state machine as a labelled gauge.
// Exactly one series of the gauge (selected by the `"state"` label) is set to 1 at any moment,
// making the lifecycle of a long-lived owner observable without manual inc/dec bookkeeping.
package promx

import (
	"maps"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// StateLabel is the label name carrying the current state value.
const StateLabel = "state"

// LabelState is a state value tracked by a [State].
type LabelState string

// State tracks a current state as a gauge:
// exactly one of the gauge's state-labeled series is set to 1 at a time,
// so a single owner's lifecycle (e.g. a connection moving through connecting→ready→closed)
// is observable without manual increment/decrement bookkeeping at the call site.
//
// The caller owns the [prometheus.GaugeVec] and is responsible for registering it
// with a registry.
// All possible states should be pre-initialised to zero
// so they appear in the /metrics output immediately (see example).
type State struct {
	mu     sync.Mutex
	metric *prometheus.GaugeVec
	labels prometheus.Labels
}

// NewState creates a State for metric with the given initial state and extra labels,
// marking the initial state series active.
func NewState(metric *prometheus.GaugeVec, state LabelState, labels prometheus.Labels) *State {
	s := &State{
		metric: metric,
		labels: make(prometheus.Labels, len(labels)+1),
	}
	maps.Copy(s.labels, labels)
	s.labels[StateLabel] = string(state)

	s.metric.With(s.labels).Inc()
	return s
}

// Remove deactivates the current state series. The State must not be used after.
func (s *State) Remove() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metric.With(s.labels).Dec()
	s.metric = nil
}

// Set deactivates the current state series and activates the one for state.
func (s *State) Set(state LabelState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metric.With(s.labels).Dec()
	s.labels[StateLabel] = string(state)
	s.metric.With(s.labels).Inc()
}
