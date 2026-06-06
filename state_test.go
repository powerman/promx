package promx_test

import (
	"sync"
	"testing"

	"github.com/powerman/check"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/powerman/promx"
)

func TestMain(m *testing.M) { check.TestMain(m) }

// gaugeValue returns the current value of a state series in the gauge.
func gaugeValue(gauge *prometheus.GaugeVec, state promx.LabelState) float64 {
	return testutil.ToFloat64(gauge.With(prometheus.Labels{promx.StateLabel: string(state)}))
}

// gaugeWithStates creates a GaugeVec with the given metric name
// and pre-initialises all the given states to zero.
func gaugeWithStates(name string, states []promx.LabelState) *prometheus.GaugeVec {
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: "Test gauge for " + name,
	}, []string{promx.StateLabel})
	for _, s := range states {
		gauge.With(prometheus.Labels{promx.StateLabel: string(s)})
	}
	return gauge
}

func TestNewState(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()
	gauge := gaugeWithStates("test_new_state",
		[]promx.LabelState{"initial", "running", "done"})

	s := promx.NewState(gauge, "initial", nil)

	t.Equal(gaugeValue(gauge, "initial"), 1.0, "initial state must be 1 after NewState")
	t.Equal(gaugeValue(gauge, "running"), 0.0, "other states must remain 0")
	t.Equal(gaugeValue(gauge, "done"), 0.0, "other states must remain 0")

	s.Remove()
}

func TestState_Set(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()
	gauge := gaugeWithStates("test_set",
		[]promx.LabelState{"idle", "active", "busy"})

	s := promx.NewState(gauge, "idle", nil)

	t.Equal(gaugeValue(gauge, "idle"), 1.0, "initial state is idle")

	s.Set("active")
	t.Equal(gaugeValue(gauge, "idle"), 0.0, "previous state idle is now 0")
	t.Equal(gaugeValue(gauge, "active"), 1.0, "new state active is now 1")

	s.Set("busy")
	t.Equal(gaugeValue(gauge, "active"), 0.0, "previous state active is now 0")
	t.Equal(gaugeValue(gauge, "busy"), 1.0, "new state busy is now 1")

	// Transition back to a previously active state.
	s.Set("active")
	t.Equal(gaugeValue(gauge, "busy"), 0.0, "previous state busy is now 0")
	t.Equal(gaugeValue(gauge, "active"), 1.0, "state active is 1 again")

	s.Remove()
}

func TestState_Remove(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()
	gauge := gaugeWithStates("test_remove",
		[]promx.LabelState{"starting", "stopped"})

	s := promx.NewState(gauge, "starting", nil)
	t.Equal(gaugeValue(gauge, "starting"), 1.0)

	s.Remove()
	t.Equal(gaugeValue(gauge, "starting"), 0.0, "gauge is zeroed after Remove")
}

func TestNewState_with_labels(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_extra_labels",
		Help: "Test gauge with extra labels",
	}, []string{"kind", promx.StateLabel})

	states := []promx.LabelState{"starting", "running"}
	for _, s := range states {
		gauge.With(prometheus.Labels{
			"kind":           "worker",
			promx.StateLabel: string(s),
		})
	}

	s := promx.NewState(gauge, "starting", prometheus.Labels{"kind": "worker"})

	t.Equal(
		testutil.ToFloat64(gauge.With(prometheus.Labels{
			"kind": "worker", promx.StateLabel: "starting",
		})),
		1.0,
	)

	s.Set("running")
	t.Equal(
		testutil.ToFloat64(gauge.With(prometheus.Labels{
			"kind": "worker", promx.StateLabel: "starting",
		})),
		0.0,
		"previous state is zeroed",
	)
	t.Equal(
		testutil.ToFloat64(gauge.With(prometheus.Labels{
			"kind": "worker", promx.StateLabel: "running",
		})),
		1.0,
		"new state is active",
	)

	s.Remove()
}

func TestState_Set_multiple_instances(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()
	gauge := gaugeWithStates("test_multi",
		[]promx.LabelState{"idle", "active"})

	s1 := promx.NewState(gauge, "idle", nil)
	s2 := promx.NewState(gauge, "active", nil)

	// Two independent state trackers, each with its own state.
	t.Equal(gaugeValue(gauge, "idle"), 1.0)
	t.Equal(gaugeValue(gauge, "active"), 1.0, "second tracker also holds active=1")

	s1.Set("active")
	t.Equal(gaugeValue(gauge, "idle"), 0.0, "s1 moved from idle to active")
	t.Equal(gaugeValue(gauge, "active"), 2.0, "both trackers now on active -> count is 2")

	s1.Remove()
	t.Equal(gaugeValue(gauge, "active"), 1.0, "s1 removed, active back to 1")

	s2.Remove()
	t.Equal(gaugeValue(gauge, "active"), 0.0, "s2 removed, active is 0")
}

func TestState_Set_concurrent(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()
	gauge := gaugeWithStates("test_concurrent",
		[]promx.LabelState{"a", "b", "c"})

	s := promx.NewState(gauge, "a", nil)

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			s.Set("b")
			s.Set("c")
			s.Set("a")
		})
	}
	wg.Wait()

	// After all goroutines finish, exactly one state series is 1.
	sum := gaugeValue(gauge, "a") +
		gaugeValue(gauge, "b") +
		gaugeValue(gauge, "c")
	t.Equal(sum, 1.0, "no matter how many concurrent transitions, exactly one state is active")

	// Final state after all Set("a") calls.
	t.Equal(gaugeValue(gauge, "a"), 1.0)

	s.Remove()
}

func TestState_Remove_panic(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	gauge := gaugeWithStates("test_panic_after_remove",
		[]promx.LabelState{"start"})
	s := promx.NewState(gauge, "start", nil)
	s.Remove()

	// Calling Set after Remove panics because s.metric is nil.
	t.PanicMatch(func() {
		s.Set("start")
	}, `nil pointer dereference`)

	// Calling Remove again also panics.
	t.PanicMatch(func() {
		s.Remove()
	}, `nil pointer dereference`)
}
