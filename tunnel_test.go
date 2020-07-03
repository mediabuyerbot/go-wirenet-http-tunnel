package httptunnel

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mediabuyerbot/httpclient"

	"github.com/mediabuyerbot/go-wirenet"
	"github.com/stretchr/testify/assert"
)

func TestTunnel_New(t *testing.T) {
	tunnel := New(nil)
	assert.NotNil(t, tunnel)

	client := &httpclient.HttpClient{}
	errhandler := NewLogErrorHandler()

	tunnel = New(
		SetClient(client),
		SetTunnelErrorHandler(errhandler),
	)
	assert.Equal(t, tunnel.client, client)
	assert.Equal(t, tunnel.errorHandler, errhandler)
}

func TestTunnel_Handle(t *testing.T) {
	addr := randomAddr(t)

	initSrv := make(chan struct{})
	initCli := make(chan struct{})
	done := make(chan struct{})
	want := []byte("HELLO")
	// http server
	httphandler := func(rw http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/req1":
			assert.Equal(t, "/req1", r.RequestURI)
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "A", r.Header.Get("X-Header-A"))
			assert.Equal(t, "B", r.Header.Get("X-Header-B"))
			assert.Equal(t, "new user agent", r.Header.Get("User-Agent"))
			rw.WriteHeader(http.StatusForbidden)
			_, err := rw.Write(want)
			assert.Nil(t, err)

		case "/req2":
			assert.Equal(t, "/req2", r.RequestURI)
			data, err := ioutil.ReadAll(r.Body)
			assert.Equal(t, want, data)
			assert.Equal(t, http.MethodPost, r.Method)
			rw.WriteHeader(http.StatusOK)
			_, err = rw.Write(want)
			assert.Nil(t, err)
		}

	}
	httpsrv := httptest.NewServer(http.HandlerFunc(httphandler))

	// server side
	server, err := wirenet.Mount(addr, wirenet.WithConnectHook(func(closer io.Closer) {
		close(initSrv)
	}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	wire, err := wirenet.Join(addr, wirenet.WithSessionOpenHook(func(session wirenet.Session) {
		close(initCli)
	}))
	assert.Nil(t, err)

	tunnel := New()
	wire.Stream("tunnel", tunnel.Handle)

	go func() {
		assert.Nil(t, wire.Connect())
	}()
	<-initCli

	go func() {
		time.Sleep(2 * time.Second)

		defer close(done)
		client := NewClient(server, "tunnel")
		ctx := context.Background()

		for sid, _ := range wire.Sessions() {
			err = client.WithTx(ctx, sid, func(streamCtx context.Context) error {

				// req 1
				req1, err := http.NewRequestWithContext(streamCtx, http.MethodGet, httpsrv.URL+"/req1", nil)
				assert.Nil(t, err)
				req1.Header.Add("X-Header-A", "A")
				req1.Header.Add("X-Header-B", "B")
				req1.Header.Set("User-Agent", "new user agent")
				resp, err := client.Do(req1)
				assert.Nil(t, err)
				data, err := ioutil.ReadAll(resp.Body)
				assert.Nil(t, err)
				assert.Nil(t, resp.Body.Close())
				assert.Equal(t, want, data)
				assert.Equal(t, http.StatusForbidden, resp.StatusCode)

				// req2
				req2, err := http.NewRequestWithContext(streamCtx, http.MethodPost, httpsrv.URL+"/req2", bytes.NewReader(want))
				assert.Nil(t, err)
				resp2, err := client.Do(req2)
				assert.Nil(t, err)
				data, err = ioutil.ReadAll(resp2.Body)
				assert.Nil(t, err)
				assert.Nil(t, resp2.Body.Close())
				assert.Equal(t, want, data)
				assert.Equal(t, http.StatusOK, resp2.StatusCode)

				return nil
			})

			assert.Nil(t, err)
		}
	}()

	<-done
}

func TestTunnel_ErrorHandle(t *testing.T) {
	addr := randomAddr(t)

	initSrv := make(chan struct{})
	initCli := make(chan struct{})
	done := make(chan struct{})
	want := []byte("HELLO")
	// http server
	httphandler := func(rw http.ResponseWriter, r *http.Request) {
		rw.Write(want)
	}
	httpsrv := httptest.NewServer(http.HandlerFunc(httphandler))

	// server side
	server, err := wirenet.Mount(addr, wirenet.WithConnectHook(func(closer io.Closer) {
		close(initSrv)
	}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	wire, err := wirenet.Join(addr, wirenet.WithSessionOpenHook(func(session wirenet.Session) {
		close(initCli)
	}))
	assert.Nil(t, err)

	tunnel := New(
		SetReadRequest(func(reader *bufio.Reader) (request *http.Request, err error) {
			req, err := http.ReadRequest(reader)
			if err != nil {
				return nil, err
			}
			if req.URL.Path == "/eof" {
				return nil, io.EOF
			}
			return nil, errors.New("some error")
		}))
	wire.Stream("tunnel", tunnel.Handle)

	go func() {
		assert.Nil(t, wire.Connect())
	}()
	<-initCli

	go func() {
		time.Sleep(2 * time.Second)

		defer close(done)
		client := NewClient(server, "tunnel")
		ctx := context.Background()

		for sid, _ := range wire.Sessions() {
			err = client.WithTx(ctx, sid, func(streamCtx context.Context) error {

				// req 1
				req1, err := http.NewRequestWithContext(streamCtx, http.MethodGet, httpsrv.URL+"/eof", nil)
				assert.Nil(t, err)
				resp, err := client.Do(req1)
				assert.Equal(t, io.ErrUnexpectedEOF, err)
				assert.Nil(t, resp)

				// req2
				req2, err := http.NewRequestWithContext(streamCtx, http.MethodGet, httpsrv.URL, bytes.NewReader(want))
				assert.Nil(t, err)
				resp2, err := client.Do(req2)
				assert.Equal(t, io.ErrUnexpectedEOF, err)
				assert.Nil(t, resp2)

				return nil
			})

			assert.Nil(t, err)
		}
	}()

	<-done
}

func TestTunnel_WithSessionHandler(t *testing.T) {
	addr := randomAddr(t)

	initSrv := make(chan struct{})
	initCli := make(chan struct{})
	done := make(chan struct{})
	payload := make([]byte, 1024*1024)

	// http server
	httphandler := func(rw http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadAll(r.Body)
		assert.Nil(t, err)
		assert.Equal(t, len(payload), len(data))
		rw.WriteHeader(http.StatusOK)
		_, err = rw.Write(payload)
		assert.Nil(t, err)
	}
	httpsrv := httptest.NewServer(http.HandlerFunc(httphandler))

	// server side
	server, err := wirenet.Mount(addr, wirenet.WithConnectHook(func(closer io.Closer) {
		close(initSrv)
	}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	wire, err := wirenet.Join(addr, wirenet.WithSessionOpenHook(func(session wirenet.Session) {
		close(initCli)
	}))
	assert.Nil(t, err)

	tunnel := New()
	wire.Stream("tunnel", tunnel.Handle)

	go func() {
		assert.Nil(t, wire.Connect())
	}()
	<-initCli

	go func() {
		time.Sleep(2 * time.Second)

		defer close(done)
		client := NewClient(server, "tunnel")
		ctx := context.Background()

		for sid, _ := range wire.Sessions() {
			sessCtx := WithSession(ctx, sid)
			req1, err := http.NewRequestWithContext(sessCtx, http.MethodPost, httpsrv.URL, bytes.NewReader(payload))
			assert.Nil(t, err)
			resp, err := client.Do(req1)
			assert.Nil(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Nil(t, err)
			data, err := ioutil.ReadAll(resp.Body)
			assert.Nil(t, err)
			assert.Equal(t, len(payload), len(data))
			assert.Nil(t, resp.Body.Close())
		}
	}()

	<-done
}

func randomAddr(t *testing.T) string {
	if t == nil {
		t = new(testing.T)
	}
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	assert.Nil(t, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.Nil(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf(":%d", port)
}
