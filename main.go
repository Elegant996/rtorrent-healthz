package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const (
	defaultHealthzPort = "9808"
)

// Command line flags
var (
	probeTimeout = flag.Duration("probe-timeout", time.Second, "Probe timeout in seconds.")
	scgiAddress  = flag.String("scgi-address", "/run/scgi/socket", "Path of the SCGI server socket that the rtorrent-healthz will connect to.")
	httpEndpoint = flag.String("http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including SCGI server health check and save requests. The default is empty string, which means the server is disabled.")
	encoding     = flag.String("encoding", "xml", "The encoding mechanism used for remote procedure calling.")
	debug        = flag.Bool("debug", false, "Sets the log level to debug.")
)

var logger *zap.Logger

type healthProbe struct {
	sessionName string
	client      *clientCodec
}

func (h *healthProbe) checkProbe(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), *probeTimeout)
	defer cancel()

	var result any
	if err := h.client.Call(ctx, "system.pid", nil, &result); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		logger.Error("Health check failed",
			zap.Error(err),
		)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`ok`))
	logger.Debug("Health check succeeded")
}

func main() {
	flag.Parse()

	logger = zap.Must(zap.NewProduction())
	if *debug {
		logger = zap.Must(zap.NewDevelopment())
		logger.Debug("Debugging enabled")
	}
	defer logger.Sync()

	logger.Info("Running rtorrent-healthz",
		zap.String("encoding", *encoding),
	)

	var addr string
	if *httpEndpoint != "" {
		addr = *httpEndpoint
	} else {
		addr = net.JoinHostPort("0.0.0.0", defaultHealthzPort)
	}

	ctx := context.Background()
	client := newClientCodec()

	logger.Info("Calling SCGI server to discover session name")

	// Connect to the SCGI server without any timeout to avoid crashing the probe when the server is not ready yet.
	// Goal: liveness probe never crashes, it only fails the probe when the server is not available (yet).
	// Since a http server for the probe is not running at this point, Kubernetes liveness probe will fail immediately
	// with "connection refused", which is good enough to fail the probe.
	var sessionName string
	if err := client.Call(ctx, "session.name", nil, &sessionName); err != nil {
		logger.Error("Cannot rerieve SCGI session name",
			zap.String("session", ""),
			zap.Error(err),
		)
		os.Exit(1)
	}
	logger.Info("SCGI session name captured",
		zap.String("session", sessionName),
	)

	hp := &healthProbe{
		sessionName: sessionName,
		client:      client,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", hp.checkProbe)
	logger.Info("ServeMux listening",
		zap.String("address", addr),
	)

	logger.Fatal("Failed to start http server",
		zap.Error(http.ListenAndServe(addr, mux)),
	)

	done := make(chan bool, 1)
	go removeSocket()
	<-done

	// If RPC server is gracefully shutdown, cleanup and exit
	os.Exit(0)
}

func removeSocket() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM)
	<-sigc
	err := os.Remove(*scgiAddress)
	if err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to remove socket",
			zap.Error(err),
			zap.String("socket", *scgiAddress),
		)
		os.Exit(1)
	}
	os.Exit(0)
}
