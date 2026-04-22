package balancer

import (
	"net/url"
	"testing"
)

func mustURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func TestRoundRobinDistribution(t *testing.T) {
	targets := []*url.URL{
		mustURL("http://backend1:8081"),
		mustURL("http://backend2:8082"),
		mustURL("http://backend3:8083"),
	}
	rb := New(targets)

	if rb.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", rb.Len())
	}

	counts := make(map[string]int)
	const total = 30
	for i := 0; i < total; i++ {
		_, target := rb.Next()
		counts[target.Host]++
	}

	// Each backend should receive exactly total/3 requests
	for _, host := range []string{"backend1:8081", "backend2:8082", "backend3:8083"} {
		if counts[host] != total/3 {
			t.Errorf("backend %s got %d requests, want %d", host, counts[host], total/3)
		}
	}
}

func TestRoundRobinSingleTarget(t *testing.T) {
	target := mustURL("http://backend:8080")
	rb := New([]*url.URL{target})

	for i := 0; i < 5; i++ {
		_, got := rb.Next()
		if got.Host != "backend:8080" {
			t.Errorf("Next().Host = %q, want %q", got.Host, "backend:8080")
		}
	}
}

func TestRoundRobinProxiesNotNil(t *testing.T) {
	rb := New([]*url.URL{mustURL("http://backend:8080")})
	proxy, _ := rb.Next()
	if proxy == nil {
		t.Error("Next() returned nil proxy")
	}
}
