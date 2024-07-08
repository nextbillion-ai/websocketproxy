package websocketproxy

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var (
	serverURL   = "ws://127.0.0.1:7777"
	backendURL  = "ws://127.0.0.1:8888"
	serverURL2  = "ws://127.0.0.1:5555"
	backendURL2 = "ws://127.0.0.1:6666"
)

func TestProxy(t *testing.T) {
	// websocket proxy
	supportedSubProtocols := []string{"test-protocol"}
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: supportedSubProtocols,
	}

	u, _ := url.Parse(backendURL)
	proxy := NewProxy(u)
	proxy.Upgrader = upgrader

	mux := http.NewServeMux()
	mux.Handle("/proxy", proxy)
	go func() {
		if err := http.ListenAndServe(":7777", mux); err != nil {
			panic(fmt.Errorf("ListenAndServe: %s", err))
		}
	}()

	time.Sleep(time.Millisecond * 100)

	// backend echo server
	go func() {
		mux2 := http.NewServeMux()
		mux2.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Don't upgrade if original host header isn't preserved
			if r.Host != "127.0.0.1:7777" {
				log.Printf("Host header set incorrectly.  Expecting 127.0.0.1:7777 got %s", r.Host)
				return
			}

			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}

			messageType, p, err := conn.ReadMessage()
			if err != nil {
				return
			}

			if err = conn.WriteMessage(messageType, p); err != nil {
				return
			}
		})

		err := http.ListenAndServe(":8888", mux2)
		if err != nil {
			panic(fmt.Errorf("ListenAndServe: %s", err))
		}
	}()

	time.Sleep(time.Millisecond * 100)

	// let's us define two subprotocols, only one is supported by the server
	clientSubProtocols := []string{"test-protocol", "test-notsupported"}
	h := http.Header{}
	for _, subprot := range clientSubProtocols {
		h.Add("Sec-WebSocket-Protocol", subprot)
	}

	// frontend server, dial now our proxy, which will reverse proxy our
	// message to the backend websocket server.
	conn, resp, err := websocket.DefaultDialer.Dial(serverURL+"/proxy", h)
	if err != nil {
		t.Fatal(err)
	}

	// check if the server really accepted only the first one
	in := func(desired string) bool {
		for _, prot := range resp.Header[http.CanonicalHeaderKey("Sec-WebSocket-Protocol")] {
			if desired == prot {
				return true
			}
		}
		return false
	}

	if !in("test-protocol") {
		t.Error("test-protocol should be available")
	}

	if in("test-notsupported") {
		t.Error("test-notsupported should be not recevied from the server.")
	}

	// now write a message and send it to the backend server (which goes trough
	// proxy..)
	msg := "hello kite"
	err = conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		t.Error(err)
	}

	messageType, p, err := conn.ReadMessage()
	if err != nil {
		t.Error(err)
	}

	if messageType != websocket.TextMessage {
		t.Error("incoming message type is not Text")
	}

	if msg != string(p) {
		t.Errorf("expecting: %s, got: %s", msg, string(p))
	}
}

func TestPing(t *testing.T) {
	// websocket proxy
	supportedSubProtocols := []string{"test-protocol"}
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: supportedSubProtocols,
	}

	u, _ := url.Parse(backendURL2)
	proxy := NewProxy(u)
	proxy.Upgrader = upgrader

	mux := http.NewServeMux()
	mux.Handle("/proxy", proxy)
	go func() {
		if err := http.ListenAndServe(":5555", mux); err != nil {
			panic(fmt.Errorf("ListenAndServe: %s", err))
		}
	}()

	time.Sleep(time.Millisecond * 100)

	// backend ping server
	go func() {
		mux2 := http.NewServeMux()
		mux2.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Don't upgrade if original host header isn't preserved
			if r.Host != "127.0.0.1:5555" {
				log.Printf("Host header set incorrectly.  Expecting 127.0.0.1:5555got %s", r.Host)
				return
			}

			var err error
			var conn *websocket.Conn

			if conn, err = upgrader.Upgrade(w, r, nil); err != nil {
				log.Println(err)
				return
			}
			pongCh := make(chan string, 1)
			conn.SetPongHandler(func(appData string) error {
				pongCh <- appData
				return nil
			})
			if _, _, err = conn.ReadMessage(); err != nil {
				log.Println(err)
				return
			}

			if err = conn.WriteControl(websocket.PingMessage, []byte("whocares"), time.Now().Add(1*time.Second)); err != nil {
				log.Println(err)
				return
			}
			if err = conn.WriteMessage(websocket.TextMessage, []byte("whocares")); err != nil {
				log.Println(err)
				return
			}
			if _, _, err = conn.ReadMessage(); err != nil {
				log.Println(err)
				return
			}

			pongMsg := <-pongCh
			if pongMsg != "whocares" {
				panic("wrong pong payload")
			}
			if err = conn.WriteMessage(websocket.TextMessage, []byte("whocares")); err != nil {
				log.Println(err)
				return
			}
		})

		err := http.ListenAndServe(":6666", mux2)
		if err != nil {
			panic(fmt.Errorf("ListenAndServe: %s", err))
		}
	}()

	time.Sleep(time.Millisecond * 100)

	// let's us define two subprotocols, only one is supported by the server
	clientSubProtocols := []string{"test-protocol", "test-notsupported"}
	h := http.Header{}
	for _, subprot := range clientSubProtocols {
		h.Add("Sec-WebSocket-Protocol", subprot)
	}

	// frontend server, dial now our proxy, which will reverse proxy our
	// message to the backend websocket server.
	conn, resp, err := websocket.DefaultDialer.Dial(serverURL2+"/proxy", h)
	if err != nil {
		t.Fatal(err)
	}

	// check if the server really accepted only the first one
	in := func(desired string) bool {
		for _, prot := range resp.Header[http.CanonicalHeaderKey("Sec-WebSocket-Protocol")] {
			if desired == prot {
				return true
			}
		}
		return false
	}

	if !in("test-protocol") {
		t.Error("test-protocol should be available")
	}

	if in("test-notsupported") {
		t.Error("test-notsupported should be not recevied from the server.")
	}
	pingCh := make(chan string, 1)
	conn.SetPingHandler(func(appData string) error {
		pingCh <- appData
		return nil
	})

	if err = conn.WriteMessage(websocket.TextMessage, []byte("whocares")); err != nil {
		t.Error(err)
	}
	if _, _, err = conn.ReadMessage(); err != nil {
		t.Error(err)
	}

	pingMsg := <-pingCh

	if pingMsg != "whocares" {
		t.Error("received ping with wrong payload")
	}
	if err = conn.WriteControl(websocket.PongMessage, []byte("whocares"), time.Now().Add(1*time.Second)); err != nil {
		t.Error(err)
	}
	if err = conn.WriteMessage(websocket.TextMessage, []byte("whocares")); err != nil {
		t.Error(err)
	}
	if _, _, err = conn.ReadMessage(); err != nil {
		t.Error(err)
	}
}
