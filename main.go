package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Command line flags
var (
	operationTimeout    = flag.Duration("timeout", time.Second, "Timeout for waiting for communication with rtorrent.")
	scgiAddress         = flag.String("scgi-address", "/run/scgi/socket", "Path of the SCGI server socket that the rtorrent-healthz will connect to.")
	httpEndpoint        = flag.String("http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including SCGI server health check and save requests. The default is empty string, which means the server is disabled.")
	encoding            = flag.String("encoding", "xml", "The encoding mechanism used for remote procedure calling.")
	debug 				= flag.Bool("debug", false, "Sets the log level to debug")
)

func main() {
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
    if *debug {
        zerolog.SetGlobalLevel(zerolog.DebugLevel)
    }

	log.Info().
		Str("encoding", *encoding).
		Msgf("Running rtorrent-healthz with encoding=%s", *encoding)

	r := newRPCCodec()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 30)
	defer cancel()

	log.Info().
		Msg("Calling SCGI server to discover session name")

	var scgiSessionName string
	if err := r.Call(ctx, "session.name", nil, &scgiSessionName); err != nil {
		log.Error().
			Err(err).
			Str("session", scgiSessionName).
			Msg("Cannot rerieve SCGI session name")
		os.Exit(1)
	}
	
	log.Info().
		Str("session", scgiSessionName).
		Msg("SCGI session name captured")

	done := make(chan bool, 1)
	go httpServer(r)
	go removeRegSocket()
	<-done

	// If RPC server is gracefully shutdown, cleanup and exit
	os.Exit(0)
}

func httpServer(r *rpcCodec) {
	if *httpEndpoint == "" {
		log.Info().
			Str("endpoint", *httpEndpoint).
			Msg("Skipping HTTP server")
		return
	}
	log.Info().
		Str("endpoint", *httpEndpoint).
		Msg("Starting HTTP server")

	ctx, cancel := context.WithTimeout(context.Background(), *operationTimeout)
	defer cancel()

	// Prepare http endpoint for healthz
	var result any
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		if err := r.Call(ctx, "system.pid", nil, &result); err != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`ok`))
			log.Debug().
				Msg("Health check succeeded")
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			log.Debug().
				Err(err).
				Msg("Health check failed")
		}
	})
	mux.HandleFunc("/save", func(w http.ResponseWriter, req *http.Request) {
		if err := r.Call(context.Background(), "session.save", nil, &result); err != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`ok`))
			log.Debug().
				Msg("Session save succeeded")
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			log.Debug().
				Err(err).
				Msg("Session save failed")
		}
	})

	log.Fatal().
		Err(http.ListenAndServe(*httpEndpoint, mux)).
		Msg("HTTP server closed")
}

func removeRegSocket() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM)
	<-sigc
	err := os.Remove(*scgiAddress)
	if err != nil && !os.IsNotExist(err) {
		log.Error().
			Err(err).
			Str("socket", *scgiAddress).
			Msg("Failed to remove socket")
		os.Exit(1)
	}
	os.Exit(0)
}