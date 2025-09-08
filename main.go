package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	defaultHealthzPort = "9808"
	retryInterval      = 2 * time.Second
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
			zap.String("session", h.sessionName),
			zap.Error(err),
		)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`ok`))
	logger.Debug("Health check succeeded",
		zap.String("session", h.sessionName),
	)
}

func (h *healthProbe) getSessionName() {
	logger.Info("Calling SCGI server to discover session name")

	var result any
	ctx, cancel := context.WithTimeout(context.Background(), *probeTimeout)
	if err := client.Call(ctx, "session.name", nil, &result); err != nil {
		logger.Info("Failed to acquire SCGI session",
			zap.Error(err),
			zap.String("session", h.sessionName),
		)
	}
	h.sessionName = result.(string)

	logger.Info("SCGI session acquired",
		zap.String("session", h.sessionName),
	)
}

func main() {
	flag.Parse()

	loggerCfg := zap.NewProductionConfig()
	if *debug {
		loggerCfg = zap.NewDevelopmentConfig()
	}
	loggerCfg.EncoderConfig.TimeKey = "timestamp"
	loggerCfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	logger = zap.Must(loggerCfg.Build())

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

	// Loop until SCGI socket has been created
	for {
		if sockInfo, err := os.Stat(*scgiAddress); err != nil {
			if os.IsNotExist(err) {
				logger.Info("Waiting for SCGI address...",
					zap.Error(err),
					zap.String("socket", *scgiAddress),
				)
				time.Sleep(retryInterval)
				continue
			}
			break
		}
		if sockInfo.Mode().Type() != os.ModeSocket {
			logger.Error("SCGI address is not valid; exiting...",
				zap.Error(err),
				zap.String("socket", *scgiAddress),
			)
			os.Exit(1)
		}
	}

	client := newClientCodec()

	hp := &healthProbe{
		sessionName: "",
		client:      client,
	}
	hp.getSessionName()

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
