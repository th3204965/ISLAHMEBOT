package httpclient

import (
	"net/http"
	"time"
)

// Shared is a globally pooled HTTP client optimized for high concurrency.
// It maintains keep-alive TCP connections across multiple API calls,
// virtually eliminating the 150-300ms TLS handshake overhead per request.
var Shared = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	},
}
