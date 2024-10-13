package httpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

var (
	// ErrPanicRecovered indicates that a panic was recovered.
	//
	// Use the [PanicError] type to retrieve stack trace information.
	ErrPanicRecovered = errors.New("panic recovered")

	// ErrInsecureScheme indicates that the scheme was not HTTPS.
	ErrInsecureScheme = errors.New("insecure scheme")
)

// PanicError is an error type that wraps a recovered panic value and the stack.
// It is returned by the [Recover] middleware.
type PanicError struct {
	Recovered any    // The recovered panic value.
	Stack     []byte // The stack trace at the point of panic recovery.
}

// Error returns a string representation of the error with the recovered value.
func (e *PanicError) Error() string {
	return fmt.Sprintf("%v: %v", ErrPanicRecovered, e.Recovered)
}

// Is compares the [ErrPanicRecovered] with the target error.
func (e *PanicError) Is(target error) bool {
	return errors.Is(target, ErrPanicRecovered)
}

// Recover is a middleware that recovers from panics in the HTTP request chain
// and returns a [PanicError] as the error.
func Recover() MiddlewareFunc {
	return func(next DoerFunc) DoerFunc {
		return func(r *http.Request) (resp *http.Response, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = &PanicError{
						Recovered: r,
						Stack:     debug.Stack(),
					}
				}
			}()

			return next(r)
		}
	}
}

// StripSlashes cleans the request URL path from multiple "/" slashes.
//
// If stripTrailing is true, it also removes trailing slashes from the path,
// except when the path is just "/".
func StripSlashes(stripTrailing bool) MiddlewareFunc {
	return func(next DoerFunc) DoerFunc {
		return func(r *http.Request) (*http.Response, error) {
			r.URL = r.URL.JoinPath()
			if stripTrailing &&
				len(r.URL.Path) > 1 && strings.HasSuffix(r.URL.Path, "/") {
				r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
			}

			return next(r)
		}
	}
}

// Secure returns an error for requests made to a non-HTTPS URL.
//
// It is ideally used in [http.RoundTripper] to intercept any redirects.
func Secure() MiddlewareFunc {
	return func(next DoerFunc) DoerFunc {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Scheme != "https" {
				return nil, ErrInsecureScheme
			}

			return next.RoundTrip(r)
		})
	}
}

// Timeout adds a timeout to the client requests.
//
// The middleware is not applied if the timeout is below or equal to zero.
func Timeout(d time.Duration) MiddlewareFunc {
	if d <= 0 {
		return nil
	}

	return func(next DoerFunc) DoerFunc {
		return func(r *http.Request) (*http.Response, error) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			r = r.WithContext(ctx)
			return next(r)
		}
	}
}

// UserAgent sets the User-Agent header for client requests.
//
// By default, Go's standard HTTP client (http.Client) adds a User-Agent header
// with a value like "Go-http-client/1.1" to each request.
// Providing an empty string will prevent sending the User-Agent header.
func UserAgent(v string) MiddlewareFunc {
	return SetHeader("User-Agent", v)
}

// Accept sets the Accept header for client requests.
//
// The Accept header informs the server about acceptable media types for the
// response, specifying the format expected by the client.
func Accept(v string) MiddlewareFunc {
	return SetHeader("Accept", v)
}

// ContentType sets the Content-Type header for client requests.
//
// The Content-Type header specifies the media type of the request payload or
// response body, indicating the format of the data being sent or received.
func ContentType(v string) MiddlewareFunc {
	return SetHeader("Content-Type", v)
}

// Authorization sets the Authorization header for the client requests.
//
// The Authorization header contains credentials that identify the client
// making the request. It is commonly used for authentication purposes, such as
// sending an access token to access protected resources.
func Authorization(v string) MiddlewareFunc {
	return SetHeader("Authorization", v)
}

// AuthorizationBearer sets the Bearer token in the Authorization header for
// client requests.
//
// The Bearer token is an access token used in OAuth 2.0 authentication,
// granting access to protected resources on behalf of an authenticated user.
func AuthorizationBearer(v string) MiddlewareFunc {
	if v == "" {
		return nil
	}

	return SetHeader("Authorization", "Bearer "+v)
}

// AuthorizationBasic is a middleware that adds Basic Authentication headers to
// client requests.
//
// It expects a username and password and adds an Authorization header with the
// encoded credentials in the format "Basic base64(user:pass)".
func AuthorizationBasic(user, pass string) MiddlewareFunc {
	return func(next DoerFunc) DoerFunc {
		return func(r *http.Request) (*http.Response, error) {
			r.SetBasicAuth(user, pass)
			return next(r)
		}
	}
}

// SetHeader sets a request header.
//
// If the key is empty the header is not set.
func SetHeader(k, v string) MiddlewareFunc {
	if k == "" {
		return nil
	}

	return func(next DoerFunc) DoerFunc {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			r.Header.Set(k, v)
			return next(r)
		})
	}
}
