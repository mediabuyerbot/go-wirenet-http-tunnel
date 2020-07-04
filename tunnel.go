package httptunnel

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/mediabuyerbot/httpclient"

	"github.com/mediabuyerbot/go-wirenet"
)

const (
	DefaultReTryCount  = 10
	DefaultHTTPTimeout = 120 * time.Second
)

// ReadRequest reads and parses an incoming request from b.
type ReadRequest func(*bufio.Reader) (*http.Request, error)

// ReadResponse reads and returns an HTTP response from r.
type ReadResponse func(*bufio.Reader, *http.Request) (*http.Response, error)

// RequestFunc
type RequestFunc func(r *http.Request)

// ResponseFunc
type ResponseFunc func(r *http.Response)

type Tunnel struct {
	client       httpclient.Client
	errorHandler ErrorHandler
	readRequest  ReadRequest
	before       []RequestFunc
	after        []ResponseFunc
}

func New(
	options ...TunnelOption,
) *Tunnel {
	defaultClient, _ := httpclient.New(
		httpclient.WithRetryCount(DefaultReTryCount),
		httpclient.WithTimeout(DefaultHTTPTimeout),
	)
	tunnel := &Tunnel{
		readRequest:  http.ReadRequest,
		client:       defaultClient,
		errorHandler: NewLogErrorHandler(),
		before:       []RequestFunc{},
		after:        []ResponseFunc{},
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		option(tunnel)
	}
	return tunnel
}

// TunnelOption sets an optional parameter for tunnel instance.
type TunnelOption func(tunnel *Tunnel)

// SetTunnelErrorHandler is used to handle non-terminal errors. By default,
// non-terminal errors are ignored. This is intended as a diagnostic measure.
func SetTunnelErrorHandler(errorHandler ErrorHandler) TunnelOption {
	return func(tunnel *Tunnel) { tunnel.errorHandler = errorHandler }
}

// SetClient sets the HTTP client.
func SetClient(client httpclient.Client) TunnelOption {
	return func(tunnel *Tunnel) { tunnel.client = client }
}

// SetReadRequest sets the parser for incoming request.
func SetReadRequest(r ReadRequest) TunnelOption {
	return func(tunnel *Tunnel) { tunnel.readRequest = r }
}

// RequestHook functions are executed on the HTTP request object before the
// request is send.
func RequestHook(before ...RequestFunc) TunnelOption {
	return func(tunnel *Tunnel) { tunnel.before = append(tunnel.before, before...) }
}

// ResponseHook functions are executed on the HTTP response writer after the
// endpoint is invoked, but before anything is written to the client.
func ResponseHook(after ...ResponseFunc) TunnelOption {
	return func(tunnel *Tunnel) { tunnel.after = append(tunnel.after, after...) }
}

// Handle implements the Handler interface.
func (tunnel Tunnel) Handle(ctx context.Context, stream wirenet.Stream) {
	defer stream.Close()
	reader := stream.Reader()
	writer := stream.Writer()
	for !stream.IsClosed() {
		req, err := tunnel.readRequest(bufio.NewReader(reader))
		if err != nil {
			if err == io.EOF {
				break
			}
			tunnel.errorHandler.Handle(ctx, err)
			break
		}
		reader.Close()

		for _, f := range tunnel.before {
			f(req)
		}

		req.RequestURI = ""
		resp, err := tunnel.client.Do(req)
		if err != nil {
			tunnel.errorHandler.Handle(ctx, err)
			break
		}

		for _, f := range tunnel.after {
			f(resp)
		}

		if err := resp.Write(writer); err != nil {
			tunnel.errorHandler.Handle(ctx, err)
			break
		}
		resp.Body.Close()
		writer.Close()
	}
}
