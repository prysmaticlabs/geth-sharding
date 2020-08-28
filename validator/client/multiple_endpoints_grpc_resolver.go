package client

import (
	"google.golang.org/grpc/resolver"
	"strings"
)

// Modification of a default grpc passthrough resolver (google.golang.org/grpc/resolver/passthrough) allowing to use multiple addresses
// in grpc endpoint. Example:
// conn, err := grpc.DialContext(ctx, "127.0.0.1:4000,127.0.0.1:4001", grpc.WithInsecure(), grpc.WithResolvers(&multipleEndpointsGrpcResolverBuilder{}))
type multipleEndpointsGrpcResolverBuilder struct{}

func (*multipleEndpointsGrpcResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &mulipleEndpointsGrpcResolver{
		target: target,
		cc:     cc,
	}
	r.start()
	return r, nil
}

func (*multipleEndpointsGrpcResolverBuilder) Scheme() string {
	return resolver.GetDefaultScheme()
}

type mulipleEndpointsGrpcResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
}

func (r *mulipleEndpointsGrpcResolver) start() {
	endpoints := strings.Split(r.target.Endpoint, ",")
	var addrs []resolver.Address
	for _, endpoint := range endpoints {
		addrs = append(addrs, resolver.Address{Addr: endpoint})
	}
	r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (*mulipleEndpointsGrpcResolver) ResolveNow(o resolver.ResolveNowOptions) {}

func (*mulipleEndpointsGrpcResolver) Close() {}
