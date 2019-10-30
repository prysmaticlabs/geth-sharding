package traceutil

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// RecoveryHandlerFunc is a function that recovers from the panic `p` by returning an `error`.
// The context can be used to extract request scoped metadata and context values.
func RecoveryHandlerFunc(ctx context.Context, p interface{}) error {
	span := trace.FromContext(ctx)
	if span != nil {
		span.AddAttributes(trace.StringAttribute("stack", string(debug.Stack())))
	}
	err := fmt.Errorf("%v", p)
	logrus.WithError(err).WithField("stack", string(debug.Stack())).Error("gRPC panicked!")
	return err
}
