// TODO(uwe): Simplify and switch to gin+graceful

package seaeye

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Server is a http.Server that can gracefully shut down.
type Server struct {
	http.Server
	Listener   net.Listener
	ConnActive int
	ConnMutex  sync.Mutex
}

// Context holds a context for http.FuncHandler.
type Context struct {
	config *Config
	builds chan *Build
	stats  func() Stats
}

// CtxHandlerFunc defines a http.FuncHandler with context.
type CtxHandlerFunc func(ctx *Context, w http.ResponseWriter, req *http.Request)

type httpError struct {
	error
	Status int
}

// NewWebServer initializes a new HTTP server. The difference to a standard
// net.http server is that it knows about the listener and can stop itself
// gracefully.
func NewWebServer(conf *Config, builds chan *Build, stats func() Stats) *Server {
	ctx := &Context{
		config: conf,
		builds: builds,
		stats:  stats,
	}

	router := mux.NewRouter()
	router.Path("/").Methods("GET").HandlerFunc(wrap(ctx, indexHandler))
	router.Path("/health").Methods("GET").HandlerFunc(wrap(ctx, healthHandler))
	router.Path("/jobs/{id}/status/{rev}").Methods("GET").HandlerFunc(wrap(ctx, statusJobHandler))
	router.Path("/login").Methods("GET").HandlerFunc(wrap(ctx, loginHandler))
	router.Path("/webhook").Methods("PUT", "POST").HandlerFunc(wrap(ctx, webhookHandler))

	srv := &Server{}
	srv.Addr = conf.HostPort
	srv.ConnState = srv.connStateHook()
	srv.Handler = router

	return srv
}

// Start start the http server, listening, accepting, and serving.
func (srv *Server) Start() error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	srv.Listener = ln

	go func() {
		log.Printf("[E][web] Socket closed: %v", srv.Serve(ln))
	}()

	return nil
}

// Stop stops the http server, gracefully draining outstanding connections.
func (srv *Server) Stop() error {
	// no new connections should use keep alive
	srv.SetKeepAlivesEnabled(false)

	// Stop listening, therefore accepting
	err := srv.Listener.Close()
	if err != nil {
		return err
	}

	// Drain active connections with grace period of 10s.
	for t := 100; t > 0 && srv.ConnActive > 0; t-- {
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// ConnStateHook is the http.Server ConnState hook called whenever a connection
// changes its state.
func (srv *Server) connStateHook() func(net.Conn, http.ConnState) {
	return func(_ net.Conn, connState http.ConnState) {
		srv.ConnMutex.Lock()
		defer srv.ConnMutex.Unlock()

		switch connState {
		case http.StateNew:
			srv.ConnActive++
		case http.StateHijacked, http.StateClosed:
			srv.ConnActive--
		}
	}
}

func wrap(ctx *Context, handler CtxHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		handler(ctx, w, req)
	}
}

func indexHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	// TODO(uwe): implement
}

func healthHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	stats := ctx.stats()
	for k, v := range stats {
		fmt.Fprintf(w, "%s=%v\n", k, v)
	}
}

func loginHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	// TODO(uwe): implement
}

func webhookHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	s, err := sourceFromRequest(req)
	if err != nil {
		log.Printf("[E][web] Invalid github webhook push event: %v", err)
		sendHTTPError(w, err)
		return
	}

	j := &Job{Config: ctx.config}
	ctx.builds <- &Build{Job: j, Source: s}
}

func statusJobHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	rev := vars["rev"]

	logFilePath, err := ctx.config.LogFilePath(id, rev)
	if err != nil {
		sendHTTPError(w, err)
		return
	}

	http.ServeFile(w, req, logFilePath)
}

func sourceFromRequest(req *http.Request) (*Source, error) {
	e, err := PushEventFromRequest(req)
	if err != nil {
		err := fmt.Errorf("failed to parse: %v", err)
		return nil, &httpError{error: err, Status: http.StatusBadRequest}
	}

	if *e.After == "" || *e.Repo.FullName == "" || *e.Repo.URL == "" {
		err := fmt.Errorf("missing field(s): after=%s, repository.full_name=%s, repository.ssh_url=%s",
			*e.After, *e.Repo.FullName, *e.Repo.URL)
		return nil, &httpError{error: err, Status: http.StatusBadRequest}
	}

	parts := strings.Split(*e.Repo.FullName, "/")
	if parts == nil || len(parts) != 2 {
		err := fmt.Errorf("invalid owner/repo %s", *e.Repo.FullName)
		return nil, &httpError{error: err, Status: http.StatusBadRequest}
	}

	s := &Source{
		Owner: parts[0],
		Repo:  parts[1],
		Rev:   *e.After,
		URL:   *e.Repo.URL,
	}
	return s, nil
}

func sendHTTPError(w http.ResponseWriter, err error) {
	if httpErr, ok := err.(*httpError); ok {
		http.Error(w, httpErr.Error(), httpErr.Status)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
