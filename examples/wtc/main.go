package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/kraciasty/httpc"
)

// The user agent string will be used when making requests.
var userAgent string = fmt.Sprintf(
	"%s (%s) Go/%s +(%s)",
	"wtc", runtime.GOOS, runtime.Version(), "github.com/kraciasty/httpc",
)

// WTC is a client for what-the-commit API.
type WTC struct {
	http httpc.Doer
}

// NewWTC returns a new wtc client.
func NewWTC(d httpc.Doer) *WTC {
	return &WTC{
		http: d,
	}
}

// Fetch a commit from wtc API.
func (c *WTC) Fetch(ctx context.Context) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://whatthecommit.com/index.txt", http.NoBody)
	if err != nil {
		return "", fmt.Errorf("wtc new request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(data), nil
}

// This simple application calls the WhatTheCommit API and demonstrates how to
// set up some common client options and prepare custom middlewares.
func main() {
	client := httpc.NewClient(
		http.DefaultClient,
		httpc.Recover(),
		httpc.StripSlashes(true),
		httpc.UserAgent(userAgent),
		httpc.Timeout(10*time.Second),
		clientLogger(),
	)

	// Make a call to the WhatTheCommit API for a cool commit message.
	wtc := NewWTC(client)
	commit, err := wtc.Fetch(context.Background())
	if err != nil {
		log.Fatalf("wtc fetch failure: %v", err)
	}
	log.Printf("Commit message: %q", strings.TrimSpace(string(commit)))
}

func clientLogger() httpc.MiddlewareFunc {
	return func(next httpc.DoerFunc) httpc.DoerFunc {
		return httpc.DoerFunc(func(r *http.Request) (*http.Response, error) {
			var sb strings.Builder
			sb.WriteString("Requesting " + r.URL.String() + "\n")
			for k, vv := range r.Header {
				sb.WriteString(k + ":\t" + strings.Join(vv, ","))
			}
			sb.WriteString("\n")

			start := time.Now()
			resp, err := next(r)
			if err != nil {
				return nil, err
			}

			took := time.Since(start)
			txt := fmt.Sprintf("Received [%d] %s %s in %v\n", resp.StatusCode, r.Method, r.URL, took)
			sb.WriteString(txt)
			for k, vv := range resp.Header {
				sb.WriteString(k + ":\t" + strings.Join(vv, ",") + "\n")
			}

			log.Print(sb.String())
			return resp, nil
		})
	}
}
