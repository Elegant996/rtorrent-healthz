package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

// Command line flags
var (
	operationTimeout    = flag.Duration("timeout", time.Second, "Timeout for waiting for communication with rtorrent.")
	scgiAddress         = flag.String("scgi-address", "/run/scgi/socket", "Path of the SCGI server socket that the rtorrent-healthz will connect to.")
	httpEndpoint        = flag.String("http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including SCGI server health check and save requests. The default is empty string, which means the server is disabled.")
	encoding            = flag.String("encoding", "xml", "The encoding mechanism used for remote procedure calling.")

	// List of supported versions
	supportedVersions = []string{"0.9.8"}
)

func main() {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	klog.Infof("Running rtorrent-healthz with encoding=%s", *encoding)

	r := newRPCCodec()

	klog.V(1).Infof("Calling SCGI server to discover session name")

	var scgiSessionName string
	if err := r.Call(context.Background(), "session.name", nil, &scgiSessionName); err != nil {
		klog.Errorf("error retrieving SCGI session name: %v", err)
		os.Exit(1)
	}
	
	klog.V(2).Infof("SCGI session name: %q", scgiSessionName)

	done := make(chan bool, 1)
	go httpServer(r)
	go removeRegSocket()
	<-done

	// If RPC server is gracefully shutdown, cleanup and exit
	os.Exit(0)
}

func httpServer(r *rpcCodec) {
	if *httpEndpoint == "" {
		klog.Infof("Skipping HTTP server because endpoint is set to: %q", *httpEndpoint)
		return
	}
	klog.Infof("Starting HTTP server at endpoint: %v\n", *httpEndpoint)

	ctx, cancel := context.WithTimeout(context.Background(), *operationTimeout)
	defer cancel()

	// Prepare http endpoint for healthz
	var result any
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		if err := r.Call(ctx, "system.api_version", nil, &result); err != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`ok`))
			klog.V(5).Infof("health check succeeded")
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			klog.Errorf("health check failed: %+v", err)
		}
	})
	mux.HandleFunc("/save", func(w http.ResponseWriter, req *http.Request) {
		if err := r.Call(context.Background(), "session.save", nil, &result); err != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`ok`))
			klog.V(5).Infof("session save succeeded")
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			klog.Errorf("session save failed: %+v", err)
		}
	})

	klog.Fatal(http.ListenAndServe(*httpEndpoint, mux))
}

func removeRegSocket() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM)
	<-sigc
	err := os.Remove(*scgiAddress)
	if err != nil && !os.IsNotExist(err) {
		klog.Errorf("failed to remove socket: %s with error: %+v", *scgiAddress, err)
		os.Exit(1)
	}
	os.Exit(0)
}