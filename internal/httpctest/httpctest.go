package httpctest

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"hash/adler32"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"

	"github.com/kraciasty/httpc"
)

// Record returns a [MiddlewareFunc] that records the requests and responses
// in the provided basepath.
// The saved responses may be replayed with [Replay].
//
// I'd recommend using something like https://github.com/dnaeon/go-vcr for
// anything more complicated than stubbing a simple response.
func Record(basepath string) httpc.MiddlewareFunc {
	_ = os.MkdirAll(basepath, 0o755)
	return func(next httpc.DoerFunc) httpc.DoerFunc {
		return httpc.DoerFunc(func(r *http.Request) (*http.Response, error) {
			reqb, err := httputil.DumpRequest(r, true)
			if err != nil {
				return nil, fmt.Errorf("dump request: %w", err)
			}

			name := buildName(reqb)
			reqname := filepath.Join(basepath, name+".req.txt")
			if err = os.WriteFile(reqname, reqb, 0o644); err != nil {
				return nil, err
			}

			resp, err := next.RoundTrip(r)
			if err != nil {
				return resp, err
			}

			resb, err := httputil.DumpResponse(resp, true)
			if err != nil {
				return nil, fmt.Errorf("dump response: %w", err)
			}

			resname := filepath.Join(basepath, name+".res.txt")
			if err = os.WriteFile(resname, resb, 0o644); err != nil {
				return nil, err
			}

			return resp, nil
		})
	}
}

// ReplayBytes is a [httpc.DoerFunc] that replays the provided bytes of a HTTP
// response, which should be created with [httputil.DumpResponse].
func ReplayBytes(data []byte) httpc.DoerFunc {
	return httpc.DoerFunc(func(r *http.Request) (*http.Response, error) {
		rd := bufio.NewReader(bytes.NewReader(data))
		return http.ReadResponse(rd, r)
	})
}

// Replay is a [httpc.DoerFunc] that replays HTTP responses stored in the
// provided filesystem.
func Replay(fsys fs.FS) httpc.DoerFunc {
	return httpc.DoerFunc(func(r *http.Request) (*http.Response, error) {
		b, err := httputil.DumpRequest(r, true)
		if err != nil {
			return nil, fmt.Errorf("dump request: %w", err)
		}

		name := buildName(b) + ".res.txt"
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("fs read file: %w", err)
		}

		rd := bufio.NewReader(bytes.NewReader(data))
		return http.ReadResponse(rd, r)
	})
}

// TryReplay works like [Replay] but in case of a miss uses the doer to do the
// actual HTTP request.
//
// The provided doer should be wrapped with [Record] if it is expected to be
// stored somewhere.
func TryReplay(do httpc.DoerFunc, fsys fs.FS) httpc.DoerFunc {
	return httpc.DoerFunc(func(r *http.Request) (*http.Response, error) {
		b, err := httputil.DumpRequest(r, true)
		if err != nil {
			return nil, fmt.Errorf("dump request: %w", err)
		}

		name := buildName(b) + ".res.txt"
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return do(r)
			}

			return nil, fmt.Errorf("fs read file: %w", err)
		}

		rd := bufio.NewReader(bytes.NewReader(data))
		return http.ReadResponse(rd, r)
	})
}

func buildName(b []byte) string {
	h := adler32.New()
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil))
}
