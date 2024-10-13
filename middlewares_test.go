package httpc_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kraciasty/httpc"
	"github.com/kraciasty/httpc/internal/httpctest"
)

var stubResponse = []byte(`HTTP/1.1 200 OK

Some stubbed stuff.`)

func ExampleRecover() {
	c := httpc.NewClient(
		httpc.DoerFunc(func(*http.Request) (*http.Response, error) {
			panic("some panic")
		}),
		httpc.Recover(),
	)
	_, err := c.Do(&http.Request{})
	var panicErr *httpc.PanicError
	if errors.As(err, &panicErr) {
		fmt.Println(panicErr.Recovered)
	}
	// Output: some panic
}

func TestRecover(t *testing.T) {
	const panicMsg = "something went wrong"
	base := httpc.DoerFunc(func(*http.Request) (*http.Response, error) {
		panic(panicMsg)
	})

	c := httpc.NewClient(base, httpc.Recover())
	_, err := c.Do(&http.Request{})
	if err == nil {
		t.Fatalf("expected an error but got nil")
	}

	if !errors.Is(err, httpc.ErrPanicRecovered) {
		t.Error("expected httpc.ErrPanicRecovered", httpc.ErrPanicRecovered)
	}

	var panicErr *httpc.PanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("expected error to be of type *httpc.PanicError")
	}

	if !strings.Contains(panicErr.Error(), panicMsg) {
		t.Errorf("error message %q does not contain %q", panicErr.Error(), panicMsg)
	}

	if !strings.Contains(panicErr.Error(), httpc.ErrPanicRecovered.Error()) {
		t.Errorf("error message %q does not contain %q", panicErr.Error(), httpc.ErrPanicRecovered.Error())
	}

	if panicErr.Recovered != panicMsg {
		t.Errorf("expected recovered value %q, got %q", panicMsg, panicErr.Recovered)
	}

	if len(panicErr.Stack) == 0 {
		t.Error("expected stack trace to be non-empty")
	}
}

func TestTimeout(t *testing.T) {
	// timeoutHandler is a custom test handler that introduces an artificial delay.
	timeoutHandler := func(delay time.Duration) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-time.After(delay)
			w.WriteHeader(http.StatusOK)
		})
	}

	tests := []struct {
		name    string
		delay   time.Duration
		timeout time.Duration
		err     error
	}{
		{
			name:    "without timeout",
			delay:   0,
			timeout: 0,
		},
		{
			name:    "deadline exceeded",
			delay:   time.Hour,
			timeout: 1,
			err:     context.DeadlineExceeded,
		},
		{
			name:    "within timeout",
			delay:   0,
			timeout: time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := setupServer(t, timeoutHandler(tt.delay))
			mw := httpc.Timeout(tt.timeout)
			req, err := http.NewRequest(http.MethodGet, srv.URL, http.NoBody)
			if err != nil {
				t.Fatalf("Cannot create request: %v", err)
			}

			start := time.Now()
			client := httpc.NewClient(srv.Client(), mw)
			resp, err := client.Do(req)
			if err != nil {
				if tt.err != nil {
					if !errors.Is(err, tt.err) {
						t.Fatalf("Expected an error but got: %v", err)
					}
					return
				}

				t.Fatalf("Received unexpected error: %v", err)
			}
			defer resp.Body.Close()

			if tt.err != nil {
				t.Fatalf("Expected an error but got nil")
				return
			}

			if tt.timeout > 0 {
				duration := time.Since(start)
				if duration >= tt.timeout {
					t.Fatalf("Request took %v, exceeded the timeout of %v", duration, tt.timeout)
				}
			}
		})
	}
}

func TestUserAgent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "skips header on empty",
			input: "",
			want:  "",
		},
		{
			name:  "sets user agent",
			input: "foobar-agent",
			want:  "foobar-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRequest(t, httpc.UserAgent(tt.input), func(t *testing.T, r *http.Request) {
				checkHeader(t, r, "User-Agent", tt.want)
			})
		})
	}
}

func TestAccept(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty",
			input: "",
		},
		{
			name:  "provided",
			input: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRequest(t, httpc.Accept(tt.input), func(t *testing.T, r *http.Request) {
				checkHeader(t, r, "Accept", tt.input)
			})
		})
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty",
			input: "",
		},
		{
			name:  "provided",
			input: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRequest(t, httpc.ContentType(tt.input), func(t *testing.T, r *http.Request) {
				checkHeader(t, r, "Content-Type", tt.input)
			})
		})
	}
}

