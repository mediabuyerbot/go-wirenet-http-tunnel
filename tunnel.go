package httptunnel

import (
	"bufio"
	"context"
	"io"
	"net/http"

	"github.com/mediabuyerbot/go-wirenet"
)

type Tunnel struct {
	client       http.Client
	errorHandler ErrorHandler
}

func NewTunnel(
	client http.Client,
	options ...TunnelOption,
) *Tunnel {
	tunnel := &Tunnel{
		client:       client,
		errorHandler: NewLogErrorHandler(),
	}
	for _, option := range options {
		option(tunnel)
	}
	return tunnel
}

// TunnelOption sets an optional parameter for tunnel instance.
type TunnelOption func(tunnel *Tunnel)

// TunnelErrorHandler is used to handle non-terminal errors. By default,
// non-terminal errors are ignored. This is intended as a diagnostic measure.
func TunnelErrorHandler(errorHandler ErrorHandler) TunnelOption {
	return func(tunnel *Tunnel) { tunnel.errorHandler = errorHandler }
}

// Handle implements the Handler interface.
func (tunnel Tunnel) Handle(ctx context.Context, stream wirenet.Stream) {
	defer stream.Close()
	reader := stream.Reader()
	writer := stream.Writer()
	for !stream.IsClosed() {
		req, err := http.ReadRequest(bufio.NewReader(reader))
		if err != nil {
			if err == io.EOF {
				break
			}
			tunnel.errorHandler.Handle(ctx, err)
			return
		}
		reader.Close()
		req.RequestURI = ""
		resp, err := tunnel.client.Do(req)
		if err != nil {
			tunnel.errorHandler.Handle(ctx, err)
			return
		}
		if err := resp.Write(writer); err != nil {
			tunnel.errorHandler.Handle(ctx, err)
			return
		}
		writer.Close()
	}
}
