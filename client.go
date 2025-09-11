package main

import (
	"context"
	"net/http"

	"github.com/Elegant996/scgi"
	"github.com/Elegant996/xmlrpc"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"
)

// clientCodec contains an RPC encoder with the desired encoding
type clientCodec struct {
	enc any
}

// newRPCCodec returns an initialized rpcCodec instance
func newClientCodec() *clientCodec {
	client := &http.Client{}
	client.Transport = scgi.NewRoundTripper(logger)

	switch *encoding {
	case "json":
		c := jrpc2.NewClient(jhttp.NewChannel(*scgiAddress, &jhttp.ChannelOptions{Client: client}), nil)
		return &clientCodec{enc: c}
	default: // xml
		c, _ := xmlrpc.NewClient(*scgiAddress, client.Transport)
		return &clientCodec{enc: c}
	}
}

// Call is the RPC invoked with desired encoding mechanism
func (c *clientCodec) Call(ctx context.Context, method string, params any, result any) (err error) {
	switch c.enc.(type) {
	case *jrpc2.Client:
		err = c.enc.(*jrpc2.Client).CallResult(ctx, method, []any{params}, result)
	default: // *xmlrpc.Client
		err = c.enc.(*xmlrpc.Client).Call(ctx, method, params, result)
	}

	return
}