func TestAuthorization(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty",
			input: "",
		},
		{
			name:  "provided",
			input: "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRequest(t, httpc.Authorization(tt.input), func(t *testing.T, r *http.Request) {
				checkHeader(t, r, "Authorization", tt.input)
			})
		})
	}
}

func TestAuthorizationBearer(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
		},
		{
			name:  "provided",
			input: "foo",
			want:  "Bearer foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRequest(t, httpc.AuthorizationBearer(tt.input), func(t *testing.T, r *http.Request) {
				checkHeader(t, r, "Authorization", tt.want)
			})
		})
	}
}

func TestBasicAuth(t *testing.T) {
	tests := []struct {
		name string
		user string
		pass string
		want string
	}{
		{
			name: "gets applied",
			user: "foo",
			pass: "bar",
			want: "Basic Zm9vOmJhcg==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRequest(t, httpc.AuthorizationBasic(tt.user, tt.pass),
				func(t *testing.T, r *http.Request) {
					checkHeader(t, r, "Authorization", tt.want)
				},
			)
		})
	}
}

func TestSetHeader(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{
			name:  "empty header key",
			key:   "",
			value: "bar",
			want:  "",
		},
		{
			name:  "empty value",
			key:   "X-Foo",
			value: "",
			want:  "",
		},
		{
			name:  "provided",
			key:   "X-Foo",
			value: "foo",
			want:  "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRequest(t, httpc.SetHeader(tt.key, tt.value),
				func(t *testing.T, r *http.Request) {
					checkHeader(t, r, tt.key, tt.want)
				},
			)
		})
	}
}

func TestStripSlashes(t *testing.T) {
	tests := []struct {
		name          string
		stripTrailing bool
		url           string
		want          string
	}{
		{
			name:          "strips slashes",
			stripTrailing: false,
			url:           "http://example.com///v1//foo/",
			want:          "http://example.com/v1/foo/",
		},
		{
			name:          "does not strip trailing slash on root",
			stripTrailing: true,
			url:           "http://example.com/",
			want:          "http://example.com/",
		},
		{
			name:          "strips slashes and trailing slash",
			stripTrailing: true,
			url:           "http://example.com///v1//foo/",
			want:          "http://example.com/v1/foo",
		},
		{
			name:          "strips slashes and multiple trailing slashes",
			stripTrailing: true,
			url:           "http://example.com///v1//foo///",
			want:          "http://example.com/v1/foo",
		},
		{
			name:          "nothing to strip",
			stripTrailing: false,
			url:           "http://example.com/v1/foo",
			want:          "http://example.com/v1/foo",
		},
		{
			name: "preserves query params",
			url:  "http://example.com///v1//foo&q=foo",
			want: "http://example.com/v1/foo&q=foo",
		},
		{
			name: "preserves anchor",
			url:  "http://example.com///v1//foo&q=foo#anchor",
			want: "http://example.com/v1/foo&q=foo#anchor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := httpc.NewRoundTripper(httpctest.ReplayBytes(stubResponse))
			req, err := http.NewRequest(http.MethodGet, tt.url, http.NoBody)
			if err != nil {
				t.Fatalf("Cannot create request: %v", err)
			}

			gotPath := make(chan string, 1)
			spy := httpc.DoerFunc(func(r *http.Request) (*http.Response, error) {
				gotPath <- r.URL.String()
				return tr.RoundTrip(r)
			})

			client := httpc.NewClient(spy, httpc.StripSlashes(tt.stripTrailing))
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Doer error: %v", err)
			}
			defer resp.Body.Close()

			if got := <-gotPath; got != tt.want {
				t.Errorf("Expected %q but got %q", tt.want, got)
			}
		})
	}
}

func TestSecure(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "fails on insecure",
			url:     "http://google.com",
			wantErr: true,
		},
		{
			name:    "proceeds on secure",
			url:     "https://google.com",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := httpc.NewRoundTripper(
				httpctest.ReplayBytes(stubResponse),
				httpc.Secure(),
			)
			c := httpc.NewClient(&http.Client{Transport: tr})
			req, err := http.NewRequest(http.MethodGet, tt.url, http.NoBody)
			if err != nil {
				t.Fatalf("Cannot create request: %v", err)
			}

			resp, err := c.Do(req)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("Expected no error but got: %v", err)
				}
				return
			}
			defer resp.Body.Close()
			if tt.wantErr {
				t.Fatalf("Expected an error but got nil")
			}
			checkStatus(t, resp, http.StatusOK)
		})
	}
}
