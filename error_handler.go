package httptunnel

import "context"

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
