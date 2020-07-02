package httptunnel

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/mediabuyerbot/go-wirenet"
)

type contextKey int

const (
	// ContextKeyStreamReader is populated in the context by stream reader.
	ContextKeyStreamReader contextKey = iota + 1

	// ContextKeyStreamWriter is populated in the context by stream writer.
	ContextKeyStreamWriter

	// ContextKeySessionID is populated in the context by current session id.
	ContextKeySessionID
)

var (
	// ErrStreamReaderNotDefined returned when no stream reader is defined in context.
	ErrStreamReaderNotDefined = errors.New("httptunnel: stream reader not defined")

	// ErrStreamWriterNotDefined returned when no stream reader is defined in context.
	ErrStreamWriterNotDefined = errors.New("httptunnel: stream writer not defined")
)

type HTTPClient struct {
	streamName string
	wire       wirenet.Wire
}

func NewHttpClient(w wirenet.Wire, streamName string) *HTTPClient {
	return &HTTPClient{
		wire:       w,
		streamName: streamName,
	}
}

func WithSession(ctx context.Context, sessionID uuid.UUID) context.Context {
	return context.WithValue(ctx, ContextKeySessionID, sessionID)
}

func (c *HTTPClient) WithTx(ctx context.Context, sessionID uuid.UUID, fn func(streamCtx context.Context) error) error {
	session, err := c.wire.Session(sessionID)
	if err != nil {
		return err
	}
	stream, err := session.OpenStream(c.streamName)
	if err != nil {
		return err
	}
	defer stream.Close()

	reader := stream.Reader()
	writer := stream.Writer()

	ctx = context.WithValue(ctx, ContextKeyStreamReader, reader)
	ctx = context.WithValue(ctx, ContextKeyStreamWriter, writer)

	return fn(ctx)
}

func (c *HTTPClient) Do(req *http.Request) (resp *http.Response, err error) {
	ctx := req.Context()
	sid, ok := ctx.Value(ContextKeySessionID).(uuid.UUID)
	if !ok {
		resp, err = c.doTx(ctx, req)
	} else {
		resp, err = c.doSession(sid, req)
	}
	return resp, err
}

func (c *HTTPClient) doSession(sid uuid.UUID, req *http.Request) (*http.Response, error) {
	session, err := c.wire.Session(sid)
	if err != nil {
		return nil, err
	}
	stream, err := session.OpenStream(c.streamName)
	if err != nil {
		return nil, err
	}

	reader := stream.Reader()
	writer := stream.Writer()

	if err := req.WriteProxy(writer); err != nil {
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(reader), req)
	if err != nil {
		return nil, err
	}
	resp.Body = readCloser{
		stream:       stream,
		body:         resp.Body,
		streamReader: reader,
	}
	return resp, nil
}

func (c *HTTPClient) doTx(ctx context.Context, req *http.Request) (*http.Response, error) {
	streamReader, ok := ctx.Value(ContextKeyStreamReader).(io.ReadCloser)
	if !ok {
		return nil, ErrStreamReaderNotDefined
	}
	streamWriter, ok := ctx.Value(ContextKeyStreamWriter).(io.WriteCloser)
	if !ok {
		return nil, ErrStreamWriterNotDefined
	}

	if err := req.WriteProxy(streamWriter); err != nil {
		return nil, err
	}
	streamWriter.Close()

	resp, err := http.ReadResponse(bufio.NewReader(streamReader), req)
	if err != nil {
		return nil, err
	}
	resp.Body = readCloser{
		body:         resp.Body,
		streamReader: streamReader,
	}
	return resp, nil
}

type readCloser struct {
	stream       io.Closer
	streamReader io.Closer
	body         io.ReadCloser
}

func (rc readCloser) Read(p []byte) (n int, err error) {
	return rc.body.Read(p)
}

func (rc readCloser) Close() error {
	rc.body.Close()
	rc.streamReader.Close()
	if rc.stream != nil {
		rc.stream.Close()
	}
	return nil
}
