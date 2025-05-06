package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
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

	ctx, cancel := context.WithTimeout(context.Background(), *probeTimeout)
	defer cancel()

	client := newClientCodec()

	logger.Info("Calling SCGI server to discover session name")

	var scgiSessionName string
	if err := client.Call(ctx, "session.name", nil, &scgiSessionName); err != nil {
		logger.Error("Cannot rerieve SCGI session name",
			zap.String("session", ""),
			zap.Error(err),
		)
		os.Exit(1)
	}

	logger.Info("SCGI session name captured",
		zap.String("session", scgiSessionName),
	)

	done := make(chan bool, 1)
	go httpServer(client)
	go removeRegSocket()
	<-done

	// If RPC server is gracefully shutdown, cleanup and exit
	os.Exit(0)
}

func httpServer(c *clientCodec) {
	if *httpEndpoint == "" {
		logger.Info("Skipping HTTP server",
			zap.String("endpoint", *httpEndpoint),
		)
		return
	}
	logger.Info("Starting HTTP server",
		zap.String("endpoint", *httpEndpoint),
	)

	// Prepare http endpoint for healthz
	var result any
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := c.Call(r.Context(), "system.pid", nil, &result); err != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			logger.Debug("Health check succeeded")
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			logger.Debug("Health check failed",
				zap.Error(err),
			)
		}
	})
	mux.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if err := c.Call(r.Context(), "session.save", nil, &result); err != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			logger.Debug("Session save succeeded")
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			logger.Debug("Session save failed",
				zap.Error(err),
			)
		}
	})

	logger.Fatal("HTTP server closed",
		zap.Error(http.ListenAndServe(*httpEndpoint, mux)),
	)
}

func removeRegSocket() {
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
