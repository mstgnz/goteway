package metrics

import (
	"strings"
	"testing"
)

func TestCounterInc(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("req_total", "help", "method")

	c.Inc("GET")
	c.Inc("GET")
	c.Inc("POST")

	var sb strings.Builder
	r.WriteText(&sb)
	out := sb.String()

	if !strings.Contains(out, `req_total{method="GET"} 2`) {
		t.Errorf("expected GET=2 in output:\n%s", out)
	}
	if !strings.Contains(out, `req_total{method="POST"} 1`) {
		t.Errorf("expected POST=1 in output:\n%s", out)
	}
	if !strings.Contains(out, "# TYPE req_total counter") {
		t.Errorf("missing TYPE line in output:\n%s", out)
	}
}

func TestGaugeIncDec(t *testing.T) {
	r := NewRegistry()
	g := r.NewGauge("active", "help", "route")

	g.Inc("/api")
	g.Inc("/api")
	g.Dec("/api")

	var sb strings.Builder
	r.WriteText(&sb)
	if !strings.Contains(sb.String(), `active{route="/api"} 1`) {
		t.Errorf("unexpected output:\n%s", sb.String())
	}
}

func TestGaugeSet(t *testing.T) {
	r := NewRegistry()
	g := r.NewGauge("connections", "help", "host")
	g.Set(42, "backend1")

	var sb strings.Builder
	r.WriteText(&sb)
	if !strings.Contains(sb.String(), `connections{host="backend1"} 42`) {
		t.Errorf("unexpected output:\n%s", sb.String())
	}
}

func TestHistogramObserve(t *testing.T) {
	r := NewRegistry()
	h := &Histogram{
		name:    "duration",
		help:    "help",
		labels:  []string{"route"},
		buckets: []float64{0.1, 0.5, 1.0},
		series:  make(map[string]*histSeries),
	}
	r.register(h)

	h.Observe(0.05, "/api") // ≤0.1 bucket
	h.Observe(0.3, "/api")  // ≤0.5 bucket
	h.Observe(2.0, "/api")  // +Inf bucket

	var sb strings.Builder
	r.WriteText(&sb)
	out := sb.String()

	if !strings.Contains(out, `duration_count{route="/api"} 3`) {
		t.Errorf("count wrong:\n%s", out)
	}
	if !strings.Contains(out, `duration_bucket{route="/api",le="0.1"} 1`) {
		t.Errorf("bucket 0.1 wrong:\n%s", out)
	}
	if !strings.Contains(out, `duration_bucket{route="/api",le="+Inf"} 3`) {
		t.Errorf("+Inf bucket wrong:\n%s", out)
	}
}

func TestBuildLabelStr(t *testing.T) {
	got := buildLabelStr([]string{"a", "b"}, []string{"x", "y"})
	if got != `{a="x",b="y"}` {
		t.Errorf("got %q", got)
	}
	if buildLabelStr(nil, nil) != "" {
		t.Error("empty labels should return empty string")
	}
}

func TestFormatFloat(t *testing.T) {
	cases := []struct{ in float64; want string }{{1.0, "1"}, {0.5, "0.5"}, {0.025, "0.025"}}
	for _, c := range cases {
		if got := formatFloat(c.in); got != c.want {
			t.Errorf("formatFloat(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
