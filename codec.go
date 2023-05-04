package main

import (
	"context"
	"net/http"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/Elegant996/scgi"
	"github.com/Elegant996/xmlrpc"
)

// rpcCodec contains an RPC client with the desired encoding
type rpcCodec struct {
	client interface{}
}

// newRPCCodec returns an initialized rpcCodec instance
func newRPCCodec() *rpcCodec {
	switch *encoding {
	case "json":
		ch		:= jhttp.NewChannel(*scgiAddress, &jhttp.ChannelOptions{Client: &http.Client{Transport: &scgi.Transport{}}})
		c		:= jrpc2.NewClient(ch, nil)
		return &rpcCodec{client: c}
	default: // xml
		c, _	:= xmlrpc.NewClient(*scgiAddress, &scgi.Transport{})
		return &rpcCodec{client: c}
	}
}

// Call is the RPC invoked with desired encoding mechanism
func (r *rpcCodec) Call(ctx context.Context, method string, params any, result any) error {
	switch r.client.(type) {
	case *jrpc2.Client:
		return r.client.(*jrpc2.Client).CallResult(ctx, method, []string{""}, result)
	default: // *xmlrpc.Client
		return r.client.(*xmlrpc.Client).Call(ctx, method, params, result)
	}
}