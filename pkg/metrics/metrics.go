// Package metrics provides a minimal Prometheus-compatible text-format metrics
// registry with zero external dependencies.
package metrics

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// DefaultRegistry is the global default registry used by the gateway.
var DefaultRegistry = NewRegistry()

// Registry holds all registered metrics.
type Registry struct {
	mu      sync.RWMutex
	metrics []metric
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry { return &Registry{} }

type metric interface{ writeTo(w io.Writer) }

func (r *Registry) register(m metric) {
	r.mu.Lock()
	r.metrics = append(r.metrics, m)
	r.mu.Unlock()
}

// WriteText writes all metrics in Prometheus text exposition format.
func (r *Registry) WriteText(w io.Writer) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.metrics {
		m.writeTo(w)
	}
}

// --- Counter ------------------------------------------------------------------

// Counter is a monotonically increasing labeled counter.
type Counter struct {
	name   string
	help   string
	labels []string
	mu     sync.RWMutex
	values map[string]*atomic.Int64
}

// NewCounter registers a Counter in the DefaultRegistry.
func NewCounter(name, help string, labelNames ...string) *Counter {
	return DefaultRegistry.NewCounter(name, help, labelNames...)
}

// NewCounter registers a Counter in r.
func (r *Registry) NewCounter(name, help string, labelNames ...string) *Counter {
	c := &Counter{name: name, help: help, labels: labelNames, values: make(map[string]*atomic.Int64)}
	r.register(c)
	return c
}

// Inc increments the counter for the given label values by 1.
func (c *Counter) Inc(labelVals ...string) {
	key := strings.Join(labelVals, "\x00")
	c.mu.RLock()
	v, ok := c.values[key]
	c.mu.RUnlock()
	if !ok {
		c.mu.Lock()
		if v, ok = c.values[key]; !ok {
			v = &atomic.Int64{}
			c.values[key] = v
		}
		c.mu.Unlock()
	}
	v.Add(1)
}

func (c *Counter) writeTo(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n", c.name, c.help)
	fmt.Fprintf(w, "# TYPE %s counter\n", c.name)
	for _, key := range c.sortedKeys() {
		c.mu.RLock()
		v := c.values[key]
		c.mu.RUnlock()
		fmt.Fprintf(w, "%s%s %d\n", c.name, buildLabelStr(c.labels, strings.Split(key, "\x00")), v.Load())
	}
}

func (c *Counter) sortedKeys() []string {
	c.mu.RLock()
	keys := make([]string, 0, len(c.values))
	for k := range c.values {
		keys = append(keys, k)
	}
	c.mu.RUnlock()
	sort.Strings(keys)
	return keys
}

// --- Gauge --------------------------------------------------------------------

// Gauge is a labeled metric that can go up and down.
type Gauge struct {
	name   string
	help   string
	labels []string
	mu     sync.RWMutex
	values map[string]*atomic.Int64
}

// NewGauge registers a Gauge in the DefaultRegistry.
func NewGauge(name, help string, labelNames ...string) *Gauge {
	return DefaultRegistry.NewGauge(name, help, labelNames...)
}

// NewGauge registers a Gauge in r.
func (r *Registry) NewGauge(name, help string, labelNames ...string) *Gauge {
	g := &Gauge{name: name, help: help, labels: labelNames, values: make(map[string]*atomic.Int64)}
	r.register(g)
	return g
}

func (g *Gauge) getOrCreate(key string) *atomic.Int64 {
	g.mu.RLock()
	v, ok := g.values[key]
	g.mu.RUnlock()
	if !ok {
		g.mu.Lock()
		if v, ok = g.values[key]; !ok {
			v = &atomic.Int64{}
			g.values[key] = v
		}
		g.mu.Unlock()
	}
	return v
}

// Inc increments the gauge by 1.
func (g *Gauge) Inc(labelVals ...string) {
	g.getOrCreate(strings.Join(labelVals, "\x00")).Add(1)
}

// Dec decrements the gauge by 1.
func (g *Gauge) Dec(labelVals ...string) {
	g.getOrCreate(strings.Join(labelVals, "\x00")).Add(-1)
}

// Set sets the gauge to an absolute value.
func (g *Gauge) Set(val int64, labelVals ...string) {
	v := g.getOrCreate(strings.Join(labelVals, "\x00"))
	for {
		old := v.Load()
		if v.CompareAndSwap(old, val) {
			break
		}
	}
}

