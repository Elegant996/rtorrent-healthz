package main

import (
	"context"
	"net/http"

	"github.com/Elegant996/scgi"
	"github.com/askasoft/pango/net/xmlrpc"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"
)

// clientCodec contains an RPC encoder with the desired encoding
type clientCodec struct {
	enc any
}

// newRPCCodec returns an initialized rpcCodec instance
func newClientCodec() clientCodec {
	client := &http.Client{}
	client.Transport = scgi.NewRoundTripper(logger)

	switch *encoding {
	case "json":
		return clientCodec{
			enc: jrpc2.NewClient(jhttp.NewChannel(*scgiAddress, &jhttp.ChannelOptions{Client: client}), nil),
		}
	default: // xml
		return clientCodec{
			enc: &xmlrpc.Client{
				Endpoint:  *scgiAddress,
				Transport: client.Transport,
			},
		}
	}
}

// Call is the RPC invoked with desired encoding mechanism
func (c clientCodec) Call(ctx context.Context, method string, params any, result any) (err error) {
	switch c.enc.(type) {
	case *jrpc2.Client:
		if params != nil {
			err = c.enc.(*jrpc2.Client).CallResult(ctx, method, params, result)
		} else {
			err = c.enc.(*jrpc2.Client).CallResult(ctx, method, []string{}, result)
		}
	default: // *xmlrpc.Client
		if params != nil {
			err = c.enc.(*xmlrpc.Client).Call(ctx, method, params, result)
		} else {
			err = c.enc.(*xmlrpc.Client).Call(ctx, method, []string{}, result)
		}
	}

	return
}
