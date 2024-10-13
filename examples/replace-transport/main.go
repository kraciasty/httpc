package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/kraciasty/httpc"
	"github.com/kraciasty/httpc/internal/httpctest"
)

// FooSDK is an example sdk that uses [http.Client] for fetching data.
type FooSDK struct {
	// Some API clients/SDKs may have a [http.Client] instead of a HTTP Doer
	// interface. If there's an option for providing the instance we may use the
	// transport which is defined as a [http.RoundTripper].
	client *http.Client
}

// FooOption for configuring the sdk.
type FooOption func(*FooSDK)

// WithClient is an option for setting a custom [http.Client].
func WithFooClient(c *http.Client) FooOption {
	return func(s *FooSDK) {
		s.client = c
	}
}

// NewFooSDK returns a new foo sdk based on the provided options.
func NewFooSDK(opts ...FooOption) *FooSDK {
	sdk := &FooSDK{
		client: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(sdk)
	}

	return sdk
}

// FetchStuff fetches stuff from the remote with a [http.Client] instance.
func (s *FooSDK) FetchStuff(ctx context.Context) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "http://sdk.local/stuff", http.NoBody)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("client do: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(data), nil
}

var stubResponse = []byte(`HTTP/1.1 200 OK

Some stubbed stuff.`)

func main() {
	replay := httpctest.ReplayBytes(stubResponse)
	mw := httpc.NewRoundTripper(
		replay,
		httpc.SetHeader("X-Foo", "bar"),
	)

	// It's only possible to replace the http client as an option for the sdk.
	client := &http.Client{Transport: mw}
	sdk := NewFooSDK(WithFooClient(client))
	stuff, err := sdk.FetchStuff(context.Background())
	if err != nil {
		log.Fatalf("SDK fetch stuff: %v", err)
	}

	fmt.Println(stuff) // Output: Some stubbed stuff.
}
