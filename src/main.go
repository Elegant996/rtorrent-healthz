package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"time"

	"github.com/Elegant996/scgi"
	"github.com/Elegant996/xmlrpc"

	"k8s.io/klog"
)

// Command line flags
var (
	probeTimeout = flag.Duration("probe-timeout", time.Second, "Probe timeout in seconds")
	scgiAddress  = flag.String("scgi-address", "/run/rtorrent/rtorrent.sock", "Address of the SCGI socket.")
	healthzHost  = flag.String("health-host", "0.0.0.0", "Host for listening healthz requests")
	healthzPort  = flag.String("health-port", "9808", "TCP port for listening healthz requests")

	xrpc *xmlrpc.Client
)

func checkHealth(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), *probeTimeout)
	defer cancel()

	klog.V(5).Infof("Sending request to rTorrent")
	var result string
	if err := xrpc.Call(ctx, "system.api_version", nil, &result); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(err.Error()))
		klog.Errorf("health check failed: %v", err)
		return
	}

	klog.V(5).Infof("system.api_version = %s", result)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`ok`))
	klog.V(5).Infof("Health check succeeded")
}

func main() {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	var err error
	xrpc, err = xmlrpc.NewClient(*scgiAddress, &scgi.Transport{})
	if err != nil {
		klog.Errorf("failed to create XMLRPC client with error: %v", err)
	}

	addr := net.JoinHostPort(*healthzHost, *healthzPort)
	http.HandleFunc("/healthz", checkHealth)
	klog.Infof("Serving requests to /healthz on: %s", addr)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		klog.Fatalf("failed to start http server with error: %v", err)
	}
}
