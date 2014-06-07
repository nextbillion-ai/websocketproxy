# WebsocketProxy [![GoDoc](https://godoc.org/github.com/koding/websocketproxy?status.png)](http://godoc.org/github.com/koding/websocketproxy) [![Build Status](https://travis-ci.org/koding/websocketproxy.png)](https://travis-ci.org/koding/websocketproxy)

WebsocketProxy is an http.Handler interface build on top of
[gorilla/websocket](https://github.com/gorilla/websocket) that you can plug
into your existing Go webserver to provide WebSocket reverse proxy.

## Install

```bash
go get github.com/koding/websocketproxy
```

## Example

Below is a simple app that runs on a server and proxies to the given backend URL

```go
package main

import (
	"flag"
	"net/http"
	"net/url"

	"github.com/koding/websocketproxy"
)

var (
	flagPort    = flag.String("port", "3000", "Port of the reverse proxy")
	flagBackend = flag.String("backend", "", "Backend URL for proxying")
)

func main() {
	u, err := url.Parse(*flagBackend)
	if err != nil {
		log.Fataln(err)
	}

	err := http.ListenAndServe(":"+*flagPort, websocketproxy.NewProxy(u))
	if err != nil {
		log.Fataln(err)
	}
}
```

Save it as `proxy.go` and run as:


```bash
go run proxy.go -backend ws://example.com -port 80
```

