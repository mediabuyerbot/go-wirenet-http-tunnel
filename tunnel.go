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
	DefaultReTryCount  = 5
	DefaultHTTPTimeout = 10 * time.Second
)

type ReadRequest func(*bufio.Reader) (*http.Request, error)
type ReadResponse func(*bufio.Reader, *http.Request) (*http.Response, error)

type Tunnel struct {
	client       httpclient.Client
	errorHandler ErrorHandler
	readRequest  ReadRequest
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

func SetClient(client httpclient.Client) TunnelOption {
	return func(tunnel *Tunnel) { tunnel.client = client }
}

func SetReadRequest(r ReadRequest) TunnelOption {
	return func(tunnel *Tunnel) { tunnel.readRequest = r }
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

		req.RequestURI = ""
		resp, err := tunnel.client.Do(req)
		if err != nil {
			tunnel.errorHandler.Handle(ctx, err)
			break
		}

		if err := resp.Write(writer); err != nil {
			tunnel.errorHandler.Handle(ctx, err)
			break
		}
		resp.Body.Close()
		writer.Close()
	}
}
