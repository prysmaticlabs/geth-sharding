package grpcutils

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// LogGRPCRequests this method logs the gRPC backend as well as request duration when the log level is set to debug
// or higher.
func LogGRPCRequests(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	// Shortcut when debug logging is not enabled.
	if logrus.GetLevel() < logrus.DebugLevel {
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	var header metadata.MD
	opts = append(
		opts,
		grpc.Header(&header),
	)
	start := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)
	logrus.WithField("backend", header["x-backend"]).
		WithField("method", method).WithField("duration", time.Now().Sub(start)).
		Debug("gRPC request finished.")
	return err
}
