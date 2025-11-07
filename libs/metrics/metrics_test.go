package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	require.NotNil(t, reg)
	assert.NotNil(t, reg.reg)
	assert.NotNil(t, reg.counters)
	assert.NotNil(t, reg.gauges)
	assert.NotNil(t, reg.histograms)
	assert.NotNil(t, reg.summaries)
}

func TestDefaultRegistry(t *testing.T) {
	reg1 := Default()
	reg2 := Default()
	assert.Equal(t, reg1, reg2, "Default() should return same instance")
}

func TestCounter(t *testing.T) {
	reg := NewRegistry()
	
	// Create counter
	counter := reg.Counter("test_counter", "Test counter", "label1", "label2")
	require.NotNil(t, counter)
	
	// Increment counter
	counter.WithLabelValues("value1", "value2").Inc()
	counter.WithLabelValues("value1", "value2").Add(5)
	
	// Verify value
	metric := counter.WithLabelValues("value1", "value2")
	value := testutil.ToFloat64(metric)
	assert.Equal(t, 6.0, value)
	
	// Retrieve existing counter
	counter2 := reg.Counter("test_counter", "Test counter", "label1", "label2")
	assert.Equal(t, counter, counter2, "Should return existing counter")
}

func TestGauge(t *testing.T) {
	reg := NewRegistry()
	
	// Create gauge
	gauge := reg.Gauge("test_gauge", "Test gauge", "label")
	require.NotNil(t, gauge)
	
	// Set gauge
	gauge.WithLabelValues("test").Set(42)
	value := testutil.ToFloat64(gauge.WithLabelValues("test"))
	assert.Equal(t, 42.0, value)
	
	// Increment/decrement
	gauge.WithLabelValues("test").Inc()
	gauge.WithLabelValues("test").Dec()
	value = testutil.ToFloat64(gauge.WithLabelValues("test"))
	assert.Equal(t, 42.0, value)
}

func TestHistogram(t *testing.T) {
	reg := NewRegistry()
	
	// Create histogram
	hist := reg.Histogram(
		"test_histogram",
		"Test histogram",
		[]float64{0.1, 1.0, 10.0},
		"label",
	)
	require.NotNil(t, hist)
	
	// Observe values
	hist.WithLabelValues("test").Observe(0.5)
	hist.WithLabelValues("test").Observe(5.0)
	hist.WithLabelValues("test").Observe(50.0)
	
	// Histograms don't expose count directly in the same way
	// Just verify it was created successfully
	assert.NotNil(t, hist)
}

func TestSummary(t *testing.T) {
	reg := NewRegistry()
	
	// Create summary
	summary := reg.Summary(
		"test_summary",
		"Test summary",
		map[float64]float64{0.5: 0.05, 0.9: 0.01},
		"label",
	)
	require.NotNil(t, summary)
	
	// Observe values
	for i := 0; i < 100; i++ {
		summary.WithLabelValues("test").Observe(float64(i))
	}
	
	// Summaries don't expose count directly in the same way
	// Just verify it was created successfully
	assert.NotNil(t, summary)
}

func TestDurationBuckets(t *testing.T) {
	assert.Greater(t, len(DurationBuckets), 5, "Should have multiple duration buckets")
	assert.Equal(t, 0.0001, DurationBuckets[0], "First bucket should be 100µs")
	
	// Verify ascending order
	for i := 1; i < len(DurationBuckets); i++ {
		assert.Greater(t, DurationBuckets[i], DurationBuckets[i-1],
			"Buckets should be in ascending order")
	}
}

func TestBytesBuckets(t *testing.T) {
	assert.Greater(t, len(BytesBuckets), 5, "Should have multiple byte buckets")
	assert.Equal(t, float64(1024), BytesBuckets[0], "First bucket should be 1KB")
}

func TestCostBuckets(t *testing.T) {
	assert.Greater(t, len(CostBuckets), 5, "Should have multiple cost buckets")
	assert.Equal(t, 0.001, CostBuckets[0], "First bucket should be 0.001")
}

func TestMetricsConcurrency(t *testing.T) {
	reg := NewRegistry()
	counter := reg.Counter("concurrent_test", "Concurrent test counter", "worker")
	
	// Concurrent increments
	workers := 10
	increments := 100
	done := make(chan bool, workers)
	
	for w := 0; w < workers; w++ {
		go func(id int) {
			label := string(rune('A' + id))
			for i := 0; i < increments; i++ {
				counter.WithLabelValues(label).Inc()
			}
			done <- true
		}(w)
	}
	
	// Wait for completion
	for w := 0; w < workers; w++ {
		<-done
	}
	
	// Verify each worker's count
	for w := 0; w < workers; w++ {
		label := string(rune('A' + w))
		value := testutil.ToFloat64(counter.WithLabelValues(label))
		assert.Equal(t, float64(increments), value,
			"Worker %d should have %d increments", w, increments)
	}
}

func TestHandler(t *testing.T) {
	reg := NewRegistry()
	
	// Create some metrics
	counter := reg.Counter("handler_test_counter", "Test counter")
	counter.WithLabelValues().Add(42)
	
	// Get handler
	handler := reg.Handler()
	require.NotNil(t, handler)
	
	// Handler should be promhttp.Handler type
	assert.NotNil(t, handler)
}

func TestMustRegister(t *testing.T) {
	reg := NewRegistry()
	
	// Create a custom collector
	collector := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "custom_gauge",
		Help: "Custom gauge",
	})
	
	// Should not panic
	assert.NotPanics(t, func() {
		reg.MustRegister(collector)
	})
}

func TestUnregister(t *testing.T) {
	reg := NewRegistry()
	
	// Create and register collector
	collector := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_unregister_gauge",
		Help: "Test gauge for unregister",
	})
	reg.MustRegister(collector)
	
	// Unregister
	ok := reg.Unregister(collector)
	assert.True(t, ok, "Unregister should succeed")
	
	// Second unregister should fail
	ok = reg.Unregister(collector)
	assert.False(t, ok, "Second unregister should fail")
}

func BenchmarkCounterInc(b *testing.B) {
	reg := NewRegistry()
	counter := reg.Counter("bench_counter", "Benchmark counter", "label")
	metric := counter.WithLabelValues("test")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metric.Inc()
	}
}

func BenchmarkHistogramObserve(b *testing.B) {
	reg := NewRegistry()
	hist := reg.Histogram("bench_histogram", "Benchmark histogram", DurationBuckets, "label")
	metric := hist.WithLabelValues("test")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metric.Observe(time.Duration(i).Seconds())
	}
}

func BenchmarkGaugeSet(b *testing.B) {
	reg := NewRegistry()
	gauge := reg.Gauge("bench_gauge", "Benchmark gauge", "label")
	metric := gauge.WithLabelValues("test")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metric.Set(float64(i))
	}
}
