// Package httpc provides utilities for enhancing HTTP clients with middleware
// support.
//
// The package defines the [Doer] interface, specifying HTTP client behavior
// and includes [DoerFunc], an adapter type for functions with the signature:
//
//	func(*http.Request) (*http.Response, error).
//
// It also offers various built-in
// [MiddlewareFunc] mechanisms to extend client functionality.
//
// Example usage with [httpc.Client]:
//
//	client := httpc.NewClient(http.DefaultClient, middlewares...)
//	req, _ := http.NewRequest(http.MethodGet, "http://localhost", http.NoBody)
//	resp, _ := client.Do(req)
//
// Example usage with [httpc.RoundTripper]:
//
//	tr := httpc.NewRoundTripper(http.DefaultTransport, middlewares...)
//	client := &http.Client{Transport: tr}
//	req, _ := http.NewRequest(http.MethodGet, "http://localhost", http.NoBody)
//	resp, _ := client.Do(req)
//
// These utilities simplify HTTP client interactions, allowing flexible request
// and response handling with middleware extensibility.
//
// Note: httpc is not intended as a comprehensive request builder but focuses on
// client-side middleware support, enabling customizations such as logging all
// requests made within an application.
package httpc

import (
	"net/http"
	"slices"
)

// Doer defines the interface for performing HTTP requests, similar to the
// standard [http.Client].
//
// It requires implementing a method that performs an HTTP request and returns
// the response or an error.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// DoerFunc is an adapter function that allows a function with the signature:
//
//	func(*http.Request) (*http.Response, error)
//
// to be used as a [Doer] or the [http.RoundTripper].
//
// It is analogous to [http.HandlerFunc] in the net/http package but for the
// client-side of HTTP interaction.
//
// The function receives an [http.Request] and returns an [http.Response] and
// an error. Implementations of this function can perform arbitrary processing
// on the request and return a corresponding response or an error if an issue
// occurs.
//
// Example usage in a middleware:
//
//	func LogRequest(log *slog.Logger) httpc.MiddlewareFunc {
//		return func(next httpc.DoerFunc) httpc.DoerFunc {
//			return func(r *http.Request) (*http.Response, error) {
//				start := time.Now()
//				resp, err := next(r)
//				if err != nil {
//					return resp, err
//				}
//
//				log.InfoContext(r.Context(), "Request sent",
//					slog.String("url", r.URL.String()),
//					slog.Int("code", resp.StatusCode),
//					slog.Duration("took", time.Since(start)),
//				)
//				return resp, err
//			}
//		}
//	}
//
// DoerFunc implements both the [Doer] and [http.RoundTripper] interfaces,
// ensuring compatibility with both client and transport layers.
//
// Some libraries may only allow setting an http.Client transport, while others
// may accept an interface compatible with the client Do method, providing
// flexibility in usage across different HTTP client implementations.
type DoerFunc func(*http.Request) (*http.Response, error)

// Do implements [Doer].
func (f DoerFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

// RoundTrip implements [http.RoundTripper].
func (f DoerFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// RoundTripper can wrap a [http.RoundTripper] with middleware.
type RoundTripper struct {
	base        http.RoundTripper
	chain       DoerFunc
	middlewares []MiddlewareFunc
}

// NewRoundTripper returns a new middleware round tripper.
func NewRoundTripper(rt http.RoundTripper, mws ...MiddlewareFunc) *RoundTripper {
	return &RoundTripper{
		base:        rt,
		chain:       applyMiddlewares(rt.RoundTrip, mws...),
		middlewares: mws,
	}
}

// RoundTrip implements [http.RoundTripper] for [http.Client]'s Transport.
func (t *RoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.chain.RoundTrip(r)
}

// With creates a new [RoundTripper] with additional middlewares.
//
// The new [RoundTripper] inherits the existing middlewares and appends the
// provided ones. For example, if the existing middlewares are (p1, p2, p3)
// and the new ones are (n1, n2), the resulting chain will be
// (p1, p2, p3, n1, n2).
func (t *RoundTripper) With(mws ...MiddlewareFunc) *RoundTripper {
	return NewRoundTripper(t.base, slices.Concat(t.middlewares, mws)...)
}

// MiddlewareFunc is a function that wraps a [DoerFunc].
// It is used in [Client] and [RoundTripper] as a client-side middleware.
//
// MiddlewareFunc can provide additional functionality and modifications,
// allowing for custom processing or behavior to be applied to HTTP requests.
type MiddlewareFunc func(next DoerFunc) DoerFunc

// Client represents an HTTP client with middleware support.
//
// This client enables custom processing and behavior to be applied to HTTP
// requests through middleware functions. Middleware can be added during
// construction or via methods like With to modify the client's behavior.
type Client struct {
	base        Doer
	chain       Doer
	middlewares []MiddlewareFunc
}

// NewClient creates a new [Client] with the provided middlewares.
//
// It initializes a new HTTP client with the specified [Doer] for performing
// HTTP requests and applies the given [MiddlewareFunc] middlewares to
// customize its behavior. The resulting client incorporates these middlewares
// into its request processing pipeline.
func NewClient(doer Doer, mws ...MiddlewareFunc) *Client {
	return &Client{
		base:        doer,
		chain:       applyMiddlewares(doer.Do, mws...),
		middlewares: mws,
	}
}

// Do performs an HTTP request using the client's middleware-enhanced request
// execution chain.
//
// It forwards the provided HTTP request through the client's middleware chain
// to handle custom processing and behavior before sending the request.
func (c *Client) Do(r *http.Request) (*http.Response, error) {
	return c.chain.Do(r)
}

// With creates a new client that includes additional middlewares.
//
// The new client will inherit the middlewares from the existing client and
// append the provided ones. For example, if the existing client has the
// middleware chain (p1, p2, p3) and the provided middlewares are (n1, n2),
// the resulting chain will be: (p1, p2, p3, n1, n2).
func (c *Client) With(mws ...MiddlewareFunc) *Client {
	return NewClient(c.base, slices.Concat(c.middlewares, mws)...)
}

// applyMiddlewares constructs a middleware chain that executes in the order
// the middlewares were passed. It iterates through the middlewares in reverse
// order, so that the first middleware in the slice is the first to execute.
//
// The constructed chain will process each middleware up to the base doer.
func applyMiddlewares(do DoerFunc, mws ...MiddlewareFunc) DoerFunc {
	chain := do
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] == nil {
			continue
		}

		chain = mws[i](chain)
	}

	return chain
}
