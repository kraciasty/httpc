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

// BarSDK is an example sdk that uses [httpc.Doer] for fetching data.
type BarSDK struct {
	// Some API clients/SDKs may accept an interface for a [http.Client].
	client httpc.Doer
}

// Option for configuring the sdk.
type BarOption func(*BarSDK)

// WithBarClient is an option for setting a custom [Doer].
func WithBarClient(c httpc.Doer) BarOption {
	return func(s *BarSDK) {
		s.client = c
	}
}

// NewBarSDK returns a new bar sdk based on the provided options.
func NewBarSDK(opts ...BarOption) *BarSDK {
	sdk := &BarSDK{
		client: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(sdk)
	}

	return sdk
}

// FetchStuff fetches stuff from the remote with a [http.Client] instance.
func (s *BarSDK) FetchStuff(ctx context.Context) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "http://bar.sdk.local/stuff", http.NoBody)
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
	client := httpc.NewClient(
		&http.Client{Transport: replay},
		httpc.SetHeader("X-Foo", "bar"),
	)

	// It's possible to provide any client that implements the Do method.
	sdk := NewBarSDK(WithBarClient(client))
	stuff, err := sdk.FetchStuff(context.Background())
	if err != nil {
		log.Fatalf("SDK fetch stuff: %v", err)
	}

	fmt.Println(stuff) // Output: Some stubbed stuff.
}
