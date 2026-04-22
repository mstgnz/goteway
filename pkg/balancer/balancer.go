package balancer

import (
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

// RoundRobin distributes requests across multiple backend targets using
// an atomic counter so it is safe for concurrent use.
type RoundRobin struct {
	targets []*url.URL
	proxies []*httputil.ReverseProxy
	counter atomic.Uint64
}

// New creates a RoundRobin balancer from a list of target URLs.
// Each target gets its own pre-built ReverseProxy.
func New(targets []*url.URL) *RoundRobin {
	proxies := make([]*httputil.ReverseProxy, len(targets))
	for i, t := range targets {
		proxies[i] = httputil.NewSingleHostReverseProxy(t)
	}
	return &RoundRobin{targets: targets, proxies: proxies}
}

// Next returns the next proxy and its target URL in round-robin order.
func (rb *RoundRobin) Next() (*httputil.ReverseProxy, *url.URL) {
	n := rb.counter.Add(1) - 1
	i := n % uint64(len(rb.proxies))
	return rb.proxies[i], rb.targets[i]
}

// Len returns the number of backends.
func (rb *RoundRobin) Len() int {
	return len(rb.targets)
}
