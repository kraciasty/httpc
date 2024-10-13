package httpc_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kraciasty/httpc"
	"github.com/kraciasty/httpc/internal/httpctest"
)

var stubDoer = httpctest.ReplayBytes(stubResponse)

func ExampleDoerFunc() {
	doer := httpc.DoerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("doer failure")
	})

	_, err := doer.Do(&http.Request{})
	fmt.Println(err) // Output: doer failure
}

func ExampleMiddlewareFunc() {
	mw := httpc.MiddlewareFunc(func(next httpc.DoerFunc) httpc.DoerFunc {
		return func(r *http.Request) (*http.Response, error) {
			fmt.Println(r.URL.String())
			return next(r)
		}
	})

	c := httpc.NewClient(stubDoer, mw)
	r, _ := http.NewRequest(http.MethodGet, "http://stuff.local", http.NoBody)
	_, _ = c.Do(r)
	// Output: http://stuff.local
}

func ExampleNewClient_middleware_order() {
	first := httpc.MiddlewareFunc(func(next httpc.DoerFunc) httpc.DoerFunc {
		return func(r *http.Request) (*http.Response, error) {
			fmt.Print("foo")
			return next(r)
		}
	})
	second := httpc.MiddlewareFunc(func(next httpc.DoerFunc) httpc.DoerFunc {
		return func(r *http.Request) (*http.Response, error) {
			fmt.Print("bar")
			return next(r)
		}
	})

	doer := httpc.DoerFunc(func(r *http.Request) (*http.Response, error) {
		fmt.Print("baz")
		return stubDoer(r)
	})
	c := httpc.NewClient(doer, first, second)
	_, _ = c.Do(&http.Request{})
	// Output: foobarbaz
}

func TestDoerFunc_roundTripperCompatible(t *testing.T) {
	want := "Hello, world!"
	adapter := httpc.DoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(want)),
		}, nil
	})

	c := &http.Client{Transport: adapter}
	resp, err := c.Get("http://localhost/hello")
	if err != nil {
		t.Fatalf("Expected success but got: %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Should read body: %v", err)
	}

	if string(got) != want {
		t.Errorf("got: %q but want %q", string(got), want)
	}
}

func TestRoundTripper(t *testing.T) {
	rt := httpc.NewRoundTripper(http.DefaultTransport, httpc.SetHeader("X-Foo", "bar"))
	c := &http.Client{Transport: rt}
	srv := setupServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Foo")
		if got != "bar" {
			t.Errorf("header mismatch: %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("Expected success but got: %v", err)
	}
	defer resp.Body.Close()
}

func TestClient_Do(t *testing.T) {
	tests := []struct {
		name    string
		mws     []httpc.MiddlewareFunc
		method  string
		body    io.Reader
		wantErr bool
		assert  func(t *testing.T, resp *http.Response)
	}{
		{
			name:   "success GET",
			method: http.MethodGet,
			assert: func(t *testing.T, resp *http.Response) {
				checkStatus(t, resp, http.StatusOK)
			},
		},
		{
			name:   "success POST with body",
			method: http.MethodPost,
			body:   bytes.NewReader([]byte("hello world")),
			assert: func(t *testing.T, resp *http.Response) {
				checkStatus(t, resp, http.StatusOK)
			},
		},
		{
			name:   "success with headers",
			method: http.MethodGet,
			mws:    []httpc.MiddlewareFunc{httpc.SetHeader("foo", "bar")},
			assert: func(t *testing.T, resp *http.Response) {
				checkStatus(t, resp, http.StatusOK)
				checkHeader(t, resp.Request, "foo", "bar")
			},
		},
		{
			name:   "middleware failure",
			method: http.MethodGet,
			mws: []httpc.MiddlewareFunc{func(next httpc.DoerFunc) httpc.DoerFunc {
				return func(*http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("middleware failure")
				}
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := setupStubServer(t)
			client := httpc.NewClient(srv.Client(), tt.mws...)

			r, err := http.NewRequest(tt.method, srv.URL, tt.body)
			if err != nil {
				t.Fatalf("New request failure: %v", err)
			}

			resp, err := client.Do(r)
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

			checkMethod(t, resp.Request, tt.method)
			if tt.assert != nil {
				tt.assert(t, resp)
			}
		})
	}
}

func TestClient_With(t *testing.T) {
	srv := setupStubServer(t)
	client := httpc.NewClient(
		srv.Client(),
		httpc.SetHeader("X-Foo", "foo"),
		httpc.SetHeader("X-Bar", "bar"),
	)
	subclient := client.With(httpc.SetHeader("X-Bar", "baz"))

	checkMiddlerwareInheritance(t, srv.URL, client.Do, subclient.Do)
}

func TestTransport_With(t *testing.T) {
	srv := setupStubServer(t)
	parentTr := httpc.NewRoundTripper(
		srv.Client().Transport,
		httpc.SetHeader("X-Foo", "foo"),
		httpc.SetHeader("X-Bar", "bar"),
	)
	tr := parentTr.With(httpc.SetHeader("X-Bar", "baz"))

	checkMiddlerwareInheritance(t, srv.URL, parentTr.RoundTrip, tr.RoundTrip)
}

// A simple started [httptest.Server] that will be release upon test completion.
func setupServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// A simple [httptest.Server] stub that responds with the request body.
func setupStubServer(t *testing.T) *httptest.Server {
	return setupServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if ct := r.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		}

		if _, err := io.Copy(w, r.Body); err != nil {
			t.Errorf("io copy: %v", err)
		}
	}))
}

// Performs an assertion of an example request made through the provided
// middleware againast a stub [httptest.Server].
// The assert function is run on the test server handler.
func assertRequest(
	t *testing.T,
	mw httpc.MiddlewareFunc,
	assert func(t *testing.T, r *http.Request),
) {
	t.Helper()

	srv := setupServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if assert != nil {
			assert(t, r)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"message":"hello world"}`))
	if err != nil {
		t.Fatalf("Cannot create request: %v", err)
	}

	client := httpc.NewClient(srv.Client(), mw)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Doer error: %v", err)
	}
	defer resp.Body.Close()
	checkStatus(t, resp, http.StatusOK)
}

func checkMethod(t *testing.T, r *http.Request, want string) {
	t.Helper()

	if got := r.Method; got != want {
		t.Errorf("Expected request method %q but got %q", want, got)
	}
}

func checkHeader(t *testing.T, r *http.Request, k, v string) {
	t.Helper()

	if got := r.Header.Get(k); got != v {
		t.Errorf("Expected request header %q to be %q, but got %q", k, v, got)
	}
}

func checkStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()

	if got := resp.StatusCode; got != want {
		t.Errorf("Expected status code %q but got %q", want, got)
	}
}

func checkMiddlerwareInheritance(t *testing.T, url string, parent, child httpc.DoerFunc) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("Should create request: %v", err)
	}

	t.Run("parent is unmodified", func(t *testing.T) {
		resp, err := parent(req)
		if err != nil {
			t.Fatalf("baseclient should succeed: %v", err)
		}
		defer resp.Body.Close()

		checkHeader(t, resp.Request, "X-Foo", "foo")
		checkHeader(t, resp.Request, "X-Bar", "bar")
	})

	t.Run("child carries previous middlewares", func(t *testing.T) {
		resp, err := child(req)
		if err != nil {
			t.Fatalf("subclient should succeed: %v", err)
		}
		defer resp.Body.Close()

		checkHeader(t, resp.Request, "X-Foo", "foo")
		checkHeader(t, resp.Request, "X-Bar", "baz")
	})
}
