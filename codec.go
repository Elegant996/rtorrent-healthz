package main

import (
	"context"
	"net/http"

	"github.com/Elegant996/scgi"
	"github.com/Elegant996/xmlrpc"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"
)

type loggerKeyType int

const loggerKey loggerKeyType = iota

// rpcCodec contains an RPC encoder with the desired encoding
type rpcCodec struct {
	encoder interface{}
}

// newRPCCodec returns an initialized rpcCodec instance
func newRPCCodec() *rpcCodec {
	client := &http.Client{}
	client.Transport = scgi.NewRoundTripper(logger)

	switch *encoding {
	case "json":
		c := jrpc2.NewClient(jhttp.NewChannel(*scgiAddress, &jhttp.ChannelOptions{Client: client}), nil)
		return &rpcCodec{encoder: c}
	default: // xml
		c, _ := xmlrpc.NewClient(*scgiAddress, client.Transport)
		return &rpcCodec{encoder: c}
	}
}

// Call is the RPC invoked with desired encoding mechanism
func (c *rpcCodec) Call(ctx context.Context, method string, params any, result any) error {
	switch c.encoder.(type) {
	case *jrpc2.Client:
		return c.encoder.(*jrpc2.Client).CallResult(ctx, method, []string{""}, result)
	default: // *xmlrpc.Client
		return c.encoder.(*xmlrpc.Client).Call(ctx, method, params, result)
	}
}
