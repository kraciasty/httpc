package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/kraciasty/httpc"
	"github.com/kraciasty/httpc/internal/httpctest"
)

const (
	url      = "http://google.com"
	basepath = "./testdata"
)

func main() {
	/// Tripper with some middlewares and record/replay.
	tr := httpc.NewRoundTripper(
		httpctest.TryReplay(
			httpc.DoerFunc(http.DefaultTransport.RoundTrip),
			os.DirFS(basepath), // Replayed from testdata
		),
		httpctest.Record(basepath),
		httpc.MiddlewareFunc(func(next httpc.DoerFunc) httpc.DoerFunc {
			return func(r *http.Request) (*http.Response, error) {
				fmt.Println("tripper middleware -->", r.URL.String())
				return next.RoundTrip(r)
			}
		}),
	)

	// Client with our custom transport and additional client middleware.
	c := httpc.NewClient(
		&http.Client{Transport: tr},
		httpc.MiddlewareFunc(func(next httpc.DoerFunc) httpc.DoerFunc {
			return func(r *http.Request) (*http.Response, error) {
				fmt.Println("doer middleware -->", r.URL.String())
				return next(r)
			}
		}),
	)

	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		log.Fatalf("new request: %v", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		log.Fatalf("client do: %v", err)
	}
	defer resp.Body.Close()

	// Output:
	// doer middleware --> http://google.com
	// tripper middleware --> http://google.com
	// tripper middleware --> http://www.google.com/
}