func (g *Gauge) writeTo(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n", g.name, g.help)
	fmt.Fprintf(w, "# TYPE %s gauge\n", g.name)
	g.mu.RLock()
	keys := make([]string, 0, len(g.values))
	for k := range g.values {
		keys = append(keys, k)
	}
	g.mu.RUnlock()
	sort.Strings(keys)
	for _, key := range keys {
		g.mu.RLock()
		v := g.values[key]
		g.mu.RUnlock()
		fmt.Fprintf(w, "%s%s %d\n", g.name, buildLabelStr(g.labels, strings.Split(key, "\x00")), v.Load())
	}
}

// --- Histogram ----------------------------------------------------------------

var defaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// Histogram tracks the distribution of observed values using fixed buckets.
type Histogram struct {
	name    string
	help    string
	labels  []string
	buckets []float64
	mu      sync.RWMutex
	series  map[string]*histSeries
}

type histSeries struct {
	buckets []float64
	counts  []atomic.Int64 // index i covers <=buckets[i]; last = +Inf
	sumMicro atomic.Int64  // sum stored as microseconds for int64 safety
	total   atomic.Int64
}

func newHistSeries(buckets []float64) *histSeries {
	return &histSeries{buckets: buckets, counts: make([]atomic.Int64, len(buckets)+1)}
}

func (s *histSeries) observe(v float64) {
	s.total.Add(1)
	s.sumMicro.Add(int64(v * 1e6))
	for i, b := range s.buckets {
		if v <= b {
			s.counts[i].Add(1)
			return
		}
	}
	s.counts[len(s.buckets)].Add(1)
}

// NewHistogram registers a Histogram with default buckets in the DefaultRegistry.
func NewHistogram(name, help string, labelNames ...string) *Histogram {
	return DefaultRegistry.NewHistogram(name, help, labelNames...)
}

// NewHistogram registers a Histogram with default buckets in r.
func (r *Registry) NewHistogram(name, help string, labelNames ...string) *Histogram {
	h := &Histogram{
		name:    name,
		help:    help,
		labels:  labelNames,
		buckets: defaultBuckets,
		series:  make(map[string]*histSeries),
	}
	r.register(h)
	return h
}

// Observe records a value (typically a duration in seconds).
func (h *Histogram) Observe(val float64, labelVals ...string) {
	key := strings.Join(labelVals, "\x00")
	h.mu.RLock()
	s, ok := h.series[key]
	h.mu.RUnlock()
	if !ok {
		h.mu.Lock()
		if s, ok = h.series[key]; !ok {
			s = newHistSeries(h.buckets)
			h.series[key] = s
		}
		h.mu.Unlock()
	}
	s.observe(val)
}

func (h *Histogram) writeTo(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n", h.name, h.help)
	fmt.Fprintf(w, "# TYPE %s histogram\n", h.name)
	h.mu.RLock()
	keys := make([]string, 0, len(h.series))
	for k := range h.series {
		keys = append(keys, k)
	}
	h.mu.RUnlock()
	sort.Strings(keys)

	for _, key := range keys {
		h.mu.RLock()
		s := h.series[key]
		h.mu.RUnlock()
		parts := strings.Split(key, "\x00")
		base := buildLabelStr(h.labels, parts)

		var cum int64
		for i, b := range h.buckets {
			cum += s.counts[i].Load()
			leLabel := buildLabelStrWith(h.labels, parts, "le", formatFloat(b))
			fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, leLabel, cum)
		}
		cum += s.counts[len(h.buckets)].Load()
		fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, buildLabelStrWith(h.labels, parts, "le", "+Inf"), cum)
		fmt.Fprintf(w, "%s_sum%s %g\n", h.name, base, float64(s.sumMicro.Load())/1e6)
		fmt.Fprintf(w, "%s_count%s %d\n", h.name, base, s.total.Load())
	}
}

// --- helpers ------------------------------------------------------------------

func buildLabelStr(names, vals []string) string {
	if len(names) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteByte('{')
	for i, n := range names {
		if i > 0 {
			sb.WriteByte(',')
		}
		v := ""
		if i < len(vals) {
			v = vals[i]
		}
		fmt.Fprintf(&sb, `%s=%q`, n, v)
	}
	sb.WriteByte('}')
	return sb.String()
}

func buildLabelStrWith(names, vals []string, extraKey, extraVal string) string {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, n := range names {
		if i > 0 {
			sb.WriteByte(',')
		}
		v := ""
		if i < len(vals) {
			v = vals[i]
		}
		fmt.Fprintf(&sb, `%s=%q`, n, v)
	}
	if len(names) > 0 {
		sb.WriteByte(',')
	}
	fmt.Fprintf(&sb, `%s=%q`, extraKey, extraVal)
	sb.WriteByte('}')
	return sb.String()
}

func formatFloat(f float64) string {
	if f == math.Trunc(f) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%g", f)
}
