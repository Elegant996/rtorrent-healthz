package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Elegant996/scgi"
	"github.com/Elegant996/xmlrpc"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"

	"k8s.io/klog"
)

// Command line flags
var (
	probeTimeout = flag.Duration("probe-timeout", time.Second, "Probe timeout in seconds.")
	scgiAddress  = flag.String("scgi-address", "/run/rtorrent/rtorrent.sock", "Address of the SCGI socket.")
	httpEndpoint = flag.String("http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including SCGI server health check and save requests. The default is empty string, which means the server is disabled.")
)

type scgiSession struct {
	sessionName string
	jsonSupport bool
}

func (s *scgiSession) checkHealthJson(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), *probeTimeout)
	defer cancel()

	ch := jhttp.NewChannel(*scgiAddress, &jhttp.ChannelOptions{Client: &http.Client{Transport: &scgi.Transport{}}})
	cli := jrpc2.NewClient(ch, nil)
	defer cli.Close()

	klog.V(5).Infof("Sending probe request to SCGI session %q", s.sessionName)
	var result string
	if err := cli.CallResult(ctx, "system.api_version", []string{""}, &result); err != nil {
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

func (s *scgiSession) saveSessionJson(w http.ResponseWriter, req *http.Request) {
	ch := jhttp.NewChannel(*scgiAddress, &jhttp.ChannelOptions{Client: &http.Client{Transport: &scgi.Transport{}}})
	cli := jrpc2.NewClient(ch, nil)
	defer cli.Close()

	klog.V(5).Infof("Sending save request to SCGI session %q", s.sessionName)
	var result int
	if err := cli.CallResult(context.Background(), "session.save", []string{""}, &result); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(err.Error()))
		klog.Errorf("session save failed: %v", err)
		return
	}

	klog.V(5).Infof("session.save = %d", result)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`ok`))
	klog.V(5).Infof("Session save succeeded")
}

func (s *scgiSession) checkHealthXml(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), *probeTimeout)
	defer cancel()

	cli, err := xmlrpc.NewClient(*scgiAddress, &scgi.Transport{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		klog.Errorf("failed to establish connection to SCGI server: %v", err)
		return
	}
	defer cli.Close()

	klog.V(5).Infof("Sending probe request to SCGI session %q", s.sessionName)
	var result string
	if err := cli.Call(ctx, "system.api_version", nil, &result); err != nil {
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

func (s *scgiSession) saveSessionXml(w http.ResponseWriter, req *http.Request) {
	cli, err := xmlrpc.NewClient(*scgiAddress, &scgi.Transport{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		klog.Errorf("failed to establish connection to SCGI server: %v", err)
		return
	}
	defer cli.Close()

	klog.V(5).Infof("Sending save request to SCGI session %q", s.sessionName)
	var result int
	if err := cli.Call(context.Background(), "session.save", nil, &result); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(err.Error()))
		klog.Errorf("session save failed: %v", err)
		return
	}

	klog.V(5).Infof("session.save = %d", result)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`ok`))
	klog.V(5).Infof("Session save succeeded")
}

func main() {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	ch := jhttp.NewChannel(*scgiAddress, &jhttp.ChannelOptions{Client: &http.Client{Transport: &scgi.Transport{}}})
	sjcli := jrpc2.NewClient(ch, nil)

	klog.Infof("calling SCGI server to discover session name")
	ss := &scgiSession{jsonSupport: true}
	if err := sjcli.CallResult(context.Background(), "session.name", []string{""}, &ss.sessionName); err != nil {
		klog.Warningf("JRPC2 not supported, falling back to XMLRPC: %v", err)
		ss.jsonSupport = false
		
		sxcli, err := xmlrpc.NewClient(*scgiAddress, &scgi.Transport{})
		if err != nil {
			klog.Errorf("failed to establish connection to SCGI server: %v", err)
		}
		
		if err := sxcli.Call(context.Background(), "session.name", nil, &ss.sessionName); err != nil {
			klog.Fatalf("failed to get SCGI session name: %v", err)
		}
		sxcli.Close()
	}
	sjcli.Close()
	
	klog.Infof("SCGI session name: %q", ss.sessionName)

	mux := http.NewServeMux()
	addr := *httpEndpoint
	if ss.jsonSupport {
		mux.HandleFunc("/healthz", ss.checkHealthJson)
		mux.HandleFunc("/save", ss.saveSessionJson)
	} else {
		mux.HandleFunc("/healthz", ss.checkHealthXml)
		mux.HandleFunc("/save", ss.saveSessionXml)
	}
	srv := &http.Server {
		Addr: addr,
		Handler: mux,
	}

    done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGTERM)

	go func() {
		<-quit
		klog.Infof("Stopping requests on: %q", addr)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		srv.SetKeepAlivesEnabled(false)
		if err := srv.Shutdown(ctx); err != nil {
			klog.Fatalf("failed to shutdown http server with error: %v", err)
		}
		close(done)
	}()
	
	klog.Infof("ServeMux listening at %q", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("failed to start http server with error: %v", err)
	}
	<-done
}