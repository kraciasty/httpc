# httpc

[![Go Reference](https://pkg.go.dev/badge/github.com/kraciasty/httpc.svg)](https://pkg.go.dev/github.com/kraciasty/httpc)
[![Go Report Card](https://goreportcard.com/badge/github.com/kraciasty/httpc)](https://goreportcard.com/report/github.com/kraciasty/httpc)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/kraciasty/httpc)
![GitHub](https://img.shields.io/github/license/kraciasty/httpc)
[![codecov](https://codecov.io/gh/kraciasty/httpc/graph/badge.svg?token=5WKSY5NPJK)](https://codecov.io/gh/kraciasty/httpc)
<img align="right" width="190" src="assets/gopher.png">

Enhance your HTTP clients with **middleware support**.\
A Go package that simplifies HTTP client setup the _Go way_.\
Enjoy simplicity and elegance with zero dependencies.

Dress _the Gopher_ in whatever _layers_ it needs!

## Rationale

_Becoming fed up with the **lack of** built-in **HTTP client middleware**
support in the standard Go library, I created this package to reduce the need
for repetitive wrappers in each project and organize things a bit with
reusability in mind._

## Features

An idiomatic `net/http` client experience, but with additional benefits:

- seamless integration with `http.Client` and `http.RoundTripper`
- reduced boilerplate (configure common things once)
- zero dependencies (standard library-based)
- extensible middleware support and a collection of common utilities

## Quickstart

Get the package:

```sh
go get -u github.com/kraciasty/httpc
```

Import the package into your Go project:

```go
import "github.com/kraciasty/httpc"
```

<details>
<summary>Example client and tripper usage</summary>

### Usage with the client

Create a client instance with the desired middlewares.

```go
// Create a new client with the desired middlewares.
client := httpc.NewClient(http.DefaultClient, middlewares...)

// Make requests just like with the standard http client.
resp, err := client.Do(req)
```

Use the clients' `With` method to create a sub-client with additional
middlewares.

```go
client := httpc.NewClient(http.DefaultClient, Foo, Bar) // uses Foo, Bar
subclient := client.With(Baz) // uses Foo, Bar, Baz
```

### Usage with the transport

Use the `RoundTripper` middleware when you can't swap the `http.Client` or want
to have the middleware specifically on the transport.

```go
// Prepare a custom transport.
tr := httpc.NewRoundTripper(http.DefaultTransport, mws...)

// Use it in a http client or something that accepts a round-tripper.
client := &http.Client{Transport: tr}

// Make requests with the middleware-wrapped transport.
resp, err := client.Do(req)
```

Use the transports' `With` method to create a sub-transport with additional
middlewares:

```go
base := http.DefaultTransport
tripper := httpc.NewRoundTripper(base, Foo, Bar) // uses Foo, Bar
subtripper := client.With(Baz) // uses Foo, Bar, Baz
```
</details>

## Documentation

For the Go code documentation reference - check [pkg.go.dev](https://pkg.go.dev/github.com/kraciasty/httpc).

## Middlewares

Middlewares sit between HTTP requests and responses, enabling to intercept and
modify them.

Each middleware function should have the following signature:

```go
type MiddlewareFunc func(DoerFunc) DoerFunc
```

This package offers some basic built-in middlewares to tailor client behavior
according to your needs.

- [Recover](https://pkg.go.dev/github.com/kraciasty/httpc#Recover) - recover from panics
- [StripSlashes](https://pkg.go.dev/github.com/kraciasty/httpc#StripSlashes) - clean the URL path
- [Secure](https://pkg.go.dev/github.com/kraciasty/httpc#Secure) - https only
- [Timeout](https://pkg.go.dev/github.com/kraciasty/httpc#Timeout) - apply timeout to requests
- [UserAgent](https://pkg.go.dev/github.com/kraciasty/httpc#UserAgent) - set the `User-Agent` header
- [Accept](https://pkg.go.dev/github.com/kraciasty/httpc#Accept) - set the `Accept` header
- [ContentType](https://pkg.go.dev/github.com/kraciasty/httpc#ContentType) - set the `Content-Type` header
- [Authorization](https://pkg.go.dev/github.com/kraciasty/httpc#Authorization), [AuthorizationBearer](https://pkg.go.dev/github.com/kraciasty/httpc#AuthorizationBearer), [AuthorizationBasic](https://pkg.go.dev/github.com/kraciasty/httpc#AuthorizationBasic) - set the `Authorization` header
- [SetHeader](https://pkg.go.dev/github.com/kraciasty/httpc#SetHeader) - set a header

Explore details about the middlewares in the reference docs at
[pkg.go.dev](https://pkg.go.dev/github.com/kraciasty/httpc#Middleware).

### Creating middlewares

Creating a middleware is as simple as:

```go
func Foo(next DoerFunc) DoerFunc {
	return DoerFunc(func(r *http.Request) (*http.Response, error) {
		// do stuff with the request or response
		return next(r)
	})
}
```

Such a middleware is ready to use with `httpc.Client` or `httpc.RoundTripper`.

> [!NOTE]
> When using a middleware with an `httpc.Client`, it should be triggered only
> once.\
> If the middleware should be triggered on each round-trip, provide it to the
> `http.Client.Transport` instead.

```go
tr := httpc.NewRoundTripper(http.DefaultTransport, Foo)
c := httpc.NewClient(
	&http.Client{Transport: tr},
	Bar,
)
```

In the example above, requests made with the `httpc.Client` will trigger `Bar`
only once, but may invoke `Foo` multiple times in conditions such as redirects.

### Setting a custom base client

The `httpc.Client` can be provided with a custom base client.

```go
client := httpc.NewClient(customClient)
```

> [!TIP]
> The `http.Client` can be provided a base client with _better defaults_.\
> The timeouts should be configured appropriately for production environments.

<details>
<summary>Example http client with better defaults</summary>
	
```go
c := &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 90 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: time.Second,
		Proxy:                 http.ProxyFromEnvironment,
		// ...
	},
}
```
</details>

### Handling OAuth2/OIDC

Retrieve an access token from a token source and add the `Authorization` header
with the access token to outgoing HTTP requests.

<details>
<summary>Example oauth2 middleware</summary>

```go
// OAuth2 is a middleware that handles OAuth2/OpenID Connect (OIDC)
// authentication. It retrieves an access token from the provided token
// source and adds the "Authorization" header with the access token to
// the outgoing HTTP requests.
//
// The token source should implement the `oauth2.TokenSource` interface,
// which provides a method to obtain an access token.
func OAuth2(ts oauth2.TokenSource) MiddlewareFunc {
	return func(next DoerFunc) DoerFunc {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			token, err := ts.Token()
			if err != nil {
				return nil, err
			}

			token.SetAuthHeader(r)
			return next(r)
		})
	}
}
```

It should be pretty straightforward to use:

```go
at := "your-personal-access-token"
ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: at})
c := httpc.NewClient(http.DefaultClient, OAuth2(ts))
```
</details>



## Examples

Check out practical examples showcasing how to use this project in the
[examples](examples) directory:

- [wtc](examples/wtc) - calls
[WhatTheCommit](https://whatthecommit.com/) API and logs request/response info
- [replace-transport](examples/replace-transport) - replace `http.Transport`
- [replace-doer](examples/replace-doer) - replaces a http `Doer`
- [redirects](examples/redirects/) - tripper and client middlewares in action

Also check out the `_test.go` files for testable examples.

## Contributing

Contributions are welcome!\
If you find any issues or want to enhance the project, please submit a pull
request.

Middlewares that are somewhat complex or require dependencies are
expected to be maintained as a third-party package in a separate repository.

<details>
<summary>Example httpc middleware module</summary>
  
```go
package foo

import (
	"net/http"

	"github.com/kraciasty/httpc"
)

type Plugin struct {
	opts Options
}

type Options struct{}

func NewPlugin(opts Options) *Plugin {
	return &Plugin{opts: opts}
}

func (p *Plugin) Middleware(next httpc.DoerFunc) httpc.DoerFunc {
	return func(next httpc.DoerFunc) httpc.DoerFunc {
		return func(r *http.Request) (*http.Response, error) {
			// do your stuff
			return next(r)
		}
	}
}
```
</details>

<details>
<summary>Example third-party middleware</summary>
  
```go
package thirdparty

import (
	"net/http"

	"github.com/kraciasty/httpc"
)

type Plugin struct {
	tr http.RoundTripper
	opts Options
}

type Options struct{}

func New(tr http.RoundTripper, opts Options) *Plugin {
	return &Plugin{
		tr: tr,
		opts: opts,
	}
}

func (p *Plugin) RoundTrip(r *http.Request) (*http.Response, error) {
	// do stuff before request
	resp, err := p.tr.RoundTrip(r)
	// do stuff after request
	return resp, err
}
```

```go
package yours

func NewMiddleware(opts thirdparty.Options) httpc.MiddlewareFunc {
	return func(next httpc.DoerFunc) httpc.DoerFunc {
		rt := thirdparty.New(next, opts)
		return func(r *http.Request) (*http.Response, error) {
			return rt.RoundTrip(r)
		}
	}
}
```

Even if your're not able to modify a third-party package to return such
a middleware, it should be fairly easy to wrap it yourself - like it's done in
the `NewMiddleware` func above.
</details>

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file
for more information.