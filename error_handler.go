package httptunnel

import (
	"context"
	"log"
	"os"
)

// ErrorHandler receives a transport error to be processed for diagnostic purposes.
// Usually this means logging the error.
type ErrorHandler interface {
	Handle(ctx context.Context, err error)
}

// The ErrorHandlerFunc type is an adapter to allow the use of
// ordinary function as ErrorHandler. If f is a function
// with the appropriate signature, ErrorHandlerFunc(f) is a
// ErrorHandler that calls f.
type ErrorHandlerFunc func(ctx context.Context, err error)

// Handle calls f(ctx, err).
func (f ErrorHandlerFunc) Handle(ctx context.Context, err error) {
	f(ctx, err)
}

type LogErrorHandler struct {
	logger *log.Logger
}

func NewLogErrorHandler() *LogErrorHandler {
	return &LogErrorHandler{
		logger: log.New(os.Stderr, "httptunnel ", log.LstdFlags),
	}
}

func (h *LogErrorHandler) Handle(ctx context.Context, err error) {
	h.logger.Println("[ERR]", err)
}
